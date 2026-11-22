// Copyright (c) 2026 Neomantra Corp.

package main

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"nimbleterminal.dev/ntgamepad"
)

// The text rendition: the controller drawn in terminal cells, for any
// terminal.

var (
	styleOn   = lipgloss.NewStyle().Foreground(lipgloss.Color("#0b161a")).Background(lipgloss.Color("#3bc9db")).Bold(true)
	styleOff  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d1dc"))
	styleDim  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7d8899"))
	styleDot  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3bc9db")).Bold(true)
	styleHead = lipgloss.NewStyle().Bold(true)
)

// grid is a small sheet of cells, each a rune and a style.
type grid struct {
	w, h  int
	runes []rune
	style []*lipgloss.Style
}

func newGrid(w, h int) *grid {
	g := &grid{w: w, h: h, runes: make([]rune, w*h), style: make([]*lipgloss.Style, w*h)}
	for i := range g.runes {
		g.runes[i] = ' '
	}
	return g
}

// put writes text starting at (x, y); cells off the sheet are dropped.
func (g *grid) put(x, y int, text string, st *lipgloss.Style) {
	for _, r := range text {
		if x >= 0 && x < g.w && y >= 0 && y < g.h {
			g.runes[y*g.w+x], g.style[y*g.w+x] = r, st
		}
		x++
	}
}

// String renders the sheet, styling each run of like cells once.
func (g *grid) String() string {
	var b strings.Builder
	for y := range g.h {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < g.w; {
			st := g.style[y*g.w+x]
			end := x
			for end < g.w && g.style[y*g.w+end] == st {
				end++
			}
			run := string(g.runes[y*g.w+x : y*g.w+end])
			if st != nil {
				run = st.Render(run)
			}
			b.WriteString(run)
			x = end
		}
	}
	return b.String()
}

const (
	textW = 62
	textH = 13
)

// textPad draws one controller in cells. A State that is not connected draws
// with nothing lit.
func textPad(p ntgamepad.State) string {
	g := newGrid(textW, textH)
	key := func(x, y int, label string, b ntgamepad.Button) {
		st := &styleOff
		if p.Connected && p.Buttons&b != 0 {
			st = &styleOn
		}
		g.put(x, y, " "+label+" ", st)
	}
	bar := func(x, y int, v float64) {
		const cells = 8
		filled := int(math.Round(v * cells))
		g.put(x, y, strings.Repeat("█", filled), &styleDot)
		g.put(x+filled, y, strings.Repeat("░", cells-filled), &styleDim)
	}
	stick := func(x, y int, sx, sy float64, click ntgamepad.Button) {
		const w, h = 13, 7
		g.put(x, y, "╭"+strings.Repeat("─", w-2)+"╮", &styleDim)
		for row := 1; row < h-1; row++ {
			g.put(x, y+row, "│", &styleDim)
			g.put(x+w-1, y+row, "│", &styleDim)
		}
		g.put(x, y+h-1, "╰"+strings.Repeat("─", w-2)+"╯", &styleDim)
		cx, cy := x+w/2, y+h/2
		g.put(cx, cy, "·", &styleDim)
		dot := "●"
		st := &styleDot
		if p.Connected && p.Buttons&click != 0 {
			dot, st = "◉", &styleOn
		}
		g.put(cx+int(math.Round(sx*5)), cy-int(math.Round(sy*2)), dot, st)
	}

	// Triggers and shoulders.
	key(1, 0, "L2", ntgamepad.ButtonL2)
	bar(6, 0, p.LeftTrigger)
	key(48, 0, "R2", ntgamepad.ButtonR2)
	bar(53, 0, p.RightTrigger)
	key(1, 1, "L1", ntgamepad.ButtonL1)
	key(48, 1, "R1", ntgamepad.ButtonR1)

	// Left stick, Select and Start, face buttons.
	stick(2, 3, p.LeftX, p.LeftY, ntgamepad.ButtonL3)
	key(20, 4, "Select", ntgamepad.ButtonSelect)
	key(31, 4, "Start", ntgamepad.ButtonStart)
	key(49, 3, "N", ntgamepad.ButtonNorth)
	key(45, 5, "W", ntgamepad.ButtonWest)
	key(53, 5, "E", ntgamepad.ButtonEast)
	key(49, 7, "S", ntgamepad.ButtonSouth)

	// D-pad and right stick.
	key(21, 7, "↑", ntgamepad.ButtonDPadUp)
	key(18, 8, "←", ntgamepad.ButtonDPadLeft)
	key(24, 8, "→", ntgamepad.ButtonDPadRight)
	key(21, 9, "↓", ntgamepad.ButtonDPadDown)
	stick(31, 6, p.RightX, p.RightY, ntgamepad.ButtonR3)
	return g.String()
}
