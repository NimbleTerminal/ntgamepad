# Where the protocols and the test data come from

What `ntgamepad` knows about each controller, where it was learned, and
which hardware the bytes in the tests were captured from.

## Nintendo Switch 2 Pro Controller (`switch2.go`)

- **Protocol**: the start-up sequence sent over the vendor USB interface, the
  flash addresses of the stick calibration, and the layout of input report 9
  are those worked out by the SDL project:
  `src/joystick/hidapi/SDL_hidapi_switch2.c` in
  <https://github.com/libsdl-org/SDL>, read on its `main` branch on
  2026-11-22. That file is "Copyright (C) 1997-2026 Sam Lantinga", with
  "code and logic contributed by Valve Corporation under the SDL zlib
  license". The code here is an independent implementation; no SDL code is
  copied.
- **Report descriptor**: read from the pad itself through the macOS I/O
  Registry, 2026-11-22. It gives the button usages (1 to 21) and the four
  12-bit stick axes the driver polls.
- **Captured data** (`switch2_test.go`): the factory stick calibration
  blocks at flash `0x13080` and `0x130C0`, and one input report with the
  sticks at rest, from one pad (USB vendor `0x057e`, product `0x2069`,
  firmware `0x0201`) on macOS 27.2, 2026-11-22. The pad's serial-number
  block was read and is not stored.
- **Browser shapes**: Chromium lists the started pad with 6 axes and 21
  buttons, Firefox with 4 axes and 21 buttons, both without a standard
  mapping: read off the demo page on macOS 27.2, 2026-11-22. Which axis is
  which is taken from how each browser numbers a generic HID gamepad, and
  has not been confirmed with a stick held.

## Sony DualSense (`dualsense.go`)

- **Plain report** (report 1): the pad's own HID descriptor, read through
  the macOS I/O Registry over Bluetooth, 2026-11-25: buttons 1 to 14, a hat
  switch, and axes X, Y, Z, Rz (sticks) and Rx, Ry (triggers).
- **Enhanced Bluetooth report** (report `0x31`): the descriptor declares it
  as one opaque block. Its layout (sticks, triggers, a counter, then hat and
  buttons) is the widely documented layout of the DualSense's input report,
  checked here against a block captured from a pad at rest.
- **Captured data** (`dualsense_test.go`): the first 14 bytes of one enhanced
  report from one pad (vendor `0x054c`, product `0x0ce6`) at rest over
  Bluetooth on macOS 27.2, 2026-11-25.

## macOS, GameController.framework (`gamepad_darwin.go`)

Measured on macOS 27.2 with a DualSense over Bluetooth, 2026-11-25, each
result bracketed by a control known to see the pad:

| Program | Controllers the framework listed |
| --- | --- |
| Swift, main run loop running | 1 |
| Swift, no run loop, or a run loop on another thread | 0 |
| Go, main goroutine on the main thread, no run loop | 0 |
| Go, the same, main run loop pumped | 1, after 50 ms |
| Go, the same, plus a minimal `NSApplication`, or without the background-events flag | 1 |

With the loop pumped and background events requested, the left stick and the
bottom face button were read live. No `Info.plist`, app bundle or
entitlement was involved in any of these.

## Linux joystick interface (`joydev.go`)

The `struct js_event` layout and the `JS_EVENT_INIT` flag are the kernel's
documented joystick API (`Documentation/input/joydev/joystick-api.rst`). The
button and axis numbering is that of the kernel's `xpad` driver. Neither has
been tried with a controller.

## Browser standard mapping (`standard.go`)

The button and axis order is the W3C Gamepad specification's "standard"
mapping (<https://www.w3.org/TR/gamepad/#remapping>).
