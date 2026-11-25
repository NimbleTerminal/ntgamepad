# Working on ntgamepad

Rules and invariants for agents and humans changing this repository. To *use*
ntgamepad, read the README and the package documentation
(`go doc nimbleterminal.dev/ntgamepad`).

## Overview

`ntgamepad` (`nimbleterminal.dev/ntgamepad`) polls game controllers from Go,
in a terminal, a native window, or the browser, and reports every controller
in one layout. The library is the root package; `cmd/ntgamepad-probe` is the
hardware check; `examples/` is a separate module with the controller viewer.

## Core constraints

- **No cgo.** Everything builds with `CGO_ENABLED=0` for macOS, Linux,
  Windows and `js/wasm`. System libraries are reached through goffi, on
  macOS only; `task cross` holds that the others do not link it.
- **Dependencies**: the library depends on goffi and nothing else. Bubble
  Tea, booba and ntpicture belong to `examples/`, a module of its own.
- **One layout.** Every platform reports the W3C Standard Gamepad: sticks in
  [-1, 1] with +X right and **+Y up**, triggers in [0, 1], sixteen buttons.
  Buttons are named by position (`ButtonSouth`), never by the letter a
  maker prints on them. A control with no place in the layout is dropped,
  and its driver says so.
- **Polls never block and never panic.** A poll does no device I/O that can
  wait. What a device or a browser hands over is untrusted: short, long,
  missing, of the wrong type. Decoders hold values in range and return
  `false` for what they cannot read.
- **Decoding is plain Go; transport is per platform.** What a report's bytes
  mean lives in an untagged file (`switch2.go`, `dualsense.go`, `joydev.go`,
  `standard.go`) with tests that run everywhere. Opening the device and
  reading it lives in a tagged file (`hid_darwin.go`, `gamepad_linux.go`,
  `gamepad_js.go`). A new platform for a pad is a new transport, not a new
  decoder.
- **Slots are stable.** `State.Index` is a slot a controller keeps while it
  stays connected, below `MaxPads`, distinct, and results come back in slot
  order. One physical pad is reported once, whichever path sees it.
- **macOS, the main thread.** The system framework shows controllers only to
  a program whose main thread runs the main run loop. The package pins the
  main goroutine to the main thread in `init` and pumps the loop from polls
  made on it. Do not move that pump off the main thread, and do not add a
  second `init` that unlocks it. The HID drivers serve pads the framework
  does not support and programs that poll from another goroutine.
- **Notes.** `Notes()` is for the developer. A `NoteCode` is stable once
  released: never rename one or change what it means; add a new one. Texts
  are English sentences, may be reworded, and are not translated.
- **License header**: every Go file starts with
  `// Copyright (c) 2026 Neomantra Corp.`

## Claims about hardware

Controllers behave as the hardware does, not as the code reads. Keep what
has been tried apart from what has only been compiled:

- The README's table says, for each platform and controller, what someone
  saw with the probe. Add a row when you try one; do not write "works" for a
  path nobody has run with the pad in their hands.
- Check a result against a control before believing it. A controller that
  has gone to sleep reads exactly like a driver that does not work: run the
  probe, or another path known to see the pad, in the same sitting.
- Bytes captured from a real pad go into the tests as hex, and
  `docs/SOURCES.md` records which pad, system and date they came from, and
  where each protocol was learned.

## Verification

- `task ci` is the gate: format, tidy, vet for six targets, the tests of the
  library and of `examples/`, and, when Node is installed, `task test:wasm`.
- `task test:wasm` runs the browser reader itself, compiled to WebAssembly,
  in Node against a made-up `navigator.getGamepads`. Touching
  `gamepad_js.go` without it is unverified.
- `task fuzz-smoke` runs every fuzz target briefly. Everything that decodes
  bytes from a device or a browser has one; a new decoder gets one.
- `task probe` and `task pad` are the checks with a controller attached.
- Tests state the behaviour they hold, in a comment, and never call `Open`
  to mean "a controller is there": hardware is absent in CI.

## Conventions

- Document a behaviour or API change in `CHANGELOG.md` and the README.
- Open work lives in `ROADMAP.md`; a prompt an item points to is in
  `docs/prompts/`.
- Do not track built binaries or generated web assets (`task web:clean`).
- Do not commit, push, tag, or release unless asked.
