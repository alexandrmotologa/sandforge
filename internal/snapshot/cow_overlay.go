package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// CowOverlay implements Snapshotter using Linux OverlayFS.
type CowOverlay struct {
	workspaceDir string
	overlayBase  string
	upperDir     string
	workDir      string
	mergedDir    string
	timeline     *Timeline
	mu           sync.Mutex
}

// NewCowOverlay initializes an OverlayFS snapshot manager.
func NewCowOverlay(workspaceDir string) (*CowOverlay, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("overlayfs snapshotter is supported only on linux")
	}

	overlayBase := filepath.Join(os.TempDir(), "sandforge-overlay")
	upperDir := filepath.Join(overlayBase, "upper")
	workDir := filepath.Join(overlayBase, "work")
	mergedDir := filepath.Join(overlayBase, "merged")

	for _, dir := range []string{upperDir, workDir, mergedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return &CowOverlay{
		workspaceDir: workspaceDir,
		overlayBase:  overlayBase,
		upperDir:     upperDir,
		workDir:      workDir,
		mergedDir:    mergedDir,
		timeline:     NewTimeline(),
	}, nil
}

// Create captures the current upperdir layer as a checkpoint.
func (co *CowOverlay) Create(ctx context.Context, triggerCommand string, description string) (*types.Snapshot, error) {
	co.mu.Lock()
	defer co.mu.Unlock()

	id := fmt.Sprintf("cow-%d", time.Now().UnixNano())
	snapDir := filepath.Join(co.overlayBase, "snapshots", id)
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return nil, err
	}

	// Copy upperdir mutations
	copyCmd := exec.CommandContext(ctx, "cp", "-a", co.upperDir+"/.", snapDir)
	if err := copyCmd.Run(); err != nil {
		return nil, fmt.Errorf("snapshotting upperdir: %w", err)
	}

	snap := &types.Snapshot{
		ID:             id,
		Timestamp:      time.Now(),
		TriggerCommand: triggerCommand,
		Description:    description,
		Status:         "clean",
	}

	co.timeline.Add(snap)
	return snap, nil
}

// Rollback restores the upperdir to a target checkpoint.
func (co *CowOverlay) Rollback(ctx context.Context, snapshotID string) error {
	co.mu.Lock()
	defer co.mu.Unlock()

	snapDir := filepath.Join(co.overlayBase, "snapshots", snapshotID)
	if _, err := os.Stat(snapDir); os.IsNotExist(err) {
		return fmt.Errorf("snapshot %s does not exist", snapshotID)
	}

	// Wipe current upperdir
	_ = os.RemoveAll(co.upperDir)
	_ = os.MkdirAll(co.upperDir, 0755)

	// Copy back snapshot contents
	copyCmd := exec.CommandContext(ctx, "cp", "-a", snapDir+"/.", co.upperDir)
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("restoring upperdir: %w", err)
	}

	co.timeline.SetCurrentID(snapshotID, "reverted")
	return nil
}

// Diff returns empty diff for overlay layer.
func (co *CowOverlay) Diff(ctx context.Context, fromID, toID string) (*types.SnapshotDiff, error) {
	return &types.SnapshotDiff{
		FromID:  fromID,
		ToID:    toID,
		Entries: []types.DiffEntry{},
		Raw:     "",
	}, nil
}

// List returns snapshots.
func (co *CowOverlay) List(ctx context.Context) ([]*types.Snapshot, error) {
	return co.timeline.List(), nil
}

// Get retrieves a snapshot.
func (co *CowOverlay) Get(ctx context.Context, snapshotID string) (*types.Snapshot, error) {
	snap, ok := co.timeline.Get(snapshotID)
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}
	return snap, nil
}
