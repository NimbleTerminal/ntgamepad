// Copyright (c) 2026 Neomantra Corp.

// Package ntgamepad polls standard game controllers in the browser (WASM) and
// on native platforms, with CGO_ENABLED=0.
//
// Open returns the platform's Reader. Poll it once a frame: it never blocks,
// and a controller that is plugged in or removed between two polls simply
// appears in or drops out of the next one.
//
//	pad := ntgamepad.Open()
//	defer pad.Close()
//	var prev ntgamepad.State
//	for frame() {
//		s := pad.Poll().WithDeadzone(0)
//		if s.JustPressed(prev)&ntgamepad.ButtonSouth != 0 {
//			jump()
//		}
//		steer(s.LeftX)
//		prev = s
//	}
//
// Every platform reports the same layout: the W3C Standard Gamepad, which is
// also SDL's and Apple's Extended Gamepad. See the README for what each
// platform reads and which controllers it has been tried with.
package ntgamepad

import "math"

// Button is a set of digital buttons, one bit each, in the W3C Standard
// Gamepad layout.
type Button uint16

const (
	ButtonSouth  Button = 1 << iota // Bottom face button (A / Cross)
	ButtonEast                      // Right face button (B / Circle)
	ButtonWest                      // Left face button (X / Square)
	ButtonNorth                     // Top face button (Y / Triangle)
	ButtonL1                        // Left shoulder bumper
	ButtonR1                        // Right shoulder bumper
	ButtonL2                        // Left trigger, past half travel
	ButtonR2                        // Right trigger, past half travel
	ButtonSelect                    // Back / Select / Share
	ButtonStart                     // Start / Options / Menu
	ButtonL3                        // Left stick click
	ButtonR3                        // Right stick click
	ButtonDPadUp
	ButtonDPadDown
	ButtonDPadLeft
	ButtonDPadRight
)

// DefaultDeadzone is the radial thumbstick deadzone WithDeadzone uses when
// given a threshold that is not positive.
const DefaultDeadzone = 0.15

// MaxPads is the most controllers a Reader reports. Index is below it.
const MaxPads = 4

// State is one polled snapshot of one controller. The zero State is "no
// controller": Connected is false and nothing is pressed.
//
// Stick axes are in [-1, 1] with +X right and +Y up (forward); triggers are
// in [0, 1].
type State struct {
	// Connected is false in the State a Reader returns when it has no
	// controller to report.
	Connected bool
	// Index is the controller's slot, in [0, MaxPads). A controller keeps its
	// slot while it stays connected, so Index tells two pads apart from one
	// poll to the next.
	Index int
	// Name is the controller's name as the platform gives it; it may be empty.
	Name                      string
	LeftX, LeftY              float64
	RightX, RightY            float64
	LeftTrigger, RightTrigger float64
	Buttons                   Button
}

// Pressed reports whether every button in b is down.
func (s State) Pressed(b Button) bool { return s.Connected && s.Buttons&b == b }

// JustPressed returns the buttons that are down in s and were not down in
// prev, the previous poll of the same controller. A controller that has just
// connected reports every button it holds; one that is not connected reports
// none.
func (s State) JustPressed(prev State) Button {
	if !s.Connected {
		return 0
	}
	if !prev.Connected {
		return s.Buttons
	}
	return s.Buttons &^ prev.Buttons
}

// JustReleased returns the buttons that were down in prev and are not down in
// s. A controller that has just disconnected reports every button it held.
func (s State) JustReleased(prev State) Button {
	if !prev.Connected {
		return 0
	}
	if !s.Connected {
		return prev.Buttons
	}
	return prev.Buttons &^ s.Buttons
}

// WithDeadzone applies a radial deadzone to both thumbsticks: deflection
// below dz reads zero, and the rest is rescaled to run from 0 at dz to 1 at
// full deflection. A dz that is not positive means DefaultDeadzone; it is
// held to at most 0.95. Triggers are held to [0, 1].
func (s State) WithDeadzone(dz float64) State {
	if !(dz > 0) {
		dz = DefaultDeadzone
	}
	if dz >= 0.95 {
		dz = 0.95
	}
	s.LeftX, s.LeftY = radialDeadzone(s.LeftX, s.LeftY, dz)
	s.RightX, s.RightY = radialDeadzone(s.RightX, s.RightY, dz)
	s.LeftTrigger = clamp(s.LeftTrigger, 0, 1)
	s.RightTrigger = clamp(s.RightTrigger, 0, 1)
	return s
}

func radialDeadzone(x, y, dz float64) (ox, oy float64) {
	x, y = clamp(x, -1, 1), clamp(y, -1, 1)
	mag := math.Hypot(x, y)
	if !(mag > dz) {
		return 0, 0
	}
	scaled := math.Min(1, (mag-dz)/(1-dz))
	k := scaled / mag
	return x * k, y * k
}

// clamp holds v to [lo, hi]; NaN reads as zero.
func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return math.Max(lo, math.Min(hi, v))
}

// Reader polls the connected controllers. Its methods never block. A Reader
// is for one goroutine at a time.
type Reader interface {
	// Poll returns the connected controller with the lowest Index, or the
	// zero State when there is none.
	Poll() State
	// PollAll returns every connected controller in Index order, appended to
	// dst[:0]. Pass the previous result to poll without allocating.
	PollAll(dst []State) []State
	// Close releases what the Reader holds. A closed Reader reports no
	// controllers.
	Close() error
}

// Open returns the platform's Reader. It always succeeds: on a platform
// without support, or with no controller attached, the Reader reports none.
//
// Tests should supply their own Reader rather than call Open.
func Open() Reader { return openPlatform() }

// first is Poll in terms of PollAll, for the platform readers.
func first(pads []State) State {
	if len(pads) == 0 {
		return State{}
	}
	return pads[0]
}
