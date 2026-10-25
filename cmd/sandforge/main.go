package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/audit"
	"github.com/alexandrmotologa/sandforge/internal/container"
	"github.com/alexandrmotologa/sandforge/internal/firewall"
	"github.com/alexandrmotologa/sandforge/internal/mcp"
	"github.com/alexandrmotologa/sandforge/internal/policy"
	"github.com/alexandrmotologa/sandforge/internal/server"
	"github.com/alexandrmotologa/sandforge/internal/snapshot"
	"github.com/alexandrmotologa/sandforge/internal/types"
)

const banner = `
  ____                  _ _____                    
 / ___|  __ _ _ __   __| |  ___|__  _ __ __ _  ___ 
 \___ \ / _` + "`" + ` | '_ \ / _` + "`" + ` | |_ / _ \| '__/ _` + "`" + ` |/ _ \
  ___) | (_| | | | | (_| |  _| (_) | | | (_| |  __/
 |____/ \__,_|_| |_|\__,_|_|  \___/|_|  \__, |\___|
                                        |___/      
 Local Sandbox, CoW Rollback & Egress Security Guard
`

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "mcp":
		runMCP(args)
	case "serve":
		runServe(args)
	case "run":
		runExec(args)
	case "rollback":
		runRollback(args)
	case "snapshots":
		runSnapshots(args)
	case "status":
		runStatus(args)
	case "init-mcp":
		runInitMCP(args)
	case "export-audit":
		runExportAudit(args)
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'sandforge help' for usage.\n", command)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(banner)
	fmt.Print(`
Usage:
  sandforge <command> [flags]

Commands:
  mcp          Start Model Context Protocol (MCP) stdio server for Claude Code and Cursor
  serve        Start SandForge HTTP API, WebSocket hub, and React 19 web studio
  run          Execute a command inside the sandbox with pre-snapshotting and diff tracking
  rollback     Restore workspace to a designated snapshot ID in under 15ms
  snapshots    List chronological snapshot timeline
  status       Display sandbox engine health and egress security policy
  init-mcp     Configure SandForge automatically for Claude Code, Cursor, or Windsurf
  export-audit Export security incident audit report to HTML, Markdown, or JSON

Flags:
  --workspace Path to workspace directory (default: current working directory)
  --port      HTTP server port (default: 8920, used with 'serve')
  --image     Docker base image (default: alpine:latest)
`)
}

func initServices(workspace string, imageName string) (*container.Executor, *firewall.Controller, *firewall.ProxyInterceptor, error) {
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolving workspace: %w", err)
	}

	gc, err := snapshot.NewGitCheckpoint(absWorkspace)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("initializing snapshotter: %w", err)
	}

	// Try Docker runtime first, fallback to isolated Process runtime
	var dockerRt container.Runtime
	if dr, err := container.NewDockerRuntime(absWorkspace, imageName); err == nil {
		if dr.IsAvailable(context.Background()) {
			dockerRt = dr
		}
	}

	processRt := container.NewProcessRuntime(absWorkspace)
	pool := container.NewPool(dockerRt, processRt, 2)
	policyEngine := policy.NewEngine()
	executor := container.NewExecutor(pool, gc, policyEngine)

	hub := firewall.NewAlertHub(1000)
	fw := firewall.NewController(hub)
	proxy := firewall.NewProxyInterceptor(fw)
	_ = proxy.Start()

	return executor, fw, proxy, nil
}

