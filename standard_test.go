// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"math"
	"testing"
)

// Each index of a standard-mapped browser gamepad's buttons array sets its
// own bit and no other.
func TestStandardStateMapsEachButtonIndex(t *testing.T) {
	for i, want := range standardButtons {
		pressed := make([]bool, 17)
		pressed[i] = true
		got := standardState(0, "pad", nil, pressed, nil)
		if got.Buttons != want {
			t.Fatalf("button index %d = %016b, want %016b", i, got.Buttons, want)
		}
	}
	// Index 16 is the centre button, which has no bit.
	pressed := make([]bool, 17)
	pressed[16] = true
	if got := standardState(0, "pad", nil, pressed, nil); got.Buttons != 0 {
		t.Fatalf("centre button set %016b, want nothing", got.Buttons)
	}
}

// The browser's +Y is down; the State's is up. X is unchanged.
func TestStandardStateFlipsY(t *testing.T) {
	got := standardState(2, "pad", []float64{0.25, 0.5, -0.75, -1}, nil, nil)
	if got.LeftX != 0.25 || got.LeftY != -0.5 || got.RightX != -0.75 || got.RightY != 1 {
		t.Fatalf("sticks = (%v, %v) (%v, %v), want (0.25, -0.5) (-0.75, 1)",
			got.LeftX, got.LeftY, got.RightX, got.RightY)
	}
	if !got.Connected || got.Index != 2 || got.Name != "pad" {
		t.Fatalf("identity = %+v", got)
	}
}

// Triggers report their analogue travel, count as pressed past half, and a
// digital trigger (pressed with no value) reads fully pulled.
func TestStandardStateTriggers(t *testing.T) {
	values := make([]float64, 16)
	values[standardLeftTrigger] = 0.4
	values[standardRightTrigger] = 0.9
	got := standardState(0, "", nil, nil, values)
	if got.LeftTrigger != 0.4 || got.RightTrigger != 0.9 {
		t.Fatalf("triggers = (%v, %v), want (0.4, 0.9)", got.LeftTrigger, got.RightTrigger)
	}
	if got.Buttons != ButtonR2 {
		t.Fatalf("buttons = %016b, want R2 only", got.Buttons)
	}
	pressed := make([]bool, 16)
	pressed[standardLeftTrigger] = true
	got = standardState(0, "", nil, pressed, make([]float64, 16))
	if got.LeftTrigger != 1 || got.Buttons != ButtonL2 {
		t.Fatalf("digital trigger = %v %016b, want 1 and L2", got.LeftTrigger, got.Buttons)
	}
}

// Short arrays and values out of range or NaN never index out of bounds and
// never leave the documented ranges.
func TestStandardStateToleratesBadInput(t *testing.T) {
	got := standardState(0, "", []float64{math.NaN(), 7}, []bool{true}, []float64{math.Inf(1), -3, math.NaN()})
	if got.LeftX != 0 || got.LeftY != -1 || got.RightX != 0 || got.RightY != 0 {
		t.Fatalf("sticks = (%v, %v) (%v, %v), want (0, -1) (0, 0)", got.LeftX, got.LeftY, got.RightX, got.RightY)
	}
	if got.Buttons != ButtonSouth {
		t.Fatalf("buttons = %016b, want South only", got.Buttons)
	}
	if got.LeftTrigger != 0 || got.RightTrigger != 0 {
		t.Fatalf("triggers = (%v, %v), want (0, 0)", got.LeftTrigger, got.RightTrigger)
	}
}
