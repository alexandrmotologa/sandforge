package snapshot

import (
	"context"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// Snapshotter defines the interface for capturing and restoring filesystem states.
type Snapshotter interface {
	// Create captures a snapshot before or after a command.
	Create(ctx context.Context, triggerCommand string, description string) (*types.Snapshot, error)

	// Rollback restores the filesystem to the target snapshot ID.
	Rollback(ctx context.Context, snapshotID string) error

	// Diff calculates the changes between two snapshots, or between a snapshot and current workspace.
	Diff(ctx context.Context, fromID, toID string) (*types.SnapshotDiff, error)

	// List returns all recorded snapshots in chronological order.
	List(ctx context.Context) ([]*types.Snapshot, error)

	// Get retrieves a snapshot by its ID.
	Get(ctx context.Context, snapshotID string) (*types.Snapshot, error)
}