func runMCP(args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	workspace := fs.String("workspace", ".", "Workspace directory path")
	image := fs.String("image", "alpine:latest", "Docker base image")
	_ = fs.Parse(args)

	executor, fw, proxy, err := initServices(*workspace, *image)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if proxy != nil {
			_ = proxy.Close()
		}
		_ = executor.Pool().Close()
	}()

	absWorkspace, _ := filepath.Abs(*workspace)
	handler := mcp.NewToolHandler(executor, fw, proxy, absWorkspace)
	serverInstance := mcp.NewServer(handler, os.Stdin, os.Stdout)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	if err := serverInstance.Serve(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "MCP server exited with error: %v\n", err)
		os.Exit(1)
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8920, "Port for web studio and API")
	workspace := fs.String("workspace", ".", "Workspace directory path")
	image := fs.String("image", "alpine:latest", "Docker base image")
	_ = fs.Parse(args)

	fmt.Print(banner)
	absWorkspace, _ := filepath.Abs(*workspace)
	fmt.Printf("Starting SandForge server on port :%d (Workspace: %s)\n", *port, absWorkspace)

	executor, fw, proxy, err := initServices(*workspace, *image)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start services: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if proxy != nil {
			_ = proxy.Close()
		}
		_ = executor.Pool().Close()
	}()

	apiServer := server.NewServer(executor, fw, proxy, absWorkspace)
	defer apiServer.Close()

	// Find static distribution directory
	distDir := filepath.Join(absWorkspace, "web", "dist")
	staticServer := server.NewStaticServer(distDir)

	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer.Handler())
	mux.Handle("/", staticServer)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	go func() {
		fmt.Printf("Web Studio & API available at: http://localhost:%d\n", *port)
		if proxy != nil && proxy.Addr() != "" {
			fmt.Printf("Egress Firewall Proxy running at: %s\n", proxy.Addr())
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down SandForge gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func runExec(args []string) {
	workspace := "."
	verifyCmd := ""
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--workspace" && i+1 < len(args) {
			workspace = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--workspace=") {
			workspace = strings.TrimPrefix(arg, "--workspace=")
		} else if arg == "--verify" && i+1 < len(args) {
			verifyCmd = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--verify=") {
			verifyCmd = strings.TrimPrefix(arg, "--verify=")
		} else if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
		}
	}

	if len(positional) == 0 {
		fmt.Println("Usage: sandforge run \"<command>\" [--verify \"<test_cmd>\"] [--workspace .]")
		os.Exit(1)
	}

	cmdStr := positional[0]
	executor, _, proxy, err := initServices(workspace, "alpine:latest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if proxy != nil {
			_ = proxy.Close()
		}
		_ = executor.Pool().Close()
	}()

	var env []string
	if proxy != nil {
		env = append(env, proxy.EnvVars()...)
	}

	absWorkspace, _ := filepath.Abs(workspace)
	opts := container.ExecOptions{
		Timeout:    60 * time.Second,
		WorkingDir: absWorkspace,
		Env:        env,
	}

	var res *types.ExecutionResult
	if verifyCmd != "" {
		res, err = executor.RunWithVerify(context.Background(), cmdStr, verifyCmd, opts)
	} else {
		res, err = executor.Run(context.Background(), cmdStr, opts)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Execution failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[Snapshot: %s] [Duration: %dms] [ExitCode: %d]\n", res.SnapshotID, res.DurationMs, res.ExitCode)
	if res.AutoRolledBack {
		fmt.Println("[SECURITY/TEST GUARD]: Verification failed. Workspace automatically rolled back to clean state!")
	}
	if res.Stdout != "" {
		fmt.Print(res.Stdout)
	}
	if res.Stderr != "" {
		fmt.Fprint(os.Stderr, res.Stderr)
	}
	if res.DiffSummary != "" {
		fmt.Printf("Changes: %s\n", res.DiffSummary)
	}

	os.Exit(res.ExitCode)
}

