// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

// NoteCode names a Note. The codes are stable: a program may act on one, and
// its meaning will not change. A Note's Text may be reworded.
type NoteCode string

const (
	// NotePollOffMainGoroutine: on macOS, polls are coming from a goroutine
	// other than the main one, so the system shows the program only the
	// controllers this package drives itself.
	NotePollOffMainGoroutine NoteCode = "poll-off-main-goroutine"
	// NoteFrameworkUnavailable: on macOS, the system's controller framework
	// could not be loaded; only the built-in drivers report controllers.
	NoteFrameworkUnavailable NoteCode = "framework-unavailable"
	// NoteDriversOff: the built-in drivers were left out (NTGAMEPAD_HID=off).
	NoteDriversOff NoteCode = "drivers-off"
	// NoteNeedsButtonPress: in a browser, no controller is listed yet; the
	// browser hides controllers from a page until a button is pressed on one.
	NoteNeedsButtonPress NoteCode = "needs-button-press"
	// NoteControllerSkipped: the browser lists a controller that has no
	// standard mapping and that this package does not know by name.
	NoteControllerSkipped NoteCode = "controller-skipped"
	// NotePermissionDenied: on Linux, a joystick device is there but the
	// program may not open it.
	NotePermissionDenied NoteCode = "permission-denied"
	// NoteUnsupportedPlatform: this package has no controller support on the
	// platform, and reports no controllers.
	NoteUnsupportedPlatform NoteCode = "unsupported-platform"
)

// Note is one thing that limits which controllers the program is shown.
type Note struct {
	// Code says which thing, for a program to act on.
	Code NoteCode
	// Text says it in an English sentence, for the developer to read. It is
	// not meant for players, and is not translated: a game that tells its
	// players something chooses its own words by Code.
	Text string
}

// String is the note's Text.
func (n Note) String() string { return n.Text }

// Notes says anything about this platform and the way the program is polling
// that limits which controllers it is shown. It is empty when there is
// nothing to say.
//
// It is for the developer: print it in a debug view or a log while wiring
// controllers up. The probe and the controller viewer show it. What it
// reports can change from one call to the next, as polls are made and
// controllers come and go.
func Notes() []Note { return platformNotes() }

// noteText is each note's sentence.
var noteText = map[NoteCode]string{
	NotePollOffMainGoroutine: "Polls are coming from a goroutine other than the main one, so macOS shows this program " +
		"only the controllers ntgamepad drives itself (Switch 2 Pro Controller, DualSense). " +
		"Poll from the main goroutine to be shown every controller macOS supports.",
	NoteFrameworkUnavailable: "GameController.framework could not be loaded: only the controllers ntgamepad drives " +
		"itself (Switch 2 Pro Controller, DualSense) are reported.",
	NoteDriversOff: "NTGAMEPAD_HID=off: the built-in drivers are left out, so the Switch 2 Pro Controller is " +
		"not reported, and nothing is reported to polls made off the main goroutine.",
	NoteNeedsButtonPress:  "A browser shows a page no controllers until a button is pressed on one.",
	NoteControllerSkipped: "The browser lists a controller with no standard mapping that ntgamepad does not know by name; it is skipped.",
	NotePermissionDenied: "A joystick device under /dev/input could not be opened: permission denied. " +
		"Add the user to the group that owns it (often \"input\").",
	NoteUnsupportedPlatform: "ntgamepad has no controller support on this platform; no controllers are reported.",
}

// notes makes the Notes for some codes, in the order given.
func notes(codes ...NoteCode) []Note {
	if len(codes) == 0 {
		return nil
	}
	out := make([]Note, len(codes))
	for i, c := range codes {
		out[i] = Note{Code: c, Text: noteText[c]}
	}
	return out
}

// macNotes is what macOS has to say, given whether the system framework
// loaded, whether any poll has been made, whether one was made from the main
// goroutine lately, and whether the built-in drivers are switched off.
func macNotes(frameworkLoaded, polled, fromMain, hidOff bool) []Note {
	var codes []NoteCode
	switch {
	case !frameworkLoaded:
		codes = append(codes, NoteFrameworkUnavailable)
	case polled && !fromMain:
		codes = append(codes, NotePollOffMainGoroutine)
	}
	if hidOff {
		codes = append(codes, NoteDriversOff)
	}
	return notes(codes...)
}

// webNotes is what a browser has to say, given how many controllers it is
// showing the page and how many of those were skipped.
func webNotes(listed, skipped int) []Note {
	var codes []NoteCode
	if listed == 0 {
		codes = append(codes, NoteNeedsButtonPress)
	}
	if skipped > 0 {
		codes = append(codes, NoteControllerSkipped)
	}
	return notes(codes...)
}
