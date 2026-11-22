// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

// Nintendo Switch 2 Pro Controller, over USB.
//
// The operating system's own controller support does not know this pad, and
// the pad itself sends nothing until it is told to: it presents a HID
// interface (0) that stays silent, and a vendor interface (1) with two bulk
// endpoints that takes commands. A driver reads the factory stick calibration
// from the pad's flash and sends a start-up sequence over the vendor
// interface, after which the HID interface reports the pad's state 250 times
// a second in report 9.
//
// The command bytes and the report layout are those worked out by the SDL
// project (src/joystick/hidapi/SDL_hidapi_switch2.c, contributed by Valve
// under the zlib license); this file is an independent implementation of the
// same protocol. What each command means is known only as far as the names
// below say.

const (
	switch2VendorID  = 0x057e // Nintendo
	switch2ProductID = 0x2069 // Switch 2 Pro Controller
	// switch2CommandInterface is the vendor interface that takes commands.
	switch2CommandInterface = 1
	// switch2ReportPro is the input report the pad is asked to send.
	switch2ReportPro = 0x09
	// switch2Name is the name reported for the pad.
	switch2Name = "Switch 2 Pro Controller"
)

// Flash addresses of the stick calibration. The factory blocks are always
// present; a user block is used only when it starts with switch2UserMagic.
const (
	switch2FlashLeftFactory  = 0x13080
	switch2FlashRightFactory = 0x130C0
	switch2FlashLeftUser     = 0x1FC040
	switch2FlashRightUser    = 0x1FC080
	// A flash reply is a 16-byte header and then 64 bytes of flash.
	switch2FlashHeader = 16
	switch2FlashBlock  = 64
	// Offset of the calibration within a factory block, and within a user
	// block after its two magic bytes.
	switch2FactoryCalOffset = 0x28
	switch2UserCalOffset    = 2
)

var switch2UserMagic = [2]byte{0xb2, 0xa1}

// switch2FlashRead is the command that reads one 64-byte block of flash.
func switch2FlashRead(address uint32) []byte {
	return []byte{
		0x02, 0x91, 0x00, 0x01, 0x00, 0x08, 0x00, 0x00,
		0x40, 0x7e, 0x00, 0x00,
		byte(address), byte(address >> 8), byte(address >> 16), byte(address >> 24),
	}
}

