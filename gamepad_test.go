// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"math"
	"testing"
)

// A deadzone zeroes small deflection, rescales the rest so full deflection
// still reads 1, and holds triggers to [0, 1].
func TestWithDeadzoneRescalesSticksAndClampsTriggers(t *testing.T) {
	raw := State{
		Connected:    true,
		LeftX:        0.08,
		LeftY:        0.05,
		RightX:       0.6,
		RightY:       -0.8,
		LeftTrigger:  -0.2,
		RightTrigger: 1.4,
	}
	got := raw.WithDeadzone(0.15)
	if got.LeftX != 0 || got.LeftY != 0 {
		t.Fatalf("inside the deadzone = (%v, %v), want (0, 0)", got.LeftX, got.LeftY)
	}
	if mag := math.Hypot(got.RightX, got.RightY); math.Abs(mag-1) > 1e-9 {
		t.Fatalf("full deflection magnitude = %v, want 1", mag)
	}
	if got.RightX <= 0 || got.RightY >= 0 {
		t.Fatalf("deadzone changed the stick's direction: (%v, %v)", got.RightX, got.RightY)
	}
	if got.LeftTrigger != 0 || got.RightTrigger != 1 {
		t.Fatalf("triggers = (%v, %v), want (0, 1)", got.LeftTrigger, got.RightTrigger)
	}
}

// A threshold that is not positive means the default, a huge one is held
// below 1 so the rescale never divides by zero, and NaN reads as centred.
func TestWithDeadzoneLimits(t *testing.T) {
	s := State{Connected: true, LeftX: DefaultDeadzone - 0.01, RightX: DefaultDeadzone + 0.1}
	for _, dz := range []float64{0, -1, math.NaN()} {
		got := s.WithDeadzone(dz)
		if got.LeftX != 0 || !(got.RightX > 0) {
			t.Fatalf("dz %v: sticks = (%v, %v), want the default deadzone", dz, got.LeftX, got.RightX)
		}
	}
	got := State{LeftX: 1, RightX: 0.9}.WithDeadzone(5)
	if got.LeftX != 1 || got.RightX != 0 || math.IsNaN(got.LeftX) {
		t.Fatalf("dz 5: sticks = (%v, %v), want (1, 0)", got.LeftX, got.RightX)
	}
	got = State{LeftX: math.NaN(), LeftY: math.Inf(1), LeftTrigger: math.NaN()}.WithDeadzone(0.2)
	if got.LeftX != 0 || got.LeftY != 1 || got.LeftTrigger != 0 {
		t.Fatalf("non-finite input = (%v, %v, %v), want (0, 1, 0)", got.LeftX, got.LeftY, got.LeftTrigger)
	}
}

// Pressed needs every asked button down, on a connected controller.
func TestPressedNeedsAllButtonsAndAConnection(t *testing.T) {
	s := State{Connected: true, Buttons: ButtonSouth | ButtonR1}
	if !s.Pressed(ButtonSouth) || !s.Pressed(ButtonSouth|ButtonR1) {
		t.Fatal("held buttons not reported pressed")
	}
	if s.Pressed(ButtonNorth) || s.Pressed(ButtonSouth|ButtonNorth) {
		t.Fatal("a button that is up reported pressed")
	}
	if (State{Buttons: ButtonSouth}).Pressed(ButtonSouth) {
		t.Fatal("a disconnected controller reported a button pressed")
	}
}

// JustPressed and JustReleased report the buttons that changed between two
// polls, and treat connecting and disconnecting as pressing and releasing.
func TestButtonEdges(t *testing.T) {
	prev := State{Connected: true, Buttons: ButtonSouth | ButtonL1}
	cur := State{Connected: true, Buttons: ButtonSouth | ButtonEast}
	if got := cur.JustPressed(prev); got != ButtonEast {
		t.Fatalf("JustPressed = %b, want East only", got)
	}
	if got := cur.JustReleased(prev); got != ButtonL1 {
		t.Fatalf("JustReleased = %b, want L1 only", got)
	}
	if got := cur.JustPressed(cur); got != 0 {
		t.Fatalf("JustPressed against itself = %b, want none", got)
	}
	var none State
	if got := cur.JustPressed(none); got != cur.Buttons {
		t.Fatalf("JustPressed on connect = %b, want every held button", got)
	}
	if got := none.JustReleased(cur); got != cur.Buttons {
		t.Fatalf("JustReleased on disconnect = %b, want every held button", got)
	}
	if none.JustPressed(cur) != 0 || cur.JustReleased(none) != 0 {
		t.Fatal("edges reported across a missing controller")
	}
}

// The platform Reader can be opened, polled and closed with no controller
// attached, never blocks, and reports nothing once closed.
func TestOpenPollCloseWithoutHardware(t *testing.T) {
	r := Open()
	pads := r.PollAll(nil)
	if len(pads) > MaxPads {
		t.Fatalf("PollAll returned %d pads, more than MaxPads", len(pads))
	}
	for i, p := range pads {
		if !p.Connected || p.Index < 0 || p.Index >= MaxPads {
			t.Fatalf("pad %d = %+v, want connected with an Index below MaxPads", i, p)
		}
		if i > 0 && pads[i-1].Index >= p.Index {
			t.Fatalf("pads not in Index order: %d then %d", pads[i-1].Index, p.Index)
		}
	}
	if got := r.Poll(); len(pads) == 0 && got.Connected {
		t.Fatalf("Poll = %+v with no pads in PollAll", got)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := r.PollAll(pads); len(got) != 0 || r.Poll().Connected {
		t.Fatal("a closed Reader still reports controllers")
	}
}
