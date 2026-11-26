// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

// A DualSense can appear as both a framework controller and a HID device.
// Each transport has a stable identifier, but neither exposes the other's.
type dualSenseSample struct {
	key   slotKey
	state State
}

type dualSenseSlot struct {
	owner   slotKey // the original claim, kept across transport changes
	gc, hid slotKey
}

type dualSenseSlots [MaxPads]dualSenseSlot

// assign preserves known identities, joins a single remaining pair, and
// keeps using the established transport when several pairings are possible.
// Both sample slices are scratch space owned by the current poll.
func (slots *dualSenseSlots) assign(table *slotTable, gc, hid []dualSenseSample, preferGC bool, dst []State) []State {
	for i, slot := range slots {
		if slot.owner == (slotKey{}) {
			continue
		}
		g, h := sampleFor(gc, slot.gc), sampleFor(hid, slot.hid)
		if g == nil && h == nil {
			table.release(slot.owner)
			slots[i] = dualSenseSlot{}
			continue
		}
		if g != nil {
			g.state.Index = i
		}
		if h != nil {
			h.state.Index = i
		}
	}

	// Remove known pairs from consideration. One GC and one HID sample left
	// can share a slot. Order and the display name cannot identify a pair.
	var gbuf, hbuf [MaxPads]*dualSenseSample
	pendingGC, pendingHID := gbuf[:0], hbuf[:0]
	for i := range gc {
		g := &gc[i]
		if g.state.Index < 0 || slots[g.state.Index].hid == (slotKey{}) {
			pendingGC = append(pendingGC, g)
		}
	}
	for i := range hid {
		h := &hid[i]
		if h.state.Index < 0 || slots[h.state.Index].gc == (slotKey{}) {
			pendingHID = append(pendingHID, h)
		}
	}
	if len(pendingGC) == 1 && len(pendingHID) == 1 {
		slots.pair(table, pendingGC[0], pendingHID[0])
	} else {
		useGC := preferFrameworkSamples(pendingGC, pendingHID, preferGC)
		pending := pendingHID
		if useGC {
			pending = pendingGC
		}
		for _, p := range pending {
			if p.state.Index >= 0 {
				continue
			}
			if i, ok := table.claim(p.key); ok {
				p.state.Index = i
				slots[i] = dualSenseSlot{owner: p.key}
				if useGC {
					slots[i].gc = p.key
				} else {
					slots[i].hid = p.key
				}
			}
		}
	}
	for _, g := range gc {
		if i := g.state.Index; i >= 0 && (preferGC || sampleFor(hid, slots[i].hid) == nil) {
			dst = append(dst, g.state)
		}
	}
	for _, h := range hid {
		if i := h.state.Index; i >= 0 && (!preferGC || sampleFor(gc, slots[i].gc) == nil) {
			dst = append(dst, h.state)
		}
	}
	return dst
}

// pair adds a second transport to an existing slot, even when the table is full.
func (slots *dualSenseSlots) pair(table *slotTable, g, h *dualSenseSample) {
	if g.state.Index >= 0 && h.state.Index >= 0 && g.state.Index != h.state.Index {
		return // both players already have distinct assignments
	}
	i := max(g.state.Index, h.state.Index)
	if i < 0 {
		var ok bool
		i, ok = table.claim(g.key)
		if !ok {
			return
		}
		slots[i].owner = g.key
	}
	slots[i].gc, slots[i].hid = g.key, h.key
	g.state.Index, h.state.Index = i, i
}

// With ambiguous identities, continue reporting through the transport that
// already owns slots. On first sight use the more complete list.
func preferFrameworkSamples(gc, hid []*dualSenseSample, preferGC bool) bool {
	for _, h := range hid {
		if h.state.Index >= 0 {
			return false
		}
	}
	for _, g := range gc {
		if g.state.Index >= 0 {
			return true
		}
	}
	if len(gc) == len(hid) {
		return preferGC
	}
	return len(gc) > len(hid)
}

func sampleFor(samples []dualSenseSample, key slotKey) *dualSenseSample {
	for i := range samples {
		if samples[i].key == key {
			return &samples[i]
		}
	}
	return nil
}
