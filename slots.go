// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import "sync"

// slotKey names one controller to a slotTable: which driver found it and
// that driver's own identifier for it.
type slotKey struct {
	kind uint8
	id   uint64
}

// slotTable hands out the State.Index of each controller: the lowest free
// slot when it is first seen, kept until it is released. It is safe for
// several drivers to use at once.
type slotTable struct {
	mu   sync.Mutex
	keys [MaxPads]slotKey
	used [MaxPads]bool
}

// claim returns the controller's slot, taking the lowest free one if it has
// none. It reports false when every slot is taken.
func (t *slotTable) claim(k slotKey) (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	free := -1
	for i := range t.keys {
		if t.used[i] && t.keys[i] == k {
			return i, true
		}
		if !t.used[i] && free < 0 {
			free = i
		}
	}
	if free < 0 {
		return 0, false
	}
	t.keys[free], t.used[free] = k, true
	return free, true
}

// release frees the controller's slot, if it holds one.
func (t *slotTable) release(k slotKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.keys {
		if t.used[i] && t.keys[i] == k {
			t.used[i] = false
		}
	}
}
