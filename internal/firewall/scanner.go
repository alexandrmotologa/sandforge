package firewall

import (
	"regexp"
	"strings"
)

type credentialPattern struct {
	Name    string
	Regex   *regexp.Regexp
	MaskLen int
}

var credentialPatterns = []credentialPattern{
	{
		Name:    "Anthropic API Key",
		Regex:   regexp.MustCompile(`sk-ant-[a-zA-Z0-9_\-]{20,}`),
		MaskLen: 12,
	},
	{
		Name:    "OpenAI API Key",
		Regex:   regexp.MustCompile(`sk-(?:proj-)?[a-zA-Z0-9_\-]{24,}`),
		MaskLen: 10,
	},
	{
		Name:    "GitHub Personal Access Token",
		Regex:   regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9_]{22,}`),
		MaskLen: 8,
	},
	{
		Name:    "SSH / RSA Private Key",
		Regex:   regexp.MustCompile(`-----BEGIN (?:RSA|EC|DSA|OPENSSH) PRIVATE KEY-----`),
		MaskLen: 15,
	},
	{
		Name:    "AWS Access Key ID",
		Regex:   regexp.MustCompile(`(?:AKIA|ASIA)[0-9A-Z]{16}`),
		MaskLen: 8,
	},
	{
		Name:    "Slack Token",
		Regex:   regexp.MustCompile(`xox[baprs]-[0-9a-zA-Z]{10,}`),
		MaskLen: 8,
	},
	{
		Name:    "Stripe Secret Key",
		Regex:   regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24,}`),
		MaskLen: 10,
	},
}

// ScanPayload scans an HTTP body, header, or URL for leaked credentials.
func ScanPayload(content string) (bool, string, string) {
	if strings.TrimSpace(content) == "" {
		return false, "", ""
	}

	for _, p := range credentialPatterns {
		loc := p.Regex.FindStringIndex(content)
		if loc != nil {
			match := content[loc[0]:loc[1]]
			masked := match
			if len(match) > p.MaskLen {
				masked = match[:p.MaskLen] + "..."
			}
			return true, p.Name, masked
		}
	}

	return false, "", ""
}
