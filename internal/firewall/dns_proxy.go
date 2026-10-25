package firewall

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// DNSProxy intercepts DNS queries and filters them against the domain allowlist.
type DNSProxy struct {
	controller *Controller
	conn       net.PacketConn
	listenAddr string
	mu         sync.Mutex
	closed     bool
}

// NewDNSProxy creates a DNS interceptor attached to the firewall controller.
func NewDNSProxy(controller *Controller) *DNSProxy {
	return &DNSProxy{
		controller: controller,
	}
}

// Start binds a UDP listener on a loopback port.
func (d *DNSProxy) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("binding dns listener: %w", err)
	}
	d.conn = conn
	d.listenAddr = conn.LocalAddr().String()

	go d.serve()
	return nil
}

// Addr returns the DNS listening address.
func (d *DNSProxy) Addr() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.listenAddr
}

// Close stops the DNS proxy.
func (d *DNSProxy) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.closed = true
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

func (d *DNSProxy) serve() {
	buf := make([]byte, 512)
	for {
		n, remoteAddr, err := d.conn.ReadFrom(buf)
		if err != nil {
			d.mu.Lock()
			isClosed := d.closed
			d.mu.Unlock()
			if isClosed {
				return
			}
			continue
		}

		go d.handleQuery(buf[:n], remoteAddr)
	}
}

func (d *DNSProxy) handleQuery(packet []byte, remoteAddr net.Addr) {
	domain := extractQueryDomain(packet)
	if domain == "" {
		return
	}

	allowed, _ := d.controller.CheckAndRecord("dns-query", domain, 53, "dns", "DNS Lookup")

	if !allowed {
		// Respond with 0.0.0.0 (Sinkhole blocked resolution)
		sinkhole := buildBlockedResponse(packet)
		if len(sinkhole) > 0 {
			_, _ = d.conn.WriteTo(sinkhole, remoteAddr)
		}
		return
	}

	// Forward allowed query to upstream DNS resolver (e.g. 1.1.1.1:53)
	upstream, err := net.DialTimeout("udp", "1.1.1.1:53", 2*time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()

	_ = upstream.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := upstream.Write(packet); err != nil {
		return
	}

	resp := make([]byte, 512)
	rn, err := upstream.Read(resp)
	if err == nil && rn > 0 {
		_, _ = d.conn.WriteTo(resp[:rn], remoteAddr)
	}
}

// extractQueryDomain extracts the query domain name from a raw standard DNS packet.
func extractQueryDomain(packet []byte) string {
	if len(packet) < 13 {
		return ""
	}

	pos := 12
	var parts []string
	for pos < len(packet) {
		length := int(packet[pos])
		if length == 0 {
			break
		}
		pos++
		if pos+length > len(packet) {
			return ""
		}
		parts = append(parts, string(packet[pos:pos+length]))
		pos += length
	}

	return strings.Join(parts, ".")
}

// buildBlockedResponse constructs a minimal synthetic DNS response returning 0.0.0.0 (A record).
func buildBlockedResponse(query []byte) []byte {
	if len(query) < 12 {
		return nil
	}

	resp := make([]byte, len(query)+16)
	copy(resp, query)

	// Set flags: standard response (QR=1, AA=1, RA=1, RCODE=0)
	resp[2] = 0x81
	resp[3] = 0x80

	// Set Answer Count = 1
	resp[6] = 0x00
	resp[7] = 0x01

	// Append synthetic A answer pointing to query name (offset 0x0c)
	idx := len(query)
	resp[idx] = 0xc0
	resp[idx+1] = 0x0c
	resp[idx+2] = 0x00 // Type A
	resp[idx+3] = 0x01
	resp[idx+4] = 0x00 // Class IN
	resp[idx+5] = 0x01
	resp[idx+6] = 0x00 // TTL = 10s
	resp[idx+7] = 0x00
	resp[idx+8] = 0x00
	resp[idx+9] = 0x0a
	resp[idx+10] = 0x00 // Data length = 4 bytes
	resp[idx+11] = 0x04
	resp[idx+12] = 0x00 // IP 0.0.0.0
	resp[idx+13] = 0x00
	resp[idx+14] = 0x00
	resp[idx+15] = 0x00

	return resp
}
