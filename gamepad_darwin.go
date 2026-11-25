// Copyright (c) 2026 Neomantra Corp.

//go:build darwin && !js

package ntgamepad

import (
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// macOS: GameController.framework, called through the Objective-C runtime
// without cgo. Each poll asks +[GCController controllers] and reads the
// extended gamepad profile's current values.
//
// Three things a program without an app bundle has to get right:
//   - Objective-C calls made on one poll stay on one OS thread, inside an
//     autorelease pool, so the objects a poll creates are freed with it.
//   - shouldMonitorBackgroundEvents is switched on, since a terminal program
//     is never the frontmost app and would otherwise read a pad at rest.
//   - The framework finds controllers through the main run loop. A program
//     that never runs one may see none; the README says what was observed.

// The framework hands a program its controllers through the main run loop,
// so they are seen only by a program whose main thread runs one. A Go
// program has no such loop, but it can run one itself: the main goroutine is
// kept on the main thread, and a poll made from it lets the main run loop
// handle what is waiting before asking for the controllers. A poll made from
// another goroutine cannot do that, and is shown only what the HID drivers
// read (switch2_darwin.go).
func init() { runtime.LockOSThread() }

type objcID uintptr
type objcSEL uintptr

type gcBridge struct {
	once sync.Once
	ok   bool
	// mu serializes polls: the call interfaces and slots are shared.
	mu sync.Mutex
	// background is set once shouldMonitorBackgroundEvents has been tried.
	background bool
	// mainPollAt is when a poll last ran on the main thread, in Unix
	// nanoseconds: only then can the framework see controllers.
	mainPollAt atomic.Int64
	// slots keeps each controller's Index while it stays connected.
	slots [MaxPads]gcSlot

	objcGetClass    unsafe.Pointer
	selRegisterName unsafe.Pointer
	objcMsgSend     unsafe.Pointer
	poolPush        unsafe.Pointer
	poolPop         unsafe.Pointer
	runLoopRun      unsafe.Pointer
	runLoopMode     uintptr
	pthreadMain     unsafe.Pointer

	cifPtr1    types.CallInterface // ptr f(ptr)
	cifPush    types.CallInterface // ptr f(void)
	cifPop     types.CallInterface // void f(ptr)
	cifMsg0    types.CallInterface // ptr msg(self, sel)
	cifMsgU    types.CallInterface // ptr msg(self, sel, u64)
	cifFloat   types.CallInterface // float msg(self, sel)
	cifBool    types.CallInterface // BOOL msg(self, sel)
	cifBoolSel types.CallInterface // BOOL msg(self, sel, sel)
	cifSetBool types.CallInterface // void msg(self, sel, BOOL)
	cifRunLoop types.CallInterface // int32 CFRunLoopRunInMode(mode, double, Boolean)
	cifIsMain  types.CallInterface // int pthread_main_np(void)

	clsGCController objcID
	selControllers  objcSEL
	selCount        objcSEL
	selObjectAtIdx  objcSEL
	selVendorName   objcSEL
	selUTF8String   objcSEL
	selExtGamepad   objcSEL
	selLeftStick    objcSEL
	selRightStick   objcSEL
	selDPad         objcSEL
	selXAxis        objcSEL
	selYAxis        objcSEL
	selUp           objcSEL
	selDown         objcSEL
	selLeft         objcSEL
	selRight        objcSEL
	selValue        objcSEL
	selIsPressed    objcSEL
	selButtonA      objcSEL
	selButtonB      objcSEL
	selButtonX      objcSEL
	selButtonY      objcSEL
	selLeftShoulder objcSEL
	selRightShould  objcSEL
	selLeftTrigger  objcSEL
	selRightTrigger objcSEL
	selButtonOpts   objcSEL
	selButtonMenu   objcSEL
	selLeftThumbBtn objcSEL
	selRightThmbBtn objcSEL
	selResponds     objcSEL
	selSetBackgrnd  objcSEL
	selBackground   objcSEL
}

// gcSlot is one controller's slot: the GCController it holds and its name,
// read once.
type gcSlot struct {
	ctrl objcID
	name string
}

var bridge gcBridge

// load resolves the runtime's functions, the class and the selectors. It
// sends no Objective-C messages, so it needs no autorelease pool.
func (b *gcBridge) load() bool {
	b.once.Do(func() {
		libobjc, err := ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
		if err != nil {
			return
		}
		if _, err := ffi.LoadLibrary("/System/Library/Frameworks/Foundation.framework/Foundation"); err != nil {
			return
		}
		if _, err := ffi.LoadLibrary("/System/Library/Frameworks/GameController.framework/GameController"); err != nil {
			return
		}
		for _, s := range []struct {
			name string
			to   *unsafe.Pointer
		}{
			{"objc_getClass", &b.objcGetClass},
			{"sel_registerName", &b.selRegisterName},
			{"objc_msgSend", &b.objcMsgSend},
			{"objc_autoreleasePoolPush", &b.poolPush},
			{"objc_autoreleasePoolPop", &b.poolPop},
		} {
			if *s.to, err = ffi.GetSymbol(libobjc, s.name); err != nil {
				return
			}
		}
		cf, err := ffi.LoadLibrary("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		if err != nil {
			return
		}
		libsystem, err := ffi.LoadLibrary("/usr/lib/libSystem.B.dylib")
		if err != nil {
			return
		}
		if b.runLoopRun, err = ffi.GetSymbol(cf, "CFRunLoopRunInMode"); err != nil {
			return
		}
		mode, err := ffi.GetSymbol(cf, "kCFRunLoopDefaultMode")
		if err != nil {
			return
		}
		b.runLoopMode = *(*uintptr)(mode) // the symbol is a CFStringRef variable
		if b.pthreadMain, err = ffi.GetSymbol(libsystem, "pthread_main_np"); err != nil {
			return
		}
		ptr, u8 := types.PointerTypeDescriptor, types.UInt8TypeDescriptor
		void := types.VoidTypeDescriptor
		for _, c := range []struct {
			cif  *types.CallInterface
			ret  *types.TypeDescriptor
			args []*types.TypeDescriptor
		}{
			{&b.cifPtr1, ptr, []*types.TypeDescriptor{ptr}},
			{&b.cifPush, ptr, nil},
			{&b.cifPop, void, []*types.TypeDescriptor{ptr}},
			{&b.cifMsg0, ptr, []*types.TypeDescriptor{ptr, ptr}},
			{&b.cifMsgU, ptr, []*types.TypeDescriptor{ptr, ptr, types.UInt64TypeDescriptor}},
			{&b.cifFloat, types.FloatTypeDescriptor, []*types.TypeDescriptor{ptr, ptr}},
			{&b.cifBool, u8, []*types.TypeDescriptor{ptr, ptr}},
			{&b.cifBoolSel, u8, []*types.TypeDescriptor{ptr, ptr, ptr}},
			{&b.cifSetBool, void, []*types.TypeDescriptor{ptr, ptr, u8}},
			{&b.cifRunLoop, types.SInt32TypeDescriptor, []*types.TypeDescriptor{ptr, types.DoubleTypeDescriptor, u8}},
			{&b.cifIsMain, types.SInt32TypeDescriptor, nil},
		} {
			if ffi.PrepareCallInterface(c.cif, types.DefaultCall, c.ret, c.args) != nil {
				return
			}
		}

		b.clsGCController = b.getClass("GCController")
		if b.clsGCController == 0 {
			return
		}
		b.selControllers = b.sel("controllers")
		b.selCount = b.sel("count")
		b.selObjectAtIdx = b.sel("objectAtIndex:")
		b.selVendorName = b.sel("vendorName")
		b.selUTF8String = b.sel("UTF8String")
		b.selExtGamepad = b.sel("extendedGamepad")
		b.selLeftStick = b.sel("leftThumbstick")
		b.selRightStick = b.sel("rightThumbstick")
		b.selDPad = b.sel("dpad")
		b.selXAxis = b.sel("xAxis")
		b.selYAxis = b.sel("yAxis")
		b.selUp = b.sel("up")
		b.selDown = b.sel("down")
		b.selLeft = b.sel("left")
		b.selRight = b.sel("right")
		b.selValue = b.sel("value")
		b.selIsPressed = b.sel("isPressed")
		b.selButtonA = b.sel("buttonA")
		b.selButtonB = b.sel("buttonB")
		b.selButtonX = b.sel("buttonX")
		b.selButtonY = b.sel("buttonY")
		b.selLeftShoulder = b.sel("leftShoulder")
		b.selRightShould = b.sel("rightShoulder")
		b.selLeftTrigger = b.sel("leftTrigger")
		b.selRightTrigger = b.sel("rightTrigger")
		b.selButtonOpts = b.sel("buttonOptions")
		b.selButtonMenu = b.sel("buttonMenu")
		b.selLeftThumbBtn = b.sel("leftThumbstickButton")
		b.selRightThmbBtn = b.sel("rightThumbstickButton")
		b.selResponds = b.sel("respondsToSelector:")
		b.selSetBackgrnd = b.sel("setShouldMonitorBackgroundEvents:")
		b.selBackground = b.sel("shouldMonitorBackgroundEvents")
		b.ok = true
	})
	return b.ok
}

func (b *gcBridge) getClass(name string) objcID {
	buf := append([]byte(name), 0)
	p := unsafe.Pointer(&buf[0])
	var res uintptr
	_, _ = ffi.CallFunction(&b.cifPtr1, b.objcGetClass, unsafe.Pointer(&res), []unsafe.Pointer{unsafe.Pointer(&p)})
	return objcID(res)
}

func (b *gcBridge) sel(name string) objcSEL {
	buf := append([]byte(name), 0)
	p := unsafe.Pointer(&buf[0])
	var res uintptr
	_, _ = ffi.CallFunction(&b.cifPtr1, b.selRegisterName, unsafe.Pointer(&res), []unsafe.Pointer{unsafe.Pointer(&p)})
	return objcSEL(res)
}

// pushPool opens an autorelease pool on the calling OS thread; popPool
// drains it. The caller holds the thread between the two.
func (b *gcBridge) pushPool() uintptr {
	var pool uintptr
	_, _ = ffi.CallFunction(&b.cifPush, b.poolPush, unsafe.Pointer(&pool), nil)
	return pool
}

func (b *gcBridge) popPool(pool uintptr) {
	_, _ = ffi.CallFunction(&b.cifPop, b.poolPop, nil, []unsafe.Pointer{unsafe.Pointer(&pool)})
}

func (b *gcBridge) send0(self objcID, cmd objcSEL) objcID {
	if self == 0 {
		return 0
	}
	var res uintptr
	_, _ = ffi.CallFunction(&b.cifMsg0, b.objcMsgSend, unsafe.Pointer(&res), []unsafe.Pointer{unsafe.Pointer(&self), unsafe.Pointer(&cmd)})
	return objcID(res)
}

// sendPtr is send0 for a message that returns a C pointer, not an object.
func (b *gcBridge) sendPtr(self objcID, cmd objcSEL) unsafe.Pointer {
	if self == 0 {
		return nil
	}
	var res unsafe.Pointer
	_, _ = ffi.CallFunction(&b.cifMsg0, b.objcMsgSend, unsafe.Pointer(&res), []unsafe.Pointer{unsafe.Pointer(&self), unsafe.Pointer(&cmd)})
	return res
}

func (b *gcBridge) sendIdx(self objcID, cmd objcSEL, idx uint64) objcID {
	if self == 0 {
		return 0
	}
	var res uintptr
	_, _ = ffi.CallFunction(&b.cifMsgU, b.objcMsgSend, unsafe.Pointer(&res), []unsafe.Pointer{unsafe.Pointer(&self), unsafe.Pointer(&cmd), unsafe.Pointer(&idx)})
	return objcID(res)
}

// Results smaller than a register are read from a full-width buffer, so a
// result written wider than its type cannot overrun.

func (b *gcBridge) sendFloat(self objcID, cmd objcSEL) float64 {
	if self == 0 {
		return 0
	}
	var res [8]byte
	_, _ = ffi.CallFunction(&b.cifFloat, b.objcMsgSend, unsafe.Pointer(&res), []unsafe.Pointer{unsafe.Pointer(&self), unsafe.Pointer(&cmd)})
	return float64(*(*float32)(unsafe.Pointer(&res)))
}

func (b *gcBridge) sendBool(self objcID, cmd objcSEL) bool {
	if self == 0 {
		return false
	}
	var res [8]byte
	_, _ = ffi.CallFunction(&b.cifBool, b.objcMsgSend, unsafe.Pointer(&res), []unsafe.Pointer{unsafe.Pointer(&self), unsafe.Pointer(&cmd)})
	return res[0] != 0
}

// responds reports whether self answers the selector.
func (b *gcBridge) responds(self objcID, to objcSEL) bool {
	if self == 0 {
		return false
	}
	var res [8]byte
	cmd := b.selResponds
	_, _ = ffi.CallFunction(&b.cifBoolSel, b.objcMsgSend, unsafe.Pointer(&res), []unsafe.Pointer{unsafe.Pointer(&self), unsafe.Pointer(&cmd), unsafe.Pointer(&to)})
	return res[0] != 0
}

func (b *gcBridge) setBool(self objcID, cmd objcSEL, v bool) {
	if self == 0 {
		return
	}
	var arg [8]byte
	if v {
		arg[0] = 1
	}
	_, _ = ffi.CallFunction(&b.cifSetBool, b.objcMsgSend, nil, []unsafe.Pointer{unsafe.Pointer(&self), unsafe.Pointer(&cmd), unsafe.Pointer(&arg)})
}

// onMainThread reports whether the caller is on the program's main thread.
func (b *gcBridge) onMainThread() bool {
	var res [8]byte
	_, _ = ffi.CallFunction(&b.cifIsMain, b.pthreadMain, unsafe.Pointer(&res), nil)
	return res[0] != 0
}

// pumpMain lets the main run loop handle what is waiting, without waiting
// for more: the framework's news of controllers coming and going, and their
// input. The caller is on the main thread.
func (b *gcBridge) pumpMain() {
	const handledSource = 4 // kCFRunLoopRunHandledSource: there may be more
	for range 8 {
		mode, secs := b.runLoopMode, 0.0
		var once [8]byte
		once[0] = 1
		var res [8]byte
		_, _ = ffi.CallFunction(&b.cifRunLoop, b.runLoopRun, unsafe.Pointer(&res),
			[]unsafe.Pointer{unsafe.Pointer(&mode), unsafe.Pointer(&secs), unsafe.Pointer(&once)})
		if *(*int32)(unsafe.Pointer(&res)) != handledSource {
			return
		}
	}
}

// monitorBackground asks the framework, once, to deliver controller input
// while the program is not the frontmost app (macOS 11.3 and later).
func (b *gcBridge) monitorBackground() {
	if b.background {
		return
	}
	b.background = true
	if b.responds(b.clsGCController, b.selSetBackgrnd) {
		b.setBool(b.clsGCController, b.selSetBackgrnd, true)
	}
}

// cString copies the C string at ptr, up to 128 bytes.
func cString(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	var out []byte
	for i := range 128 {
		c := *(*byte)(unsafe.Add(ptr, i))
		if c == 0 {
			break
		}
		out = append(out, c)
	}
	return string(out)
}

// darwinSlots hands out State.Index across the drivers on this platform.
var darwinSlots slotTable

// gcSlotKind is slotKey.kind for GameController.framework's controllers.
const gcSlotKind = 1

// darwinReader reports the controllers GameController.framework knows and,
// through this package's own driver, the Switch 2 Pro Controller, which the
// framework does not.
type darwinReader struct{ closed bool }

// openPlatform starts the Switch 2 driver and polls once, so loading the
// frameworks and the framework's own start-up (about 9 ms together) are paid
// here and not in a frame.
func openPlatform() Reader {
	// NTGAMEPAD_HID=off leaves the HID drivers out, to check what the
	// system framework reports by itself.
	if os.Getenv("NTGAMEPAD_HID") != "off" {
		sw2.begin()
	}
	r := &darwinReader{}
	_ = r.PollAll(nil)
	return r
}

func (r *darwinReader) Close() error {
	r.closed = true
	return nil
}

func (r *darwinReader) Poll() State { return first(r.PollAll(nil)) }

func (r *darwinReader) PollAll(dst []State) []State {
	dst = dst[:0]
	if r.closed {
		return dst
	}
	b := &bridge
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.load() {
		return dst
	}
	// The pool is pushed and popped on one OS thread, and every object this
	// poll autoreleases goes with it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := b.pushPool()
	defer b.popPool(pool)

	b.monitorBackground()
	if b.onMainThread() {
		b.pumpMain()
		b.mainPollAt.Store(time.Now().UnixNano())
	}
	b.assignSlots()
	for i := range b.slots {
		slot := &b.slots[i]
		if slot.ctrl == 0 {
			continue
		}
		dst = append(dst, b.read(i, slot))
	}
	dst = sw2.poll(dst, b.serves(dualSenseName))
	slices.SortFunc(dst, func(x, y State) int { return x.Index - y.Index })
	return dst
}

// assignSlots matches the framework's controllers to slots: a controller
// already in a slot keeps it, a new one with an extended gamepad takes the
// lowest free slot, and a slot whose controller is gone is freed.
func (b *gcBridge) assignSlots() {
	arr := b.send0(b.clsGCController, b.selControllers)
	var present [MaxPads]bool
	var fresh []objcID
	if arr != 0 {
		count := uint64(b.send0(arr, b.selCount))
		for i := range count {
			ctrl := b.sendIdx(arr, b.selObjectAtIdx, i)
			if ctrl == 0 || b.send0(ctrl, b.selExtGamepad) == 0 {
				continue
			}
			held := false
			for s := range b.slots {
				if b.slots[s].ctrl == ctrl {
					present[s], held = true, true
					break
				}
			}
			if !held {
				fresh = append(fresh, ctrl)
			}
		}
	}
	for s := range b.slots {
		if b.slots[s].ctrl != 0 && !present[s] {
			darwinSlots.release(slotKey{kind: gcSlotKind, id: uint64(b.slots[s].ctrl)})
			b.slots[s] = gcSlot{}
		}
	}
	for _, ctrl := range fresh {
		s, ok := darwinSlots.claim(slotKey{kind: gcSlotKind, id: uint64(ctrl)})
		if !ok {
			break
		}
		slot := gcSlot{ctrl: ctrl}
		if ns := b.send0(ctrl, b.selVendorName); ns != 0 {
			slot.name = cString(b.sendPtr(ns, b.selUTF8String))
		}
		b.slots[s] = slot
	}
}

// serves reports whether the framework is listing a controller by this
// name to polls made on the main thread, so that the HID driver for the same
// pad stands aside and the pad is shown once. The caller holds b.mu.
func (b *gcBridge) serves(name string) bool {
	if time.Since(time.Unix(0, b.mainPollAt.Load())) > 3*time.Second {
		return false // nobody polls from the main thread: the framework is blind
	}
	for i := range b.slots {
		if b.slots[i].ctrl != 0 && b.slots[i].name == name {
			return true
		}
	}
	return false
}

// read is the current State of the controller in a slot.
func (b *gcBridge) read(index int, slot *gcSlot) State {
	s := State{Connected: true, Index: index, Name: slot.name}
	gp := b.send0(slot.ctrl, b.selExtGamepad)
	ls := b.send0(gp, b.selLeftStick)
	rs := b.send0(gp, b.selRightStick)
	// GameController's +Y is already up.
	s.LeftX = clamp(b.sendFloat(b.send0(ls, b.selXAxis), b.selValue), -1, 1)
	s.LeftY = clamp(b.sendFloat(b.send0(ls, b.selYAxis), b.selValue), -1, 1)
	s.RightX = clamp(b.sendFloat(b.send0(rs, b.selXAxis), b.selValue), -1, 1)
	s.RightY = clamp(b.sendFloat(b.send0(rs, b.selYAxis), b.selValue), -1, 1)

	lt := b.send0(gp, b.selLeftTrigger)
	rt := b.send0(gp, b.selRightTrigger)
	s.LeftTrigger = clamp(b.sendFloat(lt, b.selValue), 0, 1)
	s.RightTrigger = clamp(b.sendFloat(rt, b.selValue), 0, 1)
	if s.LeftTrigger > 0.5 {
		s.Buttons |= ButtonL2
	}
	if s.RightTrigger > 0.5 {
		s.Buttons |= ButtonR2
	}

	dpad := b.send0(gp, b.selDPad)
	for _, in := range []struct {
		el  objcID
		bit Button
	}{
		{b.send0(gp, b.selButtonA), ButtonSouth},
		{b.send0(gp, b.selButtonB), ButtonEast},
		{b.send0(gp, b.selButtonX), ButtonWest},
		{b.send0(gp, b.selButtonY), ButtonNorth},
		{b.send0(gp, b.selLeftShoulder), ButtonL1},
		{b.send0(gp, b.selRightShould), ButtonR1},
		{lt, ButtonL2},
		{rt, ButtonR2},
		{b.send0(gp, b.selButtonOpts), ButtonSelect},
		{b.send0(gp, b.selButtonMenu), ButtonStart},
		{b.send0(gp, b.selLeftThumbBtn), ButtonL3},
		{b.send0(gp, b.selRightThmbBtn), ButtonR3},
		{b.send0(dpad, b.selUp), ButtonDPadUp},
		{b.send0(dpad, b.selDown), ButtonDPadDown},
		{b.send0(dpad, b.selLeft), ButtonDPadLeft},
		{b.send0(dpad, b.selRight), ButtonDPadRight},
	} {
		if b.sendBool(in.el, b.selIsPressed) {
			s.Buttons |= in.bit
		}
	}
	return s
}
