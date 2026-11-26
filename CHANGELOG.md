# `ntgamepad` CHANGELOG

## v0.2.0 (2026-11-26)

- **On macOS, importing ntgamepad now pins the main goroutine to the main OS
  thread.** Polling from that goroutine pumps the main run loop, so terminal
  programs can receive controller connections and input from
  GameController.framework. In Bubble Tea, `Update` runs on the goroutine
  that called `Run`.
- The built-in DualSense HID driver reads plain and enhanced reports over
  USB and Bluetooth when polling off the main goroutine. On macOS 27.2, one
  USB DualSense kept slot 0 as polling switched from HID to the framework.
  When multiple identical pads make pairing ambiguous, existing slots take
  priority. Set `NTGAMEPAD_HID=off` to check the framework alone.
- Browser controllers keep distinct, stable slots even when the browser
  numbers a pad past the last available slot or another pad disconnects.
  Both demos show the slot assigned by the reader. The Switch 2 compatibility
  badge reports USB detection in Chromium and Firefox on macOS. Stick
  directions still need a hardware check in each browser.
- `Notes()` explains what limits controller visibility, including polling
  off the main goroutine, a browser waiting for a button press or skipping
  an unknown mapping, and unsupported platforms. Each note has a stable
  `Code` and English `Text`. The probe and controller viewer display them.
  The Linux permission note clears on a poll with no denied devices. The
  macOS framework initialization note explains why the reader returns no pads.
- When a page's permissions policy denies gamepad access, polls return no
  controllers and `Notes()` reports `NoteGamepadAccessDenied`. The same
  reader resumes polling when access returns.
- `task test:wasm` exercises the browser reader in Node with a simulated
  `navigator.getGamepads`, including malformed list entries that previously
  caused a panic. `task fuzz-smoke` discovers targets automatically, with
  dedicated targets for DualSense reports and Switch 2 browser layouts.
- Added repository guidelines and a record of protocol sources and hardware
  observations. [AGENTS.md](./AGENTS.md) · [docs/SOURCES.md](./docs/SOURCES.md)

## v0.1.0 (2026-11-22)

Initial version of `ntgamepad`, game controller polling for Go without cgo.
It began as the unreleased `nt3d/gamepad` package and was moved to its own
module before any release.

- **One layout everywhere**: `State` reports sticks, triggers and a `Button`
  set in the W3C Standard Gamepad layout, with `Pressed`, `WithDeadzone`, and
  the button edges `JustPressed` and `JustReleased`.
- **Several controllers**: `Reader.PollAll` returns every connected controller
  (up to `MaxPads`); `State.Index` is a slot the controller keeps while it
  stays connected. `Reader.Poll` returns the first.
- **Platforms**: the browser (`navigator.getGamepads`, standard-mapped pads
  only), macOS (GameController.framework), and Linux (`/dev/input/js*`, xpad
  numbering). Elsewhere, Windows included, the reader reports no controllers.
- **macOS**: each poll runs on one OS thread inside an autorelease pool, which
  stops the process growing by about 55 bytes a poll, and switches on
  `shouldMonitorBackgroundEvents` so a terminal program receives input.
- **Switch 2 Pro Controller on macOS**: a built-in USB driver for the
  controller macOS does not recognise. It reads the stick calibration from
  the pad, sends its start-up sequence over the vendor interface
  (IOUSBHost.framework), and polls the pad's HID values without a run loop.
- **Controller viewer (`examples/pad`)**: a Bubble Tea program that shows a
  controller as a drawn picture or in terminal cells, in a terminal and,
  through booba, in a browser.
- **`cmd/ntgamepad-probe`**: prints what the attached controllers report, as
  the hardware check for a platform.
- **Fuzzing**: targets for the Linux joystick decoder, the browser mapping,
  the Switch 2 flash and report decoders, and the USB descriptor walk.
