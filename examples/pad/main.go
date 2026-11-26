// Copyright (c) 2026 Neomantra Corp.

// Command pad shows what ntgamepad reads from a controller, in two
// renditions: a picture of the controller drawn in pixels, and the same
// drawn in terminal cells. It is one Bubble Tea program that runs in a
// terminal and, through booba, in a browser.
//
//	go run ./pad            # in a terminal
//	task web:serve          # in a browser (from the repository root)
//
// g switches rendition, tab the controller shown, q quits.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	booba "nimbleterminal.dev/booba"
	"nimbleterminal.dev/ntgamepad"
	picture "nimbleterminal.dev/ntpicture"
)

const (
	pollEvery = time.Second / 60
	// presentWait is how long a picture frame may go unanswered before the
	// next one is sent anyway.
	presentWait = time.Second
)

type tickMsg time.Time

type model struct {
	pad  ntgamepad.Reader
	pads []ntgamepad.State
	prev map[int]ntgamepad.State
	// presses counts button presses for each controller slot.
	presses map[int]int
	// selected is the slot of the controller shown.
	selected int

	pic     *picture.Picture
	graphic bool
	// drawn is the State the picture last showed; awaiting is set while a
	// frame is on its way to the terminal, so frames do not pile up.
	drawn    ntgamepad.State
	hasDrawn bool
	awaiting time.Time

	width, height int
	browser       []string
	// advice is ntgamepad.Notes: what limits the controllers shown here.
	advice []ntgamepad.Note
}

func newModel(pad ntgamepad.Reader) *model {
	return &model{
		pad:     pad,
		prev:    map[int]ntgamepad.State{},
		presses: map[int]int{},
		graphic: true,
		pic: picture.New(picture.Options{
			AutoMode: true,
			StableID: true,
			Render: picture.Config{
				Fit: picture.FitContain,
				// The picture is sent at the size it is drawn and the terminal
				// scales it: resampling it up to the terminal's pixels first
				// would cost more than drawing it. Zlib is far quicker to
				// encode than PNG, and shared memory, where the terminal
				// offers it, skips encoding altogether.
				MaxRasterPixels: picW * picH,
				KittyFormat:     picture.KittyFormatZlib,
				KittyMedium:     picture.KittyMediumSharedMemory,
			},
		}),
	}
}

