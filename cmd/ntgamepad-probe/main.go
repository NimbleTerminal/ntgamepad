// Copyright (c) 2026 Neomantra Corp.

// Command ntgamepad-probe prints what ntgamepad reads from the attached
// controllers, as the hardware check for a platform: run it, press every
// button and move every stick, and compare.
//
//	go run ./cmd/ntgamepad-probe            # print a line when anything changes
//	go run ./cmd/ntgamepad-probe -for 10s   # stop after ten seconds
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"nimbleterminal.dev/ntgamepad"
)

var buttonNames = []struct {
	bit  ntgamepad.Button
	name string
}{
	{ntgamepad.ButtonSouth, "South"}, {ntgamepad.ButtonEast, "East"},
	{ntgamepad.ButtonWest, "West"}, {ntgamepad.ButtonNorth, "North"},
	{ntgamepad.ButtonL1, "L1"}, {ntgamepad.ButtonR1, "R1"},
	{ntgamepad.ButtonL2, "L2"}, {ntgamepad.ButtonR2, "R2"},
	{ntgamepad.ButtonSelect, "Select"}, {ntgamepad.ButtonStart, "Start"},
	{ntgamepad.ButtonL3, "L3"}, {ntgamepad.ButtonR3, "R3"},
	{ntgamepad.ButtonDPadUp, "Up"}, {ntgamepad.ButtonDPadDown, "Down"},
	{ntgamepad.ButtonDPadLeft, "Left"}, {ntgamepad.ButtonDPadRight, "Right"},
}

func main() {
	rate := flag.Int("hz", 60, "polls per second")
	limit := flag.Duration("for", 0, "stop after this long (0 runs until interrupted)")
	deadzone := flag.Float64("deadzone", ntgamepad.DefaultDeadzone, "thumbstick deadzone")
	flag.Parse()
	if *rate < 1 {
		*rate = 1
	}

	pad := ntgamepad.Open()
	defer pad.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	var deadline <-chan time.Time
	if *limit > 0 {
		deadline = time.After(*limit)
	}
	tick := time.NewTicker(time.Second / time.Duration(*rate))
	defer tick.Stop()

	fmt.Println("polling; press buttons and move sticks (Ctrl-C to stop)")
	var pads []ntgamepad.State
	last := map[int]string{}
	seen := 0
	var slowest time.Duration
	polls := 0
	for {
		select {
		case <-stop:
			report(polls, slowest, seen)
			return
		case <-deadline:
			report(polls, slowest, seen)
			return
		case <-tick.C:
		}
		start := time.Now()
		pads = pad.PollAll(pads)
		if d := time.Since(start); d > slowest {
			slowest = d
		}
		polls++

		present := map[int]bool{}
		for _, p := range pads {
			present[p.Index] = true
			line := describe(p.WithDeadzone(*deadzone))
			if last[p.Index] != line {
				if _, known := last[p.Index]; !known {
					seen++
					fmt.Printf("pad %d connected: %q\n", p.Index, p.Name)
				}
				last[p.Index] = line
				fmt.Printf("pad %d  %s\n", p.Index, line)
			}
		}
		for idx := range last {
			if !present[idx] {
				delete(last, idx)
				fmt.Printf("pad %d disconnected\n", idx)
			}
		}
	}
}

func describe(p ntgamepad.State) string {
	var down []string
	for _, b := range buttonNames {
		if p.Buttons&b.bit != 0 {
			down = append(down, b.name)
		}
	}
	return fmt.Sprintf("L(%+.2f %+.2f) R(%+.2f %+.2f) LT %.2f RT %.2f [%s]",
		p.LeftX, p.LeftY, p.RightX, p.RightY, p.LeftTrigger, p.RightTrigger, strings.Join(down, " "))
}

func report(polls int, slowest time.Duration, seen int) {
	fmt.Printf("%d polls, slowest %v, %d controller(s) seen\n", polls, slowest, seen)
	if seen == 0 {
		fmt.Println("no controller was reported")
	}
}
