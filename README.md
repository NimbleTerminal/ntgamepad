# ntgamepad — Game controllers for Go, native and in the browser, without cgo

<p>
    <a href="https://github.com/NimbleTerminal/ntgamepad/tags"><img src="https://img.shields.io/github/tag/NimbleTerminal/ntgamepad.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/nimbleterminal.dev/ntgamepad"><img src="https://pkg.go.dev/badge/nimbleterminal.dev/ntgamepad.svg" alt="Go Reference"></a>
    <a href="https://github.com/NimbleTerminal/ntgamepad/actions/workflows/ci.yml"><img src="https://github.com/NimbleTerminal/ntgamepad/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <a href="./CODE_OF_CONDUCT.md"><img src="https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg" alt="Code Of Conduct"></a>
</p>

`ntgamepad` (`nimbleterminal.dev/ntgamepad`) polls game controllers from Go
programs: in a terminal, in a native window, or in the browser as
WebAssembly. It builds with `CGO_ENABLED=0` on every platform and reports
every controller in one layout.

It includes a driver for the **Nintendo Switch 2 Pro Controller** on macOS,
which the operating system does not recognise.

## Quickstart

```sh
go get nimbleterminal.dev/ntgamepad@latest
```

Requires Go 1.26.8 or later.

```go
pad := ntgamepad.Open()
defer pad.Close()

var prev ntgamepad.State
for frame() {
	s := pad.Poll().WithDeadzone(0) // 0 means the default deadzone
	if s.JustPressed(prev)&ntgamepad.ButtonSouth != 0 {
		jump()
	}
	steer(s.LeftX)
	prev = s
}
```

## What's inside

- **One layout.** Every platform reports the W3C Standard Gamepad: two
  sticks (`+X` right, `+Y` up), two triggers in `[0, 1]`, and sixteen
  buttons as a bit set.
- **Polling, not events.** Call `Poll` (the first controller) or `PollAll`
  (all of them, up to `MaxPads`) once a frame. Neither blocks. A controller
  plugged in or removed shows up in the next poll; `State.Index` is a slot
  it keeps while connected.
- **Button edges.** `JustPressed(prev)` and `JustReleased(prev)` compare two
  polls, so a game does not track the previous button mask itself.
