// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import "testing"

// A known DualSense pair shares one slot, even at capacity. Switching input
// source or unplugging one pad leaves the other player's inputs in place.
func TestDualSenseHandoff(t *testing.T) {
	var table slotTable
	var slots dualSenseSlots
	table.claim(slotKey{9, 1})
	table.claim(slotKey{9, 2})
	sample := func(kind uint8, id uint64, button Button) dualSenseSample {
		return dualSenseSample{slotKey{kind, id}, State{Index: -1, Connected: true, Name: dualSenseName, Buttons: button}}
	}
	g1, g2 := sample(1, 10, ButtonWest), sample(1, 11, ButtonEast)
	h1, h2 := sample(3, 20, ButtonSouth), sample(3, 21, ButtonNorth)
	check := func(g, h []dualSenseSample, main bool, want map[int]Button) {
		t.Helper()
		out := slots.assign(&table, g, h, main, nil)
		if len(out) != len(want) {
			t.Fatalf("got %+v, want %v", out, want)
		}
		for _, p := range out {
			b, ok := want[p.Index]
			if !ok || b != p.Buttons {
				t.Fatalf("got %+v, want %v", out, want)
			}
			delete(want, p.Index)
		}
	}
	check(nil, []dualSenseSample{h1}, false, map[int]Button{2: ButtonSouth})
	check([]dualSenseSample{g1}, []dualSenseSample{h1}, true, map[int]Button{2: ButtonWest})
	check([]dualSenseSample{g1}, []dualSenseSample{h2, h1}, true, map[int]Button{2: ButtonWest, 3: ButtonNorth})
	check([]dualSenseSample{g2, g1}, []dualSenseSample{h1, h2}, true, map[int]Button{2: ButtonWest, 3: ButtonEast})
	check(nil, []dualSenseSample{h2, h1}, false, map[int]Button{2: ButtonSouth, 3: ButtonNorth})
	check([]dualSenseSample{g2}, nil, true, map[int]Button{3: ButtonEast})
	check(nil, nil, true, nil)
	if got, ok := table.claim(slotKey{9, 3}); !ok || got != 2 {
		t.Fatalf("released slot = %d, %v", got, ok)
	}
}

// Two HID pads seen together keep their physical identities when a single
// indistinguishable framework controller appears, and when both appear.
func TestDualSenseAmbiguousHandoff(t *testing.T) {
	var slots dualSenseSlots
	var table slotTable
	for _, ids := range [][]uint64{nil, {91}, {92, 91}} {
		var gc []dualSenseSample
		for _, id := range ids {
			gc = append(gc, dualSenseSample{slotKey{1, id}, State{Index: -1, Name: dualSenseName}})
		}
		hid := []dualSenseSample{
			{slotKey{3, 7}, State{Index: -1, Name: dualSenseName, Buttons: ButtonSouth}},
			{slotKey{3, 8}, State{Index: -1, Name: dualSenseName, Buttons: ButtonEast}},
		}
		if len(ids) > 0 {
			hid[0], hid[1] = hid[1], hid[0]
		}
		out := slots.assign(&table, gc, hid, true, nil)
		if len(out) != 2 {
			t.Fatalf("framework %v: got %+v", ids, out)
		}
		for _, p := range out {
			if (p.Index == 0 && p.Buttons == ButtonSouth) || (p.Index == 1 && p.Buttons == ButtonEast) {
				continue
			}
			t.Fatalf("framework %v: controller moved: %+v", ids, out)
		}
	}
}
