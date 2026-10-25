package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitCheckpointLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sandforge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gc, err := NewGitCheckpoint(tempDir)
	if err != nil {
		t.Fatalf("NewGitCheckpoint failed: %v", err)
	}

	ctx := context.Background()

	// Initial file
	testFile := filepath.Join(tempDir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("version 1\n"), 0644); err != nil {
		t.Fatalf("write file error: %v", err)
	}

	// Snapshot 1
	snap1, err := gc.Create(ctx, "initial state", "checkpoint 1")
	if err != nil {
		t.Fatalf("Create snap1 failed: %v", err)
	}
	if snap1.ID == "" {
		t.Fatalf("expected snap1 to have an ID")
	}

	// Modify file and add new file
	if err := os.WriteFile(testFile, []byte("version 2 modified\n"), 0644); err != nil {
		t.Fatalf("modify file error: %v", err)
	}
	extraFile := filepath.Join(tempDir, "unwanted.txt")
	if err := os.WriteFile(extraFile, []byte("agent created this\n"), 0644); err != nil {
		t.Fatalf("write extra file error: %v", err)
	}

	// Snapshot 2
	snap2, err := gc.Create(ctx, "agent command", "checkpoint 2")
	if err != nil {
		t.Fatalf("Create snap2 failed: %v", err)
	}

	// Diff between snap1 and snap2
	diff, err := gc.Diff(ctx, snap1.ID, snap2.ID)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if len(diff.Entries) == 0 {
		t.Fatalf("expected diff entries between snap1 and snap2")
	}

	// Test instant rollback to snap1
	if err := gc.Rollback(ctx, snap1.ID); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Verify unwanted file was deleted and hello.txt was reverted
	if _, err := os.Stat(extraFile); !os.IsNotExist(err) {
		t.Fatalf("expected unwanted.txt to be removed after rollback")
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("read reverted file error: %v", err)
	}
	if strings.TrimSpace(string(content)) != "version 1" {
		t.Fatalf("expected 'version 1', got: %q", string(content))
	}
}
