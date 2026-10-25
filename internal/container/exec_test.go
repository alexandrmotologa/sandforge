package container

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/snapshot"
)

func TestExecutorLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sandforge-exec-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gc, err := snapshot.NewGitCheckpoint(tempDir)
	if err != nil {
		t.Fatalf("git checkpoint error: %v", err)
	}

	processRt := NewProcessRuntime(tempDir)
	pool := NewPool(nil, processRt, 2)
	executor := NewExecutor(pool, gc)

	ctx := context.Background()

	// Run command that creates a file
	cmd := "echo hello from sandbox > test.txt"
	res, err := executor.Run(ctx, cmd, ExecOptions{
		Timeout:    10 * time.Second,
		WorkingDir: tempDir,
	})
	if err != nil {
		t.Fatalf("executor.Run failed: %v", err)
	}

	if res.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	if res.SnapshotID == "" {
		t.Fatalf("expected snapshot ID to be populated")
	}

	// Verify file was created
	testFile := filepath.Join(tempDir, "test.txt")
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test.txt: %v", err)
	}
	if !strings.Contains(string(content), "hello from sandbox") {
		t.Fatalf("unexpected content: %s", string(content))
	}

	// Test rollback through snapshotter
	if err := gc.Rollback(ctx, res.SnapshotID); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	// Verify file is gone after rollback
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatalf("expected test.txt to be removed after rollback")
	}
}
