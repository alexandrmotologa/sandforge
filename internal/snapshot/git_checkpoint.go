package snapshot

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// GitCheckpoint implements Snapshotter using Git tree and ref operations.
type GitCheckpoint struct {
	workspaceDir string
	timeline     *Timeline
	mu           sync.Mutex
}

// NewGitCheckpoint initializes the git-backed snapshotter in the target workspace.
func NewGitCheckpoint(workspaceDir string) (*GitCheckpoint, error) {
	absPath, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("resolving workspace path: %w", err)
	}

	gc := &GitCheckpoint{
		workspaceDir: absPath,
		timeline:     NewTimeline(),
	}

	if err := gc.ensureGitRepo(); err != nil {
		return nil, fmt.Errorf("initializing git repository: %w", err)
	}

	return gc, nil
}

func (gc *GitCheckpoint) ensureGitRepo() error {
	gitDir := filepath.Join(gc.workspaceDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init", "-q")
		cmd.Dir = gc.workspaceDir
		if err := cmd.Run(); err != nil {
			return err
		}

		// Configure local git identity if not set to prevent commit failures
		_, _ = gc.runGit("config", "user.name", "SandForge")
		_, _ = gc.runGit("config", "user.email", "sandforge@local")
	}

	return nil
}

func (gc *GitCheckpoint) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = gc.workspaceDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (gc *GitCheckpoint) generateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("snap-%s-%s", time.Now().Format("150405"), hex.EncodeToString(b))
}

// Create captures an instant snapshot via a private Git index.
func (gc *GitCheckpoint) Create(ctx context.Context, triggerCommand string, description string) (*types.Snapshot, error) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	id := gc.generateID()
	tempIndex := filepath.Join(gc.workspaceDir, ".git", fmt.Sprintf("sandforge_idx_%s", id))
	defer func() {
		_ = os.Remove(tempIndex)
	}()

	// Stage current workspace into temporary index
	addCmd := exec.CommandContext(ctx, "git", "add", "-A")
	addCmd.Dir = gc.workspaceDir
	addCmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tempIndex)
	if out, err := addCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git add failed: %w: %s", err, string(out))
	}

	// Write tree object from temporary index
	wtCmd := exec.CommandContext(ctx, "git", "write-tree")
	wtCmd.Dir = gc.workspaceDir
	wtCmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tempIndex)
	treeOut, err := wtCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git write-tree failed: %w: %s", err, string(treeOut))
	}
	treeSha := strings.TrimSpace(string(treeOut))

	// Determine parent commit if available
	var parentArgs []string
	if headSha, err := gc.runGit("rev-parse", "HEAD"); err == nil && headSha != "" {
		parentArgs = []string{"-p", headSha}
	}

	// Create commit object directly
	commitMsg := fmt.Sprintf("sandforge snapshot: %s", triggerCommand)
	if description != "" {
		commitMsg = fmt.Sprintf("sandforge snapshot: %s (%s)", triggerCommand, description)
	}

	args := append([]string{"commit-tree", treeSha, "-m", commitMsg}, parentArgs...)
	commitCmd := exec.CommandContext(ctx, "git", args...)
	commitCmd.Dir = gc.workspaceDir
	commitOut, err := commitCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git commit-tree failed: %w: %s", err, string(commitOut))
	}
	commitSha := strings.TrimSpace(string(commitOut))

	// Record reference
	refName := fmt.Sprintf("refs/sandforge/%s", id)
	if _, err := gc.runGit("update-ref", refName, commitSha); err != nil {
		return nil, fmt.Errorf("recording ref %s: %w", refName, err)
	}

	// Count files in tree
	fileCount := 0
	if treeList, err := gc.runGit("ls-tree", "-r", "--name-only", treeSha); err == nil && treeList != "" {
		lines := strings.Split(treeList, "\n")
		fileCount = len(lines)
	}

	snap := &types.Snapshot{
		ID:             id,
		Timestamp:      time.Now(),
		TriggerCommand: triggerCommand,
		Description:    description,
		FilesCount:     fileCount,
		Status:         "clean",
		CommitRef:      commitSha,
	}

	gc.timeline.Add(snap)
	return snap, nil
}

