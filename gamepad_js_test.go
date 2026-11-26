// Copyright (c) 2026 Neomantra Corp.

//go:build js && wasm

package ntgamepad

import (
	"math"
	"slices"
	"syscall/js"
	"testing"

	"nimbleterminal.dev/ntgamepad/internal/browser"
)

// These tests run the browser reader itself, compiled to WebAssembly, against
// a navigator.getGamepads made up here: `task test:wasm` runs them in Node.

// fakePad is one entry of navigator.getGamepads().
type fakePad struct {
	index   int
	id      string
	mapping string
	axes    []float64
	pressed map[int]bool    // button index: pressed
	values  map[int]float64 // button index: analogue value
	buttons int
}

func (p fakePad) value() js.Value {
	axes := make([]any, len(p.axes))
	for i, a := range p.axes {
		axes[i] = a
	}
	buttons := make([]any, p.buttons)
	for i := range buttons {
		buttons[i] = map[string]any{"pressed": p.pressed[i], "value": p.values[i]}
	}
	return js.ValueOf(map[string]any{
		"index": p.index, "id": p.id, "mapping": p.mapping, "connected": true,
		"axes": axes, "buttons": buttons,
	})
}

// browserShows makes navigator.getGamepads return pads until the test ends;
// a nil entry is an empty slot, as browsers leave.
func browserShows(t *testing.T, pads ...*fakePad) {
	t.Helper()
	list := make([]any, len(pads))
	for i, p := range pads {
		if p != nil {
			list[i] = p.value()
		}
	}
	fn := js.FuncOf(func(js.Value, []js.Value) any { return js.ValueOf(list) })
	restore := installGetGamepads(fn.Value)
	t.Cleanup(func() {
		restore()
		fn.Release()
	})
}

// installGetGamepads makes navigator.getGamepads the given function and
// returns what puts things back. Node has a navigator of its own that cannot
// be replaced, only added to; where there is none, one is made.
func installGetGamepads(fn js.Value) (restore func()) {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		js.Global().Set("navigator", map[string]any{})
		nav = js.Global().Get("navigator")
	}
	old := nav.Get("getGamepads")
	nav.Set("getGamepads", fn)
	return func() { nav.Set("getGamepads", old) }
}

func noteCodes() []NoteCode {
	var out []NoteCode
	for _, n := range Notes() {
		out = append(out, n.Code)
	}
	return out
}

// A controller with the standard mapping reads in the standard layout, with
// the browser's +Y down turned to +Y up, and its browser index as its slot.
func TestWebStandardPad(t *testing.T) {
	browserShows(t, nil, &fakePad{
		index: 1, id: "Xbox Wireless Controller (STANDARD GAMEPAD)", mapping: "standard",
		axes: []float64{0.5, -1, 0, 0.25}, buttons: 17,
		pressed: map[int]bool{0: true, 12: true, 7: true},
		values:  map[int]float64{0: 1, 12: 1, 6: 0.3, 7: 0.9},
	})
	r := Open()
	defer r.Close()
	pads := r.PollAll(nil)
	if len(pads) != 1 {
		t.Fatalf("pads = %+v, want one", pads)
	}
	p := pads[0]
	if !p.Connected || p.Index != 1 || p.Name != "Xbox Wireless Controller (STANDARD GAMEPAD)" {
		t.Fatalf("identity = %+v", p)
	}
	if p.LeftX != 0.5 || p.LeftY != 1 || p.RightX != 0 || p.RightY != -0.25 {
		t.Fatalf("sticks = (%v, %v) (%v, %v), want (0.5, 1) (0, -0.25)", p.LeftX, p.LeftY, p.RightX, p.RightY)
	}
	if want := ButtonSouth | ButtonDPadUp | ButtonR2; p.Buttons != want {
		t.Fatalf("buttons = %016b, want %016b", p.Buttons, want)
	}
	if p.LeftTrigger != 0.3 || p.RightTrigger != 0.9 {
		t.Fatalf("triggers = (%v, %v), want (0.3, 0.9)", p.LeftTrigger, p.RightTrigger)
	}
	if got := r.Poll(); got != p {
		t.Fatalf("Poll = %+v, want the same pad", got)
	}
	if got := noteCodes(); got != nil {
		t.Fatalf("notes = %q with a pad in use, want none", got)
	}
}