func tick() tea.Cmd {
	return tea.Tick(pollEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) Init() tea.Cmd { return tea.Batch(m.pic.Init(), tick()) }

// shown is the controller being shown, with the deadzone applied; a State
// that is not connected when there is none.
func (m *model) shown() ntgamepad.State {
	for _, p := range m.pads {
		if p.Index == m.selected {
			return p.WithDeadzone(0)
		}
	}
	if len(m.pads) > 0 {
		return m.pads[0].WithDeadzone(0)
	}
	return ntgamepad.State{}
}

// poll reads the controllers and counts new presses.
func (m *model) poll() {
	m.pads = m.pad.PollAll(m.pads)
	seen := map[int]bool{}
	for _, p := range m.pads {
		seen[p.Index] = true
		for b := p.JustPressed(m.prev[p.Index]); b != 0; b &= b - 1 {
			m.presses[p.Index]++
		}
		m.prev[p.Index] = p
	}
	for idx := range m.prev {
		if !seen[idx] {
			delete(m.prev, idx)
		}
	}
	m.browser = browserPads(m.pad)
	m.advice = ntgamepad.Notes()
}

// redraw sends the picture a new frame when the controller shown has changed
// and the last frame has been presented.
func (m *model) redraw(now time.Time) tea.Cmd {
	if !m.graphic {
		return nil
	}
	p := m.shown()
	if m.hasDrawn && p == m.drawn {
		return nil
	}
	if !m.awaiting.IsZero() && now.Sub(m.awaiting) < presentWait {
		return nil
	}
	m.drawn, m.hasDrawn, m.awaiting = p, true, now
	return m.pic.SetImageImmutable(drawPad(p))
}

// bodyRows is how many rows the rendition may use: everything but the header,
// the notes and the footer.
func (m *model) bodyRows() int {
	return max(0, m.height-2-m.noteRows()-1)
}

func (m *model) noteRows() int {
	rows := 1 + len(m.advice)
	if inBrowser {
		rows += max(1, len(m.browser))
	}
	return rows
}

func (m *model) layout() tea.Cmd {
	return m.pic.SetSize(m.width, m.bodyRows())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch v := msg.(type) {
	case tea.KeyPressMsg:
		switch v.String() {
		case "q", "ctrl+c":
			_ = m.pad.Close()
			return m, tea.Sequence(m.pic.Close(), tea.Quit)
		case "g":
			m.graphic = !m.graphic
			m.hasDrawn, m.awaiting = false, time.Time{}
			cmds = append(cmds, m.pic.SetVisible(m.graphic), m.redraw(time.Now()))
		case "tab":
			if len(m.pads) > 0 {
				next := m.pads[0].Index
				for i, p := range m.pads {
					if p.Index == m.selected {
						next = m.pads[(i+1)%len(m.pads)].Index
					}
				}
				m.selected = next
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		cmds = append(cmds, m.layout())
	case tickMsg:
		rows := m.noteRows()
		m.poll()
		if m.noteRows() != rows {
			cmds = append(cmds, m.layout())
		}
		cmds = append(cmds, m.redraw(time.Time(v)), tick())
	case picture.PresentedMsg:
		if v.Owner == m.pic {
			m.awaiting = time.Time{}
		}
	case picture.StaleFrameMsg:
		if v.Owner == m.pic {
			m.hasDrawn, m.awaiting = false, time.Time{}
		}
	case picture.ErrorMsg:
		if v.Owner == m.pic {
			m.awaiting = time.Time{}
		}
	}
	cmds = append(cmds, m.pic.Update(msg))
	return m, tea.Batch(cmds...)
}

func (m *model) header() string {
	p := m.shown()
	if !p.Connected {
		return styleHead.Render("ntgamepad") + styleDim.Render(" · no controller yet: connect one and press a button on it")
	}
	others := ""
	if len(m.pads) > 1 {
		others = fmt.Sprintf(" (%d connected, tab for the next)", len(m.pads))
	}
	return styleHead.Render(fmt.Sprintf("Pad %d", p.Index)) + " " + p.Name + styleDim.Render(others)
}

func (m *model) readout() string {
	p := m.shown()
	if !p.Connected {
		return ""
	}
	return styleDim.Render(fmt.Sprintf("L %+.2f %+.2f · R %+.2f %+.2f · LT %.2f · RT %.2f · %d presses",
		p.LeftX, p.LeftY, p.RightX, p.RightY, p.LeftTrigger, p.RightTrigger, m.presses[p.Index]))
}

func (m *model) notes() string {
	lines := []string{m.readout()}
	for _, a := range m.advice {
		lines = append(lines, styleNote.Render("note: "+a.Text))
	}
	if inBrowser {
		if len(m.browser) == 0 {
			lines = append(lines, styleDim.Render("the browser reports no controllers"))
		}
		for _, b := range m.browser {
			lines = append(lines, styleDim.Render("browser: "+b))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *model) View() tea.View {
	rendition := "picture"
	var body string
	if m.graphic {
		body = m.pic.View()
	} else {
		rendition = "cells"
		body = textPad(m.shown())
	}
	body = lipgloss.Place(m.width, m.bodyRows(), lipgloss.Center, lipgloss.Center, body)
	footer := styleDim.Render("g: rendition (" + rendition + ") · tab: next controller · q: quit")
	clip := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	v := tea.NewView(strings.Join([]string{
		clip.Render(m.header()), "", body, clip.Render(m.notes()), clip.Render(footer),
	}, "\n"))
	v.AltScreen = true
	return v
}

func main() {
	pad := ntgamepad.Open()
	defer pad.Close()
	if err := booba.Run(newModel(pad)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
