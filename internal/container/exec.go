package container

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/snapshot"
	"github.com/alexandrmotologa/sandforge/internal/types"
)

// Executor coordinates pre-execution snapshotting, sandboxed execution, and diff tracking.
type Executor struct {
	pool        *Pool
	snapshotter snapshot.Snapshotter
}

// NewExecutor creates a new coordinated command executor.
func NewExecutor(pool *Pool, snapshotter snapshot.Snapshotter) *Executor {
	return &Executor{
		pool:        pool,
		snapshotter: snapshotter,
	}
}

// Run executes a command with an automatic pre-execution snapshot and change recording.
func (e *Executor) Run(ctx context.Context, cmdStr string, opts ExecOptions) (*types.ExecutionResult, error) {
	var preSnap *types.Snapshot
	var err error

	// Step 1: Create pre-execution snapshot if snapshotter is configured
	if e.snapshotter != nil {
		preSnap, err = e.snapshotter.Create(ctx, cmdStr, "Pre-command checkpoint")
		if err != nil {
			return nil, fmt.Errorf("creating pre-execution snapshot: %w", err)
		}
	}

	// Step 2: Execute command in sandbox
	result, execErr := e.pool.Execute(ctx, cmdStr, opts)
	if execErr != nil {
		return nil, fmt.Errorf("sandboxed execution failed: %w", execErr)
	}

	if preSnap != nil {
		result.SnapshotID = preSnap.ID

		// Step 3: Compute diff against current working state to see what changed
		diff, diffErr := e.snapshotter.Diff(ctx, preSnap.ID, "working")
		if diffErr == nil && diff != nil && len(diff.Entries) > 0 {
			var summaries []string
			for _, entry := range diff.Entries {
				summaries = append(summaries, fmt.Sprintf("%s (%s: +%d/-%d)", entry.Path, entry.Status, entry.Additions, entry.Deletions))
			}
			result.DiffSummary = strings.Join(summaries, ", ")
		} else {
			result.DiffSummary = "No filesystem changes"
		}
	}

	result.Timestamp = time.Now()
	return result, nil
}

// Pool returns the underlying runner pool.
func (e *Executor) Pool() *Pool {
	return e.pool
}

// Snapshotter returns the snapshot engine.
func (e *Executor) Snapshotter() snapshot.Snapshotter {
	return e.snapshotter
}
