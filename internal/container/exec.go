package container

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/policy"
	"github.com/alexandrmotologa/sandforge/internal/snapshot"
	"github.com/alexandrmotologa/sandforge/internal/types"
)

// Executor coordinates pre-execution snapshotting, sandboxed execution, and diff tracking.
type Executor struct {
	pool        *Pool
	snapshotter snapshot.Snapshotter
	policy      *policy.Engine
}

// NewExecutor creates a new coordinated command executor.
func NewExecutor(pool *Pool, snapshotter snapshot.Snapshotter, policyEngine *policy.Engine) *Executor {
	return &Executor{
		pool:        pool,
		snapshotter: snapshotter,
		policy:      policyEngine,
	}
}

// Run executes a command with an automatic pre-execution snapshot and change recording.
func (e *Executor) Run(ctx context.Context, cmdStr string, opts ExecOptions) (*types.ExecutionResult, error) {
	// Policy check pre-execution
	if e.policy != nil {
		if allowed, violation := e.policy.Check(cmdStr); !allowed {
			return &types.ExecutionResult{
				Command:         cmdStr,
				ExitCode:        126,
				Error:           "Execution blocked by SandForge Policy Engine: " + violation,
				PolicyViolation: violation,
				Timestamp:       time.Now(),
			}, nil
		}
	}

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
			result.DiffSummary = formatDiffSummary(diff)
		} else {
			result.DiffSummary = "No filesystem changes"
		}
	}

	result.Timestamp = time.Now()
	return result, nil
}

// RunWithVerify executes a command, then automatically runs a verification test command.
// If verification fails, it automatically rolls back the workspace to the pre-execution checkpoint.
func (e *Executor) RunWithVerify(ctx context.Context, cmdStr string, verifyCmd string, opts ExecOptions) (*types.ExecutionResult, error) {
	// Policy check on primary command
	if e.policy != nil {
		if allowed, violation := e.policy.Check(cmdStr); !allowed {
			return &types.ExecutionResult{
				Command:         cmdStr,
				ExitCode:        126,
				Error:           "Execution blocked by SandForge Policy Engine: " + violation,
				PolicyViolation: violation,
				Timestamp:       time.Now(),
			}, nil
		}
		if verifyCmd != "" {
			if allowed, violation := e.policy.Check(verifyCmd); !allowed {
				return &types.ExecutionResult{
					Command:         cmdStr,
					ExitCode:        126,
					Error:           "Verification command blocked by SandForge Policy Engine: " + violation,
					PolicyViolation: violation,
					Timestamp:       time.Now(),
				}, nil
			}
		}
	}

	var preSnap *types.Snapshot
	var err error

	if e.snapshotter != nil {
		preSnap, err = e.snapshotter.Create(ctx, cmdStr, "Pre-command checkpoint (Verified Run)")
		if err != nil {
			return nil, fmt.Errorf("creating pre-execution snapshot: %w", err)
		}
	}

	result, execErr := e.pool.Execute(ctx, cmdStr, opts)
	if execErr != nil {
		return nil, fmt.Errorf("sandboxed execution failed: %w", execErr)
	}

	if preSnap != nil {
		result.SnapshotID = preSnap.ID
	}

	// If primary command failed or no verification command was specified, finish here
	if result.ExitCode != 0 || verifyCmd == "" {
		if preSnap != nil {
			diff, _ := e.snapshotter.Diff(ctx, preSnap.ID, "working")
			result.DiffSummary = formatDiffSummary(diff)
		}
		result.Timestamp = time.Now()
		return result, nil
	}

	// Execute verification test command
	verifyRes, verifyErr := e.pool.Execute(ctx, verifyCmd, opts)
	result.VerifyCommand = verifyCmd
	result.Verified = true

	if verifyErr != nil {
		result.VerifyPassed = false
		result.VerifyStderr = verifyErr.Error()
	} else {
		result.VerifyPassed = (verifyRes.ExitCode == 0)
		result.VerifyStdout = verifyRes.Stdout
		result.VerifyStderr = verifyRes.Stderr
	}

	// If verification failed, auto-rollback!
	if !result.VerifyPassed && preSnap != nil {
		_ = e.snapshotter.Rollback(ctx, preSnap.ID)
		result.AutoRolledBack = true
		exitCode := 1
		if verifyRes != nil {
			exitCode = verifyRes.ExitCode
		}
		result.DiffSummary = fmt.Sprintf("Auto-reverted to %s: verification tests failed (exit code %d)", preSnap.ID, exitCode)
	} else if preSnap != nil {
		diff, _ := e.snapshotter.Diff(ctx, preSnap.ID, "working")
		result.DiffSummary = formatDiffSummary(diff)
	}

	result.Timestamp = time.Now()
	return result, nil
}

func formatDiffSummary(diff *types.SnapshotDiff) string {
	if diff == nil || len(diff.Entries) == 0 {
		return "No filesystem changes"
	}
	var summaries []string
	for _, entry := range diff.Entries {
		summaries = append(summaries, fmt.Sprintf("%s (%s: +%d/-%d)", entry.Path, entry.Status, entry.Additions, entry.Deletions))
	}
	return strings.Join(summaries, ", ")
}

// Pool returns the underlying runner pool.
func (e *Executor) Pool() *Pool {
	return e.pool
}

// Snapshotter returns the snapshot engine.
func (e *Executor) Snapshotter() snapshot.Snapshotter {
	return e.snapshotter
}

// Policy returns the safety policy engine.
func (e *Executor) Policy() *policy.Engine {
	return e.policy
}
