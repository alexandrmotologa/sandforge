# SandForge: Master Implementation Plan

Local Ephemeral Execution Sandbox, Copy-on-Write Rollback, and Egress Security Guard for Autonomous AI Coding Agents.

SandForge provides an isolated execution sandbox and security supervisor for autonomous coding agents (Claude Code, Devin, Cursor, Aider). It executes shell commands in sub-150ms isolated containers or isolated processes, captures Copy-on-Write snapshots before each file modification, offers instant sub-15ms rollback on regressions, and enforces an egress firewall to block credential exfiltration caused by prompt injection.

---

## 1. Architecture Overview

Autonomous coding agents need terminal access to run builds, execute tests, and install dependencies. Bare-metal execution risks three primary hazards:
1. Destructive commands: accidental file deletions, unintended git resets, and unrecoverable database migrations.
2. Secret exfiltration: untrusted code or repository text directing the agent via prompt injection to read `.env` files or credentials and transmit them to external servers.
3. Compromised packages: dependency installs running malicious lifecycle scripts on the host workstation.

SandForge solves these hazards locally:
- Ephemeral container engine: maintains pre-warmed idle containers via Docker Engine SDK or Podman, mounting workspaces with resource bounds.
- Fallback process isolation: provides process isolation and memory caps on workstations without a running Docker daemon.
- Copy-on-Write snapshots: captures filesystem state before every command via Git ref checkpointing (`git write-tree`) and Linux OverlayFS.
- Sub-15ms rollback: restores workspace state instantly to any checkpoint in the timeline.
- Strict egress firewall: blocks outbound connections to unapproved IP ranges, private subnets, and cloud metadata services (`169.254.169.254`), filtering DNS against a strict domain allowlist.
- Model Context Protocol (MCP) server: exposes JSON-RPC 2.0 tools (`sandforge_run`, `sandforge_rollback`, `sandforge_snapshot`, `sandforge_diff`, `sandforge_status`, `sandforge_network_alerts`) for Claude Code, Cursor, and Windsurf.
- React 19 web studio: visual dashboard with snapshot history, side by side diff inspection, real time network audit log, and terminal execution logs.

```
+-----------------------------------------------------------------------------------+
|                                     SANDFORGE                                     |
+-----------------------------------------+-----------------------------------------+
                                          |
        +---------------------------------+---------------------------------+
        |                                                                   |
+-------v-------------------------+                 +-----------------------v-------+
|   Pre-Warmed Container Engine   |                 |    Copy-on-Write Rollback     |
|  * Sub-150ms Ephemeral Startup  |                 |  * Pre-Execution FS Snapshot  |
|  * Docker SDK / Process Guard   |                 |  * Git & OverlayFS Checkpoints|
|  * Workspace Mount & Resource Caps               |  * Sub-15ms Time-Travel Undo  |
+-------+-------------------------+                 +-----------------------+-------+
        |                                                                   |
        +---------------------------------+---------------------------------+
                                          |
                        +-----------------v-----------------+
                        |    Egress Network & Secret Guard  |
                        |  * Domain Allowlist DNS Filter    |
                        |  * Blocks Outbound IP Exfil       |
                        |  * Prompt Injection Mitigation    |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |    Model Context Protocol (MCP)   |
                        |  * sandforge_run                  |
                        |  * sandforge_rollback / diff      |
                        |  * Standard JSON-RPC 2.0 Pipe     |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |   Interactive Web Studio & API    |
                        |  * Snapshot Tree & Diff Viewer    |
                        |  * Egress Network Alert Log       |
                        |  * Live Terminal Execution Stream |
                        +-----------------------------------+
```

---

## 2. Directory Structure

```
sandforge/
├── cmd/
│   └── sandforge/
│       └── main.go               # CLI entrypoint (mcp, serve, run, rollback, status)
├── internal/
│   ├── container/
│   │   ├── runtime.go            # Docker SDK & local process runner interfaces
│   │   ├── docker.go             # Docker client implementation
│   │   ├── process.go            # Fallback isolated host runner
│   │   ├── pool.go               # Pre-warmed container worker pool
│   │   ├── exec.go               # PTY/stream execution with timeout management
│   │   └── limits.go             # CPU, RAM, and process limits
│   ├── snapshot/
│   │   ├── snapshot.go           # Core Snapshotter interface
│   │   ├── git_checkpoint.go     # Git write-tree ref checkpoint engine (<15ms)
│   │   ├── cow_overlay.go        # Linux OverlayFS snapshotter
│   │   ├── timeline.go           # Snapshot hierarchy and undo/redo tree
│   │   └── diff.go               # Unified diff computation between checkpoints
│   ├── firewall/
│   │   ├── firewall.go           # Egress firewall controller
│   │   ├── dns_proxy.go          # Domain allowlist DNS interception
│   │   ├── proxy_interceptor.go  # Outbound TCP/HTTP traffic guard
│   │   └── alerts.go             # Security event logger and alert dispatcher
│   ├── mcp/
│   │   ├── server.go             # MCP JSON-RPC 2.0 stdio server
│   │   ├── tools.go              # sandforge_run, rollback, diff, snapshots, alerts
│   │   └── schema.go             # Tool definitions and input schemas
│   └── server/
│       ├── api.go                # REST API router
│       ├── websocket.go          # Real time streaming hub
│       └── static.go             # Embedded static frontend handler
├── web/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Header.tsx        # Status bar and navigation
│   │   │   ├── SnapshotTimeline.tsx # Interactive snapshot history tree
│   │   │   ├── DiffViewerModal.tsx  # Side by side code diff viewer
│   │   │   ├── EgressLogTable.tsx   # Network firewall event table
│   │   │   ├── ProcessMonitor.tsx   # Resource usage and metrics
│   │   │   └── TerminalConsole.tsx  # Real time execution terminal
│   │   ├── hooks/
│   │   │   └── useSandboxStream.ts  # WebSocket data hook
│   │   ├── App.tsx               # Root React application
│   │   ├── index.css             # Tailwind and design tokens
│   │   └── main.tsx              # Application entrypoint
│   ├── package.json
│   ├── vite.config.ts
│   └── tailwind.config.js
├── docs/
│   ├── images/                   # Screenshots and demo GIF
│   └── architecture.md
├── go.mod
├── go.sum
└── README.md
```

