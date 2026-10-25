package firewall

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Honeytoken represents an active synthetic decoy credential used as an intrusion tripwire.
type Honeytoken struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// HoneyVault stores and monitors decoy tripwire tokens.
type HoneyVault struct {
	mu     sync.RWMutex
	tokens map[string]Honeytoken
}

// NewHoneyVault initializes a tripwire vault populated with default decoy credentials.
func NewHoneyVault() *HoneyVault {
	v := &HoneyVault{
		tokens: make(map[string]Honeytoken),
	}

	// Seed default synthetic honeytokens
	v.addDefaultTokens()
	return v
}

func (v *HoneyVault) addDefaultTokens() {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	suffix := hex.EncodeToString(b)

	v.tokens["OPENAI"] = Honeytoken{
		Name:        "Decoy OpenAI API Key",
		Value:       fmt.Sprintf("sk-proj-sandforge-decoy-%s", suffix),
		Description: "Tripwire for unauthorized .env exfiltration",
	}
	v.tokens["STRIPE"] = Honeytoken{
		Name:        "Decoy Stripe Secret Key",
		Value:       fmt.Sprintf("sk_live_sandforge_trap_%s", suffix),
		Description: "Tripwire for billing credential exfiltration",
	}
	v.tokens["AWS"] = Honeytoken{
		Name:        "Decoy AWS Access Key",
		Value:       fmt.Sprintf("AKIA_SANDFORGE_TRAP_%s", strings.ToUpper(suffix[:12])),
		Description: "Tripwire for cloud infrastructure credential exfiltration",
	}
}

// Check inspects an outbound payload or URL to see if it contains any tripwire honeytoken.
func (v *HoneyVault) Check(content string) (bool, string, string) {
	if strings.TrimSpace(content) == "" {
		return false, "", ""
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, token := range v.tokens {
		if strings.Contains(content, token.Value) {
			masked := token.Value
			if len(masked) > 12 {
				masked = masked[:12] + "..."
			}
			return true, token.Name, masked
		}
	}

	return false, "", ""
}

// Count returns the number of active honeytokens.
func (v *HoneyVault) Count() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.tokens)
}

// EnvStrings returns decoy environment variables formatted as KEY=VAL.
func (v *HoneyVault) EnvStrings() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var envs []string
	for k, t := range v.tokens {
		envs = append(envs, fmt.Sprintf("SANDFORGE_HONEY_%s=%s", k, t.Value))
	}
	return envs
}

// WriteDecoyEnv writes a .env.decoy file to a workspace directory for honeytoken protection.
func (v *HoneyVault) WriteDecoyEnv(workspaceDir string) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("# SandForge Intrusion Detection Decoy Secrets\n")
	sb.WriteString("# Transmitting any of these keys outside the workspace triggers an immediate alert.\n")
	for k, t := range v.tokens {
		sb.WriteString(fmt.Sprintf("%s_API_KEY=%s\n", k, t.Value))
	}

	target := filepath.Join(workspaceDir, ".env.decoy")
	return os.WriteFile(target, []byte(sb.String()), 0600)
}
