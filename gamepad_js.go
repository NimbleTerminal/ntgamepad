// Copyright (c) 2026 Neomantra Corp.

//go:build js && wasm

package ntgamepad

import (
	"slices"
	"sync/atomic"
	"syscall/js"
)

// webListed and webSkipped are how many controllers the browser showed the
// page at the last poll, and how many of those the reader skipped.
var webListed, webSkipped atomic.Int32

// webReader reads navigator.getGamepads(). A browser shows a page no
// controllers until the user presses a button on one. Controllers whose
// mapping is "standard" have a known layout; of the others, only the ones
// this package knows by name (the Switch 2 Pro Controller) are used.
type webReader struct {
	closed bool
	axes   []float64
	down   []bool
	vals   []float64
}

func openPlatform() Reader { return &webReader{} }

func (r *webReader) Close() error {
	r.closed = true
	return nil
}

func (r *webReader) Poll() State { return first(r.PollAll(nil)) }

func (r *webReader) PollAll(dst []State) []State {
	dst = dst[:0]
	if r.closed {
		return dst
	}
	nav := js.Global().Get("navigator")
	if !nav.Truthy() || nav.Get("getGamepads").Type() != js.TypeFunction {
		return dst
	}
	pads := nav.Call("getGamepads")
	if pads.Type() != js.TypeObject {
		return dst
	}
	var listed, skipped int32
	var taken [MaxPads]bool
	defer func() { webListed.Store(listed); webSkipped.Store(skipped) }()
	for i := 0; i < pads.Length() && len(dst) < MaxPads; i++ {
		gp := pads.Index(i)
		// A slot the browser has left empty is null; anything else that is
		// not a connected gamepad object is passed over too.
		if gp.Type() != js.TypeObject || !gp.Get("connected").Truthy() {
			continue
		}
		listed++
		r.axes, r.down, r.vals = r.axes[:0], r.down[:0], r.vals[:0]
		if axes := gp.Get("axes"); axes.Type() == js.TypeObject {
			for j := 0; j < axes.Length() && j < 8; j++ {
				r.axes = append(r.axes, number(axes.Index(j)))
			}
		}
		if btns := gp.Get("buttons"); btns.Type() == js.TypeObject {
			for j := 0; j < btns.Length() && j < 32; j++ {
				b := btns.Index(j)
				if b.Type() != js.TypeObject {
					r.down, r.vals = append(r.down, false), append(r.vals, 0)
					continue
				}
				r.down = append(r.down, b.Get("pressed").Truthy())
				r.vals = append(r.vals, number(b.Get("value")))
			}
		}
		name := ""
		if id := gp.Get("id"); id.Type() == js.TypeString {
			name = id.String()
		}
		// The browser's own index is the slot, when it is one of ours and
		// free; a pad the browser numbers past MaxPads takes the lowest free.
		index := -1
		if idx := gp.Get("index"); idx.Type() == js.TypeNumber {
			if n := idx.Int(); n >= 0 && n < MaxPads && !taken[n] {
				index = n
			}
		}
		for n := 0; index < 0 && n < MaxPads; n++ {
			if !taken[n] {
				index = n
			}
		}
		if index < 0 {
			break // every slot is in use
		}
		standard := false
		if m := gp.Get("mapping"); m.Type() == js.TypeString {
			standard = m.String() == "standard"
		}
		switch {
		case standard:
			dst = append(dst, standardState(index, name, r.axes, r.down, r.vals))
			taken[index] = true
		case isSwitch2Web(name, len(r.axes), len(r.down)):
			dst = append(dst, switch2WebState(index, r.axes, r.down))
			taken[index] = true
		default:
			skipped++
		}
	}
	slices.SortFunc(dst, func(a, b State) int { return a.Index - b.Index })
	return dst
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
	return webNotes(int(webListed.Load()), int(webSkipped.Load()))
}
