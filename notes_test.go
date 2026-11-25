// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"slices"
	"testing"
)

// macOS speaks up when polls come from the wrong goroutine, when the system
// framework is missing, and when the built-in drivers are off; a program
// polling from its main goroutine hears nothing, and neither does one that
// has not polled yet.
func TestMacNotes(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		loaded, polled, fromMain, hidOff bool
		want                             []string
	}{
		{"polling from main", true, true, true, false, nil},
		{"not polled yet", true, false, false, false, nil},
		{"polling off main", true, true, false, false, []string{noteMacOffMain}},
		{"no framework", false, true, true, false, []string{noteMacNoFramework}},
		{"drivers off, main", true, true, true, true, []string{noteMacHIDOff}},
		{"drivers off, off main", true, true, false, true, []string{noteMacOffMain, noteMacHIDOff}},
	} {
		if got := macNotes(tc.loaded, tc.polled, tc.fromMain, tc.hidOff); !slices.Equal(got, tc.want) {
			t.Errorf("%s: notes = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A browser explains an empty list, and says when it lists a controller the
// package skipped.
func TestWebNotes(t *testing.T) {
	if got := webNotes(0, 0); !slices.Equal(got, []string{noteWebGesture}) {
		t.Errorf("no controllers: %q", got)
	}
	if got := webNotes(1, 0); got != nil {
		t.Errorf("one controller in use: %q, want nothing", got)
	}
	if got := webNotes(2, 1); !slices.Equal(got, []string{noteWebSkipped}) {
		t.Errorf("one skipped: %q", got)
	}
}

// Notes can be asked for at any time, before a Reader is opened too.
func TestNotesNeverPanics(t *testing.T) {
	_ = Notes()
	r := Open()
	_ = r.Poll()
	_ = Notes()
	_ = r.Close()
	_ = Notes()
}
