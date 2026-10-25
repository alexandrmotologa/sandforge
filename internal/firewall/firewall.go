package firewall

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// DefaultAllowedDomains provides trusted package registries and public developer infrastructure.
var DefaultAllowedDomains = []string{
	"registry.npmjs.org",
	"registry.yarnpkg.com",
	"pypi.org",
	"files.pythonhosted.org",
	"proxy.golang.org",
	"sum.golang.org",
	"github.com",
	"api.github.com",
	"raw.githubusercontent.com",
	"crates.io",
	"static.crates.io",
	"index.crates.io",
	"repo.maven.apache.org",
	"api.anthropic.com",
	"api.openai.com",
}

// DefaultBlockedCIDRs contains private and metadata network ranges.
var DefaultBlockedCIDRs = []string{
	"169.254.169.254/32", // AWS/GCP/Azure instance metadata
	"10.0.0.0/8",         // Private RFC1918
	"172.16.0.0/12",       // Private RFC1918
	"192.168.0.0/16",     // Private RFC1918
	"100.64.0.0/10",      // Carrier grade NAT
}

// Controller evaluates and enforces outbound connection policy.
type Controller struct {
	mu             sync.RWMutex
	allowedDomains map[string]struct{}
	blockedNets    []*net.IPNet
	alertHub       *AlertHub
	honeyVault     *HoneyVault
	strictMode     bool
	deepPacketScan bool
}

// NewController initializes the firewall with default security policy.
func NewController(alertHub *AlertHub) *Controller {
	c := &Controller{
		allowedDomains: make(map[string]struct{}),
		alertHub:       alertHub,
		honeyVault:     NewHoneyVault(),
		strictMode:     true,
		deepPacketScan: true,
	}

	for _, domain := range DefaultAllowedDomains {
		c.allowedDomains[strings.ToLower(domain)] = struct{}{}
	}

	for _, cidr := range DefaultBlockedCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			c.blockedNets = append(c.blockedNets, ipNet)
		}
	}

	return c
}

// AddAllowedDomain adds a domain to the allowlist.
func (c *Controller) AddAllowedDomain(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.allowedDomains[strings.ToLower(strings.TrimSpace(domain))] = struct{}{}
}

// RemoveAllowedDomain removes a domain from the allowlist.
func (c *Controller) RemoveAllowedDomain(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.allowedDomains, strings.ToLower(strings.TrimSpace(domain)))
}

// AllowedDomains returns the active allowlist.
func (c *Controller) AllowedDomains() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	domains := make([]string, 0, len(c.allowedDomains))
	for d := range c.allowedDomains {
		domains = append(domains, d)
	}
	return domains
}

// IsAllowed evaluates whether an outbound destination host is permitted.
func (c *Controller) IsAllowed(target string) (bool, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	host := strings.ToLower(strings.TrimSpace(target))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// 1. IP check
	if ip := net.ParseIP(host); ip != nil {
		for _, blockedNet := range c.blockedNets {
			if blockedNet.Contains(ip) {
				return false, fmt.Sprintf("IP %s falls into blocked subnet %s (cloud metadata or private subnet)", ip, blockedNet)
			}
		}

		if c.strictMode {
			return false, fmt.Sprintf("Direct outbound IP connections (%s) are forbidden in strict mode", host)
		}
	}

	// 2. Exact domain match
	if _, ok := c.allowedDomains[host]; ok {
		return true, "Domain is in allowlist"
	}

	// 3. Subdomain match (e.g. pkg.github.com matches github.com)
	for allowed := range c.allowedDomains {
		if strings.HasSuffix(host, "."+allowed) {
			return true, fmt.Sprintf("Subdomain matches allowed base domain %s", allowed)
		}
	}

	return false, fmt.Sprintf("Domain '%s' is not in allowed domain list", host)
}

// CheckAndRecord evaluates a request, records an alert, and returns the decision.
func (c *Controller) CheckAndRecord(process string, host string, port int, protocol string, snippet string) (bool, *types.NetworkAlert) {
	allowed, reason := c.IsAllowed(host)

	action := "ALLOW"
	severity := "info"
	if !allowed {
		action = "BLOCK"
		severity = "critical"
	}

	alert := c.alertHub.Record(process, host, port, protocol, action, reason, snippet, severity)
	return allowed, alert
}

// HoneyVault returns the honeytoken tripwire vault.
func (c *Controller) HoneyVault() *HoneyVault {
	return c.honeyVault
}

// Config returns the current firewall configuration.
func (c *Controller) Config() types.FirewallConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return types.FirewallConfig{
		AllowedDomains:    c.AllowedDomains(),
		BlockedIPRanges:   DefaultBlockedCIDRs,
		InterceptDNS:      true,
		StrictMode:        c.strictMode,
		DeepPacketScan:    c.deepPacketScan,
		HoneytokensActive: c.honeyVault.Count(),
	}
}

// AlertHub returns the underlying event bus.
func (c *Controller) AlertHub() *AlertHub {
	return c.alertHub
}
