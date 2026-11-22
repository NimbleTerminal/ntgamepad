// Copyright (c) 2026 Neomantra Corp.

package ntgamepad_test

import (
	"fmt"

	"nimbleterminal.dev/ntgamepad"
)

// scripted is a Reader that replays fixed states, as a test would supply.
type scripted struct {
	frames []ntgamepad.State
}

func (s *scripted) Poll() ntgamepad.State {
	if len(s.frames) == 0 {
		return ntgamepad.State{}
	}
	f := s.frames[0]
	s.frames = s.frames[1:]
	return f
}

func (s *scripted) PollAll(dst []ntgamepad.State) []ntgamepad.State {
	dst = dst[:0]
	if p := s.Poll(); p.Connected {
		dst = append(dst, p)
	}
	return dst
}

func (s *scripted) Close() error { return nil }

// Poll once a frame, keep the previous State, and act on the buttons that
// went down since.
func Example() {
	var pad ntgamepad.Reader = &scripted{frames: []ntgamepad.State{
		{Connected: true, LeftX: 0.05},
		{Connected: true, LeftX: 0.8, Buttons: ntgamepad.ButtonSouth},
		{Connected: true, LeftX: 0.8, Buttons: ntgamepad.ButtonSouth},
	}}
	defer pad.Close()

	var prev ntgamepad.State
	for frame := range 3 {
		s := pad.Poll().WithDeadzone(0)
		if s.JustPressed(prev)&ntgamepad.ButtonSouth != 0 {
			fmt.Printf("frame %d: jump\n", frame)
		}
		fmt.Printf("frame %d: steer %.2f\n", frame, s.LeftX)
		prev = s
	}
	// Output:
	// frame 0: steer 0.00
	// frame 1: jump
	// frame 1: steer 0.76
	// frame 2: steer 0.76
}
