// Copyright (c) 2026 Neomantra Corp.

//go:build js && wasm

// Command ntgamepad-web is the browser demo: a page that shows what
// ntgamepad reads from the controllers the browser offers, redrawn every
// frame. Build it with `task web:build` and open it with `task web:serve`.
//
// A browser shows a page no controllers until a button is pressed on one.
// Below the controllers ntgamepad reports, the page lists every controller
// the browser itself knows, so one that ntgamepad skips (because the browser
// gives it no standard mapping) can be told from one the browser cannot see.
package main

import (
	"fmt"
	"html"
	"strings"
	"syscall/js"

	"nimbleterminal.dev/ntgamepad"
)

var buttons = []struct {
	bit  ntgamepad.Button
	name string
}{
	{ntgamepad.ButtonDPadUp, "↑"}, {ntgamepad.ButtonDPadDown, "↓"},
	{ntgamepad.ButtonDPadLeft, "←"}, {ntgamepad.ButtonDPadRight, "→"},
	{ntgamepad.ButtonSouth, "South"}, {ntgamepad.ButtonEast, "East"},
	{ntgamepad.ButtonWest, "West"}, {ntgamepad.ButtonNorth, "North"},
	{ntgamepad.ButtonL1, "L1"}, {ntgamepad.ButtonR1, "R1"},
	{ntgamepad.ButtonL2, "L2"}, {ntgamepad.ButtonR2, "R2"},
	{ntgamepad.ButtonSelect, "Select"}, {ntgamepad.ButtonStart, "Start"},
	{ntgamepad.ButtonL3, "L3"}, {ntgamepad.ButtonR3, "R3"},
}

func main() {
	doc := js.Global().Get("document")
	out := doc.Call("getElementById", "pads")
	seen := doc.Call("getElementById", "browser")
	pad := ntgamepad.Open()
	defer pad.Close()

	var pads []ntgamepad.State
	prev := map[int]ntgamepad.State{}
	presses := map[int]int{}
	var lastPads, lastSeen string

	var frame js.Func
	frame = js.FuncOf(func(js.Value, []js.Value) any {
		pads = pad.PollAll(pads)
		var b strings.Builder
		if len(pads) == 0 {
			b.WriteString(`<p class="empty">No controller yet. Connect one and press any button on it.</p>`)
		}
		for _, raw := range pads {
			presses[raw.Index] += countBits(raw.JustPressed(prev[raw.Index]))
			prev[raw.Index] = raw
			writePad(&b, raw.WithDeadzone(0), presses[raw.Index])
		}
		if s := b.String(); s != lastPads {
			out.Set("innerHTML", s)
			lastPads = s
		}
		if s := browserPads(pads); s != lastSeen {
			seen.Set("innerHTML", s)
			lastSeen = s
		}
		js.Global().Call("requestAnimationFrame", frame)
		return nil
	})
	js.Global().Call("requestAnimationFrame", frame)
	select {}
}

func countBits(b ntgamepad.Button) int {
	n := 0
	for ; b != 0; b &= b - 1 {
		n++
	}
	return n
}

func writePad(b *strings.Builder, p ntgamepad.State, presses int) {
	fmt.Fprintf(b, `<section class="pad"><h2>Pad %d <span>%s</span></h2>`, p.Index, html.EscapeString(p.Name))
	b.WriteString(`<div class="row">`)
	stick(b, "Left stick", p.LeftX, p.LeftY)
	stick(b, "Right stick", p.RightX, p.RightY)
	fmt.Fprintf(b, `<div class="triggers"><label>LT %.2f<meter min="0" max="1" value="%.3f"></meter></label>`+
		`<label>RT %.2f<meter min="0" max="1" value="%.3f"></meter></label>`+
		`<p>%d button presses</p></div>`, p.LeftTrigger, p.LeftTrigger, p.RightTrigger, p.RightTrigger, presses)
	b.WriteString(`</div><div class="buttons">`)
	for _, btn := range buttons {
		class := "btn"
		if p.Buttons&btn.bit != 0 {
			class = "btn on"
		}
		fmt.Fprintf(b, `<span class="%s">%s</span>`, class, btn.name)
	}
	b.WriteString(`</div></section>`)
}

// stick draws a stick as a dot in a square; +Y is up.
func stick(b *strings.Builder, name string, x, y float64) {
	fmt.Fprintf(b, `<figure><svg viewBox="-1.2 -1.2 2.4 2.4" role="img" aria-label="%s at %.2f, %.2f">`+
		`<circle r="1" class="well"/><line x1="-1" x2="1" class="axis"/><line y1="-1" y2="1" class="axis"/>`+
		`<circle cx="%.3f" cy="%.3f" r="0.16" class="dot"/></svg><figcaption>%s<br>%+.2f, %+.2f</figcaption></figure>`,
		name, x, y, x, -y, name, x, y)
}

// browserPads lists what navigator.getGamepads() itself returns, and for each
// whether ntgamepad reported it.
func browserPads(used []ntgamepad.State) string {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() || nav.Get("getGamepads").Type() != js.TypeFunction {
		return `<p class="empty">This browser has no Gamepad API.</p>`
	}
	list := nav.Call("getGamepads")
	var b strings.Builder
	for i := 0; i < list.Length(); i++ {
		gp := list.Index(i)
		if !gp.Truthy() {
			continue
		}
		mapping := gp.Get("mapping").String()
		if mapping != "standard" {
			mapping = "none"
		}
		note := "skipped: no standard mapping, and not a controller ntgamepad knows by name"
		for _, p := range used {
			if p.Index == gp.Get("index").Int() {
				note = "used"
				if mapping == "none" {
					note = "used, with ntgamepad's own mapping"
				}
			}
		}
		fmt.Fprintf(&b, `<li><code>%s</code> · %d axes, %d buttons · mapping: %s · %s</li>`,
			html.EscapeString(gp.Get("id").String()), gp.Get("axes").Length(), gp.Get("buttons").Length(), mapping, note)
	}
	if b.Len() == 0 {
		return `<p class="empty">The browser reports no controllers.</p>`
	}
	return "<ul>" + b.String() + "</ul>"
}
