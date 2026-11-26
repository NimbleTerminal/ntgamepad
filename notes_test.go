// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import (
	"slices"
	"strings"
	"testing"
)

// codesOf is the codes of some notes, in order.
func codesOf(ns []Note) []NoteCode {
	var out []NoteCode
	for _, n := range ns {
		out = append(out, n.Code)
	}
	return out
}

// macOS speaks up when polls come from the wrong goroutine, when the system
// framework is missing, and when the built-in drivers are off; a program
// polling from its main goroutine hears nothing, and neither does one that
// has not polled yet.
func TestMacNotes(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		loaded, polled, fromMain, hidOff bool
		want                             []NoteCode
	}{
		{"polling from main", true, true, true, false, nil},
		{"not polled yet", true, false, false, false, nil},
		{"polling off main", true, true, false, false, []NoteCode{NotePollOffMainGoroutine}},
		{"no framework", false, true, true, false, []NoteCode{NoteFrameworkUnavailable}},
		{"drivers off, main", true, true, true, true, []NoteCode{NoteDriversOff}},
		{"drivers off, off main", true, true, false, true, []NoteCode{NotePollOffMainGoroutine, NoteDriversOff}},
	} {
		if got := codesOf(macNotes(tc.loaded, tc.polled, tc.fromMain, tc.hidOff)); !slices.Equal(got, tc.want) {
			t.Errorf("%s: notes = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A browser explains an empty list, and says when it lists a controller the
// package skipped.
func TestWebNotes(t *testing.T) {
	if got := webNoteCode(0, 0); got != NoteNeedsButtonPress {
		t.Errorf("no controllers: %q", got)
	}
	if got := webNoteCode(1, 0); got != "" {
		t.Errorf("one controller in use: %v, want nothing", got)
	}
	if got := webNoteCode(2, 1); got != NoteControllerSkipped {
		t.Errorf("one skipped: %q", got)
	}
}

// Every code has a sentence, a note prints as its sentence, and the codes
// are the stable strings a program may have stored or matched.
func TestEveryNoteCodeHasText(t *testing.T) {
	stable := map[NoteCode]string{
		NotePollOffMainGoroutine: "poll-off-main-goroutine",
		NoteFrameworkUnavailable: "framework-unavailable",
		NoteDriversOff:           "drivers-off",
		NoteGamepadAccessDenied:  "gamepad-access-denied",
		NoteNeedsButtonPress:     "needs-button-press",
		NoteControllerSkipped:    "controller-skipped",
		NotePermissionDenied:     "permission-denied",
		NoteUnsupportedPlatform:  "unsupported-platform",
	}
	if len(noteText) != len(stable) {
		t.Fatalf("%d codes have text, %d are listed here", len(noteText), len(stable))
	}
	for code, want := range stable {
		if string(code) != want {
			t.Errorf("code %q changed; it was %q", code, want)
		}
		ns := notes(code)
		if len(ns) != 1 || ns[0].Code != code || !strings.HasSuffix(ns[0].Text, ".") || len(ns[0].Text) < 20 {
			t.Errorf("code %q: note %+v, want one note with a full sentence", code, ns)
		}
		if ns[0].String() != ns[0].Text {
			t.Errorf("code %q: String() is not the text", code)
		}
	}
	if notes() != nil {
		t.Error("no codes made a non-nil slice")
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
