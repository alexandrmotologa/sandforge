package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallConfig configures SandForge into the designated AI coding client configuration file.
func InstallConfig(clientType string, workspaceDir string, binaryPath string) (string, error) {
	absWorkspace, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolving workspace path: %w", err)
	}

	if binaryPath == "" {
		binaryPath = "sandforge"
	}

	serverConfig := map[string]interface{}{
		"command": binaryPath,
		"args":    []string{"mcp", "--workspace", absWorkspace},
	}

	var targetPath string
	clientNormalized := strings.ToLower(strings.TrimSpace(clientType))

	home, _ := os.UserHomeDir()

	switch clientNormalized {
	case "claude":
		targetPath = filepath.Join(home, ".claude", "config.json")
		if _, err := os.Stat(filepath.Join(absWorkspace, "claude.json")); err == nil {
			targetPath = filepath.Join(absWorkspace, "claude.json")
		}

	case "cursor":
		targetPath = filepath.Join(absWorkspace, ".cursor", "mcp.json")

	case "windsurf":
		targetPath = filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")

	default:
		return "", fmt.Errorf("unsupported client '%s', supported: claude, cursor, windsurf", clientType)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("creating directory for config: %w", err)
	}

	var data map[string]interface{}
	if content, err := os.ReadFile(targetPath); err == nil && len(content) > 0 {
		_ = json.Unmarshal(content, &data)
	}

	if data == nil {
		data = make(map[string]interface{})
	}

	mcpServers, ok := data["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		data["mcpServers"] = mcpServers
	}

	mcpServers["sandforge"] = serverConfig

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializing config: %w", err)
	}

	if err := os.WriteFile(targetPath, bytes, 0644); err != nil {
		return "", fmt.Errorf("writing config file %s: %w", targetPath, err)
	}

	return targetPath, nil
}
