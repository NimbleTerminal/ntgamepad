// Copyright (c) 2026 Neomantra Corp.

package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/ntgamepad"
)

// sample is a controller with a few buttons held and both sticks pushed.
var sample = ntgamepad.State{
	Connected: true, Index: 0, Name: "Switch 2 Pro Controller",
	LeftX: 0.8, LeftY: 0.6, RightX: -1, RightY: -0.3, LeftTrigger: 1,
	Buttons: ntgamepad.ButtonSouth | ntgamepad.ButtonDPadUp | ntgamepad.ButtonL2 | ntgamepad.ButtonR1 | ntgamepad.ButtonStart,
}

// scripted is a Reader that always reports the same controllers.
type scripted []ntgamepad.State

func (s scripted) Poll() ntgamepad.State {
	if len(s) == 0 {
		return ntgamepad.State{}
	}
	return s[0]
}
func (s scripted) PollAll(dst []ntgamepad.State) []ntgamepad.State { return append(dst[:0], s...) }
func (scripted) Close() error                                      { return nil }

// The picture changes when a button goes down or a stick moves, and a
// controller that is not connected still draws its outline. Set
// PAD_WRITE_PNG to a directory to write the pictures for looking at.
func TestDrawPadShowsState(t *testing.T) {
	rest := sample
	rest.Buttons, rest.LeftX, rest.LeftY, rest.RightX, rest.RightY, rest.LeftTrigger = 0, 0, 0, 0, 0, 0
	held, idle, none := drawPad(sample), drawPad(rest), drawPad(ntgamepad.State{})
	if held.Bounds().Dx() != picW || held.Bounds().Dy() != picH {
		t.Fatalf("picture is %v, want %dx%d", held.Bounds(), picW, picH)
	}
	diff := func(a, b []uint8) int {
		n := 0
		for i := range a {
			if a[i] != b[i] {
				n++
			}
		}
		return n
	}
	if diff(held.Pix, idle.Pix) < 2000 {
		t.Fatal("held buttons and pushed sticks barely change the picture")
	}
	if diff(idle.Pix, none.Pix) < 2000 {
		t.Fatal("a missing controller draws the same as a connected one")
	}
	if dir := os.Getenv("PAD_WRITE_PNG"); dir != "" {
		for name, img := range map[string]*image.RGBA{"held.png": held, "idle.png": idle, "none.png": none} {
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}

// The cell rendition lights the held buttons, moves the stick dots, and fits
// its sheet; nothing is lit for a controller that is not connected.
func TestTextPadShowsState(t *testing.T) {
	held := ansi.Strip(textPad(sample))
	lines := strings.Split(held, "\n")
	if len(lines) != textH {
		t.Fatalf("sheet has %d rows, want %d", len(lines), textH)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != textW {
			t.Fatalf("row %d is %d cells wide, want %d", i, w, textW)
		}
	}
	for _, want := range []string{"L2", "R1", "Select", "Start", "N", "S", "●"} {
		if !strings.Contains(held, want) {
			t.Fatalf("sheet lacks %q:\n%s", want, held)
		}
	}
	if textPad(sample) == textPad(ntgamepad.State{}) {
		t.Fatal("a missing controller draws the same as one with buttons held")
	}
	if strings.Contains(textPad(ntgamepad.State{Buttons: ntgamepad.ButtonSouth}), styleOn.Render(" S ")) {
		t.Fatal("a controller that is not connected has a button lit")
	}
	if !strings.Contains(textPad(sample), styleOn.Render(" S ")) {
		t.Fatal("the held South button is not lit")
	}
}

// The program polls on each tick, counts presses once per press, names the
// controller in its header, and g switches between the two renditions.
func TestModelPollsAndSwitchesRendition(t *testing.T) {
	m := newModel(scripted{sample})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	for range 3 {
		m.Update(tickMsg(time.Now()))
	}
	if got := m.presses[0]; got != 5 {
		t.Fatalf("presses = %d after three polls of five held buttons, want 5", got)
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Pad 0 Switch 2 Pro Controller") || !strings.Contains(view, "rendition (picture)") {
		t.Fatalf("picture view:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	view = ansi.Strip(m.View().Content)
	if !strings.Contains(view, "rendition (cells)") || !strings.Contains(view, "Select") {
		t.Fatalf("cells view:\n%s", view)
	}
	if got := strings.Count(view, "\n") + 1; got > 30 {
		t.Fatalf("view is %d rows in a 30-row terminal", got)
	}

	empty := newModel(scripted{})
	empty.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	empty.Update(tickMsg(time.Now()))
	if !strings.Contains(ansi.Strip(empty.View().Content), "no controller yet") {
		t.Fatal("no hint when there is no controller")
	}
}
