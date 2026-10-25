<p align="center">
  <img src="docs/images/logo.png?raw=true" alt="SandForge Logo" width="130" style="border-radius: 24px;" />
</p>

<h1 align="center">SandForge</h1>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go" alt="Go 1.23" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat&logo=react" alt="React 19" />
  <img src="https://img.shields.io/badge/Protocol-MCP-10b981?style=flat" alt="Model Context Protocol" />
  <img src="https://img.shields.io/badge/Rollback-%3C15ms-emerald?style=flat" alt="Sub-15ms Rollback" />
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License" />
</p>

<p align="center">
  Local ephemeral execution sandbox, Copy-on-Write rollback engine, and egress security guard for autonomous AI coding agents.
</p>

---

<p align="center">
  <img src="docs/images/sandforge_demo.gif" alt="SandForge Animated Demo" width="100%" />
</p>

SandForge isolates shell commands issued by coding agents such as Claude Code, Cursor, Devin, and Aider. It executes commands in pre-warmed isolated containers or sandboxed processes in under 150ms, captures filesystem checkpoints before each execution, restores prior state in under 15ms on regressions, and blocks outbound connections to unapproved hosts to prevent credential theft caused by prompt injection.

---

## Capabilities

- **Sub-150ms Execution**: Dispatches commands through a pre-warmed pool of idle execution runners, reducing startup overhead compared to cold Docker spins or cloud sandboxes.
- **Sub-15ms Copy-on-Write Rollback**: Creates an instant Git ref checkpoint (`refs/sandforge/*`) via isolated Git index trees before any tool mutation. Restores modified, deleted, and untracked files in milliseconds.
- **Test-Driven Auto-Rollback**: Accepts a verification test command alongside the target command (`sandforge_run_with_verify`). If post-execution tests fail, SandForge immediately reverts the workspace to the pre-command checkpoint.
- **Deep Packet Inspection & Honeytoken Vault**: Inspects egress HTTP payloads and queries for leaked credentials (OpenAI, Anthropic, GitHub PAT, AWS, SSH keys) and plants synthetic honeypot tripwires to catch prompt injection exfiltration before bytes leave the host.
- **Destructive Command Interceptor**: Deterministic policy engine that inspects AST and token patterns before execution, blocking root wipes (`rm -rf /`), forced git pushes, fork bombs, and system shutdowns.
- **Egress Firewall**: Enforces a strict domain allowlist for package registries (`registry.npmjs.org`, `pypi.org`, `proxy.golang.org`, `github.com`, `crates.io`). Blocks private IP ranges and cloud instance metadata (`169.254.169.254`), neutralizing prompt injection exfiltration attacks.
- **1-Click MCP Installer**: Generates ready-to-run Model Context Protocol configurations for Claude Code, Cursor, and Windsurf (`sandforge init-mcp --client claude`).
- **Audit Export & Incident Reports**: Generates formal incident and compliance reports in HTML, Markdown, and JSON formats for post-mortem analysis.
- **Web Studio & Real-time Stream**: React 19 dashboard showing visual snapshot timelines, side-by-side diffs, active process monitors, live console with policy block alerts, and real-time egress network audit tables over WebSocket.

---

## Screenshots

| Snapshot History & Timeline | Network Egress Guard & Allowlist |
| :---: | :---: |
| ![Snapshot Timeline](docs/images/sandforge-1.png) | ![Egress Guard](docs/images/sandforge-2.png) |

| Side-by-Side Diff Inspector | Interactive Execution Console |
| :---: | :---: |
| ![Diff Inspector](docs/images/sandforge-3.png) | ![Interactive Console](docs/images/sandforge-4.png) |

---

## Architecture

SandForge coordinates three core subsystems:

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
|  * Docker SDK / Process Runner  |                 |  * Git & OverlayFS Checkpoints|
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

1. **Execution Runtime**: Uses the Docker Engine SDK to manage pre-warmed idle containers mounting the target workspace with CPU quotas, memory ceilings, and process caps. On hosts without a running Docker daemon, it switches to a sanitized process runner that strips host environment secrets.
2. **Snapshotter**: Leverages Git object plumbing (`git write-tree` on isolated index files) or Linux OverlayFS layers to capture snapshots without modifying user git branches.
3. **Network Guard**: Runs an HTTP and HTTPS forward proxy combined with a local DNS resolver. When tools like `curl`, `pip`, or `npm` attempt outbound connections, requests to unknown IPs or unauthorized external domains receive an immediate HTTP 403 Forbidden response and trigger a security alert.

