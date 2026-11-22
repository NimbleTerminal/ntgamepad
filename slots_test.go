// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import "testing"

// A controller takes the lowest free slot and keeps it; a released slot is
// the next one handed out; a full table refuses.
func TestSlotTable(t *testing.T) {
	var tab slotTable
	a, b, c := slotKey{1, 10}, slotKey{1, 11}, slotKey{2, 10}
	for want, k := range []slotKey{a, b, c} {
		if got, ok := tab.claim(k); !ok || got != want {
			t.Fatalf("claim %v = %d %v, want %d", k, got, ok, want)
		}
	}
	if got, _ := tab.claim(b); got != 1 {
		t.Fatalf("a second claim moved the controller to slot %d", got)
	}
	tab.release(a)
	tab.release(slotKey{9, 9}) // not held: no effect
	if got, _ := tab.claim(slotKey{3, 1}); got != 0 {
		t.Fatalf("the freed slot was not reused: got %d", got)
	}
	if got, _ := tab.claim(c); got != 2 {
		t.Fatalf("controller c moved to slot %d", got)
	}
	tab.claim(slotKey{3, 2})
	if _, ok := tab.claim(slotKey{3, 3}); ok {
		t.Fatalf("a fifth controller was given a slot of %d", MaxPads)
	}
}
