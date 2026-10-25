package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/audit"
	"github.com/alexandrmotologa/sandforge/internal/container"
	"github.com/alexandrmotologa/sandforge/internal/firewall"
	"github.com/alexandrmotologa/sandforge/internal/types"
)

// Server holds HTTP routing, WebSocket hub, and business engine references.
type Server struct {
	executor    *container.Executor
	firewall    *firewall.Controller
	proxy       *firewall.ProxyInterceptor
	hub         *Hub
	workspace   string
	startTime   time.Time
	mux         *http.ServeMux
	alertCancel context.CancelFunc
}

// NewServer initializes the API server.
func NewServer(executor *container.Executor, fw *firewall.Controller, proxy *firewall.ProxyInterceptor, workspace string) *Server {
	hub := NewHub()
	go hub.Run()

	s := &Server{
		executor:  executor,
		firewall:  fw,
		proxy:     proxy,
		hub:       hub,
		workspace: workspace,
		startTime: time.Now(),
		mux:       http.NewServeMux(),
	}

	s.setupRoutes()
	s.startAlertRelay()
	return s
}

func (s *Server) startAlertRelay() {
	alertCh := s.firewall.AlertHub().Subscribe()
	ctx, cancel := context.WithCancel(context.Background())
	s.alertCancel = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				s.firewall.AlertHub().Unsubscribe(alertCh)
				return
			case alert, ok := <-alertCh:
				if !ok {
					return
				}
				s.hub.BroadcastEvent("alert", alert)
			}
		}
	}()
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/api/ws", s.hub.HandleWebSocket)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/snapshots", s.handleSnapshots)
	s.mux.HandleFunc("/api/snapshots/rollback", s.handleRollback)
	s.mux.HandleFunc("/api/snapshots/diff", s.handleDiff)
	s.mux.HandleFunc("/api/exec", s.handleExec)
	s.mux.HandleFunc("/api/exec/verify", s.handleExecVerify)
	s.mux.HandleFunc("/api/policy", s.handlePolicy)
	s.mux.HandleFunc("/api/audit/report", s.handleAuditReport)
	s.mux.HandleFunc("/api/firewall/rules", s.handleFirewallRules)
	s.mux.HandleFunc("/api/firewall/alerts", s.handleAlerts)
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Enable CORS for local Vite studio
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	active, idle := s.executor.Pool().Stats()
	totalAlerts, blocked := s.firewall.AlertHub().Stats()
	snapshots, _ := s.executor.Snapshotter().List(ctx)

	policyCfg := types.PolicyConfig{}
	if s.executor.Policy() != nil {
		policyCfg = s.executor.Policy().Config()
	}

	status := types.SandboxStatus{
		State:            "ready",
		Mode:             s.executor.Pool().ActiveRuntime(ctx).Mode(),
		WorkspacePath:    s.workspace,
		ActiveContainers: active,
		IdleContainers:   idle,
		TotalSnapshots:   len(snapshots),
		TotalAlerts:      totalAlerts,
		BlockedAttempts:  blocked,
		Firewall:         s.firewall.Config(),
		Policy:           policyCfg,
		ConnectedClients: make([]types.ConnectedClient, 0),
		UptimeSeconds:    int64(time.Since(s.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		list, err := s.executor.Snapshotter().List(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var req struct {
			Description string `json:"description"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		snap, err := s.executor.Snapshotter().Create(ctx, "manual_studio_snapshot", req.Description)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.hub.BroadcastEvent("snapshot", snap)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(snap)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SnapshotID == "" {
		http.Error(w, "missing or invalid snapshot_id", http.StatusBadRequest)
		return
	}

	if err := s.executor.Snapshotter().Rollback(r.Context(), req.SnapshotID); err != nil {
		http.Error(w, fmt.Sprintf("rollback failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.hub.BroadcastEvent("rollback", map[string]string{"snapshot_id": req.SnapshotID})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"snapshot_id": req.SnapshotID,
		"message":     "Rollback completed successfully",
	})
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		http.Error(w, "missing 'from' query parameter", http.StatusBadRequest)
		return
	}
	if to == "" {
		to = "working"
	}

	diff, err := s.executor.Snapshotter().Diff(r.Context(), from, to)
	if err != nil {
		http.Error(w, fmt.Sprintf("diff failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(diff)
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command        string `json:"command"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Command) == "" {
		http.Error(w, "missing or invalid 'command'", http.StatusBadRequest)
		return
	}

	timeout := 60 * time.Second
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	var env []string
	if s.proxy != nil {
		env = append(env, s.proxy.EnvVars()...)
	}

	res, err := s.executor.Run(r.Context(), req.Command, container.ExecOptions{
		Timeout:    timeout,
		WorkingDir: s.workspace,
		Env:        env,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.hub.BroadcastEvent("execution", res)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleExecVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command        string `json:"command"`
		VerifyCommand  string `json:"verify_command"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Command) == "" || strings.TrimSpace(req.VerifyCommand) == "" {
		http.Error(w, "missing required fields 'command' and 'verify_command'", http.StatusBadRequest)
		return
	}

	timeout := 60 * time.Second
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	var env []string
	if s.proxy != nil {
		env = append(env, s.proxy.EnvVars()...)
	}

	res, err := s.executor.RunWithVerify(r.Context(), req.Command, req.VerifyCommand, container.ExecOptions{
		Timeout:    timeout,
		WorkingDir: s.workspace,
		Env:        env,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("verified execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.hub.BroadcastEvent("execution", res)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handlePolicy(w http.ResponseWriter, r *http.Request) {
	if s.executor.Policy() == nil {
		http.Error(w, "Policy engine not initialized", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.executor.Policy().Config())

	case http.MethodPost:
		var cfg types.PolicyConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid policy json", http.StatusBadRequest)
			return
		}
		s.executor.Policy().UpdateConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.executor.Policy().Config())

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAuditReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	active, idle := s.executor.Pool().Stats()
	totalAlerts, blocked := s.firewall.AlertHub().Stats()
	snapshots, _ := s.executor.Snapshotter().List(ctx)
	alerts := s.firewall.AlertHub().List(100)

	policyCfg := types.PolicyConfig{}
	if s.executor.Policy() != nil {
		policyCfg = s.executor.Policy().Config()
	}

	status := types.SandboxStatus{
		State:            "ready",
		Mode:             s.executor.Pool().ActiveRuntime(ctx).Mode(),
		WorkspacePath:    s.workspace,
		ActiveContainers: active,
		IdleContainers:   idle,
		TotalSnapshots:   len(snapshots),
		TotalAlerts:      totalAlerts,
		BlockedAttempts:  blocked,
		Firewall:         s.firewall.Config(),
		Policy:           policyCfg,
		UptimeSeconds:    int64(time.Since(s.startTime).Seconds()),
	}

	format := r.URL.Query().Get("format")
	switch format {
	case "md", "markdown":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(audit.GenerateMarkdownReport(status, alerts, snapshots)))

	case "json":
		data, _ := audit.GenerateJSONReport(status, alerts, snapshots)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)

	default: // html default
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(audit.GenerateHTMLReport(status, alerts, snapshots)))
	}
}

func (s *Server) handleFirewallRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.firewall.Config())

	case http.MethodPost:
		var req struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Domain) == "" {
			http.Error(w, "missing domain", http.StatusBadRequest)
			return
		}
		s.firewall.AddAllowedDomain(req.Domain)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.firewall.Config())

	case http.MethodDelete:
		var req struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Domain) == "" {
			http.Error(w, "missing domain", http.StatusBadRequest)
			return
		}
		s.firewall.RemoveAllowedDomain(req.Domain)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.firewall.Config())

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	alerts := s.firewall.AlertHub().List(limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(alerts)
}

// Close gracefully releases server resources.
func (s *Server) Close() {
	if s.alertCancel != nil {
		s.alertCancel()
	}
	s.hub.Close()
}
