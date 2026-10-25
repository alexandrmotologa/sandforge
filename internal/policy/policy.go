package policy

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

type heuristicRule struct {
	Name        string
	Pattern     *regexp.Regexp
	Explanation string
	Category    string
}

// Engine evaluates shell commands against destructive heuristic policies.
type Engine struct {
	mu     sync.RWMutex
	config types.PolicyConfig
	rules  []heuristicRule
}

// NewEngine initializes the policy engine with standard protection rules.
func NewEngine() *Engine {
	e := &Engine{
		config: types.PolicyConfig{
			BlockRootDeletions: true,
			BlockForceGitPush:  true,
			BlockForkBombs:     true,
			BlockSystemHalt:    true,
			BlockedPatterns:    make([]string, 0),
		},
	}
	e.rebuildRules()
	return e
}

func (e *Engine) rebuildRules() {
	var rules []heuristicRule

	if e.config.BlockRootDeletions {
		rules = append(rules,
			heuristicRule{
				Name:        "Root Filesystem Deletion",
				Pattern:     regexp.MustCompile(`(?i)\brm\s+-[a-z]*r[a-z]*f[a-z]*\s+(?:/|/\*|\.\./|~|c:\\)`),
				Explanation: "Attempts to delete root, parent, or system drive filesystem",
				Category:    "filesystem",
			},
			heuristicRule{
				Name:        "Windows System Drive Wipe",
				Pattern:     regexp.MustCompile(`(?i)\brmdir\s+/[sS]\s+/[qQ]\s+[cC]:\\`),
				Explanation: "Attempts to delete entire C: drive volume",
				Category:    "filesystem",
			},
		)
	}

	if e.config.BlockForceGitPush {
		rules = append(rules, heuristicRule{
			Name:        "Destructive Git Force Push",
			Pattern:     regexp.MustCompile(`(?i)\bgit\s+push\s+(?:.*--force\b|.*-f\b)`),
			Explanation: "Force pushing may overwrite or destroy remote repository branches",
			Category:    "git",
		})
	}

	if e.config.BlockForkBombs {
		rules = append(rules, heuristicRule{
			Name:        "Fork Bomb / Denial of Service",
			Pattern:     regexp.MustCompile(`(?i):\(\)\s*\{\s*:\|:&\s*\};:|while\s+true\s*;\s*do\s+.*&\s*done`),
			Explanation: "Denial of service loop designed to exhaust system process table",
			Category:    "dos",
		})
	}

	if e.config.BlockSystemHalt {
		rules = append(rules, heuristicRule{
			Name:        "System Shutdown / Reboot",
			Pattern:     regexp.MustCompile(`(?i)\b(?:shutdown\s+-[sShHrR]|reboot\b|init\s+0\b|poweroff\b)`),
			Explanation: "Commands attempting to power off or reboot host machine",
			Category:    "system",
		})
	}

	// Add custom patterns from config
	for _, pat := range e.config.BlockedPatterns {
		if re, err := regexp.Compile(`(?i)` + pat); err == nil {
			rules = append(rules, heuristicRule{
				Name:        "Custom Policy Rule",
				Pattern:     re,
				Explanation: fmt.Sprintf("Command matched custom pattern '%s'", pat),
				Category:    "custom",
			})
		}
	}

	e.rules = rules
}

// Check evaluates a command against the active policy set.
func (e *Engine) Check(cmdStr string) (bool, string) {
	trimmed := strings.TrimSpace(cmdStr)
	if trimmed == "" {
		return true, ""
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if rule.Pattern.MatchString(trimmed) {
			return false, fmt.Sprintf("%s: %s", rule.Name, rule.Explanation)
		}
	}

	return true, ""
}

// Config returns the current configuration.
func (e *Engine) Config() types.PolicyConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// UpdateConfig updates the policy configuration and rebuilds the rule matcher.
func (e *Engine) UpdateConfig(cfg types.PolicyConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = cfg
	e.rebuildRules()
}