// The Switch 2 Pro Controller as Chromium lists it (six axes, by usage) and
// as Firefox does (four, in report order) both read by the pad's layout: B
// is the bottom face button, and the right stick is found on either's axes.
func TestWebSwitch2InChromiumAndFirefox(t *testing.T) {
	for name, pad := range map[string]*fakePad{
		"chromium": {id: "Switch 2 Pro Controller (Vendor: 057e Product: 2069)", axes: []float64{0.9, -0.9, 0, 0, 0, 0.4}},
		"firefox":  {id: "057e-2069-Switch 2 Pro Controller", axes: []float64{0.9, -0.9, 0, 0.4}},
	} {
		t.Run(name, func(t *testing.T) {
			pad.buttons, pad.pressed = 21, map[int]bool{0: true, 13: true, 20: true}
			browserShows(t, pad)
			r := Open()
			defer r.Close()
			p := r.Poll()
			if !p.Connected || p.Name != switch2Name {
				t.Fatalf("pad = %+v, want the Switch 2 pad", p)
			}
			if want := ButtonSouth | ButtonL2; p.Buttons != want {
				t.Fatalf("buttons = %016b, want %016b (B and ZL; C is not reported)", p.Buttons, want)
			}
			if p.LeftX != 1 || p.LeftY != -1 || p.LeftTrigger != 1 {
				t.Fatalf("left stick (%v, %v), left trigger %v; want (1, -1) and 1", p.LeftX, p.LeftY, p.LeftTrigger)
			}
			if math.Abs(p.RightX) > 0.05 || p.RightY < 0.4 || p.RightY > 0.6 {
				t.Fatalf("right stick = (%v, %v), want about (0, 0.5)", p.RightX, p.RightY)
			}
		})
	}
}

// A controller with no standard mapping that the package does not know is
// skipped, and the notes say so; an empty list gets the button-press note.
func TestWebSkipsUnknownPadsAndSaysSo(t *testing.T) {
	browserShows(t, &fakePad{id: "Generic USB Joystick (Vendor: 0079 Product: 0006)", axes: []float64{0, 0}, buttons: 12})
	r := Open()
	defer r.Close()
	if pads := r.PollAll(nil); len(pads) != 0 {
		t.Fatalf("pads = %+v, want the unknown pad skipped", pads)
	}
	if got := noteCodes(); !slices.Equal(got, []NoteCode{NoteControllerSkipped}) {
		t.Fatalf("notes = %q, want the skipped note", got)
	}

	browserShows(t)
	if pads := r.PollAll(nil); len(pads) != 0 {
		t.Fatalf("pads = %+v with none listed", pads)
	}
	if got := noteCodes(); !slices.Equal(got, []NoteCode{NoteNeedsButtonPress}) {
		t.Fatalf("notes = %q, want the button-press note", got)
	}
}

