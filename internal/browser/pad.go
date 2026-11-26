// Copyright (c) 2026 Neomantra Corp.

// Package browser shares the last poll's diagnostics with the two browser demos.
package browser

import "fmt"

// Pad describes a connected entry in navigator.getGamepads.
type Pad struct {
	Name                string
	Axes, Buttons       int
	Standard, Supported bool
	Slot                int // -1 when the reader skipped this pad
}

func (p Pad) Description() string {
	mapping := "none"
	if p.Standard {
		mapping = "standard"
	}
	var status string
	switch {
	case p.Slot >= 0:
		status = fmt.Sprintf("used in slot %d", p.Slot)
	case p.Supported:
		status = "skipped (all slots occupied)"
	default:
		status = "skipped (unknown mapping)"
	}
	return fmt.Sprintf("%d axes, %d buttons · mapping %s · %s", p.Axes, p.Buttons, mapping, status)
}

// Visit reports whether the Gamepad API was available at the last poll.
// Only the WebAssembly reader implements this internal diagnostic interface.
func Visit(reader any, visit func(Pad)) bool {
	r, ok := reader.(interface{ VisitBrowserPads(func(Pad)) bool })
	return ok && r.VisitBrowserPads(visit)
}