func runRollback(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: sandforge rollback <snapshot_id> [--workspace .]")
		os.Exit(1)
	}

	snapID := args[0]
	workspace := "."
	for i, arg := range args {
		if arg == "--workspace" && i+1 < len(args) {
			workspace = args[i+1]
		}
	}

	absWorkspace, _ := filepath.Abs(workspace)
	gc, err := snapshot.NewGitCheckpoint(absWorkspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize snapshotter: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()
	if err := gc.Rollback(context.Background(), snapID); err != nil {
		fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Workspace successfully rolled back to %s in %dms\n", snapID, time.Since(start).Milliseconds())
}

func runSnapshots(args []string) {
	workspace := "."
	for i, arg := range args {
		if arg == "--workspace" && i+1 < len(args) {
			workspace = args[i+1]
		}
	}

	absWorkspace, _ := filepath.Abs(workspace)
	gc, err := snapshot.NewGitCheckpoint(absWorkspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize snapshotter: %v\n", err)
		os.Exit(1)
	}

	list, err := gc.List(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list snapshots: %v\n", err)
		os.Exit(1)
	}

	if len(list) == 0 {
		fmt.Println("No snapshots recorded yet.")
		return
	}

	fmt.Printf("%-22s %-20s %-8s %s\n", "SNAPSHOT ID", "TIMESTAMP", "STATUS", "COMMAND")
	fmt.Println(strings.Repeat("-", 80))
	for _, s := range list {
		fmt.Printf("%-22s %-20s %-8s %s\n", s.ID, s.Timestamp.Format("2006-01-02 15:04:05"), s.Status, s.TriggerCommand)
	}
}

func runStatus(args []string) {
	workspace := "."
	for i, arg := range args {
		if arg == "--workspace" && i+1 < len(args) {
			workspace = args[i+1]
		}
	}

	executor, fw, _, err := initServices(workspace, "alpine:latest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = executor.Pool().Close()
	}()

	ctx := context.Background()
	rt := executor.Pool().ActiveRuntime(ctx)
	active, idle := executor.Pool().Stats()
	totalAlerts, blocked := fw.AlertHub().Stats()
	snapshots, _ := executor.Snapshotter().List(ctx)

	fmt.Println("SandForge Status:")
	fmt.Printf("  Isolation Mode:    %s\n", rt.Mode())
	fmt.Printf("  Active Containers: %d\n", active)
	fmt.Printf("  Idle Pool Size:    %d\n", idle)
	fmt.Printf("  Total Snapshots:   %d\n", len(snapshots))
	fmt.Printf("  Network Alerts:    %d total (%d blocked exfiltrations)\n", totalAlerts, blocked)
	fmt.Printf("  Allowed Domains:   %d domains configured\n", len(fw.AllowedDomains()))
	fmt.Printf("  Honeytokens:       %d active decoy tripwires\n", fw.HoneyVault().Count())
}

func runInitMCP(args []string) {
	fs := flag.NewFlagSet("init-mcp", flag.ExitOnError)
	client := fs.String("client", "claude", "Target AI client: claude, cursor, or windsurf")
	workspace := fs.String("workspace", ".", "Target workspace directory path")
	binary := fs.String("bin", "", "Path to sandforge binary (default: sandforge in PATH)")
	_ = fs.Parse(args)

	cfgPath, err := mcp.InstallConfig(*client, *workspace, *binary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure MCP client: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully configured SandForge MCP server for '%s'!\n", *client)
	fmt.Printf("Config file: %s\n", cfgPath)
	fmt.Println("You can now launch your agent to execute commands safely inside SandForge.")
}

func runExportAudit(args []string) {
	fs := flag.NewFlagSet("export-audit", flag.ExitOnError)
	workspace := fs.String("workspace", ".", "Target workspace directory path")
	format := fs.String("format", "md", "Output format: md, html, or json")
	output := fs.String("output", "", "Optional destination file path")
	_ = fs.Parse(args)

	executor, fw, _, err := initServices(*workspace, "alpine:latest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = executor.Pool().Close()
	}()

	ctx := context.Background()
	snapshots, _ := executor.Snapshotter().List(ctx)
	alerts := fw.AlertHub().List(200)

	status := types.SandboxStatus{
		State:            "ready",
		Mode:             executor.Pool().ActiveRuntime(ctx).Mode(),
		WorkspacePath:    *workspace,
		TotalSnapshots:   len(snapshots),
		TotalAlerts:      len(alerts),
		BlockedAttempts:  0,
		Firewall:         fw.Config(),
		Policy:           executor.Policy().Config(),
	}

	for _, a := range alerts {
		if a.Action == "BLOCK" {
			status.BlockedAttempts++
		}
	}

	var content string
	switch strings.ToLower(*format) {
	case "html":
		content = audit.GenerateHTMLReport(status, alerts, snapshots)
	case "json":
		bytes, _ := audit.GenerateJSONReport(status, alerts, snapshots)
		content = string(bytes)
	default:
		content = audit.GenerateMarkdownReport(status, alerts, snapshots)
	}

	if *output != "" {
		if err := os.WriteFile(*output, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Writing output failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Audit report exported successfully to: %s\n", *output)
	} else {
		fmt.Println(content)
	}
}
