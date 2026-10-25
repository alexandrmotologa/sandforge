package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/container"
	"github.com/alexandrmotologa/sandforge/internal/firewall"
	"github.com/alexandrmotologa/sandforge/internal/types"
)

// GetAvailableTools returns all MCP tools registered by SandForge.
func GetAvailableTools() []Tool {
	return []Tool{
		{
			Name:        "sandforge_run",
			Description: "Execute a shell command inside the SandForge isolated sandbox. Automatically captures a pre-execution Copy-on-Write snapshot and returns stdout, stderr, exit code, and filesystem diff summary.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"command": {
						Type:        "string",
						Description: "The exact shell command to execute in the sandbox.",
					},
					"timeout_seconds": {
						Type:        "integer",
						Description: "Maximum time in seconds before command execution is terminated (default: 60).",
					},
				},
				Required: []string{"command"},
			},
		},
		{
			Name:        "sandforge_rollback",
			Description: "Instantly restore the workspace filesystem to a prior snapshot state in under 15ms. Reverts all file modifications and deletes newly created files.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"snapshot_id": {
						Type:        "string",
						Description: "The snapshot identifier to revert back to (e.g., snap-142030-a1b2c3).",
					},
				},
				Required: []string{"snapshot_id"},
			},
		},
		{
			Name:        "sandforge_snapshot",
			Description: "Create a named manual filesystem checkpoint of the current workspace.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"description": {
						Type:        "string",
						Description: "Description or comment explaining the checkpoint.",
					},
				},
			},
		},
		{
			Name:        "sandforge_diff",
			Description: "Inspect unified file diffs between two snapshots, or between a snapshot and the current working workspace.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"from_snapshot": {
						Type:        "string",
						Description: "Base snapshot ID to compare from.",
					},
					"to_snapshot": {
						Type:        "string",
						Description: "Target snapshot ID or 'working' for current directory changes (default: working).",
					},
				},
				Required: []string{"from_snapshot"},
			},
		},
		{
			Name:        "sandforge_list_snapshots",
			Description: "List the timeline of captured snapshots with command triggers and timestamps.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"limit": {
						Type:        "integer",
						Description: "Maximum number of snapshots to return (default: 20).",
					},
				},
			},
		},
		{
			Name:        "sandforge_network_alerts",
			Description: "List egress firewall events and blocked credential exfiltration attempts.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"limit": {
						Type:        "integer",
						Description: "Maximum number of alerts to return (default: 50).",
					},
				},
			},
		},
		{
			Name:        "sandforge_status",
			Description: "Report sandbox engine health, container pool capacity, active isolation mode, and firewall configuration.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
	}
}

// ToolHandler handles tool execution requests.
type ToolHandler struct {
	executor   *container.Executor
	firewall   *firewall.Controller
	proxy      *firewall.ProxyInterceptor
	startTime  time.Time
	workspace  string
}

// NewToolHandler creates a new tool handler.
func NewToolHandler(executor *container.Executor, fw *firewall.Controller, proxy *firewall.ProxyInterceptor, workspace string) *ToolHandler {
	return &ToolHandler{
		executor:  executor,
		firewall:  fw,
		proxy:     proxy,
		startTime: time.Now(),
		workspace: workspace,
	}
}

// ExecuteTool routes and executes an MCP tool call.
func (h *ToolHandler) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
	switch name {
	case "sandforge_run":
		cmd, ok := args["command"].(string)
		if !ok || cmd == "" {
			return errorResult("missing required argument 'command'"), nil
		}
		timeoutSec := 60
		if t, ok := args["timeout_seconds"].(float64); ok && t > 0 {
			timeoutSec = int(t)
		}

		// Inject proxy env vars to ensure egress firewall filtering
		var env []string
		if h.proxy != nil {
			env = append(env, h.proxy.EnvVars()...)
		}

		res, err := h.executor.Run(ctx, cmd, container.ExecOptions{
			Timeout:    time.Duration(timeoutSec) * time.Second,
			WorkingDir: h.workspace,
			Env:        env,
		})
		if err != nil {
			return errorResult(fmt.Sprintf("execution failed: %v", err)), nil
		}

		b, _ := json.MarshalIndent(res, "", "  ")
		return textResult(string(b)), nil

	case "sandforge_rollback":
		snapID, ok := args["snapshot_id"].(string)
		if !ok || snapID == "" {
			return errorResult("missing required argument 'snapshot_id'"), nil
		}

		err := h.executor.Snapshotter().Rollback(ctx, snapID)
		if err != nil {
			return errorResult(fmt.Sprintf("rollback to %s failed: %v", snapID, err)), nil
		}

		return textResult(fmt.Sprintf("Successfully restored workspace to snapshot: %s", snapID)), nil

	case "sandforge_snapshot":
		desc := ""
		if d, ok := args["description"].(string); ok {
			desc = d
		}

		snap, err := h.executor.Snapshotter().Create(ctx, "manual_snapshot", desc)
		if err != nil {
			return errorResult(fmt.Sprintf("creating snapshot failed: %v", err)), nil
		}

		b, _ := json.MarshalIndent(snap, "", "  ")
		return textResult(string(b)), nil

	case "sandforge_diff":
		fromID, ok := args["from_snapshot"].(string)
		if !ok || fromID == "" {
			return errorResult("missing required argument 'from_snapshot'"), nil
		}
		toID := "working"
		if to, ok := args["to_snapshot"].(string); ok && to != "" {
			toID = to
		}

		diff, err := h.executor.Snapshotter().Diff(ctx, fromID, toID)
		if err != nil {
			return errorResult(fmt.Sprintf("diff failed: %v", err)), nil
		}

		b, _ := json.MarshalIndent(diff, "", "  ")
		return textResult(string(b)), nil

	case "sandforge_list_snapshots":
		limit := 20
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}

		list, err := h.executor.Snapshotter().List(ctx)
		if err != nil {
			return errorResult(fmt.Sprintf("listing snapshots failed: %v", err)), nil
		}

		if len(list) > limit {
			list = list[len(list)-limit:]
		}

		b, _ := json.MarshalIndent(list, "", "  ")
		return textResult(string(b)), nil

	case "sandforge_network_alerts":
		limit := 50
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}

		alerts := h.firewall.AlertHub().List(limit)
		b, _ := json.MarshalIndent(alerts, "", "  ")
		return textResult(string(b)), nil

	case "sandforge_status":
		active, idle := h.executor.Pool().Stats()
		totalAlerts, blocked := h.firewall.AlertHub().Stats()
		snapshots, _ := h.executor.Snapshotter().List(ctx)

		status := types.SandboxStatus{
			State:            "ready",
			Mode:             h.executor.Pool().ActiveRuntime(ctx).Mode(),
			WorkspacePath:    h.workspace,
			ActiveContainers: active,
			IdleContainers:   idle,
			TotalSnapshots:   len(snapshots),
			TotalAlerts:      totalAlerts,
			BlockedAttempts:  blocked,
			Firewall:         h.firewall.Config(),
			UptimeSeconds:    int64(time.Since(h.startTime).Seconds()),
		}

		b, _ := json.MarshalIndent(status, "", "  ")
		return textResult(string(b)), nil

	default:
		return errorResult(fmt.Sprintf("unknown tool: %s", name)), nil
	}
}

func textResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: false,
	}
}

func errorResult(msg string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: msg,
			},
		},
		IsError: true,
	}
}
