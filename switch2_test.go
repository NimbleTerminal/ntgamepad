// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"encoding/hex"
	"math"
	"strings"
	"testing"
)

func unhex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Flash replies captured from a real Switch 2 Pro Controller: the factory
// calibration blocks of its left and right sticks.
const (
	switch2LeftFactoryReply = "02 01 00 01 00 f8 00 00 40 00 00 00 80 30 01 00" +
		"01 ad d9 9a 55 56 65 a0 00 0a a0 00 0a e2 20 0e e2 20 0e 9a ad d9 9a ad d9 0a a5 50 0a a5 50 2f f6 62 2f f6 62 0a ff ff" +
		"b6 47 87 8e 36 66 86 a6 64 ff ff ff ff ff ff ff ff ff ff ff ff ff ff ff"
	switch2RightFactoryReply = "02 01 00 01 00 f8 00 00 40 00 00 00 c0 30 01 00" +
		"01 ad d9 9a 55 56 65 a0 00 0a a0 00 0a e2 20 0e e2 20 0e 9a ad d9 9a ad d9 0a a5 50 0a a5 50 2f f6 62 2f f6 62 0a ff ff" +
		"57 d8 80 1b b6 65 0d 86 65 ff ff ff ff ff ff ff ff ff ff ff ff ff ff ff"
	// Report 9 with the sticks at rest and nothing pressed.
	switch2RestReport = "09 35 1b 00 00 00 d6 27 8c 79 38 81 38 00 00 1e a8 40 00 0c"
)

func capturedCalibration(t *testing.T) switch2Calibration {
	t.Helper()
	erased := make([]byte, switch2FlashBlock)
	for i := range erased {
		erased[i] = 0xff
	}
	return switch2Calibration{
		left:  switch2StickFrom(switch2FlashPayload(unhex(t, switch2LeftFactoryReply)), erased),
		right: switch2StickFrom(switch2FlashPayload(unhex(t, switch2RightFactoryReply)), erased),
	}
}

// The factory calibration decodes to twelve-bit neutral, maximum and minimum
// travel for each axis, and an erased user block leaves the factory one in use.
func TestSwitch2ParsesCapturedCalibration(t *testing.T) {
	cal := capturedCalibration(t)
	wantLeft := switch2Stick{
		x: switch2Axis{neutral: 0x7b6, max: 0x68e, min: 0x686},
		y: switch2Axis{neutral: 0x874, max: 0x663, min: 0x64a},
	}
	wantRight := switch2Stick{
		x: switch2Axis{neutral: 0x857, max: 0x61b, min: 0x60d},
		y: switch2Axis{neutral: 0x80d, max: 0x65b, min: 0x658},
	}
	if cal.left != wantLeft {
		t.Fatalf("left stick = %+v, want %+v", cal.left, wantLeft)
	}
	if cal.right != wantRight {
		t.Fatalf("right stick = %+v, want %+v", cal.right, wantRight)
	}
	if !cal.left.valid() || !cal.right.valid() {
		t.Fatal("captured calibration reads as not usable")
	}
}

// A user calibration block that starts with the magic bytes replaces the
// factory one; without the magic, or when erased, it is ignored.
func TestSwitch2UserCalibrationOverridesFactory(t *testing.T) {
	factory := switch2FlashPayload(unhex(t, switch2LeftFactoryReply))
	user := make([]byte, switch2FlashBlock)
	copy(user, []byte{0xb2, 0xa1, 0x00, 0x08, 0x80, 0x00, 0x06, 0x60, 0x00, 0x05, 0x50})
	got := switch2StickFrom(factory, user)
	want := switch2Stick{
		x: switch2Axis{neutral: 0x800, max: 0x600, min: 0x500},
		y: switch2Axis{neutral: 0x800, max: 0x600, min: 0x500},
	}
	if got != want {
		t.Fatalf("with a user block = %+v, want %+v", got, want)
	}
	user[0] = 0
	if got := switch2StickFrom(factory, user); got.x.neutral != 0x7b6 {
		t.Fatalf("without the magic the user block was used: %+v", got)
	}
	if got := switch2StickFrom(nil, nil); got.valid() {
		t.Fatalf("no flash at all gave a usable calibration: %+v", got)
	}
}

// The captured resting report reads as sticks near centre, inside the
// default deadzone, with nothing pressed.
func TestSwitch2RestingReportIsCentred(t *testing.T) {
	in, ok := switch2ParseReport(unhex(t, switch2RestReport))
	if !ok {
		t.Fatal("report 9 was not recognised")
	}
	if in.buttons != 0 || in.lx != 0x7d6 || in.ly != 0x8c2 || in.rx != 0x879 || in.ry != 0x813 {
		t.Fatalf("raw fields = %+v", in)
	}
	s := switch2State(2, in, capturedCalibration(t))
	if !s.Connected || s.Index != 2 || s.Name != switch2Name || s.Buttons != 0 {
		t.Fatalf("state = %+v", s)
	}
	for name, v := range map[string]float64{"LeftX": s.LeftX, "LeftY": s.LeftY, "RightX": s.RightX, "RightY": s.RightY} {
		if math.Abs(v) > 0.06 {
			t.Fatalf("%s = %v at rest, want within 0.06 of centre", name, v)
		}
	}
	d := s.WithDeadzone(0)
	if d.LeftX != 0 || d.LeftY != 0 || d.RightX != 0 || d.RightY != 0 {
		t.Fatalf("resting sticks escape the default deadzone: %+v", d)
	}
}

