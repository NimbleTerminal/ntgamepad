// Copyright (c) 2026 Neomantra Corp.

//go:build js && wasm

package ntgamepad

import (
	"slices"
	"sync/atomic"
	"syscall/js"

	"nimbleterminal.dev/ntgamepad/internal/browser"
)

// webLastNote holds the NoteCode from the last completed browser poll.
var webLastNote atomic.Value

// webReader reads navigator.getGamepads(). A browser shows a page no
// controllers until the user presses a button on one. Controllers whose
// mapping is "standard" have a known layout; of the others, only the ones
// this package knows by name (the Switch 2 Pro Controller) are used.
type webReader struct {
	closed    bool
	available bool
	pads      []webPad
	// Each occupied slot holds its browser index plus one. Zero is free.
	slots [MaxPads]int
	axes  []float64
	down  []bool
	vals  []float64
}

type webPad struct {
	browser.Pad
	index int
	value js.Value
}

func (r *webReader) VisitBrowserPads(visit func(browser.Pad)) bool {
	for _, pad := range r.pads {
		visit(pad.Pad)
	}
	return r.available
}

func openPlatform() Reader { return &webReader{} }

func (r *webReader) Close() error {
	*r = webReader{closed: true}
	return nil
}

func (r *webReader) Poll() State { return first(r.PollAll(nil)) }

func (r *webReader) PollAll(dst []State) []State {
	dst = dst[:0]
	clear(r.pads)
	r.pads = r.pads[:0]
	r.available = false
	if r.closed {
		return dst
	}
	note := NoteNeedsButtonPress
	defer func() { webLastNote.Store(note) }()
	nav := js.Global().Get("navigator")
	if nav.Type() != js.TypeObject || nav.Get("getGamepads").Type() != js.TypeFunction {
		return dst
	}
	list, denied := getGamepads(nav)
	if denied {
		note = NoteGamepadAccessDenied
		return dst
	}
	if list.Type() != js.TypeObject {
		return dst
	}
	r.available = true
	previous := r.slots
	clear(r.slots[:])
	var skipped int
	for i, count := 0, list.Length(); i < count; i++ {
		gp := list.Index(i)
		if gp.Type() != js.TypeObject || !gp.Get("connected").Truthy() {
			continue
		}
		p := webPad{index: i, value: gp}
		if idx := gp.Get("index"); idx.Type() == js.TypeNumber && idx.Int() >= 0 {
			p.index = idx.Int()
		}
		if id := gp.Get("id"); id.Type() == js.TypeString {
			p.Name = id.String()
		}
		p.Axes, p.Buttons = arrayLength(gp.Get("axes")), arrayLength(gp.Get("buttons"))
		p.Standard = gp.Get("mapping").Equal(js.ValueOf("standard"))
		p.Supported = p.Standard || isSwitch2Web(p.Name, p.Axes, p.Buttons)
		p.Slot = -1
		if p.Supported {
			// Reserve returning pads before new arrivals can claim their slots.
			if slot := slices.Index(previous[:], p.index+1); slot >= 0 && r.slots[slot] == 0 {
				p.Slot, r.slots[slot] = slot, p.index+1
			}
		} else {
			skipped++
		}
		r.pads = append(r.pads, p)
	}
	for i := range r.pads {
		p := &r.pads[i]
		if !p.Supported {
			continue
		}
		if p.Slot < 0 {
			if slices.Contains(r.slots[:], p.index+1) {
				continue // repeated browser index
			}
			slot := p.index
			if slot >= MaxPads || r.slots[slot] != 0 {
				slot = slices.Index(r.slots[:], 0)
			}
			if slot < 0 {
				continue
			}
			p.Slot, r.slots[slot] = slot, p.index+1
		}
		dst = append(dst, r.read(p))
	}
	note = webNoteCode(len(r.pads), skipped)
	slices.SortFunc(dst, func(a, b State) int { return a.Index - b.Index })
	return dst
}

// getGamepads translates the browser's permissions-policy exception at the JS call.
func getGamepads(nav js.Value) (list js.Value, denied bool) {
	defer func() {
		if failure := recover(); failure != nil {
			if err, ok := failure.(js.Error); ok && err.Value.Type() == js.TypeObject &&
				err.Value.Get("name").Equal(js.ValueOf("SecurityError")) {
				denied = true
				return
			}
			panic(failure)
		}
	}()
	return nav.Call("getGamepads"), false
}

func arrayLength(v js.Value) int {
	if v.Type() != js.TypeObject {
		return 0
	}
	return v.Length()
}

func (r *webReader) read(p *webPad) State {
	r.axes, r.down, r.vals = r.axes[:0], r.down[:0], r.vals[:0]
	axes := p.value.Get("axes")
	for j := range min(p.Axes, 8) {
		r.axes = append(r.axes, number(axes.Index(j)))
	}
	btns := p.value.Get("buttons")
	for j := range min(p.Buttons, 32) {
		b := btns.Index(j)
		var down bool
		var value float64
		if b.Type() == js.TypeObject {
			down = b.Get("pressed").Truthy()
			value = number(b.Get("value"))
		}
		r.down, r.vals = append(r.down, down), append(r.vals, value)
	}
	if p.Standard {
		return standardState(p.Slot, p.Name, r.axes, r.down, r.vals)
	}
	return switch2WebState(p.Slot, r.axes, r.down)
}

// number is v as a float, or zero when v is not a number.
func number(v js.Value) float64 {
	if v.Type() != js.TypeNumber {
		return 0
	}
	return v.Float()
}

// platformNotes is what the browser has to say to the developer.
func platformNotes() []Note {
	code, ok := webLastNote.Load().(NoteCode)
	if !ok {
		return notes(NoteNeedsButtonPress)
	}
	if code == "" {
		return nil
	}
	return notes(code)
}
