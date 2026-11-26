// Copyright (c) 2026 Neomantra Corp.

//go:build js && wasm

package main

import (
	"nimbleterminal.dev/ntgamepad/internal/browser"

	"nimbleterminal.dev/ntgamepad"
)

// inBrowser reports that controllers come from the browser's Gamepad API.
const inBrowser = true

// browserPads describes each controller the browser itself reports, and
// whether ntgamepad used it: a controller the package skips can then be told
// from one the browser cannot see.
func browserPads(reader ntgamepad.Reader) []string {
	var out []string
	available := browser.Visit(reader, func(p browser.Pad) {
		out = append(out, p.Name+" · "+p.Description())
	})
	if !available {
		return []string{"this page cannot access the Gamepad API"}
	}
	return out
}
