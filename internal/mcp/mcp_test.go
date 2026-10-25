package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/container"
	"github.com/alexandrmotologa/sandforge/internal/firewall"
	"github.com/alexandrmotologa/sandforge/internal/snapshot"
)

func TestMCPServerLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sandforge-mcp-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gc, err := snapshot.NewGitCheckpoint(tempDir)
	if err != nil {
		t.Fatalf("git checkpoint error: %v", err)
	}

	processRt := container.NewProcessRuntime(tempDir)
	pool := container.NewPool(nil, processRt, 2)
	executor := container.NewExecutor(pool, gc, nil)

	hub := firewall.NewAlertHub(50)
	fw := firewall.NewController(hub)
	handler := NewToolHandler(executor, fw, nil, tempDir)

	// Test 1: Initialize
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	// Test 2: List tools
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	// Test 3: Call sandforge_run tool
	runReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"sandforge_run","arguments":{"command":"echo mcp_works"}}}` + "\n"

	input := bytes.NewBufferString(initReq + listReq + runReq)
	var output bytes.Buffer

	server := NewServer(handler, input, &output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Serve(ctx); err != nil {
		t.Fatalf("server.Serve error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 response lines, got: %d (%s)", len(lines), output.String())
	}

	var initResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("unmarshal initResp: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("unexpected error in init: %v", initResp.Error)
	}

	var listResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatalf("unmarshal listResp: %v", err)
	}
	resultMap, ok := listResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result in listResp")
	}
	tools, ok := resultMap["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		t.Fatalf("expected tools list to have items")
	}
}

