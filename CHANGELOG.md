# Changelog

## Unreleased

- **DualSense on macOS**: a built-in HID driver for Sony's DualSense, over
  Bluetooth and USB, reading both the pad's plain report and its enhanced
  Bluetooth report. GameController.framework lists controllers only to
  programs that run a main run loop, so a terminal program saw none; the
  driver needs no run loop. A pad the driver reads is not reported a second
  time through the framework.

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
