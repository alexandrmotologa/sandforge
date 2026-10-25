package container

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerRuntime manages containerized command execution via the Docker Engine SDK.
type DockerRuntime struct {
	cli          *client.Client
	workspaceDir string
	imageName    string
}

// NewDockerRuntime attempts to connect to the Docker daemon.
func NewDockerRuntime(workspaceDir string, imageName string) (*DockerRuntime, error) {
	if imageName == "" {
		imageName = "alpine:latest"
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerRuntime{
		cli:          cli,
		workspaceDir: workspaceDir,
		imageName:    imageName,
	}, nil
}

// Mode returns "docker".
func (d *DockerRuntime) Mode() string {
	return "docker"
}

// IsAvailable checks if the Docker daemon responds to ping.
func (d *DockerRuntime) IsAvailable(ctx context.Context) bool {
	if d.cli == nil {
		return false
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := d.cli.Ping(pingCtx)
	return err == nil
}

// Close closes the Docker client connection.
func (d *DockerRuntime) Close() error {
	if d.cli != nil {
		return d.cli.Close()
	}
	return nil
}

// Execute runs a command inside an isolated container with volume-mounted workspace.
func (d *DockerRuntime) Execute(ctx context.Context, cmdStr string, opts ExecOptions) (*types.ExecutionResult, error) {
	start := time.Now()

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Ensure base image is pulled if not present locally
	_, _, inspectErr := d.cli.ImageInspectWithRaw(execCtx, d.imageName)
	if inspectErr != nil {
		reader, pullErr := d.cli.ImagePull(execCtx, d.imageName, image.PullOptions{})
		if pullErr == nil {
			_, _ = io.Copy(io.Discard, reader)
			_ = reader.Close()
		}
	}

	limits := opts.Limits
	if limits.MemoryBytes == 0 {
		limits = DefaultLimits()
	}

	// Create ephemeral container
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/workspace:rw", d.workspaceDir),
		},
		Resources: container.Resources{
			Memory:   limits.MemoryBytes,
			NanoCPUs: limits.NanoCPUs,
		},
	}

	resp, err := d.cli.ContainerCreate(
		execCtx,
		&container.Config{
			Image:      d.imageName,
			Cmd:        []string{"sh", "-c", cmdStr},
			WorkingDir: "/workspace",
			Env:        opts.Env,
			Tty:        false,
		},
		hostConfig,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("container create failed: %w", err)
	}

	containerID := resp.ID
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopCancel()
		_ = d.cli.ContainerRemove(stopCtx, containerID, container.RemoveOptions{Force: true})
	}()

	if err := d.cli.ContainerStart(execCtx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("container start failed: %w", err)
	}

	statusCh, errCh := d.cli.ContainerWait(execCtx, containerID, container.WaitConditionNotRunning)

	out, err := d.cli.ContainerLogs(execCtx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("retrieving container logs: %w", err)
	}
	defer out.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, out)

	var exitCode int
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("waiting for container: %w", err)
		}
	case waitStatus := <-statusCh:
		exitCode = int(waitStatus.StatusCode)
	}

	duration := time.Since(start)

	return &types.ExecutionResult{
		Command:    cmdStr,
		ExitCode:   exitCode,
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		DurationMs: duration.Milliseconds(),
		Timestamp:  time.Now(),
	}, nil
}
