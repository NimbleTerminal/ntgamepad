// Copyright (c) 2026 Neomantra Corp.

//go:build js && wasm

package ntgamepad

import "syscall/js"

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
	if !pads.Truthy() {
		return dst
	}
	for i := 0; i < pads.Length() && len(dst) < MaxPads; i++ {
		gp := pads.Index(i)
		if !gp.Truthy() || !gp.Get("connected").Bool() {
			continue
		}
		r.axes, r.down, r.vals = r.axes[:0], r.down[:0], r.vals[:0]
		if axes := gp.Get("axes"); axes.Truthy() {
			for j := 0; j < axes.Length() && j < 8; j++ {
				r.axes = append(r.axes, number(axes.Index(j)))
			}
		}
		if btns := gp.Get("buttons"); btns.Truthy() {
			for j := 0; j < btns.Length() && j < 32; j++ {
				b := btns.Index(j)
				if !b.Truthy() {
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
		index := len(dst)
		if idx := gp.Get("index"); idx.Type() == js.TypeNumber {
			if n := idx.Int(); n >= 0 && n < MaxPads {
				index = n
			}
		}
		standard := false
		if m := gp.Get("mapping"); m.Type() == js.TypeString {
			standard = m.String() == "standard"
		}
		switch {
		case standard:
			dst = append(dst, standardState(index, name, r.axes, r.down, r.vals))
		case isSwitch2Web(name, len(r.axes), len(r.down)):
			dst = append(dst, switch2WebState(index, r.axes, r.down))
		}
	}
	return dst
}

// number is v as a float, or zero when v is not a number.
func number(v js.Value) float64 {
	if v.Type() != js.TypeNumber {
		return 0
	}
	return v.Float()
}
