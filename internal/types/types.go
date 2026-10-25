package types

import "time"

// ExecutionResult captures the output of a command executed in the sandbox.
type ExecutionResult struct {
	Command          string    `json:"command"`
	ExitCode         int       `json:"exit_code"`
	Stdout           string    `json:"stdout"`
	Stderr           string    `json:"stderr"`
	DurationMs       int64     `json:"duration_ms"`
	SnapshotID       string    `json:"snapshot_id,omitempty"`
	DiffSummary      string    `json:"diff_summary,omitempty"`
	Error            string    `json:"error,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
	VerifyCommand    string    `json:"verify_command,omitempty"`
	Verified         bool      `json:"verified,omitempty"`
	VerifyPassed     bool      `json:"verify_passed,omitempty"`
	VerifyStdout     string    `json:"verify_stdout,omitempty"`
	VerifyStderr     string    `json:"verify_stderr,omitempty"`
	AutoRolledBack   bool      `json:"auto_rolled_back,omitempty"`
	PolicyViolation  string    `json:"policy_violation,omitempty"`
}

// Snapshot represents a captured filesystem checkpoint.
type Snapshot struct {
	ID             string    `json:"id"`
	ParentID       string    `json:"parent_id,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	TriggerCommand string    `json:"trigger_command"`
	Description    string    `json:"description,omitempty"`
	FilesCount     int       `json:"files_count"`
	SizeBytes      int64     `json:"size_bytes"`
	Status         string    `json:"status"` // clean, mutated, reverted
	CommitRef      string    `json:"commit_ref,omitempty"`
}

// DiffEntry describes the changes to a single file.
type DiffEntry struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // added, modified, deleted
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch,omitempty"`
}

// SnapshotDiff contains all file differences between two states.
type SnapshotDiff struct {
	FromID  string      `json:"from_id"`
	ToID    string      `json:"to_id"`
	Entries []DiffEntry `json:"entries"`
	Raw     string      `json:"raw"`
}

// NetworkAlert records a network request evaluated by the egress firewall.
type NetworkAlert struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	Process        string    `json:"process"`
	TargetHost     string    `json:"target_host"`
	TargetPort     int       `json:"target_port"`
	Protocol       string    `json:"protocol"` // tcp, udp, dns, http
	Action         string    `json:"action"`   // ALLOW, BLOCK
	Reason         string    `json:"reason"`
	PayloadSnippet string    `json:"payload_snippet,omitempty"`
	Severity       string    `json:"severity"` // info, warning, critical
}

// FirewallConfig defines network egress filtering rules.
type FirewallConfig struct {
	AllowedDomains    []string `json:"allowed_domains"`
	BlockedIPRanges   []string `json:"blocked_ip_ranges"`
	InterceptDNS      bool     `json:"intercept_dns"`
	StrictMode        bool     `json:"strict_mode"`
	DeepPacketScan    bool     `json:"deep_packet_scan"`
	HoneytokensActive int      `json:"honeytokens_active"`
}

// PolicyConfig defines destructive command prevention heuristics.
type PolicyConfig struct {
	BlockRootDeletions bool     `json:"block_root_deletions"`
	BlockForceGitPush  bool     `json:"block_force_git_push"`
	BlockForkBombs     bool     `json:"block_fork_bombs"`
	BlockSystemHalt    bool     `json:"block_system_halt"`
	BlockedPatterns    []string `json:"blocked_patterns"`
}

// ConnectedClient tracks an active MCP agent connection.
type ConnectedClient struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
	CallsCount  int       `json:"calls_count"`
}

// SandboxStatus reports operational health and metrics.
type SandboxStatus struct {
	State            string            `json:"state"` // ready, busy, degraded
	Mode             string            `json:"mode"`  // docker, process
	WorkspacePath    string            `json:"workspace_path"`
	ActiveContainers int               `json:"active_containers"`
	IdleContainers   int               `json:"idle_containers"`
	TotalSnapshots   int               `json:"total_snapshots"`
	TotalAlerts      int               `json:"total_alerts"`
	BlockedAttempts  int               `json:"blocked_attempts"`
	Firewall         FirewallConfig    `json:"firewall"`
	Policy           PolicyConfig      `json:"policy"`
	ConnectedClients []ConnectedClient `json:"connected_clients"`
	UptimeSeconds    int64             `json:"uptime_seconds"`
}
