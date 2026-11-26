// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

// Sony DualSense (PlayStation 5) controller, read over HID.
//
// macOS knows this pad, but only shows it to programs that run a main run
// loop. Polls on the main goroutine pump that loop. The HID path can read
// the pad from another goroutine without a run loop or a start-up sequence.
//
// The pad reports its state in one of two forms, and which one depends on
// what else on the machine has spoken to it:
//
//   - Report 1, described by the pad's HID descriptor with ordinary usages:
//     buttons 1 to 14, a hat switch, and six axes. This is what it sends over
//     USB, and over Bluetooth until something asks for more.
//   - Report 0x31, over Bluetooth once the pad is in its enhanced mode (the
//     system's own controller service puts it there): one opaque block the
//     descriptor does not describe, laid out as below.
//
// A driver reads both and uses whichever arrived last.

const (
	dualSenseVendorID  = 0x054c // Sony
	dualSenseProductID = 0x0ce6 // DualSense Wireless Controller
	dualSenseName      = "DualSense Wireless Controller"
	// dualSenseEnhancedReport is the Bluetooth report of the enhanced mode.
	dualSenseEnhancedReport = 0x31
	// dualSenseHatNeutral is the hat switch with no direction held; 0 to 7
	// are the eight directions clockwise from up.
	dualSenseHatNeutral = 8
)

// Buttons of report 1, as HID button usages (1-based): usage n is index n-1.
// The PlayStation button and the touchpad click (usages 13 and 14) have no
// place in the standard layout and are not reported.
var dualSenseButtons = [...]Button{
	0:  ButtonWest,   // Square
	1:  ButtonSouth,  // Cross
	2:  ButtonEast,   // Circle
	3:  ButtonNorth,  // Triangle
	4:  ButtonL1,     //
	5:  ButtonR1,     //
	6:  ButtonL2,     //
	7:  ButtonR2,     //
	8:  ButtonSelect, // Create
	9:  ButtonStart,  // Options
	10: ButtonL3,     //
	11: ButtonR3,     //
}

// dualSenseButtonCount is how many button usages the driver reads.
const dualSenseButtonCount = len(dualSenseButtons)

// dualSenseInput is the pad's state in the pad's own units, whichever report
// it came from: sticks and triggers 0 to 255 (sticks rest near 128, with Y
// growing downward), the hat 0 to 8, and the buttons as a bit per usage.
type dualSenseInput struct {
	lx, ly, rx, ry uint8
	l2, r2         uint8
	hat            uint8
	buttons        uint32 // bit n is button usage n+1
}

// dualSenseParseEnhanced reads the block of report 0x31 as the HID layer
// hands it over: a tag byte, then the sticks, the triggers, a counter, and
// three bytes of hat and buttons. It reports false for a block too short to
// hold them.
func dualSenseParseEnhanced(b []byte) (dualSenseInput, bool) {
	if len(b) < 11 {
		return dualSenseInput{}, false
	}
	in := dualSenseInput{lx: b[1], ly: b[2], rx: b[3], ry: b[4], l2: b[5], r2: b[6], hat: b[8] & 0x0f}
	// Byte 8's high half is Square, Cross, Circle, Triangle; byte 9 is the
	// next eight buttons in usage order; byte 10 starts with PS and touchpad.
	in.buttons = uint32(b[8]>>4) | uint32(b[9])<<4 | uint32(b[10])<<12
	return in, true
}

// dualSenseState is the pad's State.
func dualSenseState(index int, in dualSenseInput) State {
	s := State{Connected: true, Index: index, Name: dualSenseName}
	for bit, b := range dualSenseButtons {
		if in.buttons&(1<<bit) != 0 {
			s.Buttons |= b
		}
	}
	// The hat's directions clockwise from up; odd values are diagonals.
	switch in.hat {
	case 7, 0, 1:
		s.Buttons |= ButtonDPadUp
	case 3, 4, 5:
		s.Buttons |= ButtonDPadDown
	}
	switch in.hat {
	case 1, 2, 3:
		s.Buttons |= ButtonDPadRight
	case 5, 6, 7:
		s.Buttons |= ButtonDPadLeft
	}
	axis := func(v uint8) float64 { return clamp((float64(v)-128)/127, -1, 1) }
	s.LeftX, s.LeftY = axis(in.lx), -axis(in.ly)
	s.RightX, s.RightY = axis(in.rx), -axis(in.ry)
	s.LeftTrigger, s.RightTrigger = float64(in.l2)/255, float64(in.r2)/255
	// The trigger buttons follow the travel, so a light touch is not a press.
	s.Buttons &^= ButtonL2 | ButtonR2
	if s.LeftTrigger > 0.5 {
		s.Buttons |= ButtonL2
	}
	if s.RightTrigger > 0.5 {
		s.Buttons |= ButtonR2
	}
	return s
}