- **No cgo.** Builds with `CGO_ENABLED=0` everywhere. Only the macOS build
  links an FFI library ([goffi](https://github.com/go-webgpu/goffi)); the
  others use the standard library alone.

## Platforms

| Platform | Reads | State |
| --- | --- | --- |
| Browser (`js/wasm`) | `navigator.getGamepads()` | Demo page runs in Chrome and Firefox; both list the Switch 2 Pro Controller, Safari does not |
| macOS | GameController.framework, plus a built-in driver for the Switch 2 Pro Controller over USB | Switch 2 Pro Controller detected and read; the framework path is **not yet tried with a controller** |
| Linux | `/dev/input/js0`–`js3` | Compiles; event decoding tested without a device |
| Windows, others | nothing | Reports no controllers |

"Tried with a controller" means someone ran `cmd/ntgamepad-probe` and pressed
every button. Add a row when you do.

| Platform | Controller | Result |
| --- | --- | --- |
| macOS 27.2, USB | Nintendo Switch 2 Pro Controller | Works with the built-in driver: sticks and buttons read correctly in the probe. Unplugging and replugging while a program runs is not yet tried. |

### Browser

A browser shows a page no controllers until the user presses a button on one.
Controllers the browser reports with `mapping: "standard"` are used. Others
have no known layout and are skipped, except those this package knows by
name.

The Switch 2 Pro Controller is one of those, in Chromium browsers and
Firefox on macOS, with two limits:

- A page cannot start the pad. Something native must have sent the start-up
  sequence since the pad was plugged in: run `cmd/ntgamepad-probe`, or any
  program that uses this package, once.
- A page cannot read the pad's calibration, so the sticks use a nominal one
  and may rest a few percent off centre, inside the default deadzone.

Safari does not show the pad to a page at all: it takes its controllers from
macOS, which does not recognise this one.

The [demo](#demo) page shows what the package reads beside what the browser
itself reports.

### macOS

Needs macOS 11 or later. The reader switches on
`GCController.shouldMonitorBackgroundEvents` (macOS 11.3), because a terminal
program is never the frontmost app and would otherwise read every controller
at rest.

GameController.framework finds controllers through the main run loop. A
command-line program that never runs one may be shown no controllers at all.
This is unverified: run the probe with a controller attached to find out.

### Switch 2 Pro Controller (macOS, USB)

macOS does not recognise this controller, and the controller sends nothing
until it is told to. `ntgamepad` carries its own driver for it: a background
goroutine finds the pad, reads its stick calibration, and sends the start-up
sequence over the pad's vendor USB interface; polls then read the pad's HID
values. Nothing needs installing, and no permission is asked for.

- USB only. Bluetooth is not supported.
- B is `ButtonSouth` and A is `ButtonEast`: buttons are reported by position,
  not by the letter printed on them. ZL and ZR are switches, so the triggers
  read 0 or 1.
- Home, Capture, C and the rear paddles are not reported: the standard layout
  has no place for them.
- A pad plugged in while the program runs appears within about a second.
- Another program holding the pad's vendor interface (a second driver) keeps
  this one from starting it.

See [Acknowledgements](#acknowledgements) for where the protocol comes from.

### Linux

Uses the kernel's joystick interface with the numbering of the `xpad` driver
(Xbox controllers and those that present as one). Controllers handled by
other drivers number their buttons and axes differently and will read wrong.
The user needs read access to `/dev/input/js*`.

## Checking a controller

```sh
go run ./cmd/ntgamepad-probe            # prints a line whenever anything changes
go run ./cmd/ntgamepad-probe -for 10s   # stops by itself
```

Press each button and move each stick, and check the line that prints.

## Demo

The controller viewer (`examples/pad`) is one Bubble Tea program with two
renditions of the controller, switched with `g`:

- **Picture:** the controller drawn in pixels and shown through
  [ntpicture](https://github.com/NimbleTerminal/ntpicture): sticks that move,
  buttons that light.
- **Cells:** the same in terminal cells, for any terminal.

```sh
task pad                # in this terminal
task web:serve          # in a browser, at http://127.0.0.1:8017/
```

In the browser the same program runs as WebAssembly inside a terminal, through
[booba](https://github.com/NimbleTerminal/booba). There it also lists what the
browser itself reports, so a controller the package skips can be told from
one the browser cannot see. A plain HTML version of the page, without a
terminal, is at `plain.html`.

The viewer lives in its own module, `examples/`, so that Bubble Tea, booba and
ntpicture are not dependencies of the library.

## Testing code that uses it

`Reader` is an interface. Give your code a `Reader` of your own that returns
the states you want; do not call `Open` in tests. `example_test.go` has one.

## Open Collaboration

Run `task ci` before submitting changes. It checks Go formatting and module
tidiness, vets for six targets (macOS, Linux and Windows natively, and
`js/wasm`), and runs the tests of the library and the examples. It requires
Go 1.26.8+ and [Task](https://taskfile.dev). Everything that decodes bytes
from a device or a browser has a fuzz target. Open work is in
[ROADMAP.md](./ROADMAP.md).

```sh
task ci                 # the full gate
task fuzz-smoke         # every fuzz target, 5s each (override with FUZZTIME=30s)
task probe              # print what the attached controllers report
task pad                # the controller viewer, in this terminal
task web:serve          # build and serve the browser demos
```

Contributions and feedback are welcome. Please adhere to our
[Code of Conduct](./CODE_OF_CONDUCT.md).

- [Report an issue](https://github.com/NimbleTerminal/ntgamepad/issues)
- [Submit a pull request](https://github.com/NimbleTerminal/ntgamepad/pulls)

## Acknowledgements

`ntgamepad` began as a package of [nt3d](https://github.com/NimbleTerminal/nt3d).

The Switch 2 Pro Controller's start-up sequence and report layout are those
worked out by the [SDL](https://github.com/libsdl-org/SDL) project
(`SDL_hidapi_switch2.c`, contributed by Valve under the zlib license); the
driver here is an independent implementation of that protocol.

The macOS build calls system frameworks through
[goffi](https://github.com/go-webgpu/goffi).

## License

Released under the [MIT License](./LICENSE).

Copyright (c) 2026 [Neomantra Corp](https://www.neomantra.com).

----
Made with :heart: and :fire: by the team behind [Nimble Terminal](https://nimbleterminal.dev).
