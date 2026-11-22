# Prompt: a shim that makes the Switch 2 Pro Controller work everywhere on macOS

Build an installable shim so that a Nintendo Switch 2 Pro Controller plugged
into a Mac over USB works for every program, not only those that link
`ntgamepad`: native games, Chromium browsers, and Safari. Do it in two
stages. Stage 1 is known to be possible and should ship. Stage 2 rests on
two unknowns; settle them with a spike before building anything around it.

Read this whole prompt before starting.

## Why

macOS does not recognise this controller (USB vendor `0x057e`, product
`0x2069`), and the controller sends nothing until told to. `ntgamepad`
carries its own driver (`switch2.go`, `switch2_darwin.go`): it sends a
start-up sequence over the pad's vendor USB interface, after which the pad's
HID interface streams input reports. That fixes programs that use
`ntgamepad`. Measured on macOS 27.2 (2026-11-22):

- **GameController.framework lists no controller**, with or without a
  running main run loop, before or after the pad has been started. A native
  Swift program asking `GCController.controllers()` returned zero each time.
- **Safari shows a page no controller.** It appears to take its controllers
  from that framework, and has no WebUSB or WebHID to fall back on.
- **Chrome shows a started pad** as a generic gamepad with no standard
  mapping (6 axes, 21 buttons). `ntgamepad`'s browser reader maps that shape
  itself. A pad that has not been started since it was plugged in is silent
  in Chrome too.
- **The pad keeps streaming after the program that started it exits**, until
  it is unplugged.

## Stage 1: a start-up agent

A small background program that starts every Switch 2 Pro Controller as it
is plugged in, so the pad is a live HID gamepad for the whole machine.

- Reuse the existing driver; do not write a second copy of the protocol. The
  agent is close to "open a `Reader` and wait". Decide whether the driver's
  one-second scan is good enough or whether the agent should wait on I/O Kit
  matching notifications instead.
- Install as a per-user LaunchAgent with a documented install and uninstall
  (`launchctl bootstrap` / `bootout`). No root, no kernel or driver
  extension. The Swift prototype and the Go driver both ran unsigned from a
  terminal, so no entitlement is expected; confirm that still holds when
  launched by launchd.
- It must not hold the pad's vendor interface open after the handshake:
  another program (SDL, a second copy of this package) may want it.
- Give it a status command that lists the pads it has started.
- Check that two programs sending the handshake to the same pad do not
  disturb each other, since the agent and an `ntgamepad` program will both
  try.

Done when: after a fresh plug-in and with nothing else running, the browser
demo (`task web:serve`) in Chrome shows the pad on the first button press,
and unplugging and replugging is handled without restarting the agent.

Stage 1 does not fix Safari or GameController.framework programs. Say so in
the README.

## Stage 2: a virtual controller (spike first)

The idea: the shim reads the Switch 2 pad and publishes a second, virtual
HID device that looks like a controller macOS already supports. Apple's
framework would pick that up, which would cover Safari and native games and
give Chrome a standard mapping.

Two things are not known, and either can end the idea:

1. **Signing and entitlements.** Creating a virtual HID device on macOS
   needs an Apple entitlement and a signed, notarised program. Find the
   current rules: which API (CoreHID's virtual device, `IOHIDUserDevice`, or
   a DriverKit extension), which entitlement each needs, and whether Apple
   grants it to a developer who is not the hardware vendor. A driver
   extension that claims the USB device directly needs transport
   entitlements usually tied to the vendor ID; treat that route as unlikely.
2. **Whether the framework accepts an imitation.** GameController.framework
   may match on more than the HID descriptor and the vendor and product
   numbers: it may require a real USB or Bluetooth transport.

The spike, about a day, on the user's machine with their developer signing:

- Create a virtual HID device imitating one supported controller. Pick the
  one whose descriptor and reports are best documented.
- Check, in this order: does `GCController.controllers()` list it; does
  Safari's `navigator.getGamepads()` list it; does Chrome give it
  `mapping: "standard"`.
- Feed it one hand-made report and confirm a button reads as pressed.

Report the result before going further. If the framework refuses the
imitation, stop and say so: stage 1 is then the whole product. If it works,
propose the design (process model, how the real pad is hidden so programs
do not see two controllers, installer, notarisation) and wait for a decision.

## Constraints

- No cgo, as in the rest of this module. If stage 2 cannot be done without
  it, say so and propose where the cgo or Swift part would live (a separate
  module or a helper binary, not this package).
- Do not sign, notarise, publish, or install anything on the user's machine
  beyond their own login session without asking.
- Do not copy code from SDL. The protocol facts in `switch2.go` came from
  SDL's `SDL_hidapi_switch2.c` (zlib licence, contributed by Valve) and are
  credited in the README; keep the credit.
- Apple may add native support for this pad in a macOS update, as it did
  for the original Switch Pro Controller. Check the current macOS before
  starting stage 2; if the framework already lists the pad, stage 2 is
  unnecessary.

## Report back

- Stage 1: what was built, how it is installed and removed, and what was
  tested with the real pad (plug, unplug, replug, sleep and wake, two pads
  if available) versus only reasoned about.
- Stage 2: the answer to each unknown, with what was actually run, and a
  recommendation to proceed or stop.
