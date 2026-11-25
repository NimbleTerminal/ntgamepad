// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

// Notes says, in plain sentences, anything about this platform and the way
// the program is polling that limits which controllers it is shown. It is
// empty when there is nothing to say.
//
// It is for the developer, not the player: print it in a debug view or a
// log while wiring controllers up. The probe and the controller viewer show
// it. What it reports can change from one call to the next, as polls are
// made and controllers come and go.
func Notes() []string { return platformNotes() }

// The notes a platform can give.
const (
	noteMacOffMain = "Polls are coming from a goroutine other than the main one, so macOS shows this program " +
		"only the controllers ntgamepad drives itself (Switch 2 Pro Controller, DualSense). " +
		"Poll from the main goroutine to be shown every controller macOS supports."
	noteMacNoFramework = "GameController.framework could not be loaded: only the controllers ntgamepad drives " +
		"itself (Switch 2 Pro Controller, DualSense) are reported."
	noteMacHIDOff = "NTGAMEPAD_HID=off: the built-in drivers are left out, so the Switch 2 Pro Controller is " +
		"not reported, and nothing is reported to polls made off the main goroutine."
	noteWebGesture  = "A browser shows a page no controllers until a button is pressed on one."
	noteWebSkipped  = "The browser lists a controller with no standard mapping that ntgamepad does not know by name; it is skipped."
	noteLinuxDenied = "A joystick device under /dev/input could not be opened: permission denied. Add the user to the group that owns it (often \"input\")."
	noteUnsupported = "ntgamepad has no controller support on this platform; no controllers are reported."
)

// macNotes is what macOS has to say, given whether the system framework
// loaded, whether any poll has been made, whether one was made from the main
// goroutine lately, and whether the built-in drivers are switched off.
func macNotes(frameworkLoaded, polled, fromMain, hidOff bool) []string {
	var out []string
	switch {
	case !frameworkLoaded:
		out = append(out, noteMacNoFramework)
	case polled && !fromMain:
		out = append(out, noteMacOffMain)
	}
	if hidOff {
		out = append(out, noteMacHIDOff)
	}
	return out
}

// webNotes is what a browser has to say, given how many controllers it is
// showing the page and how many of those were skipped.
func webNotes(listed, skipped int) []string {
	var out []string
	if listed == 0 {
		out = append(out, noteWebGesture)
	}
	if skipped > 0 {
		out = append(out, noteWebSkipped)
	}
	return out
}