// Each button usage of the report sets its own bit, by position: Nintendo's B
// is the bottom face button. ZL and ZR are switches and read as full triggers.
func TestSwitch2ButtonsByPosition(t *testing.T) {
	cal := capturedCalibration(t)
	for bit, want := range switch2Buttons {
		s := switch2State(0, switch2Input{buttons: 1 << bit, lx: 0x7b6, ly: 0x874, rx: 0x857, ry: 0x80d}, cal)
		if s.Buttons != want {
			t.Fatalf("usage %d = %016b, want %016b", bit+1, s.Buttons, want)
		}
		if (s.LeftTrigger == 1) != (want == ButtonL2) || (s.RightTrigger == 1) != (want == ButtonR2) {
			t.Fatalf("usage %d: triggers = (%v, %v)", bit+1, s.LeftTrigger, s.RightTrigger)
		}
	}
	// Home, Capture, the paddles and C are read past the table and ignored.
	s := switch2State(0, switch2Input{buttons: 0x1f << 16}, cal)
	if s.Buttons != 0 {
		t.Fatalf("extra buttons set %016b", s.Buttons)
	}
	in, _ := switch2ParseReport(unhex(t, "09 00 1b 01 08 00 d6 27 8c 79 38 81"))
	if got := switch2State(0, in, cal).Buttons; got != ButtonSouth|ButtonDPadUp {
		t.Fatalf("report with B and up held = %016b", got)
	}
}

// A stick reads 0 at its neutral value, reaches exactly ±1 at its calibrated
// travel, and holds there beyond it. Without a calibration it rests at
// mid-scale.
func TestSwitch2AxisValue(t *testing.T) {
	a := switch2Axis{neutral: 2000, max: 1600, min: 1500}
	for raw, want := range map[uint16]float64{2000: 0, 3600: 1, 4095: 1, 500: -1, 0: -1, 2800: 0.5, 1250: -0.5} {
		if got := a.value(raw); math.Abs(got-want) > 1e-12 {
			t.Fatalf("raw %d = %v, want %v", raw, got, want)
		}
	}
	var none switch2Axis
	if none.valid() || none.value(2048) != 0 || none.value(4095) != 1 || none.value(0) != -1 {
		t.Fatalf("uncalibrated axis: %v %v %v", none.value(2048), none.value(4095), none.value(0))
	}
	if (switch2Axis{neutral: 0xfff, max: 0xfff, min: 0xfff}).valid() {
		t.Fatal("erased flash reads as a usable calibration")
	}
}

// Reports that are short or not report 9 are refused, and short calibration
// data decodes as none, without a panic.
func TestSwitch2RefusesShortInput(t *testing.T) {
	if _, ok := switch2ParseReport([]byte{9, 0, 0}); ok {
		t.Fatal("a 3-byte report was accepted")
	}
	if _, ok := switch2ParseReport(unhex(t, "05 35 1b 00 00 00 d6 27 8c 79 38 81")); ok {
		t.Fatal("report 5 was accepted as report 9")
	}
	if switch2ParseStick([]byte{1, 2, 3}).valid() || switch2FlashPayload([]byte{1, 2}) != nil {
		t.Fatal("short calibration data was used")
	}
}

// The start-up sequence follows the pad's framing: byte 5 of each command is
// the length of what follows its 8-byte header. The last command lights the
// player light for the slot.
func TestSwitch2CommandFraming(t *testing.T) {
	for slot := range 9 {
		cmds := switch2Init(slot)
		for i, c := range cmds {
			if len(c) < 8 || c[1] != 0x91 || len(c) != int(c[5])+8 {
				t.Fatalf("slot %d command %d is misframed: % x", slot, i, c)
			}
		}
	}
	if got := switch2Init(0)[len(switch2Init(0))-1][8]; got != 0x1 {
		t.Fatalf("slot 0 light = %#x, want 0x1", got)
	}
	if got := switch2Init(3)[len(switch2Init(3))-1][8]; got != 0xf {
		t.Fatalf("slot 3 light = %#x, want 0xf", got)
	}
	read := switch2FlashRead(0x1FC040)
	if len(read) != int(read[5])+8 || read[12] != 0x40 || read[13] != 0xc0 || read[14] != 0x1f || read[15] != 0 {
		t.Fatalf("flash read command = % x", read)
	}
}

