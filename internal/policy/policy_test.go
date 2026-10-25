package policy

import (
	"testing"
)

func TestPolicyEngineDestructiveCommands(t *testing.T) {
	engine := NewEngine()

	// 1. Root deletion
	allowed, reason := engine.Check("rm -rf /")
	if allowed {
		t.Fatalf("expected 'rm -rf /' to be blocked")
	}
	if reason == "" {
		t.Fatalf("expected reason for block")
	}

	// 2. Git force push
	allowed, _ = engine.Check("git push origin main --force")
	if allowed {
		t.Fatalf("expected 'git push --force' to be blocked")
	}

	// 3. Fork bomb
	allowed, _ = engine.Check(":(){ :|:& };:")
	if allowed {
		t.Fatalf("expected fork bomb to be blocked")
	}

	// 4. Shutdown command
	allowed, _ = engine.Check("shutdown -h now")
	if allowed {
		t.Fatalf("expected shutdown to be blocked")
	}

	// 5. Safe commands should be allowed
	safeCommands := []string{
		"npm test",
		"git status",
		"go build -v ./...",
		"echo hello world",
		"rm -rf build_temp",
	}

	for _, cmd := range safeCommands {
		allowed, r := engine.Check(cmd)
		if !allowed {
			t.Fatalf("expected safe command '%s' to be allowed, blocked because: %s", cmd, r)
		}
	}
}

func TestPolicyEngineCustomPatterns(t *testing.T) {
	engine := NewEngine()

	cfg := engine.Config()
	cfg.BlockedPatterns = append(cfg.BlockedPatterns, `\bdrop\s+database\b`)
	engine.UpdateConfig(cfg)

	allowed, _ := engine.Check("psql -c 'DROP DATABASE production'")
	if allowed {
		t.Fatalf("expected custom pattern 'DROP DATABASE' to be blocked")
	}

	allowed, _ = engine.Check("SELECT * FROM users")
	if !allowed {
		t.Fatalf("expected select query to be allowed")
	}
}