---

## Installation

### Prerequisites

- Go 1.23 or newer
- Git 2.40 or newer
- Node.js 20 or newer (for web studio development)
- Docker Desktop or Podman (optional; local process isolation is used if Docker is absent)

### Build from source

```bash
git clone https://github.com/alexandrmotologa/sandforge.git
cd sandforge
go build -o bin/sandforge ./cmd/sandforge
```

---

## Command Line Usage

### Start Web Studio and API

```bash
sandforge serve --port 8920 --workspace .
```

Open `http://localhost:8920` in your browser to access the live dashboard.

### Execute Sandboxed Commands

Run commands directly with automated pre-execution snapshotting:

```bash
sandforge run "npm test"
sandforge run "python -m pytest"
```

### Run with Test-Driven Auto-Rollback

Execute a command and automatically revert all filesystem changes if verification tests fail:

```bash
sandforge run "npm install buggy-package" --verify "npm test"
```

### Destructive Command Protection & Policy

Commands violating security policies are stopped prior to invocation:

```bash
sandforge run "rm -rf /"
# [POLICY BLOCKED] Root Filesystem Deletion: Attempts to delete root, parent, or system drive filesystem (Exit 126)
```

### Inspect Snapshots and Rollback

```bash
# List recorded snapshot checkpoints
sandforge snapshots

# Rollback to any prior state in under 15ms
sandforge rollback snap-142030-a1b2c3

# View engine health, decoy honeytokens, and firewall status
sandforge status

# Export incident audit report
sandforge export-audit --format html --output audit-report.html
```

---

## Model Context Protocol (MCP) Setup

SandForge connects directly into Claude Code, Cursor, and Windsurf via standard stdio JSON-RPC 2.0.

### 1-Click MCP Configuration

Automatically generate or install client configurations:

```bash
# Configure Claude Code
sandforge init-mcp --client claude --workspace .

# Configure Cursor IDE
sandforge init-mcp --client cursor --workspace .

# Configure Windsurf
sandforge init-mcp --client windsurf --workspace .
```

### Manual Configuration Example

Add SandForge to your `claude.json` or `~/.claude/config.json`:

```json
{
  "mcpServers": {
    "sandforge": {
      "command": "sandforge",
      "args": ["mcp", "--workspace", "/path/to/project"]
    }
  }
}
```

### Exposed MCP Tools

- `sandforge_run(command, timeout_seconds)`: Executes a shell command with pre-snapshotting, returning stdout, stderr, exit code, and filesystem mutation summary.
- `sandforge_run_with_verify(command, verify_command, timeout_seconds)`: Executes command followed by verification test; automatically reverts workspace if verification fails.
- `sandforge_policy_check(command)`: Pre-evaluates whether a command violates safety policies before running it.
- `sandforge_rollback(snapshot_id)`: Restores workspace to a specific checkpoint in milliseconds.
- `sandforge_snapshot(description)`: Creates a named manual checkpoint.
- `sandforge_diff(from_snapshot, to_snapshot)`: Returns unified diff between checkpoints or against current working directory.
- `sandforge_list_snapshots(limit)`: Lists historical checkpoints.
- `sandforge_network_alerts(limit)`: Returns network traffic events, honeytoken tripwire triggers, and blocked exfiltration attempts.
- `sandforge_status()`: Returns sandbox health, decoy trap counts, and connected client sessions.

---

## REST API Reference

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/status` | `GET` | Health, isolation mode, active honeytokens, and capacity |
| `/api/snapshots` | `GET`, `POST` | List or create checkpoints |
| `/api/snapshots/rollback` | `POST` | Revert workspace to snapshot ID |
| `/api/snapshots/diff` | `GET` | Unified diff between snapshots |
| `/api/exec` | `POST` | Execute command with pre-checkpoint and policy evaluation |
| `/api/exec/verify` | `POST` | Execute command with automated test verification and auto-rollback |
| `/api/policy` | `GET`, `POST` | View or configure command interception policies |
| `/api/audit/report` | `GET` | Export security incident report in HTML, Markdown, or JSON |
| `/api/firewall/rules` | `GET`, `POST`, `DELETE` | View, add, or remove domain rules |
| `/api/firewall/alerts` | `GET` | Network security log and exfil alerts |
| `/api/ws` | `WS` | Real-time WebSocket event stream |

---

## Testing

Run unit and integration suites:

```bash
go test -v ./...
```

Verify the React 19 frontend:

```bash
cd web
npm run build
```

---

## License

MIT License. Developed by Alexandr Motologa.