// Rollback resets the workspace back to the target snapshot state in milliseconds.
func (gc *GitCheckpoint) Rollback(ctx context.Context, snapshotID string) error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	snap, ok := gc.timeline.Get(snapshotID)
	var commitSha string
	if ok && snap.CommitRef != "" {
		commitSha = snap.CommitRef
	} else {
		refSha, err := gc.runGit("rev-parse", fmt.Sprintf("refs/sandforge/%s", snapshotID))
		if err != nil {
			return fmt.Errorf("snapshot %s not found: %w", snapshotID, err)
		}
		commitSha = refSha
	}

	// Restore all tracked workspace files from the commit tree
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", commitSha, "--", ".")
	checkoutCmd.Dir = gc.workspaceDir
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		outStr := string(out)
		// If the commit tree was empty or an executable/locked file could not be unlinked on Windows, proceed
		if !strings.Contains(outStr, "did not match any file(s) known to git") &&
			!strings.Contains(outStr, "unable to unlink") {
			return fmt.Errorf("git checkout restore failed: %w: %s", err, outStr)
		}
	}

	// Clean any newly created untracked files and directories
	cleanCmd := exec.CommandContext(ctx, "git", "clean", "-fd")
	cleanCmd.Dir = gc.workspaceDir
	if out, err := cleanCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clean failed: %w: %s", err, string(out))
	}

	gc.timeline.SetCurrentID(snapshotID, "reverted")
	return nil
}

// Diff returns the differences between two snapshots or against the current working directory.
func (gc *GitCheckpoint) Diff(ctx context.Context, fromID, toID string) (*types.SnapshotDiff, error) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	fromSnap, ok := gc.timeline.Get(fromID)
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", fromID)
	}

	var targetRef string
	var tempIndex string

	if toID == "" || toID == "working" {
		tempIndex = filepath.Join(gc.workspaceDir, ".git", fmt.Sprintf("sandforge_diff_idx_%d", time.Now().UnixNano()))
		defer func() {
			_ = os.Remove(tempIndex)
		}()

		// Stage current workspace into temporary index
		addCmd := exec.CommandContext(ctx, "git", "add", "-A")
		addCmd.Dir = gc.workspaceDir
		addCmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tempIndex)
		if out, err := addCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("staging for diff failed: %w: %s", err, string(out))
		}

		// Write tree
		wtCmd := exec.CommandContext(ctx, "git", "write-tree")
		wtCmd.Dir = gc.workspaceDir
		wtCmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tempIndex)
		treeOut, err := wtCmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("diff write-tree failed: %w: %s", err, string(treeOut))
		}
		targetRef = strings.TrimSpace(string(treeOut))
	} else {
		toSnap, ok := gc.timeline.Get(toID)
		if !ok {
			return nil, fmt.Errorf("snapshot %s not found", toID)
		}
		targetRef = toSnap.CommitRef
	}

	diffCmd := exec.CommandContext(ctx, "git", "diff-tree", "-p", "-r", "--no-commit-id", fromSnap.CommitRef, targetRef)
	diffCmd.Dir = gc.workspaceDir
	rawBytes, err := diffCmd.CombinedOutput()
	if err != nil && len(rawBytes) == 0 {
		return nil, fmt.Errorf("git diff-tree failed: %w", err)
	}

	raw := string(rawBytes)
	entries := ParseGitDiff(raw)

	return &types.SnapshotDiff{
		FromID:  fromID,
		ToID:    toID,
		Entries: entries,
		Raw:     raw,
	}, nil
}

// List returns all registered snapshots.
func (gc *GitCheckpoint) List(ctx context.Context) ([]*types.Snapshot, error) {
	return gc.timeline.List(), nil
}

// Get finds a snapshot by ID.
func (gc *GitCheckpoint) Get(ctx context.Context, snapshotID string) (*types.Snapshot, error) {
	snap, ok := gc.timeline.Get(snapshotID)
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}
	return snap, nil
}
