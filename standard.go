// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

// standardButtons is the W3C Standard Gamepad's button order: index i of a
// browser Gamepad's buttons array is standardButtons[i]. Index 16, the centre
// (home) button, has no bit here.
var standardButtons = [16]Button{
	ButtonSouth, ButtonEast, ButtonWest, ButtonNorth,
	ButtonL1, ButtonR1, ButtonL2, ButtonR2,
	ButtonSelect, ButtonStart, ButtonL3, ButtonR3,
	ButtonDPadUp, ButtonDPadDown, ButtonDPadLeft, ButtonDPadRight,
}

// Indices of the two triggers in the standard button order.
const (
	standardLeftTrigger  = 6
	standardRightTrigger = 7
)

// standardState builds a State from a browser Gamepad whose mapping is
// "standard": axes are left X, left Y, right X, right Y with +Y down, and
// buttons are in standardButtons order, each with a pressed flag and an
// analogue value in [0, 1]. Short or missing arrays read as not pressed and
// centred, and values out of range or NaN are held in range.
func standardState(index int, name string, axes []float64, pressed []bool, values []float64) State {
	s := State{Connected: true, Index: index, Name: name}
	axis := func(i int) float64 {
		if i >= len(axes) {
			return 0
		}
		return clamp(axes[i], -1, 1)
	}
	// The browser's +Y is down; ours is up.
	s.LeftX, s.LeftY = axis(0), -axis(1)
	s.RightX, s.RightY = axis(2), -axis(3)
	for i, bit := range standardButtons {
		var down bool
		var val float64
		if i < len(pressed) {
			down = pressed[i]
		}
		if i < len(values) {
			val = clamp(values[i], 0, 1)
		}
		if down || val > 0.5 {
			s.Buttons |= bit
		}
		switch i {
		case standardLeftTrigger:
			s.LeftTrigger = triggerValue(down, val)
		case standardRightTrigger:
			s.RightTrigger = triggerValue(down, val)
		}
	}
	return s
}

// triggerValue is a trigger's travel. A controller with digital triggers
// reports pressed with no analogue value; that reads as fully pulled.
func triggerValue(down bool, val float64) float64 {
	if down && val == 0 {
		return 1
	}
	return val
}
