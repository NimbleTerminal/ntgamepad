// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"math"
	"strings"
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
	f.Add([]byte{1, 2}, math.MinInt)
	f.Add([]byte{0, 0, 0, 0, 0x00, 0x80, 0x02, 0xff, 0, 0, 0, 0, 1, 0, 0x01, 0xff}, 9)
	f.Fuzz(func(t *testing.T, data []byte, split int) {
		var whole joydev
		if used := whole.feed(data); used != len(data)/joydevEventSize*joydevEventSize {
			t.Fatalf("feed used %d of %d bytes", used, len(data))
		}
		inRange(t, whole.state(0, ""))

		// The same bytes arriving in two reads, with the partial event kept.
		split %= len(data) + 1
		if split < 0 {
			split = -split
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
// input report. Arbitrary calibration data still yields a State in range, and
// a report is accepted only when it is report 9 and long enough.
func FuzzSwitch2(f *testing.F) {
	f.Add(unhex(f, switch2LeftFactoryReply), unhex(f, switch2RestReport))
	f.Add([]byte{0xb6, 0x47, 0x87, 0x8e, 0x36, 0x66, 0x86, 0xa6, 0x64}, []byte{9, 0x35, 0x1b, 1, 8, 0, 0xd6, 0x27, 0x8c, 0x79, 0x38, 0x81})
	f.Add([]byte{}, []byte{})
	f.Add([]byte{0xb2, 0xa1, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []byte{9})
	f.Fuzz(func(t *testing.T, flash, report []byte) {
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
	})
}

// FuzzSwitch2Web exercises ID recognition and browser mapping with independent
// button and axis arrays, including partial arrays and non-finite readings.
func FuzzSwitch2Web(f *testing.F) {
	f.Add("Switch 2 Pro Controller (Vendor: 057e Product: 2069)", []byte{127, 128, 0, 25, 0, 190}, make([]byte, 21), 0.9)
	f.Add("057e-2069-Switch 2 Pro Controller", []byte{127, 128, 25, 190}, make([]byte, 21), 0.0)
	f.Add("", []byte{0}, []byte{}, math.NaN())
	f.Add("", []byte{}, []byte{}, math.Inf(-1))
	f.Fuzz(func(t *testing.T, id string, raw, buttons []byte, x float64) {
		axes := make([]float64, min(len(raw), 8))
		for i := range axes {
			axes[i] = (float64(raw[i]) - 128) / 127
		}
		if len(axes) > 0 {
			axes[0] = x
		}
		down := make([]bool, min(len(buttons), 32))
		for i := range down {
			down[i] = buttons[i]&1 != 0
		}
		lower := strings.ToLower(id)
		want := (len(axes) == 4 || len(axes) == 6) && len(buttons) == 21 &&
			strings.Contains(lower, "057e") && strings.Contains(lower, "2069")
		if isSwitch2Web(id, len(axes), len(buttons)) != want {
			t.Fatalf("unexpected recognition for %q, %d axes, %d buttons", id, len(axes), len(buttons))
		}
		s := switch2WebState(0, axes, down)
		inRange(t, s)
		if len(axes) == 6 {
			firefox := []float64{axes[0], axes[1], axes[3], axes[5]}
			if s != switch2WebState(0, firefox, down) {
				t.Fatal("Chromium and Firefox disagree for equivalent stick inputs")
			}
		}
	})
}

// FuzzDualSense checks that short enhanced reports are rejected, axes stay
// in range, and the trigger buttons follow travel even if their bits differ.
func FuzzDualSense(f *testing.F) {
	f.Add(unhex(f, dualSenseRestReport))
	f.Add(make([]byte, 10))
	f.Add([]byte{0xd1, 0, 255, 255, 0, 127, 128, 0, 15, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, report []byte) {
		in, ok := dualSenseParseEnhanced(report)
		if ok != (len(report) >= 11) {
			t.Fatalf("enhanced report of %d bytes accepted: %v", len(report), ok)
		}
		if !ok {
			return
		}
		s := dualSenseState(0, in)
		inRange(t, s)
		if s.Pressed(ButtonL2) != (report[5] >= 128) || s.Pressed(ButtonR2) != (report[6] >= 128) {
			t.Fatalf("trigger bytes %d, %d produced buttons %016b", report[5], report[6], s.Buttons)
		}
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
