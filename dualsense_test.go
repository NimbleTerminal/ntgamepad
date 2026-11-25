// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"math"
	"testing"
)

// The enhanced report captured from a real DualSense at rest over Bluetooth
// reads as sticks near centre, triggers released, nothing pressed.
func TestDualSenseEnhancedReportAtRest(t *testing.T) {
	in, ok := dualSenseParseEnhanced(unhex(t, "d1 80 7e 7f 7f 00 00 01 08 00 00 00 be 4c"))
	if !ok {
		t.Fatal("the captured report was refused")
	}
	if in != (dualSenseInput{lx: 0x80, ly: 0x7e, rx: 0x7f, ry: 0x7f, hat: dualSenseHatNeutral}) {
		t.Fatalf("raw fields = %+v", in)
	}
	s := dualSenseState(1, in)
	if !s.Connected || s.Index != 1 || s.Name != dualSenseName || s.Buttons != 0 {
		t.Fatalf("state = %+v", s)
	}
	for name, v := range map[string]float64{"LeftX": s.LeftX, "LeftY": s.LeftY, "RightX": s.RightX, "RightY": s.RightY} {
		if math.Abs(v) > 0.03 {
			t.Fatalf("%s = %v at rest", name, v)
		}
	}
	if s.LeftTrigger != 0 || s.RightTrigger != 0 {
		t.Fatalf("triggers = (%v, %v) at rest", s.LeftTrigger, s.RightTrigger)
	}
}

// Each button bit of the enhanced report lands on its usage, by position:
// Cross is the bottom face button. PS and the touchpad click are dropped.
func TestDualSenseEnhancedButtons(t *testing.T) {
	report := func(b8, b9, b10 byte) State {
		in, ok := dualSenseParseEnhanced([]byte{0xd1, 128, 128, 128, 128, 0, 0, 0, b8, b9, b10})
		if !ok {
			t.Fatal("report refused")
		}
		return dualSenseState(0, in)
	}
	for _, tc := range []struct {
		b8, b9, b10 byte
		want        Button
	}{
		{0x18, 0, 0, ButtonWest},      // Square
		{0x28, 0, 0, ButtonSouth},     // Cross
		{0x48, 0, 0, ButtonEast},      // Circle
		{0x88, 0, 0, ButtonNorth},     // Triangle
		{0x08, 0x01, 0, ButtonL1},     //
		{0x08, 0x02, 0, ButtonR1},     //
		{0x08, 0x10, 0, ButtonSelect}, // Create
		{0x08, 0x20, 0, ButtonStart},  // Options
		{0x08, 0x40, 0, ButtonL3},     //
		{0x08, 0x80, 0, ButtonR3},     //
		{0x08, 0, 0x03, 0},            // PS and touchpad: no place in the layout
		{0x00, 0, 0, ButtonDPadUp},    //
		{0x01, 0, 0, ButtonDPadUp | ButtonDPadRight},
		{0x02, 0, 0, ButtonDPadRight},
		{0x03, 0, 0, ButtonDPadRight | ButtonDPadDown},
		{0x04, 0, 0, ButtonDPadDown},
		{0x05, 0, 0, ButtonDPadDown | ButtonDPadLeft},
		{0x06, 0, 0, ButtonDPadLeft},
		{0x07, 0, 0, ButtonDPadLeft | ButtonDPadUp},
	} {
		if got := report(tc.b8, tc.b9, tc.b10).Buttons; got != tc.want {
			t.Fatalf("bytes %02x %02x %02x = %016b, want %016b", tc.b8, tc.b9, tc.b10, got, tc.want)
		}
	}
}

// Sticks read +Y up and reach exactly ±1; triggers run 0 to 1 and count as
// pressed past half travel, whatever the trigger's own button bit says.
func TestDualSenseAxesAndTriggers(t *testing.T) {
	s := dualSenseState(0, dualSenseInput{lx: 255, ly: 0, rx: 0, ry: 255, l2: 255, r2: 100, hat: dualSenseHatNeutral, buttons: 1 << 7})
	if s.LeftX != 1 || s.LeftY != 1 || s.RightX != -1 || s.RightY != -1 {
		t.Fatalf("sticks = (%v, %v) (%v, %v), want (1, 1) (-1, -1)", s.LeftX, s.LeftY, s.RightX, s.RightY)
	}
	if s.LeftTrigger != 1 || math.Abs(s.RightTrigger-100.0/255) > 1e-12 {
		t.Fatalf("triggers = (%v, %v)", s.LeftTrigger, s.RightTrigger)
	}
	if s.Buttons != ButtonL2 {
		t.Fatalf("buttons = %016b, want L2 only: R2 is under half travel", s.Buttons)
	}
}

// A block too short to hold the buttons is refused, and a hat value past
// neutral presses nothing.
func TestDualSenseRefusesShortAndOddInput(t *testing.T) {
	if _, ok := dualSenseParseEnhanced([]byte{0xd1, 1, 2, 3, 4, 5, 6, 7, 8, 9}); ok {
		t.Fatal("a 10-byte block was accepted")
	}
	if _, ok := dualSenseParseEnhanced(nil); ok {
		t.Fatal("an empty block was accepted")
	}
	if got := dualSenseState(0, dualSenseInput{hat: 15, lx: 128, ly: 128, rx: 128, ry: 128}).Buttons; got != 0 {
		t.Fatalf("hat 15 pressed %016b", got)
	}
}
