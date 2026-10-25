package firewall

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// AlertHub stores network security events and notifies subscribers.
type AlertHub struct {
	mu          sync.RWMutex
	alerts      []*types.NetworkAlert
	subscribers map[chan *types.NetworkAlert]struct{}
	maxAlerts   int
}

// NewAlertHub creates a new alert manager.
func NewAlertHub(maxAlerts int) *AlertHub {
	if maxAlerts <= 0 {
		maxAlerts = 1000
	}
	return &AlertHub{
		alerts:      make([]*types.NetworkAlert, 0),
		subscribers: make(map[chan *types.NetworkAlert]struct{}),
		maxAlerts:   maxAlerts,
	}
}

func (h *AlertHub) generateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("alt-%s", hex.EncodeToString(b))
}

// Record appends an alert and broadcasts to active subscribers.
func (h *AlertHub) Record(process, host string, port int, protocol, action, reason, snippet, severity string) *types.NetworkAlert {
	alert := &types.NetworkAlert{
		ID:             h.generateID(),
		Timestamp:      time.Now(),
		Process:        process,
		TargetHost:     host,
		TargetPort:     port,
		Protocol:       protocol,
		Action:         action,
		Reason:         reason,
		PayloadSnippet: snippet,
		Severity:       severity,
	}

	h.mu.Lock()
	if len(h.alerts) >= h.maxAlerts {
		h.alerts = h.alerts[1:]
	}
	h.alerts = append(h.alerts, alert)

	// Broadcast non-blocking
	for ch := range h.subscribers {
		select {
		case ch <- alert:
		default:
		}
	}
	h.mu.Unlock()

	return alert
}

// List returns recorded alerts, newest first.
func (h *AlertHub) List(limit int) []*types.NetworkAlert {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := len(h.alerts)
	if limit <= 0 || limit > total {
		limit = total
	}

	res := make([]*types.NetworkAlert, limit)
	for i := 0; i < limit; i++ {
		res[i] = h.alerts[total-1-i]
	}
	return res
}

// Subscribe returns a channel receiving newly recorded alerts.
func (h *AlertHub) Subscribe() chan *types.NetworkAlert {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan *types.NetworkAlert, 64)
	h.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a listener.
func (h *AlertHub) Unsubscribe(ch chan *types.NetworkAlert) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.subscribers, ch)
	close(ch)
}

// Stats returns the total and blocked alert counts.
func (h *AlertHub) Stats() (total int, blocked int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	blockedCount := 0
	for _, a := range h.alerts {
		if a.Action == "BLOCK" {
			blockedCount++
		}
	}
	return len(h.alerts), blockedCount
}
