// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"math"
	"testing"
)

// inRange fails the test unless s is a State a caller may rely on: sticks in
// [-1, 1], triggers in [0, 1], nothing NaN.
func inRange(t *testing.T, s State) {
	t.Helper()
	for name, v := range map[string]float64{"LeftX": s.LeftX, "LeftY": s.LeftY, "RightX": s.RightX, "RightY": s.RightY} {
		if !(v >= -1 && v <= 1) {
			t.Fatalf("%s = %v, outside [-1, 1]", name, v)
		}
	}
	for name, v := range map[string]float64{"LeftTrigger": s.LeftTrigger, "RightTrigger": s.RightTrigger} {
		if !(v >= 0 && v <= 1) {
			t.Fatalf("%s = %v, outside [0, 1]", name, v)
		}
	}
}

// FuzzJoydev feeds a Linux joystick device arbitrary bytes, split anywhere
// as reads may be. It never panics, uses only whole events, ends in the same
// state however the bytes were split, and reports a State in range.
func FuzzJoydev(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0xff, 0x7f, 0x02, 0x00, 0, 0, 0, 0, 1, 0, 0x81, 0x03}, 5)
	f.Add([]byte{1, 2, 3}, 1)
	f.Add([]byte{0, 0, 0, 0, 0x00, 0x80, 0x02, 0xff, 0, 0, 0, 0, 1, 0, 0x01, 0xff}, 9)
	f.Fuzz(func(t *testing.T, data []byte, split int) {
		var whole joydev
		if used := whole.feed(data); used != len(data)/joydevEventSize*joydevEventSize {
			t.Fatalf("feed used %d of %d bytes", used, len(data))
		}
		inRange(t, whole.state(0, ""))

		// The same bytes arriving in two reads, with the partial event kept.
		if split < 0 {
			split = -split
		}
		if len(data) > 0 {
			split %= len(data) + 1
		} else {
			split = 0
		}
		var parts joydev
		used := parts.feed(data[:split])
		rest := append(append([]byte(nil), data[used:split]...), data[split:]...)
		parts.feed(rest)
		if parts.state(0, "") != whole.state(0, "") {
			t.Fatalf("split at %d changed the state: %+v, want %+v", split, parts.state(0, ""), whole.state(0, ""))
		}
	})
}

// FuzzStandardState maps arbitrary browser gamepad arrays. It never panics
// or indexes past a short array, and reports a State in range.
func FuzzStandardState(f *testing.F) {
	f.Add([]byte{1, 0, 1}, 0.5, -2.0, math.NaN(), math.Inf(1), 0.4, 7.0)
	f.Add([]byte{}, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0)
	f.Fuzz(func(t *testing.T, down []byte, a0, a1, a2, a3, v6, v7 float64) {
		pressed := make([]bool, len(down))
		values := make([]float64, len(down))
		for i, b := range down {
			pressed[i] = b&1 != 0
			values[i] = float64(b) / 128
		}
		if len(values) > standardRightTrigger {
			values[standardLeftTrigger], values[standardRightTrigger] = v6, v7
		}
		axes := []float64{a0, a1, a2, a3}
		for n := 0; n <= len(axes); n++ {
			s := standardState(1, "pad", axes[:n], pressed, values)
			inRange(t, s)
			if !s.Connected || s.Index != 1 {
				t.Fatalf("identity = %+v", s)
			}
			inRange(t, s.WithDeadzone(a0))
		}
	})
}

// FuzzSwitch2 decodes arbitrary bytes as the Switch 2 pad's flash replies and
// input report, and arbitrary numbers as a browser's view of it. Nothing
// panics, a calibration made of anything still yields a State in range, and
// a report is accepted only when it is report 9 and long enough.
func FuzzSwitch2(f *testing.F) {
	f.Add([]byte{0xb6, 0x47, 0x87, 0x8e, 0x36, 0x66, 0x86, 0xa6, 0x64}, []byte{9, 0x35, 0x1b, 1, 8, 0, 0xd6, 0x27, 0x8c, 0x79, 0x38, 0x81}, 0.9, -0.9)
	f.Add([]byte{}, []byte{}, math.NaN(), math.Inf(-1))
	f.Add([]byte{0xb2, 0xa1, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []byte{9}, 2.0, -2.0)
	f.Fuzz(func(t *testing.T, flash, report []byte, ax, ay float64) {
		cal := switch2Calibration{
			left:  switch2StickFrom(switch2FlashPayload(flash), flash),
			right: switch2StickFrom(flash, switch2FlashPayload(flash)),
		}
		_ = switch2ParseStick(flash)
		in, ok := switch2ParseReport(report)
		if ok != (len(report) >= 12 && report[0] == switch2ReportPro) {
			t.Fatalf("report of %d bytes accepted: %v", len(report), ok)
		}
		if in.lx > 0xfff || in.ly > 0xfff || in.rx > 0xfff || in.ry > 0xfff {
			t.Fatalf("a stick reading exceeds 12 bits: %+v", in)
		}
		inRange(t, switch2State(0, in, cal))
		inRange(t, switch2State(0, switch2Input{buttons: 0xffffffff, lx: 0xffff, ly: 0, rx: 0xfff, ry: 1}, cal))

		pressed := make([]bool, len(report))
		for i, b := range report {
			pressed[i] = b&1 != 0
		}
		for _, axes := range [][]float64{nil, {ax}, {ax, ay, ay, ax}, {ax, ay, 0, ay, 0, ax}} {
			inRange(t, switch2WebState(0, axes, pressed))
		}
		_ = isSwitch2Web(string(report), len(flash), len(report))
	})
}

// FuzzUSBBulkEndpoints walks arbitrary bytes as a USB configuration
// descriptor. It never panics or loops, and any endpoints it reports have
// the direction it says.
func FuzzUSBBulkEndpoints(f *testing.F) {
	f.Add([]byte{9, 4, 1, 0, 2, 0xff, 0, 0, 0, 7, 5, 0x02, 2, 0x40, 0, 0, 7, 5, 0x82, 2, 0x40, 0, 0}, byte(1))
	f.Add([]byte{0}, byte(0))
	f.Add([]byte{2, 5, 2, 4}, byte(2))
	f.Fuzz(func(t *testing.T, config []byte, iface byte) {
		in, out, ok := usbBulkEndpoints(config, iface)
		if ok && (in&0x80 == 0 || out&0x80 != 0) {
			t.Fatalf("endpoints in %#x out %#x have the wrong direction", in, out)
		}
	})
}