// switch2Init is the start-up sequence, sent in order; a reply of up to 64
// bytes follows each command and is read and ignored. slot picks which of the
// pad's player lights is lit.
func switch2Init(slot int) [][]byte {
	lights := [...]byte{0x1, 0x3, 0x7, 0xf, 0x9, 0x5, 0xd, 0x6}
	if slot < 0 {
		slot = 0
	}
	return [][]byte{
		{0x07, 0x91, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
		// Set the feature mask.
		{0x0c, 0x91, 0x00, 0x02, 0x00, 0x04, 0x00, 0x00, 0x27, 0x00, 0x00, 0x00},
		{0x11, 0x91, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
		{
			0x0a, 0x91, 0x00, 0x08, 0x00, 0x14, 0x00, 0x00,
			0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0x35, 0x00, 0x46, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
		},
		// Enable the features.
		{0x0c, 0x91, 0x00, 0x04, 0x00, 0x04, 0x00, 0x00, 0x27, 0x00, 0x00, 0x00},
		{0x01, 0x91, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00},
		{0x01, 0x91, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
		{0x08, 0x91, 0x00, 0x02, 0x00, 0x04, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00},
		// Choose the input report.
		{0x03, 0x91, 0x00, 0x0a, 0x00, 0x04, 0x00, 0x00, switch2ReportPro, 0x00, 0x00, 0x00},
		// Start sending input reports.
		{
			0x03, 0x91, 0x00, 0x0d, 0x00, 0x08, 0x00, 0x00,
			0x01, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		},
		// Light the player light for the slot.
		{
			0x09, 0x91, 0x00, 0x07, 0x00, 0x08, 0x00, 0x00,
			lights[slot%len(lights)], 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}
}

// switch2Axis is one stick axis's calibration, in the pad's 12-bit units:
// its resting value and how far it travels each way from there.
type switch2Axis struct {
	neutral, max, min uint16
}

// valid reports whether the calibration can be used. Erased flash reads as
// all ones, and a zero travel would divide by zero.
func (a switch2Axis) valid() bool {
	return a.neutral != 0 && a.neutral != 0xfff &&
		a.max != 0 && a.max != 0xfff && a.min != 0 && a.min != 0xfff
}

// value maps a raw 12-bit reading to [-1, 1]. Without a usable calibration
// the reading is taken to rest at mid-scale and travel the full range.
func (a switch2Axis) value(raw uint16) float64 {
	if !a.valid() {
		return clamp((float64(raw)-2048)/2047, -1, 1)
	}
	d := float64(raw) - float64(a.neutral)
	if d < 0 {
		return clamp(d/float64(a.min), -1, 1)
	}
	return clamp(d/float64(a.max), -1, 1)
}

// switch2Stick is one stick's calibration.
type switch2Stick struct {
	x, y switch2Axis
}

func (s switch2Stick) valid() bool { return s.x.valid() && s.y.valid() }

// switch2Calibration is the pad's two sticks.
type switch2Calibration struct {
	left, right switch2Stick
}

// switch2ParseStick decodes the nine bytes of one stick's calibration: six
// 12-bit values packed in pairs, neutral then maximum then minimum travel,
// X before Y. Fewer than nine bytes decode as no calibration.
func switch2ParseStick(b []byte) switch2Stick {
	if len(b) < 9 {
		return switch2Stick{}
	}
	pair := func(p []byte) (lo, hi uint16) {
		return uint16(p[0]) | uint16(p[1]&0x0f)<<8, uint16(p[1])>>4 | uint16(p[2])<<4
	}
	var s switch2Stick
	s.x.neutral, s.y.neutral = pair(b[0:3])
	s.x.max, s.y.max = pair(b[3:6])
	s.x.min, s.y.min = pair(b[6:9])
	return s
}

// switch2FlashPayload is the 64 bytes of flash in a reply to switch2FlashRead,
// or nil when the reply is short.
func switch2FlashPayload(reply []byte) []byte {
	if len(reply) < switch2FlashHeader+switch2FlashBlock {
		return nil
	}
	return reply[switch2FlashHeader : switch2FlashHeader+switch2FlashBlock]
}

// switch2StickFrom picks a stick's calibration from its factory and user
// flash blocks: the user's when present and usable, else the factory's.
func switch2StickFrom(factory, user []byte) switch2Stick {
	if len(user) >= switch2UserCalOffset+9 && user[0] == switch2UserMagic[0] && user[1] == switch2UserMagic[1] {
		if s := switch2ParseStick(user[switch2UserCalOffset:]); s.valid() {
			return s
		}
	}
	if len(factory) >= switch2FactoryCalOffset+9 {
		return switch2ParseStick(factory[switch2FactoryCalOffset:])
	}
	return switch2Stick{}
}

// Buttons of report 9, as HID button usages (1-based): usage n is bit n-1 of
// the report's 21-bit button field. Home, Capture, C and the two rear paddles
// (usages 17 to 21) have no place in the standard layout and are not
// reported.
//
// Nintendo prints A on the right and B at the bottom; buttons are reported
// by where they are, so B is ButtonSouth.
var switch2Buttons = [...]Button{
	0:  ButtonSouth,     // B
	1:  ButtonEast,      // A
	2:  ButtonWest,      // Y
	3:  ButtonNorth,     // X
	4:  ButtonR1,        // R
	5:  ButtonR2,        // ZR
	6:  ButtonStart,     // +
	7:  ButtonR3,        // right stick click
	8:  ButtonDPadDown,  //
	9:  ButtonDPadRight, //
	10: ButtonDPadLeft,  //
	11: ButtonDPadUp,    //
	12: ButtonL1,        // L
	13: ButtonL2,        // ZL
	14: ButtonSelect,    // -
	15: ButtonL3,        // left stick click
}

// switch2ButtonCount is how many button usages the driver reads.
const switch2ButtonCount = len(switch2Buttons)

// switch2Input is what one poll reads from the pad: the button field and the
// four raw 12-bit stick axes.
type switch2Input struct {
	buttons        uint32 // bit n is button usage n+1
	lx, ly, rx, ry uint16
}

// switch2ParseReport reads the fields out of a raw report 9, for a transport
// that delivers whole reports. It reports false for another report or a
// short one.
func switch2ParseReport(r []byte) (switch2Input, bool) {
	if len(r) < 12 || r[0] != switch2ReportPro {
		return switch2Input{}, false
	}
	pair := func(p []byte) (lo, hi uint16) {
		return uint16(p[0]) | uint16(p[1]&0x0f)<<8, uint16(p[1])>>4 | uint16(p[2])<<4
	}
	in := switch2Input{buttons: uint32(r[3]) | uint32(r[4])<<8 | uint32(r[5])<<16}
	in.lx, in.ly = pair(r[6:9])
	in.rx, in.ry = pair(r[9:12])
	return in, true
}

// switch2State is the pad's State. The pad's sticks already read +Y up. Its
// triggers are switches, so they read 0 or 1.
func switch2State(index int, in switch2Input, cal switch2Calibration) State {
	s := State{Connected: true, Index: index, Name: switch2Name}
	for bit, b := range switch2Buttons {
		if in.buttons&(1<<bit) != 0 {
			s.Buttons |= b
		}
	}
	if s.Buttons&ButtonL2 != 0 {
		s.LeftTrigger = 1
	}
	if s.Buttons&ButtonR2 != 0 {
		s.RightTrigger = 1
	}
	s.LeftX, s.LeftY = cal.left.x.value(in.lx), cal.left.y.value(in.ly)
	s.RightX, s.RightY = cal.right.x.value(in.rx), cal.right.y.value(in.ry)
	return s
}

// usbBulkEndpoints finds the bulk IN and OUT endpoint addresses of one
// interface in a USB configuration descriptor. It reports false when the
// interface lacks either or the descriptor is malformed.
func usbBulkEndpoints(config []byte, iface byte) (in, out byte, ok bool) {
	const (
		descInterface = 4
		descEndpoint  = 5
		attrBulk      = 2
	)
	current := -1
	var haveIn, haveOut bool
	for i := 0; i < len(config); {
		length := int(config[i])
		if length < 2 || i+length > len(config) {
			return 0, 0, false
		}
		d := config[i : i+length]
		switch d[1] {
		case descInterface:
			if length >= 3 {
				current = int(d[2])
			}
		case descEndpoint:
			if current == int(iface) && length >= 4 && d[3]&3 == attrBulk {
				if d[2]&0x80 != 0 {
					in, haveIn = d[2], true
				} else {
					out, haveOut = d[2], true
				}
			}
		}
		i += length
	}
	return in, out, haveIn && haveOut
}

// In a browser the pad shows up, once something has started it, as a generic
// HID gamepad with no standard mapping: its 21 buttons in usage order, and
// its four stick axes scaled from the pad's 12 bits to [-1, 1] with +Y up.
// Browsers number the axes differently:
//
//	Chromium  six axes, by HID usage: left stick 0 and 1, right stick 3 and 5
//	Firefox   four axes, in report order: left stick 0 and 1, right stick 2 and 3
//
// A page cannot read the pad's flash, so the sticks use a nominal
// calibration: at rest they may read a few percent off centre, inside the
// default deadzone.
const (
	switch2WebAxesByUsage = 6
	switch2WebAxesInOrder = 4
	switch2WebButtons     = 21
)

// switch2Nominal is a typical pad's calibration: centred, travelling about
// 1650 of the 2048 units each way.
var switch2Nominal = switch2Calibration{
	left:  switch2Stick{x: switch2Axis{2048, 1650, 1650}, y: switch2Axis{2048, 1650, 1650}},
	right: switch2Stick{x: switch2Axis{2048, 1650, 1650}, y: switch2Axis{2048, 1650, 1650}},
}

// isSwitch2Web reports whether a browser gamepad without a standard mapping
// is this pad, from its id and its shape. Browsers word the id differently
// but all include the vendor and product numbers.
func isSwitch2Web(id string, axes, buttons int) bool {
	return (axes == switch2WebAxesByUsage || axes == switch2WebAxesInOrder) && buttons == switch2WebButtons &&
		containsFold(id, "057e") && containsFold(id, "2069")
}

// containsFold reports whether s contains the lower-case ASCII sub, ignoring
// the case of s.
func containsFold(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// switch2WebState is the pad's State from a browser's generic view of it;
// the number of axes tells which browser's numbering applies.
func switch2WebState(index int, axes []float64, pressed []bool) State {
	var in switch2Input
	for i := range min(len(pressed), switch2ButtonCount) {
		if pressed[i] {
			in.buttons |= 1 << i
		}
	}
	raw := func(i int) uint16 {
		if i >= len(axes) {
			return 2048
		}
		return uint16((clamp(axes[i], -1, 1) + 1) * 2047.5)
	}
	rx, ry := 3, 5
	if len(axes) == switch2WebAxesInOrder {
		rx, ry = 2, 3
	}
	in.lx, in.ly, in.rx, in.ry = raw(0), raw(1), raw(rx), raw(ry)
	return switch2State(index, in, switch2Nominal)
}
