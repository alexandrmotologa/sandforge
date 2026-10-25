package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alexandrmotologa/sandforge/internal/container"
	"github.com/alexandrmotologa/sandforge/internal/firewall"
	"github.com/alexandrmotologa/sandforge/internal/snapshot"
	"github.com/alexandrmotologa/sandforge/internal/types"
)

func setupTestServer(t *testing.T) (*Server, string, func()) {
	tempDir, err := os.MkdirTemp("", "sandforge-server-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}

	gc, err := snapshot.NewGitCheckpoint(tempDir)
	if err != nil {
		t.Fatalf("git checkpoint error: %v", err)
	}

	processRt := container.NewProcessRuntime(tempDir)
	pool := container.NewPool(nil, processRt, 2)
	executor := container.NewExecutor(pool, gc)

	hub := firewall.NewAlertHub(100)
	fw := firewall.NewController(hub)

	srv := NewServer(executor, fw, nil, tempDir)

	cleanup := func() {
		srv.Close()
		_ = os.RemoveAll(tempDir)
	}

	return srv, tempDir, cleanup
}

func TestServerStatusEndpoint(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", w.Code)
	}

	var status types.SandboxStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode status: %v", err)
	}

	if status.State != "ready" {
		t.Fatalf("expected state ready, got: %s", status.State)
	}
}

func TestServerExecAndRollback(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Run command via /api/exec
	execBody := []byte(`{"command":"echo test_api > api_out.txt"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/exec", bytes.NewReader(execBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected exec status 200, got: %d (%s)", w.Code, w.Body.String())
	}

	var execRes types.ExecutionResult
	if err := json.NewDecoder(w.Body).Decode(&execRes); err != nil {
		t.Fatalf("failed to decode exec result: %v", err)
	}
	if execRes.SnapshotID == "" {
		t.Fatalf("expected snapshot ID from exec")
	}

	// 2. Rollback via /api/snapshots/rollback
	rollbackBody := []byte(`{"snapshot_id":"` + execRes.SnapshotID + `"}`)
	rbReq := httptest.NewRequest(http.MethodPost, "/api/snapshots/rollback", bytes.NewReader(rollbackBody))
	rbReq.Header.Set("Content-Type", "application/json")
	rbW := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rbW, rbReq)

	if rbW.Code != http.StatusOK {
		t.Fatalf("expected rollback status 200, got: %d (%s)", rbW.Code, rbW.Body.String())
	}
}

func TestServerFirewallRules(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Add domain
	addBody := []byte(`{"domain":"custom-registry.internal"}`)
	addReq := httptest.NewRequest(http.MethodPost, "/api/firewall/rules", bytes.NewReader(addBody))
	addW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(addW, addReq)

	if addW.Code != http.StatusOK {
		t.Fatalf("expected add rule status 200, got: %d", addW.Code)
	}

	// Delete domain
	delReq := httptest.NewRequest(http.MethodDelete, "/api/firewall/rules", bytes.NewReader(addBody))
	delW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected delete rule status 200, got: %d", delW.Code)
	}
}
