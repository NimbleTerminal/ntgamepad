// Copyright (c) 2026 Neomantra Corp.

//go:build !js

package main

import "nimbleterminal.dev/ntgamepad"

// inBrowser reports that controllers come from the browser's Gamepad API.
const inBrowser = false

// browserPads is empty outside a browser.
func browserPads([]ntgamepad.State) []string { return nil }
