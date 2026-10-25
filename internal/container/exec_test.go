package container

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/policy"
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
	executor := NewExecutor(pool, gc, policy.NewEngine())

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

func TestRunWithVerifyAutoRollback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sandforge-verify-test-*")
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
	executor := NewExecutor(pool, gc, policy.NewEngine())

	ctx := context.Background()

	// Initial clean file
	keepFile := filepath.Join(tempDir, "stable.txt")
	_ = os.WriteFile(keepFile, []byte("stable code\n"), 0644)
	_, _ = gc.Create(ctx, "initial", "base")

	// 1. Run a command that breaks verification test (verify command exits with 1)
	mutationFile := filepath.Join(tempDir, "broken_feature.txt")
	cmd := "echo buggy code > broken_feature.txt"
	failingVerifyCmd := "cmd /C exit 1"

	res, err := executor.RunWithVerify(ctx, cmd, failingVerifyCmd, ExecOptions{
		Timeout:    10 * time.Second,
		WorkingDir: tempDir,
	})
	if err != nil {
		t.Fatalf("RunWithVerify returned error: %v", err)
	}

	if !res.Verified {
		t.Fatalf("expected verified to be true")
	}
	if res.VerifyPassed {
		t.Fatalf("expected verification to fail")
	}
	if !res.AutoRolledBack {
		t.Fatalf("expected AutoRolledBack to be true when verification fails")
	}

	// Verify that the broken file was auto-rolled back and does not exist!
	if _, err := os.Stat(mutationFile); !os.IsNotExist(err) {
		t.Fatalf("expected broken_feature.txt to be deleted by auto-rollback")
	}

	// 2. Run a command that passes verification (verify command exits with 0)
	goodFile := filepath.Join(tempDir, "good_feature.txt")
	goodCmd := "echo passing code > good_feature.txt"
	passingVerifyCmd := "cmd /C exit 0"

	goodRes, err := executor.RunWithVerify(ctx, goodCmd, passingVerifyCmd, ExecOptions{
		Timeout:    10 * time.Second,
		WorkingDir: tempDir,
	})
	if err != nil {
		t.Fatalf("good RunWithVerify returned error: %v", err)
	}

	if !goodRes.VerifyPassed {
		t.Fatalf("expected good verification to pass")
	}
	if goodRes.AutoRolledBack {
		t.Fatalf("expected AutoRolledBack to be false when verification passes")
	}

	// Verify good file exists!
	if _, err := os.Stat(goodFile); os.IsNotExist(err) {
		t.Fatalf("expected good_feature.txt to remain in workspace")
	}
}

func TestPolicyInterceptionInExecutor(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sandforge-policy-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gc, _ := snapshot.NewGitCheckpoint(tempDir)
	processRt := NewProcessRuntime(tempDir)
	pool := NewPool(nil, processRt, 2)
	executor := NewExecutor(pool, gc, policy.NewEngine())

	// Attempt destructive root delete
	res, err := executor.Run(context.Background(), "rm -rf /", ExecOptions{})
	if err != nil {
		t.Fatalf("expected no go error, got: %v", err)
	}
	if res.ExitCode != 126 {
		t.Fatalf("expected exit code 126 for policy block, got: %d", res.ExitCode)
	}
	if res.PolicyViolation == "" {
		t.Fatalf("expected PolicyViolation to be populated")
	}
}
