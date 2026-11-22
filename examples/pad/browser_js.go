// Copyright (c) 2026 Neomantra Corp.

//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"nimbleterminal.dev/ntgamepad"
)

// inBrowser reports that controllers come from the browser's Gamepad API.
const inBrowser = true

// browserPads describes each controller the browser itself reports, and
// whether ntgamepad used it: a controller the package skips can then be told
// from one the browser cannot see.
func browserPads(used []ntgamepad.State) []string {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() || nav.Get("getGamepads").Type() != js.TypeFunction {
		return []string{"this browser has no Gamepad API"}
	}
	list := nav.Call("getGamepads")
	var out []string
	for i := 0; i < list.Length(); i++ {
		gp := list.Index(i)
		if !gp.Truthy() {
			continue
		}
		mapping := "none"
		if m := gp.Get("mapping"); m.Type() == js.TypeString && m.String() == "standard" {
			mapping = "standard"
		}
		note := "skipped (no standard mapping, and not known by name)"
		for _, p := range used {
			if p.Index == gp.Get("index").Int() {
				note = "used"
				if mapping == "none" {
					note = "used, with ntgamepad's own mapping"
				}
			}
		}
		out = append(out, fmt.Sprintf("%s · %d axes, %d buttons · mapping %s · %s",
			gp.Get("id").String(), gp.Get("axes").Length(), gp.Get("buttons").Length(), mapping, note))
	}
	return out
}
