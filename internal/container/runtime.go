package container

import (
	"context"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// ExecOptions specifies execution parameters for a sandboxed command.
type ExecOptions struct {
	Timeout    time.Duration
	WorkingDir string
	Env        []string
	Limits     ResourceLimits
}

// Runtime is the abstraction for executing commands in an isolated environment.
type Runtime interface {
	// Execute runs a command and returns the execution result.
	Execute(ctx context.Context, cmdStr string, opts ExecOptions) (*types.ExecutionResult, error)

	// Close releases container or runtime resources.
	Close() error

	// Mode returns the active isolation mode (e.g. "docker", "process").
	Mode() string

	// IsAvailable returns whether the runtime engine is ready.
	IsAvailable(ctx context.Context) bool
}
