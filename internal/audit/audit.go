package audit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// GenerateMarkdownReport compiles a full human-readable audit report in Markdown.
func GenerateMarkdownReport(status types.SandboxStatus, alerts []*types.NetworkAlert, snapshots []*types.Snapshot) string {
	var sb strings.Builder

	sb.WriteString("# SandForge Security and Execution Audit Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated**: %s  \n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Workspace**: `%s`  \n", status.WorkspacePath))
	sb.WriteString(fmt.Sprintf("**Isolation Engine**: `%s`  \n", status.Mode))
	sb.WriteString(fmt.Sprintf("**Total Checkpoints**: %d | **Blocked Threats**: %d  \n\n", len(snapshots), status.BlockedAttempts))

	sb.WriteString("---\n\n")
	sb.WriteString("## 1. Security Incidents & Blocked Egress Attempts\n\n")

	var blockedAlerts []*types.NetworkAlert
	for _, a := range alerts {
		if a.Action == "BLOCK" {
			blockedAlerts = append(blockedAlerts, a)
		}
	}

	if len(blockedAlerts) == 0 {
		sb.WriteString("No blocked security incidents recorded during this session.\n\n")
	} else {
		sb.WriteString("| Timestamp | Severity | Target Host | Port | Process | Reason |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- |\n")
		for _, a := range blockedAlerts {
			sb.WriteString(fmt.Sprintf("| %s | `%s` | `%s` | %d | %s | %s |\n",
				a.Timestamp.Format("15:04:05"),
				a.Severity,
				a.TargetHost,
				a.TargetPort,
				a.Process,
				a.Reason,
			))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 2. Checkpoint History & CoW Rollbacks\n\n")
	if len(snapshots) == 0 {
		sb.WriteString("No filesystem checkpoints recorded.\n\n")
	} else {
		sb.WriteString("| Snapshot ID | Timestamp | Status | Command | Files |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
		for _, s := range snapshots {
			sb.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | `%s` | %d |\n",
				s.ID,
				s.Timestamp.Format("15:04:05"),
				s.Status,
				s.TriggerCommand,
				s.FilesCount,
			))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 3. Active Firewall & Security Policy\n\n")
	sb.WriteString(fmt.Sprintf("- **DNS Interception**: %v\n", status.Firewall.InterceptDNS))
	sb.WriteString(fmt.Sprintf("- **Strict Mode**: %v\n", status.Firewall.StrictMode))
	sb.WriteString(fmt.Sprintf("- **Deep Packet Scan**: %v\n", status.Firewall.DeepPacketScan))
	sb.WriteString(fmt.Sprintf("- **Honeytokens Active**: %d\n", status.Firewall.HoneytokensActive))
	sb.WriteString(fmt.Sprintf("- **Allowed Domains**: %s\n\n", strings.Join(status.Firewall.AllowedDomains, ", ")))

	return sb.String()
}

// GenerateHTMLReport creates a polished, standalone HTML audit report.
func GenerateHTMLReport(status types.SandboxStatus, alerts []*types.NetworkAlert, snapshots []*types.Snapshot) string {
	md := GenerateMarkdownReport(status, alerts, snapshots)

	var blockedRows strings.Builder
	for _, a := range alerts {
		if a.Action == "BLOCK" {
			blockedRows.WriteString(fmt.Sprintf(`
				<tr style="background: rgba(239,68,68,0.1);">
					<td style="padding: 8px; border: 1px solid #334155;">%s</td>
					<td style="padding: 8px; border: 1px solid #334155; color: #ef4444; font-weight: bold;">%s</td>
					<td style="padding: 8px; border: 1px solid #334155; font-family: monospace;">%s:%d</td>
					<td style="padding: 8px; border: 1px solid #334155;">%s</td>
					<td style="padding: 8px; border: 1px solid #334155;">%s</td>
				</tr>
			`, a.Timestamp.Format("15:04:05"), a.Severity, a.TargetHost, a.TargetPort, a.Process, a.Reason))
		}
	}

	if blockedRows.Len() == 0 {
		blockedRows.WriteString(`<tr><td colspan="5" style="padding: 12px; text-align: center; color: #94a3b8;">No blocked security threats detected.</td></tr>`)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8"/>
	<title>SandForge Security Incident Report</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #090d16; color: #f8fafc; padding: 2rem; max-width: 900px; margin: 0 auto; line-height: 1.5; }
		h1 { color: #38bdf8; font-size: 1.5rem; margin-bottom: 0.5rem; }
		.card { background: #0f172a; border: 1px solid #1e293b; border-radius: 8px; padding: 1.25rem; margin-bottom: 1.5rem; }
		table { width: 100%%; border-collapse: collapse; font-size: 0.85rem; margin-top: 1rem; }
		th { background: #1e293b; text-align: left; padding: 8px; border: 1px solid #334155; color: #94a3b8; }
		.badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 0.75rem; font-weight: 600; }
		.badge-danger { background: rgba(239,68,68,0.2); color: #f87171; border: 1px solid rgba(239,68,68,0.4); }
		.badge-success { background: rgba(16,185,129,0.2); color: #34d399; border: 1px solid rgba(16,185,129,0.4); }
	</style>
</head>
<body>
	<h1>SandForge Security &amp; Execution Audit Report</h1>
	<p style="color: #94a3b8; font-size: 0.85rem;">Generated on %s &bull; Workspace: <code>%s</code></p>

	<div class="card">
		<h3 style="margin-top:0; color: #f1f5f9;">Session Overview</h3>
		<p><strong>Isolation Mode:</strong> %s | <strong>Checkpoints Recorded:</strong> %d | <strong>Blocked Threats:</strong> <span class="badge badge-danger">%d</span></p>
	</div>

	<div class="card">
		<h3 style="margin-top:0; color: #f87171;">Blocked Egress &amp; Exfiltration Events</h3>
		<table>
			<thead>
				<tr>
					<th>Time</th>
					<th>Severity</th>
					<th>Target Host</th>
					<th>Process</th>
					<th>Violation Details</th>
				</tr>
			</thead>
			<tbody>
				%s
			</tbody>
		</table>
	</div>

	<details style="margin-top: 1.5rem; color: #64748b; font-size: 0.8rem;">
		<summary style="cursor: pointer;">View Raw Markdown Export</summary>
		<pre style="background: #020617; padding: 1rem; border-radius: 6px; overflow-x: auto; color: #cbd5e1;">%s</pre>
	</details>
</body>
</html>`, time.Now().Format("2006-01-02 15:04:05"), status.WorkspacePath, status.Mode, len(snapshots), status.BlockedAttempts, blockedRows.String(), md)
}

// GenerateJSONReport creates a raw JSON audit export.
func GenerateJSONReport(status types.SandboxStatus, alerts []*types.NetworkAlert, snapshots []*types.Snapshot) ([]byte, error) {
	data := map[string]interface{}{
		"generated_at": time.Now().Format(time.RFC3339),
		"status":       status,
		"alerts":       alerts,
		"snapshots":    snapshots,
	}
	return json.MarshalIndent(data, "", "  ")
}
