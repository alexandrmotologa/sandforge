# SandForge Model Context Protocol (MCP) Setup Guide

SandForge provides native Model Context Protocol (MCP) support, enabling AI coding agents to execute bash and terminal commands inside an isolated sandbox with automated rollback and egress firewall defense.

---

## 1. Claude Code Setup

To configure SandForge as the execution engine for Claude Code:

1. Open or create `~/.claude/config.json` (or project root `claude.json`).
2. Add the `sandforge` server definition under `mcpServers`:

```json
{
  "mcpServers": {
    "sandforge": {
      "command": "sandforge",
      "args": ["mcp", "--workspace", "."]
    }
  }
}
```

3. Restart Claude Code. When Claude issues terminal commands, it will invoke `sandforge_run`.

---

## 2. Cursor Setup

1. Open Cursor and navigate to **Settings** > **Features** > **MCP**.
2. Click **+ Add New MCP Server**.
3. Configure the following values:
   - **Name**: `sandforge`
   - **Type**: `command`
   - **Command**: `sandforge mcp --workspace .`
4. Click **Save**. The server status indicator will turn green.

---

## 3. Windsurf Setup

In your Windsurf configuration file (`~/.codeium/windsurf/mcp_config.json`):

```json
{
  "mcpServers": {
    "sandforge": {
      "command": "sandforge",
      "args": ["mcp", "--workspace", "."]
    }
  }
}
```

---

## 4. MCP Tools Reference

### `sandforge_run`

Executes a shell command inside the sandbox. Automatically creates a pre-execution snapshot.

- **Parameters**:
  - `command` (string, required): Shell command to execute.
  - `timeout_seconds` (integer, optional, default: 60): Timeout limit.
- **Returns**: JSON object containing:
  - `exit_code`: Process exit code.
  - `stdout`: Standard output.
  - `stderr`: Standard error.
  - `duration_ms`: Execution time in milliseconds.
  - `snapshot_id`: Snapshot identifier captured immediately before execution.
  - `diff_summary`: Human-readable summary of files added, modified, or deleted.

### `sandforge_rollback`

Restores the workspace to any recorded snapshot in under 15ms.

- **Parameters**:
  - `snapshot_id` (string, required): The target checkpoint ID (e.g. `snap-142030-a1b2c3`).
- **Returns**: Confirmation text string.

### `sandforge_diff`

Inspects unified differences between snapshots or against the current working tree.

- **Parameters**:
  - `from_snapshot` (string, required): Base snapshot ID.
  - `to_snapshot` (string, optional, default: "working"): Comparison target.
- **Returns**: Formatted unified diff patch and structured file list.

### `sandforge_snapshot`

Creates a named manual checkpoint of current workspace state.

- **Parameters**:
  - `description` (string, optional): Annotation explaining the checkpoint.
- **Returns**: Created snapshot object.

### `sandforge_network_alerts`

Lists outbound network attempts evaluated by the egress firewall.

- **Parameters**:
  - `limit` (integer, optional, default: 50): Max entries to return.
- **Returns**: Array of network audit alerts, including blocked exfiltration attempts.

### `sandforge_status`

Reports sandbox health, isolation runtime mode, container pool capacity, and firewall rules.