---

## 3. Implementation Plan & Progress

### Phase 1: Go Module Initialization & Core Interfaces
- Initialize Go module `github.com/alexandrmotologa/sandforge` (Go 1.23+).
- Define domain models: `ExecutionResult`, `Snapshot`, `DiffEntry`, `NetworkAlert`, `FirewallRule`.
- Implement CLI flags and configuration loader (`cmd/sandforge/main.go`).

### Phase 2: Snapshot & Copy-on-Write Rollback Engine
- Implement `internal/snapshot/git_checkpoint.go`:
  - Rapid filesystem snapshotting via `git write-tree` and ephemeral git references.
  - Snapshot creation in under 15ms without external dependencies.
  - Automatic preservation of untracked files and working tree state.
- Implement `internal/snapshot/cow_overlay.go`:
  - Linux OverlayFS snapshotter with separate upper and work directories.
- Implement `internal/snapshot/timeline.go`:
  - Timeline storage with branch, parent, and rollback capabilities.
- Implement `internal/snapshot/diff.go`:
  - Computation of unified diffs between any two snapshots or against working tree.

### Phase 3: Container & Sandbox Execution Runtime
- Implement `internal/container/runtime.go`:
  - Unified `Runtime` interface supporting `Execute(ctx, cmd, opts)` and `Close()`.
- Implement `internal/container/docker.go`:
  - Docker Engine SDK integration with volume mounts, memory caps, CPU quotas, and pid limits.
- Implement `internal/container/process.go`:
  - Fallback process sandbox with environment sanitization and resource supervision.
- Implement `internal/container/pool.go`:
  - Pre-warmed pool maintaining idle ready containers to ensure execution start in under 150ms.
- Implement `internal/container/exec.go`:
  - Command execution with timeout management, stdout and stderr separation, and exit code capture.

### Phase 4: Egress Firewall & Secret Exfiltration Guard
- Implement `internal/firewall/dns_proxy.go`:
  - DNS interceptor testing outbound domain requests against an allowlist:
    `registry.npmjs.org`, `pypi.org`, `proxy.golang.org`, `github.com`, `crates.io`, `api.anthropic.com`, `api.openai.com`.
  - Blocks requests to cloud metadata (`169.254.169.254`), private networks, and unauthorized hosts.
- Implement `internal/firewall/proxy_interceptor.go`:
  - Transparent forward proxy and connection monitor recording source process, target address, byte counts, and decisions.
- Implement `internal/firewall/alerts.go`:
  - Security alert dispatcher alerting on exfiltration attempts.

### Phase 5: Model Context Protocol (MCP) Server
- Implement `internal/mcp/server.go`:
  - Standard JSON-RPC 2.0 stdio server handling `initialize`, `tools/list`, and `tools/call`.
- Expose MCP tools:
  - `sandforge_run`: executes command, automatically snapshots before execution, returns stdout, stderr, exit code, diff summary, and snapshot ID.
  - `sandforge_rollback`: reverts workspace to a previous snapshot ID.
  - `sandforge_snapshot`: captures an explicit named checkpoint.
  - `sandforge_diff`: retrieves unified diff for a snapshot.
  - `sandforge_list_snapshots`: lists timeline of checkpoints.
  - `sandforge_network_alerts`: lists network activity and blocked egress attempts.
  - `sandforge_status`: returns sandbox health, pool capacity, and configuration.

### Phase 6: REST API & WebSocket Server
- Implement `internal/server/api.go`:
  - REST endpoints for snapshots, rollbacks, command execution, firewall rules, and alerts.
- Implement `internal/server/websocket.go`:
  - Real time broadcast of command execution logs, CPU/memory usage, and security events.
- Implement `internal/server/static.go`:
  - Serving of embedded or local React 19 web assets.

### Phase 7: React 19 Web Studio
- Setup Vite + React 19 + Tailwind CSS in `web/`.
- Build components:
  - `SnapshotTimeline`: visual tree showing each command run by agents with pass/fail badges and instant rollback buttons.
  - `DiffViewerModal`: side by side unified diff inspector.
  - `EgressLogTable`: live table of allowed and blocked network requests with severity indicators.
  - `ProcessMonitor`: gauges for memory, CPU, and container pool status.
  - `TerminalConsole`: interactive terminal with streaming command logs.

### Phase 8: Verification, Test Suite & Documentation
- Unit and integration tests for snapshot engine, rollback fidelity, firewall blocking, and MCP protocol.
- Simulated attack tests: prompt injection with `curl` exfiltration, destructive command `rm -rf` with clean rollback.
- Production README.md and documentation following humanizer guidelines.
- UI screenshots and animated demo GIF for portfolio synchronization.
