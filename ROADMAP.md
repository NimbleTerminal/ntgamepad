# ntgamepad roadmap

What is open, roughly in priority order within each section. Finished items
move to Done with their commit. A prompt an item points to is in
`docs/prompts/`.

## Next

- [ ] **Unplug and replug the Switch 2 Pro Controller while a program runs.**
  The pad works in the probe; dropping a pad that is unplugged and picking it
  up again has not been tried on hardware.
- [ ] **Confirm the Switch 2 mapping in Chrome and Firefox.** The browser
  reader maps the pad's generic shape: in Chromium six axes (right stick on 3
  and 5), in Firefox four (right stick on 2 and 3). Both browsers' shapes were
  read off the demo page; the stick directions have not been seen running.

## Switch 2 Pro Controller

- [ ] **A shim for the whole machine**: a start-up agent so the pad works in
  any program and in Chromium browsers, then a spike on a virtual controller
  for Safari and GameController.framework programs.
  Prompt: [`docs/prompts/2026-11-22-switch2-shim.md`](docs/prompts/2026-11-22-switch2-shim.md).
  The agent cannot run under ntconfine as it stands: it needs the USB and
  HID devices, which ntconfine's policy has no way to grant.
- [ ] **The buttons the standard layout has no place for**: Home, Capture, C
  and the two rear paddles are read from the pad and dropped. Decide how a
  `State` would carry them.
- [ ] **Rumble and motion.** The pad has both; the driver uses neither.
- [ ] **Bluetooth.** USB only today.
- [ ] **Other platforms.** The driver is macOS only. Linux and Windows need
  their own way to send the start-up sequence.

## Platforms

- [ ] **macOS, other pads through the framework**: an Xbox controller, a
  DualShock 4 and the original Switch Pro Controller should now work in a
  terminal program that polls from its main goroutine. None has been tried.
- [ ] **Check every DualSense button, and USB.** Found and read at rest over
  Bluetooth in its enhanced mode; the plain report and USB are untried.
- [ ] **Linux**: reads the legacy joystick interface with `xpad` numbering,
  untested on hardware. Consider evdev, which reports what each button is.
- [ ] **Windows**: reports no controllers. XInput can be called without cgo.
- [ ] **Browser**: a controller with no standard mapping is skipped unless
  the package knows it by name.

## Done

- [x] **First release (v0.1.0)**: the library, the Switch 2 Pro Controller
  driver, the controller viewer, fuzz targets and CI.
- [x] **Browser demo published** at <https://nimbleterminal.github.io/ntgamepad/>.
- [x] **v0.2.0**: every controller macOS supports in a terminal program,
  through the system framework; a DualSense HID driver; `Notes()`.