// The bulk endpoints are found on the asked interface only, and a malformed
// descriptor is refused.
func TestUSBBulkEndpoints(t *testing.T) {
	config := unhex(t, "09 02 30 00 02 01 00 80 fa"+ // configuration
		"09 04 00 00 02 03 00 00 00"+ // interface 0, HID
		"07 05 81 03 40 00 04"+ // interrupt IN
		"07 05 01 03 40 00 04"+ // interrupt OUT
		"09 04 01 00 02 ff 00 00 00"+ // interface 1, vendor
		"07 05 02 02 40 00 00"+ // bulk OUT
		"07 05 82 02 40 00 00") // bulk IN
	in, out, ok := usbBulkEndpoints(config, 1)
	if !ok || in != 0x82 || out != 0x02 {
		t.Fatalf("interface 1 = in %#x out %#x ok %v, want 0x82 0x02 true", in, out, ok)
	}
	if _, _, ok := usbBulkEndpoints(config, 0); ok {
		t.Fatal("interface 0 has interrupt endpoints only, but bulk ones were found")
	}
	if _, _, ok := usbBulkEndpoints(append(config[:len(config):len(config)], 0), 1); ok {
		t.Fatal("a descriptor with a zero-length entry was accepted")
	}
	if _, _, ok := usbBulkEndpoints(config[:len(config)-3], 1); ok {
		t.Fatal("a truncated descriptor was accepted")
	}
}

// A browser's generic view of the pad maps by the same button positions, and
// its sticks reach full deflection with the nominal calibration: axes 0, 1
// (left) and 3, 5 (right), +Y up.
func TestSwitch2WebState(t *testing.T) {
	pressed := make([]bool, 21)
	pressed[0], pressed[11], pressed[13], pressed[20] = true, true, true, true // B, up, ZL, C
	axes := []float64{0.9, -0.9, 0, 0.02, 0, 0.4}
	s := switch2WebState(1, axes, pressed)
	if want := ButtonSouth | ButtonDPadUp | ButtonL2; s.Buttons != want {
		t.Fatalf("buttons = %016b, want %016b", s.Buttons, want)
	}
	if s.LeftTrigger != 1 || s.RightTrigger != 0 {
		t.Fatalf("triggers = (%v, %v), want (1, 0)", s.LeftTrigger, s.RightTrigger)
	}
	if s.LeftX != 1 || s.LeftY != -1 {
		t.Fatalf("left stick = (%v, %v), want full right and down", s.LeftX, s.LeftY)
	}
	if math.Abs(s.RightX) > 0.05 || s.RightY < 0.4 || s.RightY > 0.6 {
		t.Fatalf("right stick = (%v, %v), want about (0, 0.5)", s.RightX, s.RightY)
	}
	if !s.Connected || s.Index != 1 || s.Name != switch2Name {
		t.Fatalf("identity = %+v", s)
	}
	// Firefox reports four axes in report order: the right stick is 2 and 3.
	ff := switch2WebState(0, []float64{0.9, -0.9, 0.02, 0.4}, pressed)
	if ff.LeftX != 1 || ff.LeftY != -1 || math.Abs(ff.RightX) > 0.05 || ff.RightY < 0.4 || ff.RightY > 0.6 {
		t.Fatalf("four-axis sticks = (%v, %v) (%v, %v), want (1, -1) and about (0, 0.5)", ff.LeftX, ff.LeftY, ff.RightX, ff.RightY)
	}
	if ff.Buttons != s.Buttons {
		t.Fatalf("four-axis buttons = %016b, want %016b", ff.Buttons, s.Buttons)
	}
	if rest := switch2WebState(0, make([]float64, 6), nil); rest.LeftX != 0 && math.Abs(rest.LeftX) > 0.01 {
		t.Fatalf("centred axes read %v", rest.LeftX)
	}
	if got := switch2WebState(0, nil, nil); got.LeftX != 0 || got.Buttons != 0 {
		t.Fatalf("missing arrays = %+v, want everything at rest", got)
	}
}

// The pad is recognised in a browser by its vendor and product numbers,
// however the browser words the id, and only with the shape it is known by.
func TestIsSwitch2Web(t *testing.T) {
	for _, id := range []string{
		"Switch 2 Pro Controller (Vendor: 057e Product: 2069)",
		"057E-2069-Switch 2 Pro Controller",
	} {
		if !isSwitch2Web(id, 6, 21) || !isSwitch2Web(id, 4, 21) {
			t.Fatalf("%q was not recognised", id)
		}
	}
	if isSwitch2Web("Switch 2 Pro Controller (Vendor: 057e Product: 2069)", 5, 21) || isSwitch2Web("057e-2069-x", 4, 17) {
		t.Fatal("recognised with a different shape")
	}
	if isSwitch2Web("Pro Controller (Vendor: 057e Product: 2009)", 6, 21) || isSwitch2Web("", 6, 21) {
		t.Fatal("another controller was recognised")
	}
}
