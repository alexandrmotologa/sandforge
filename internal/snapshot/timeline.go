package snapshot

import (
	"sync"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// Timeline manages the history tree of checkpoints.
type Timeline struct {
	mu           sync.RWMutex
	snapshots    []*types.Snapshot
	byID         map[string]*types.Snapshot
	currentID    string
}

// NewTimeline creates an empty snapshot timeline.
func NewTimeline() *Timeline {
	return &Timeline{
		snapshots: make([]*types.Snapshot, 0),
		byID:      make(map[string]*types.Snapshot),
	}
}

// Add appends a new snapshot and updates the active pointer.
func (t *Timeline) Add(s *types.Snapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if s.Timestamp.IsZero() {
		s.Timestamp = time.Now()
	}
	if s.Status == "" {
		s.Status = "clean"
	}
	s.ParentID = t.currentID
	t.currentID = s.ID

	t.snapshots = append(t.snapshots, s)
	t.byID[s.ID] = s
}

// Get finds a snapshot by its ID.
func (t *Timeline) Get(id string) (*types.Snapshot, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.byID[id]
	return s, ok
}

// List returns a copy of all snapshots in chronological order.
func (t *Timeline) List() []*types.Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*types.Snapshot, len(t.snapshots))
	copy(result, t.snapshots)
	return result
}

// CurrentID returns the active snapshot ID.
func (t *Timeline) CurrentID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentID
}

// SetCurrentID updates the active snapshot pointer after rollback.
func (t *Timeline) SetCurrentID(id string, markStatus string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.currentID = id
	if markStatus != "" {
		if s, ok := t.byID[id]; ok {
			s.Status = markStatus
		}
	}
}

// MarkStatus updates the status flag of a snapshot (clean, mutated, reverted).
func (t *Timeline) MarkStatus(id string, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if s, ok := t.byID[id]; ok {
		s.Status = status
	}
}
