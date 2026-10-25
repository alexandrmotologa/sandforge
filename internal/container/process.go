package container

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// ProcessRuntime runs commands inside an isolated host process with sanitized environment.
type ProcessRuntime struct {
	workspaceDir string
}

// NewProcessRuntime creates a host-isolated process runner.
func NewProcessRuntime(workspaceDir string) *ProcessRuntime {
	return &ProcessRuntime{
		workspaceDir: workspaceDir,
	}
}

// Mode returns "process".
func (p *ProcessRuntime) Mode() string {
	return "process"
}

// IsAvailable is always true for process execution.
func (p *ProcessRuntime) IsAvailable(ctx context.Context) bool {
	return true
}

// Close releases any process-level state.
func (p *ProcessRuntime) Close() error {
	return nil
}

// Execute executes a command with sanitized environment and timeout controls.
func (p *ProcessRuntime) Execute(ctx context.Context, cmdStr string, opts ExecOptions) (*types.ExecutionResult, error) {
	start := time.Now()

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd.exe", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(execCtx, "sh", "-c", cmdStr)
	}

	workDir := opts.WorkingDir
	if workDir == "" {
		workDir = p.workspaceDir
	}
	cmd.Dir = workDir

	// Environment sanitization: filter out sensitive credentials by default
	sanitizedEnv := sanitizeEnvironment(os.Environ())
	if len(opts.Env) > 0 {
		sanitizedEnv = append(sanitizedEnv, opts.Env...)
	}
	cmd.Env = sanitizedEnv

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	var errStr string
	if runErr != nil {
		if exitError, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
			errStr = runErr.Error()
		}
	}

	return &types.ExecutionResult{
		Command:    cmdStr,
		ExitCode:   exitCode,
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		DurationMs: duration.Milliseconds(),
		Error:      errStr,
		Timestamp:  time.Now(),
	}, nil
}

// sanitizeEnvironment removes sensitive token and key variables to avoid credential leakage.
func sanitizeEnvironment(env []string) []string {
	sensitivePrefixes := []string{
		"AWS_",
		"AZURE_",
		"GCP_",
		"OPENAI_",
		"ANTHROPIC_",
		"GITHUB_TOKEN",
		"GH_TOKEN",
		"SLACK_",
		"STRIPE_",
		"DATABASE_URL",
		"SECRET",
		"PRIVATE_KEY",
	}

	var safe []string
	for _, kv := range env {
		upper := strings.ToUpper(kv)
		isSensitive := false
		for _, prefix := range sensitivePrefixes {
			if strings.HasPrefix(upper, prefix) {
				isSensitive = true
				break
			}
		}
		if !isSensitive {
			safe = append(safe, kv)
		}
	}
	return safe
}