// Several controllers come back in slot order, at most MaxPads; malformed
// entries are passed over without a panic; a closed reader reports nothing.
func TestWebSeveralPadsAndBadEntries(t *testing.T) {
	std := func(i int) *fakePad {
		return &fakePad{index: i, id: "pad", mapping: "standard", axes: []float64{0, 0, 0, 0}, buttons: 17}
	}
	browserShows(t, std(0), nil, std(2), std(3), std(4), std(5))
	r := Open()
	pads := r.PollAll(nil)
	if len(pads) != MaxPads {
		t.Fatalf("%d pads, want %d", len(pads), MaxPads)
	}
	// The browser numbered them 0, 2, 3, 4 and 5: the first three keep their
	// numbers, and the pad numbered 4 takes the free slot 1.
	for i, p := range pads {
		if p.Index != i {
			t.Fatalf("pad %d has slot %d; slots must be distinct, in order, and below MaxPads", i, p.Index)
		}
	}

	// Entries that are not gamepads at all.
	fn := js.FuncOf(func(js.Value, []js.Value) any {
		return js.ValueOf([]any{map[string]any{"connected": true}, 7, "pad", map[string]any{"connected": true, "mapping": "standard", "axes": "no", "buttons": 3}})
	})
	defer fn.Release()
	defer installGetGamepads(fn.Value)()
	for _, p := range r.PollAll(nil) {
		if p.LeftX != 0 || p.Buttons != 0 {
			t.Fatalf("a malformed entry read as %+v", p)
		}
	}

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if len(r.PollAll(nil)) != 0 || r.Poll().Connected {
		t.Fatal("a closed reader still reports controllers")
	}
}

// A surviving overflow pad keeps its slot when lower browser indexes come
// and go. Both demos see the assigned slot from the very same poll.
func TestWebKeepsSlotAcrossPolls(t *testing.T) {
	r := Open()
	defer r.Close()
	pad := func(i int) *fakePad { return &fakePad{index: i, id: "identical", mapping: "standard"} }
	check := func(want ...int) {
		t.Helper()
		pads := r.PollAll(nil)
		var slots []int
		browser.Visit(r, func(p browser.Pad) { slots = append(slots, p.Slot) })
		if !slices.Equal(slots, want) {
			t.Fatalf("browser entries got slots %v, want %v", slots, want)
		}
		ordered := slices.Clone(want)
		ordered = slices.DeleteFunc(ordered, func(n int) bool { return n < 0 })
		slices.Sort(ordered)
		var reported []int
		for _, p := range pads {
			reported = append(reported, p.Index)
		}
		if !slices.Equal(reported, ordered) {
			t.Fatalf("reported slots %v, want %v", reported, ordered)
		}
	}
	browserShows(t, pad(0), nil, nil, nil, pad(4))
	check(0, 1)
	browserShows(t, nil, nil, nil, nil, pad(4))
	check(1)
	browserShows(t, nil, pad(1), pad(2), pad(3), pad(4), pad(5))
	check(0, 2, 3, 1, -1)
	browserShows(t, nil, nil, pad(2), pad(3), pad(4), pad(5))
	check(2, 3, 1, 0)
}

// A document denied gamepad access reports no pads. The same reader can
// resume polling, preserving the slot of a controller that stayed connected.
func TestWebRecoversFromDeniedAccess(t *testing.T) {
	r := Open()
	defer r.Close()
	low := &fakePad{index: 0, id: "first", mapping: "standard"}
	high := &fakePad{index: 4, id: "survivor", mapping: "standard", buttons: 1, pressed: map[int]bool{0: true}}
	browserShows(t, low, nil, nil, nil, high)
	pads := r.PollAll(nil)
	if len(pads) != 2 || pads[1].Index != 1 {
		t.Fatalf("initial pads = %+v", pads)
	}

	deny := js.Global().Get("Function").New(`throw new DOMException("Gamepad access denied", "SecurityError")`)
	t.Cleanup(installGetGamepads(deny))
	if got := r.PollAll(pads); len(got) != 0 || r.Poll().Connected {
		t.Fatalf("denied poll reported controllers: %+v", got)
	}
	if got := noteCodes(); !slices.Equal(got, []NoteCode{NoteGamepadAccessDenied}) {
		t.Fatalf("denied poll notes = %v", got)
	}
	browserShows(t, nil, nil, nil, nil, high)
	got := r.Poll()
	if got.Index != 1 || got.Name != high.id || !got.Pressed(ButtonSouth) {
		t.Fatalf("recovered pad = %+v", got)
	}
	if got := Notes(); len(got) != 0 {
		t.Fatalf("notes after access returned = %v", got)
	}
}
