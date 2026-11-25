# Changelog

## v0.2.0 (2026-11-25)

- **Every controller macOS supports, in a terminal program**:
  GameController.framework hands a program its controllers through the main
  run loop, so a Go terminal program was shown none. The package now keeps
  the main goroutine on the main thread (`runtime.LockOSThread` in `init`)
  and a poll made from it lets the main run loop handle what is waiting, so
  the framework's controllers and their input arrive. Poll from the main
  goroutine to get them; a Bubble Tea `Update` already does.
- **DualSense HID driver on macOS**: for programs that poll from another
  goroutine, a built-in driver reads Sony's DualSense over HID, over
  Bluetooth and USB, in both its plain and its enhanced report. It stands
  aside when the framework is showing the program the same pad.
- **`Notes()`**: what limits the controllers a program is shown, for the
  developer: polling off the main goroutine on macOS, a browser waiting for
  a button press or listing a controller the package skipped, a joystick
  device Linux will not let it open, a platform with no support. Each `Note`
  has a stable `Code` a program can act on and an English `Text`, which is
  not translated. The probe and the controller viewer print them.
- `NTGAMEPAD_HID=off` leaves the HID drivers out, as a diagnostic.
- **Browser reader tested as WebAssembly**: `task test:wasm` runs it in Node
  against a made-up `navigator.getGamepads`. It found and fixed two faults:
  two controllers could be given the same slot when the browser numbered
  one past `MaxPads`, and a list entry that was not a gamepad object
  panicked the reader.
- `AGENTS.md` (the repository's rules) and `docs/SOURCES.md` (where each
  protocol and captured report comes from).
- Behaviour change on macOS: importing the package now pins the main
  goroutine to the main thread, and a poll from the main goroutine runs what
  is waiting on the main run loop.

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
