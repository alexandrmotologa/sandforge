export interface PolicyConfig {
  block_root_deletions: boolean;
  block_force_git_push: boolean;
  block_fork_bombs: boolean;
  block_system_halt: boolean;
  blocked_patterns: string[];
}

export interface ConnectedClient {
  id: string;
  name: string;
  version: string;
  connected_at: string;
  last_seen: string;
  calls_count: number;
}

export interface SandboxStatus {
  state: string;
  mode: string;
  workspace_path: string;
  active_containers: number;
  idle_containers: number;
  total_snapshots: number;
  total_alerts: number;
  blocked_attempts: number;
  firewall: FirewallConfig;
  policy: PolicyConfig;
  connected_clients: ConnectedClient[];
  uptime_seconds: number;
}

export interface FirewallConfig {
  allowed_domains: string[];
  blocked_ip_ranges: string[];
  intercept_dns: boolean;
  strict_mode: boolean;
  deep_packet_scan: boolean;
  honeytokens_active: number;
}

export interface Snapshot {
  id: string;
  parent_id?: string;
  timestamp: string;
  trigger_command: string;
  description?: string;
  files_count: number;
  size_bytes: number;
  status: string;
  commit_ref?: string;
}

export interface DiffEntry {
  path: string;
  status: 'added' | 'modified' | 'deleted';
  additions: number;
  deletions: number;
  patch?: string;
}

export interface SnapshotDiff {
  from_id: string;
  to_id: string;
  entries: DiffEntry[];
  raw: string;
}

export interface NetworkAlert {
  id: string;
  timestamp: string;
  process: string;
  target_host: string;
  target_port: number;
  protocol: string;
  action: 'ALLOW' | 'BLOCK';
  reason: string;
  payload_snippet?: string;
  severity: 'info' | 'warning' | 'critical';
}

export interface ExecutionResult {
  command: string;
  exit_code: number;
  stdout: string;
  stderr: string;
  duration_ms: number;
  snapshot_id?: string;
  diff_summary?: string;
  error?: string;
  timestamp: string;
  verify_command?: string;
  verified?: boolean;
  verify_passed?: boolean;
  verify_stdout?: string;
  verify_stderr?: string;
  auto_rolled_back?: boolean;
  policy_violation?: string;
}
