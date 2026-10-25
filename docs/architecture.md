# SandForge Architecture and Design Specification

SandForge provides an isolated execution boundary, Copy-on-Write rollback engine, and network egress control plane for autonomous software engineering agents.

---

## 1. Threat Model and Security Objectives

Autonomous coding agents need terminal tools to inspect codebases, run test suites, install packages, and compile projects. When granted raw workstation access, three threats arise:

1. **Unintended Filesystem Destruction**: Agents may misinterpret instructions or shell syntax, triggering destructive commands such as `rm -rf`, `git reset --hard`, accidental file overwrites, or schema truncation.
2. **Secret Exfiltration via Prompt Injection**: Malicious repositories, untrusted dependencies, or issues can inject prompts directing the model to read `.env`, SSH keys, or cloud credentials and transmit them via network tools (`curl`, `wget`, `nc`) to attacker servers.
3. **Poisoned Dependency Lifecycle Hooks**: Untrusted npm, pip, or cargo packages executing preinstall or postinstall scripts with arbitrary shell access on developer workstations.

SandForge neutralizes these threats through pre-command snapshotting, process environment scrubbing, and a transparent egress firewall.

---

## 2. Copy-on-Write Snapshotting Engine

SandForge implements sub-15ms filesystem snapshotting through Git plumbing.

### Git Checkpoint Algorithm

Instead of copying entire workspace directories, SandForge uses Git's content-addressable storage:

1. Before any command executes, SandForge creates an ephemeral Git index path:
   `.git/sandforge_idx_<snapshot_id>`
2. SandForge stages the full workspace into this isolated index:
   `git add -A` with `GIT_INDEX_FILE=<temp_index>`
3. SandForge computes the root tree SHA object directly:
   `git write-tree`
4. SandForge writes an immutable commit object referencing the tree:
   `git commit-tree <tree_sha> -m "sandforge checkpoint"`
5. SandForge points a persistent reference to the commit:
   `refs/sandforge/<snapshot_id>`
6. The temporary index file is deleted.

Because Git only computes SHA hashes for modified file blocks and stores delta pointers, checkpoint creation completes in under 15 milliseconds even on large codebases.

### Rollback Mechanism

When a user or agent issues a rollback to `<snapshot_id>`:
1. SandForge identifies the commit SHA from `refs/sandforge/<snapshot_id>`.
2. SandForge checks out the tracked tree state:
   `git checkout <commit_sha> -- .`
3. SandForge removes untracked files and directories created by the command:
   `git clean -fd`
4. The workspace returns to the exact pre-execution state.

---

## 3. Container Runtime and Process Isolation

SandForge supports dual isolation modes:

### Docker Container Engine

When Docker or Podman is active:
- Ephemeral containers run with `/workspace` volume mounted.
- Resource limits are enforced via cgroups v2:
  - Memory ceiling: 2 GB
  - CPU quota: 2 cores (2,000,000,000 ns)
  - Process limits: 512 pids
- Standard streams (stdout, stderr) are demultiplexed via the Docker Engine SDK.

### Sanitized Process Isolation

When Docker is unavailable:
- Commands run in an isolated process tree with platform shells (`sh -c` on Unix, `cmd.exe /C` on Windows).
- Host environment variables are scrubbed, stripping sensitive tokens:
  `AWS_*`, `AZURE_*`, `GCP_*`, `OPENAI_*`, `ANTHROPIC_*`, `GITHUB_TOKEN`, `SLACK_*`, `STRIPE_*`, `DATABASE_URL`.
- Hard timeouts cancel runaway infinite loops.

---

## 4. Egress Firewall and DNS Interception

Coding tools like `curl`, `pip`, `npm`, `cargo`, and `git` honor standard proxy environment variables: `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`.

SandForge automatically binds an internal HTTP and HTTPS forward proxy on loopback. Every outbound connection is inspected against two security layers:

### Layer 1: Destination Host and IP Filter

- Direct connections to private RFC1918 subnets (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) are blocked.
- Connections to cloud instance metadata (`169.254.169.254/32`) are blocked.
- Direct outbound IP connections are blocked in strict mode.

### Layer 2: Domain Allowlist

Traffic is allowed only to verified package registries and developer APIs:
- `registry.npmjs.org`, `registry.yarnpkg.com`
- `pypi.org`, `files.pythonhosted.org`
- `proxy.golang.org`, `sum.golang.org`
- `github.com`, `api.github.com`, `raw.githubusercontent.com`
- `crates.io`, `static.crates.io`, `index.crates.io`
- `repo.maven.apache.org`
- `api.anthropic.com`, `api.openai.com`

When an unapproved destination is contacted:
1. SandForge terminates the connection with `403 Forbidden`.
2. A critical security alert is emitted with destination host, process name, and payload details.
3. The event is broadcast via WebSocket to the live web studio and recorded in the audit log.

---

## 5. Model Context Protocol (MCP) Interface

SandForge implements the standard JSON-RPC 2.0 stdio transport defined by the Model Context Protocol:

- `initialize`: declares tool capabilities to host LLMs.
- `tools/list`: registers `sandforge_run`, `sandforge_rollback`, `sandforge_diff`, `sandforge_snapshot`, `sandforge_network_alerts`, and `sandforge_status`.
- `tools/call`: executes sandboxed operations and returns formatted JSON text results.
