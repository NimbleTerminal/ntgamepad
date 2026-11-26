// Copyright (c) 2026 Neomantra Corp.

//go:build linux && !js

package ntgamepad

import (
	"slices"
	"syscall"
	"testing"
)

// Denial on one device survives checks of the other slots in the same poll.
// Removing that device clears the note on the next poll.
func TestLinuxPermissionNoteTracksLastPoll(t *testing.T) {
	r := &linuxReader{}
	defer r.Close()
	denied := true
	open := func(path string, flags int, mode uint32) (int, error) {
		if denied && path == "/dev/input/js0" {
			return -1, syscall.EACCES
		}
		return -1, syscall.ENOENT
	}
	r.pollAll(nil, open)
	if got := codesOf(Notes()); !slices.Equal(got, []NoteCode{NotePermissionDenied}) {
		t.Fatalf("after denial: %v", got)
	}
	denied = false
	r.pollAll(nil, open)
	if got := Notes(); len(got) != 0 {
		t.Fatalf("after removal: %v", got)
	}
}
