// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"encoding/binary"
	"testing"
)

// jsEvent encodes one Linux struct js_event.
func jsEvent(typ, num byte, val int16) []byte {
	ev := make([]byte, joydevEventSize)
	binary.LittleEndian.PutUint32(ev[0:4], 1234)
	binary.LittleEndian.PutUint16(ev[4:6], uint16(val))
	ev[6], ev[7] = typ, num
	return ev
}

func jsEvents(evs ...[]byte) []byte {
	var out []byte
	for _, ev := range evs {
		out = append(out, ev...)
	}
	return out
}

// The events a device replays on open (marked INIT) set its state like any
// other, so a trigger at rest reads 0 and a held button reads down.
func TestJoydevAppliesInitEvents(t *testing.T) {
	var j joydev
	used := j.feed(jsEvents(
		jsEvent(joydevAxis|joydevInit, 2, -32767),
		jsEvent(joydevAxis|joydevInit, 5, -32767),
		jsEvent(joydevButton|joydevInit, 0, 1),
	))
	if used != 3*joydevEventSize {
		t.Fatalf("feed used %d bytes, want %d", used, 3*joydevEventSize)
	}
	s := j.state(1, "/dev/input/js1")
	if !s.Connected || s.Index != 1 || s.Name != "/dev/input/js1" {
		t.Fatalf("identity = %+v", s)
	}
	if s.LeftTrigger != 0 || s.RightTrigger != 0 {
		t.Fatalf("triggers at rest = (%v, %v), want (0, 0)", s.LeftTrigger, s.RightTrigger)
	}
	if s.Buttons != ButtonSouth {
		t.Fatalf("buttons = %016b, want South only", s.Buttons)
	}
}

// Before a device has reported its trigger axes they read released, not half
// pulled.
func TestJoydevTriggersRestBeforeFirstEvent(t *testing.T) {
	var j joydev
	s := j.state(0, "")
	if s.LeftTrigger != 0 || s.RightTrigger != 0 || s.Buttons != 0 {
		t.Fatalf("fresh device = %+v, want everything at rest", s)
	}
}

// Sticks map with +Y up, triggers run 0 to 1 and set L2/R2 past half, and the
// hat axes become the four d-pad buttons.
func TestJoydevAxesAndHat(t *testing.T) {
	var j joydev
	j.feed(jsEvents(
		jsEvent(joydevAxis, 0, 32767),  // left stick right
		jsEvent(joydevAxis, 1, -32767), // left stick up
		jsEvent(joydevAxis, 3, -16384), // right stick half left
		jsEvent(joydevAxis, 4, 32767),  // right stick down
		jsEvent(joydevAxis, 2, 32767),  // left trigger full
		jsEvent(joydevAxis, 5, 0),      // right trigger half
		jsEvent(joydevAxis, 6, -32767), // hat left
		jsEvent(joydevAxis, 7, 32767),  // hat down
	))
	s := j.state(0, "")
	if s.LeftX != 1 || s.LeftY != 1 || s.RightY != -1 {
		t.Fatalf("sticks = (%v, %v) (%v, %v)", s.LeftX, s.LeftY, s.RightX, s.RightY)
	}
	if s.RightX > -0.49 || s.RightX < -0.51 {
		t.Fatalf("right X = %v, want about -0.5", s.RightX)
	}
	if s.LeftTrigger != 1 || s.RightTrigger != 0.5 {
		t.Fatalf("triggers = (%v, %v), want (1, 0.5)", s.LeftTrigger, s.RightTrigger)
	}
	if want := ButtonL2 | ButtonDPadLeft | ButtonDPadDown; s.Buttons != want {
		t.Fatalf("buttons = %016b, want %016b", s.Buttons, want)
	}
	// -32768 is one past the divisor; it must still read exactly -1.
	j.feed(jsEvent(joydevAxis, 0, -32768))
	if got := j.state(0, "").LeftX; got != -1 {
		t.Fatalf("axis at -32768 = %v, want -1", got)
	}
}

// Each xpad button number sets its bit; the guide button (8) sets none, and a
// release clears the bit.
func TestJoydevButtons(t *testing.T) {
	for num, want := range joydevButtons {
		var j joydev
		j.feed(jsEvent(joydevButton, byte(num), 1))
		if got := j.state(0, "").Buttons; got != want {
			t.Fatalf("button %d = %016b, want %016b", num, got, want)
		}
		j.feed(jsEvent(joydevButton, byte(num), 0))
		if got := j.state(0, "").Buttons; got != 0 {
			t.Fatalf("button %d still down after release: %016b", num, got)
		}
	}
}

// A read can end mid-event: feed uses only whole events and reports how many
// bytes, and numbers beyond the tables are ignored without a panic.
func TestJoydevPartialAndOutOfRangeEvents(t *testing.T) {
	var j joydev
	buf := jsEvents(
		jsEvent(joydevButton, 200, 1),
		jsEvent(joydevAxis, 200, 32767),
		jsEvent(0x40, 0, 1),
		jsEvent(joydevButton, 1, 1),
	)
	used := j.feed(buf[:len(buf)-3])
	if used != 3*joydevEventSize {
		t.Fatalf("feed used %d bytes of a partial read, want %d", used, 3*joydevEventSize)
	}
	if got := j.state(0, "").Buttons; got != 0 {
		t.Fatalf("buttons after out-of-range events = %016b, want none", got)
	}
	if used := j.feed(buf[used:]); used != joydevEventSize {
		t.Fatalf("feed used %d bytes of the remainder, want %d", used, joydevEventSize)
	}
	if got := j.state(0, "").Buttons; got != ButtonEast {
		t.Fatalf("buttons = %016b, want East", got)
	}
	if used := j.feed(nil); used != 0 {
		t.Fatalf("feed(nil) used %d bytes", used)
	}
}
