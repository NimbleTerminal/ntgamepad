// Copyright (c) 2026 Neomantra Corp.

//go:build darwin && !js

package ntgamepad

import (
	"runtime"
	"slices"
	"sync"
	"time"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// macOS driver for the pads this package reads over HID itself, without cgo:
// the Switch 2 Pro Controller (see switch2.go), which the system does not
// recognise, and the DualSense (see dualsense.go), which the system shows
// only to programs that run a main run loop.
//
// Two things run apart:
//
//   - A background goroutine looks for the pad once a second. When one
//     appears it opens the pad's vendor USB interface through
//     IOUSBHost.framework, reads the stick calibration, sends the start-up
//     sequence, and lets the interface go. It then opens the pad's HID device
//     through IOKit and looks up its button and axis elements.
//   - A poll asks IOKit for the current value of each of those elements. That
//     reads a value the system already holds, so it does no device I/O and
//     needs no run loop, and it never waits.
//
// A pad that is unplugged fails its next poll and is dropped; the goroutine
// then frees what it held.

// cfn is one C function and how to call it.
type cfn struct {
	ptr unsafe.Pointer
	cif types.CallInterface
}

func (f *cfn) call(ret unsafe.Pointer, args ...unsafe.Pointer) {
	_, _ = ffi.CallFunction(&f.cif, f.ptr, ret, args)
}

const (
	// slotKey.kind for the pads this driver reads over HID.
	switch2SlotKind   = 2
	dualSenseSlotKind = 3
	// HID usage pages and usages of the elements a poll reads.
	hidPageDesktop = 0x01
	hidPageButton  = 0x09
	hidUsageX      = 0x30 // then Y, Z, Rx, Ry, Rz in order
	hidUsageRz     = 0x35
	hidUsageHat    = 0x39
)

// Indices into sw2Pad.axis: the desktop usages X to Rz, in usage order.
const (
	axisX = iota
	axisY
	axisZ
	axisRx
	axisRy
	axisRz
	axisCount
)

// hidKind is one kind of pad this driver reads over HID.
type hidKind struct {
	vendor, product int64
	slotKind        uint8
	// usb marks a pad found and started through its USB command interface
	// (the Switch 2 pad); the others are found as HID devices and need no
	// starting.
	usb bool
}

var (
	kindSwitch2   = &hidKind{vendor: switch2VendorID, product: switch2ProductID, slotKind: switch2SlotKind, usb: true}
	kindDualSense = &hidKind{vendor: dualSenseVendorID, product: dualSenseProductID, slotKind: dualSenseSlotKind}
)

// sw2Pad is one connected pad. The goroutine builds it and never changes it
// afterwards, except to free it once dead is set.
type sw2Pad struct {
	kind     *hidKind
	location uint64
	slot     int
	cal      switch2Calibration // the Switch 2 pad's sticks
	// Retained: the HID manager, the set of devices it matched (which keeps
	// dev alive) and dev's element array (which keeps the elements alive).
	manager, devices, elements uintptr
	dev                        uintptr
	buttons                    [switch2ButtonCount]uintptr // by button usage, from 1
	axis                       [axisCount]uintptr
	hat                        uintptr
	// enhanced is the element holding the DualSense's enhanced report, when
	// the device has one.
	enhanced uintptr
	dead     bool // set by a poll that failed; guarded by sw2Driver.mu
}

type sw2Driver struct {
	start sync.Once
	ok    bool

	// mu guards pads and each pad's dead flag. Polls hold it briefly; the
	// goroutine holds it only to publish or remove a pad, never during I/O.
	mu   sync.Mutex
	pads []*sw2Pad
	// seenHID is the DualSense locations the goroutine's last pass found;
	// only the goroutine uses it.
	seenHID []uint64

	ioServiceMatching, ioServiceGetMatchingServices, ioIteratorNext, ioObjectRelease *cfn
	ioRegistryEntryCreateCFProperty, cfRelease                                       *cfn
	hidManagerCreate, hidManagerSetDeviceMatching, hidManagerOpen, hidManagerClose   *cfn
	hidManagerCopyDevices, hidDeviceGetProperty, hidDeviceCopyMatchingElements       *cfn
	hidElementGetUsagePage, hidElementGetUsage, hidDeviceGetValue                    *cfn
	hidValueGetIntegerValue, hidValueGetTimeStamp, hidValueGetLength                 *cfn
	hidValueGetBytePtr, hidElementGetReportID, hidElementGetType                     *cfn

	// objc_msgSend, by the shape of the message.
	msg       *cfn // id msg(id, SEL)
	msgObj    *cfn // id msg(id, SEL, id)
	msgU64    *cfn // id msg(id, SEL, u64)
	msgI64Ret *cfn // long long msg(id, SEL)
	msgVoid   *cfn // void msg(id, SEL)
	msgSet    *cfn // void msg(id, SEL, id, id)
	msgBytes  *cfn // id msg(id, SEL, const void *, u64)
	msgPipe   *cfn // id msg(id, SEL, u64, NSError **)
	msgInit   *cfn // id msg(id, SEL, u32, u64, id, NSError **, block)
	msgIO     *cfn // BOOL msg(id, SEL, id, u64 *, double, NSError **)

	clsString, clsNumber, clsDict, clsData, clsInterface objcID

	selAlloc, selRelease, selCount, selObjectAtIdx, selAllObjects, selLongLong objcSEL
	selStringUTF8, selNumberInt, selDictionary, selSetObject                   objcSEL
	selDataBytes, selDataLength, selMutableBytes                               objcSEL
	selInitService, selConfigDescriptor, selCopyPipe, selSendIO, selDestroy    objcSEL
}

var sw2 sw2Driver

// begin starts the driver once: it resolves everything it calls and starts
// the goroutine. It is a no-op when the frameworks cannot be loaded.
func (d *sw2Driver) begin() {
	d.start.Do(func() {
		bridge.mu.Lock()
		loaded := bridge.load()
		bridge.mu.Unlock()
		if !loaded || !d.load() {
			return
		}
		d.ok = true
		go d.run()
	})
}

func (d *sw2Driver) load() bool {
	iokit, err := ffi.LoadLibrary("/System/Library/Frameworks/IOKit.framework/IOKit")
	if err != nil {
		return false
	}
	cf, err := ffi.LoadLibrary("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	if err != nil {
		return false
	}
	if _, err := ffi.LoadLibrary("/System/Library/Frameworks/IOUSBHost.framework/IOUSBHost"); err != nil {
		return false
	}
	ptr, u32, u64, i32 := types.PointerTypeDescriptor, types.UInt32TypeDescriptor, types.UInt64TypeDescriptor, types.SInt32TypeDescriptor
	void, u8, dbl, i64 := types.VoidTypeDescriptor, types.UInt8TypeDescriptor, types.DoubleTypeDescriptor, types.SInt64TypeDescriptor
	good := true
	fn := func(lib unsafe.Pointer, name string, ret *types.TypeDescriptor, args ...*types.TypeDescriptor) *cfn {
		f := &cfn{}
		if lib != nil {
			p, err := ffi.GetSymbol(lib, name)
			if err != nil {
				good = false
				return f
			}
			f.ptr = p
		} else {
			f.ptr = bridge.objcMsgSend
		}
		if ffi.PrepareCallInterface(&f.cif, types.DefaultCall, ret, args) != nil {
			good = false
		}
		return f
	}
	d.ioServiceMatching = fn(iokit, "IOServiceMatching", ptr, ptr)
	d.ioServiceGetMatchingServices = fn(iokit, "IOServiceGetMatchingServices", i32, u32, ptr, ptr)
	d.ioIteratorNext = fn(iokit, "IOIteratorNext", u32, u32)
	d.ioObjectRelease = fn(iokit, "IOObjectRelease", i32, u32)
	d.ioRegistryEntryCreateCFProperty = fn(iokit, "IORegistryEntryCreateCFProperty", ptr, u32, ptr, ptr, u32)
	d.cfRelease = fn(cf, "CFRelease", void, ptr)
	d.hidManagerCreate = fn(iokit, "IOHIDManagerCreate", ptr, ptr, u32)
	d.hidManagerSetDeviceMatching = fn(iokit, "IOHIDManagerSetDeviceMatching", void, ptr, ptr)
	d.hidManagerOpen = fn(iokit, "IOHIDManagerOpen", i32, ptr, u32)
	d.hidManagerClose = fn(iokit, "IOHIDManagerClose", i32, ptr, u32)
	d.hidManagerCopyDevices = fn(iokit, "IOHIDManagerCopyDevices", ptr, ptr)
	d.hidDeviceGetProperty = fn(iokit, "IOHIDDeviceGetProperty", ptr, ptr, ptr)
	d.hidDeviceCopyMatchingElements = fn(iokit, "IOHIDDeviceCopyMatchingElements", ptr, ptr, ptr, u32)
	d.hidElementGetUsagePage = fn(iokit, "IOHIDElementGetUsagePage", u32, ptr)
	d.hidElementGetUsage = fn(iokit, "IOHIDElementGetUsage", u32, ptr)
	d.hidDeviceGetValue = fn(iokit, "IOHIDDeviceGetValue", i32, ptr, ptr, ptr)
	d.hidValueGetIntegerValue = fn(iokit, "IOHIDValueGetIntegerValue", i64, ptr)
	d.hidValueGetTimeStamp = fn(iokit, "IOHIDValueGetTimeStamp", u64, ptr)
	d.hidValueGetLength = fn(iokit, "IOHIDValueGetLength", i64, ptr)
	d.hidValueGetBytePtr = fn(iokit, "IOHIDValueGetBytePtr", ptr, ptr)
	d.hidElementGetReportID = fn(iokit, "IOHIDElementGetReportID", u32, ptr)
	d.hidElementGetType = fn(iokit, "IOHIDElementGetType", u32, ptr)

	d.msg = fn(nil, "", ptr, ptr, ptr)
	d.msgObj = fn(nil, "", ptr, ptr, ptr, ptr)
	d.msgU64 = fn(nil, "", ptr, ptr, ptr, u64)
	d.msgI64Ret = fn(nil, "", i64, ptr, ptr)
	d.msgVoid = fn(nil, "", void, ptr, ptr)
	d.msgSet = fn(nil, "", void, ptr, ptr, ptr, ptr)
	d.msgBytes = fn(nil, "", ptr, ptr, ptr, ptr, u64)
	d.msgPipe = fn(nil, "", ptr, ptr, ptr, u64, ptr)
	d.msgInit = fn(nil, "", ptr, ptr, ptr, u32, u64, ptr, ptr, ptr)
	d.msgIO = fn(nil, "", u8, ptr, ptr, ptr, ptr, dbl, ptr)
	if !good {
		return false
	}

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	d.clsString = bridge.getClass("NSString")
	d.clsNumber = bridge.getClass("NSNumber")
	d.clsDict = bridge.getClass("NSMutableDictionary")
	d.clsData = bridge.getClass("NSMutableData")
	d.clsInterface = bridge.getClass("IOUSBHostInterface")
	if d.clsString == 0 || d.clsNumber == 0 || d.clsDict == 0 || d.clsData == 0 || d.clsInterface == 0 {
		return false
	}
	d.selAlloc = bridge.sel("alloc")
	d.selRelease = bridge.sel("release")
	d.selCount = bridge.sel("count")
	d.selObjectAtIdx = bridge.sel("objectAtIndex:")
	d.selAllObjects = bridge.sel("allObjects")
	d.selLongLong = bridge.sel("longLongValue")
	d.selStringUTF8 = bridge.sel("stringWithUTF8String:")
	d.selNumberInt = bridge.sel("numberWithLongLong:")
	d.selDictionary = bridge.sel("dictionary")
	d.selSetObject = bridge.sel("setObject:forKey:")
	d.selDataBytes = bridge.sel("dataWithBytes:length:")
	d.selDataLength = bridge.sel("dataWithLength:")
	d.selMutableBytes = bridge.sel("mutableBytes")
	d.selInitService = bridge.sel("initWithIOService:options:queue:error:interestHandler:")
	d.selConfigDescriptor = bridge.sel("configurationDescriptor")
	d.selCopyPipe = bridge.sel("copyPipeWithAddress:error:")
	d.selSendIO = bridge.sel("sendIORequestWithData:bytesTransferred:completionTimeout:error:")
	d.selDestroy = bridge.sel("destroy")
	return true
}

// Message helpers. Each keeps its arguments in locals for the call.

func (d *sw2Driver) send(self uintptr, cmd objcSEL) uintptr {
	if self == 0 {
		return 0
	}
	var res uintptr
	d.msg.call(unsafe.Pointer(&res), unsafe.Pointer(&self), unsafe.Pointer(&cmd))
	return res
}

// sendPtr is send for a message that returns a C pointer, not an object.
func (d *sw2Driver) sendPtr(self uintptr, cmd objcSEL) unsafe.Pointer {
	if self == 0 {
		return nil
	}
	var res unsafe.Pointer
	d.msg.call(unsafe.Pointer(&res), unsafe.Pointer(&self), unsafe.Pointer(&cmd))
	return res
}

func (d *sw2Driver) sendObj(self uintptr, cmd objcSEL, arg uintptr) uintptr {
	if self == 0 {
		return 0
	}
	var res uintptr
	d.msgObj.call(unsafe.Pointer(&res), unsafe.Pointer(&self), unsafe.Pointer(&cmd), unsafe.Pointer(&arg))
	return res
}

func (d *sw2Driver) sendU64(self uintptr, cmd objcSEL, arg uint64) uintptr {
	if self == 0 {
		return 0
	}
	var res uintptr
	d.msgU64.call(unsafe.Pointer(&res), unsafe.Pointer(&self), unsafe.Pointer(&cmd), unsafe.Pointer(&arg))
	return res
}

func (d *sw2Driver) sendVoid(self uintptr, cmd objcSEL) {
	if self != 0 {
		d.msgVoid.call(nil, unsafe.Pointer(&self), unsafe.Pointer(&cmd))
	}
}

// longLong is [obj longLongValue], for an NSNumber; zero for nil.
func (d *sw2Driver) longLong(obj uintptr) int64 {
	if obj == 0 {
		return 0
	}
	var res int64
	cmd := d.selLongLong
	d.msgI64Ret.call(unsafe.Pointer(&res), unsafe.Pointer(&obj), unsafe.Pointer(&cmd))
	return res
}

// str is an autoreleased NSString, which is also a CFString.
func (d *sw2Driver) str(s string) uintptr {
	buf := append([]byte(s), 0)
	p := unsafe.Pointer(&buf[0])
	cls, cmd := uintptr(d.clsString), d.selStringUTF8
	var res uintptr
	d.msgObj.call(unsafe.Pointer(&res), unsafe.Pointer(&cls), unsafe.Pointer(&cmd), unsafe.Pointer(&p))
	runtime.KeepAlive(buf)
	return res
}

func (d *sw2Driver) release(cf uintptr) {
	if cf != 0 {
		d.cfRelease.call(nil, unsafe.Pointer(&cf))
	}
}

// low32 reads a 32-bit result out of a full-width buffer.
func low32(buf *uint64) uint32 { return uint32(*buf) }

// registryNumber is an integer property of an I/O Registry entry, and
// whether it is there.
func (d *sw2Driver) registryNumber(entry uint32, key string) (int64, bool) {
	k := d.str(key)
	var alloc uintptr
	var opts uint32
	var prop uintptr
	d.ioRegistryEntryCreateCFProperty.call(unsafe.Pointer(&prop), unsafe.Pointer(&entry), unsafe.Pointer(&k), unsafe.Pointer(&alloc), unsafe.Pointer(&opts))
	if prop == 0 {
		return 0, false
	}
	defer d.release(prop)
	return d.longLong(prop), true
}

// usbPad is a pad's command interface as found in the I/O Registry.
type usbPad struct {
	service  uint32 // the caller releases it
	location uint64
}

// findUSB lists the command interface of every attached pad.
func (d *sw2Driver) findUSB() []usbPad {
	name := append([]byte("IOUSBHostInterface"), 0)
	np := unsafe.Pointer(&name[0])
	var matching uintptr
	d.ioServiceMatching.call(unsafe.Pointer(&matching), unsafe.Pointer(&np))
	runtime.KeepAlive(name)
	if matching == 0 {
		return nil
	}
	var port uint32
	var iter uint32
	iterp := unsafe.Pointer(&iter)
	var kr uint64
	// The call consumes the matching dictionary.
	d.ioServiceGetMatchingServices.call(unsafe.Pointer(&kr), unsafe.Pointer(&port), unsafe.Pointer(&matching), unsafe.Pointer(&iterp))
	if low32(&kr) != 0 || iter == 0 {
		return nil
	}
	defer d.objectRelease(iter)
	var out []usbPad
	for {
		var next uint64
		d.ioIteratorNext.call(unsafe.Pointer(&next), unsafe.Pointer(&iter))
		svc := low32(&next)
		if svc == 0 {
			return out
		}
		vendor, _ := d.registryNumber(svc, "idVendor")
		product, _ := d.registryNumber(svc, "idProduct")
		iface, ok := d.registryNumber(svc, "bInterfaceNumber")
		if vendor != switch2VendorID || product != switch2ProductID || !ok || iface != switch2CommandInterface {
			d.objectRelease(svc)
			continue
		}
		loc, _ := d.registryNumber(svc, "locationID")
		out = append(out, usbPad{service: svc, location: uint64(loc)})
	}
}

func (d *sw2Driver) objectRelease(obj uint32) {
	if obj != 0 {
		var kr uint64
		d.ioObjectRelease.call(unsafe.Pointer(&kr), unsafe.Pointer(&obj))
	}
}

// usbLink is an open command interface.
type usbLink struct {
	d              *sw2Driver
	iface, in, out uintptr
}

// openUSB opens a pad's command interface and its two bulk pipes. It fails
// when another program holds the interface.
func (d *sw2Driver) openUSB(service uint32) (*usbLink, bool) {
	cls, alloc := uintptr(d.clsInterface), d.selAlloc
	var obj uintptr
	d.msg.call(unsafe.Pointer(&obj), unsafe.Pointer(&cls), unsafe.Pointer(&alloc))
	if obj == 0 {
		return nil, false
	}
	var options uint64
	var queue, errp, handler uintptr
	cmd := d.selInitService
	var iface uintptr
	d.msgInit.call(unsafe.Pointer(&iface), unsafe.Pointer(&obj), unsafe.Pointer(&cmd), unsafe.Pointer(&service),
		unsafe.Pointer(&options), unsafe.Pointer(&queue), unsafe.Pointer(&errp), unsafe.Pointer(&handler))
	if iface == 0 {
		return nil, false // init released the object
	}
	l := &usbLink{d: d, iface: iface}
	desc := d.sendPtr(iface, d.selConfigDescriptor)
	if desc == nil {
		l.close()
		return nil, false
	}
	// A configuration descriptor's total length is its bytes 2 and 3.
	head := unsafe.Slice((*byte)(desc), 4)
	total := int(head[2]) | int(head[3])<<8
	if total < 9 || total > 4096 {
		l.close()
		return nil, false
	}
	config := append([]byte(nil), unsafe.Slice((*byte)(desc), total)...)
	inAddr, outAddr, ok := usbBulkEndpoints(config, switch2CommandInterface)
	if !ok {
		l.close()
		return nil, false
	}
	l.in, l.out = l.pipe(inAddr), l.pipe(outAddr)
	if l.in == 0 || l.out == 0 {
		l.close()
		return nil, false
	}
	return l, true
}

func (l *usbLink) pipe(address byte) uintptr {
	d := l.d
	addr := uint64(address)
	var errp, res uintptr
	cmd := d.selCopyPipe
	d.msgPipe.call(unsafe.Pointer(&res), unsafe.Pointer(&l.iface), unsafe.Pointer(&cmd), unsafe.Pointer(&addr), unsafe.Pointer(&errp))
	return res
}

func (l *usbLink) close() {
	d := l.d
	d.sendVoid(l.in, d.selRelease)
	d.sendVoid(l.out, d.selRelease)
	d.sendVoid(l.iface, d.selDestroy)
	d.sendVoid(l.iface, d.selRelease)
	*l = usbLink{d: d}
}

// transfer moves data over one pipe and returns how many bytes went, or -1.
func (l *usbLink) transfer(pipe, data uintptr, timeout float64) int {
	d := l.d
	var n uint64
	np := unsafe.Pointer(&n)
	var errp uintptr
	cmd := d.selSendIO
	var ok [8]byte
	d.msgIO.call(unsafe.Pointer(&ok), unsafe.Pointer(&pipe), unsafe.Pointer(&cmd), unsafe.Pointer(&data),
		unsafe.Pointer(&np), unsafe.Pointer(&timeout), unsafe.Pointer(&errp))
	if ok[0] == 0 {
		return -1
	}
	return int(n)
}

// write sends one command.
func (l *usbLink) write(cmd []byte) bool {
	d := l.d
	p := unsafe.Pointer(&cmd[0])
	n := uint64(len(cmd))
	cls, sel := uintptr(d.clsData), d.selDataBytes
	var data uintptr
	d.msgBytes.call(unsafe.Pointer(&data), unsafe.Pointer(&cls), unsafe.Pointer(&sel), unsafe.Pointer(&p), unsafe.Pointer(&n))
	runtime.KeepAlive(cmd)
	return data != 0 && l.transfer(l.out, data, 1.0) == len(cmd)
}

// read receives up to 64 bytes, or nil when nothing came in time.
func (l *usbLink) read() []byte {
	d := l.d
	data := d.sendU64(uintptr(d.clsData), d.selDataLength, 64)
	if data == 0 {
		return nil
	}
	n := l.transfer(l.in, data, 0.2)
	if n <= 0 || n > 64 {
		return nil
	}
	bytes := d.sendPtr(data, d.selMutableBytes)
	if bytes == nil {
		return nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(bytes), n)...)
}

// flash reads one block of the pad's flash, or nil.
func (l *usbLink) flash(address uint32) []byte {
	if !l.write(switch2FlashRead(address)) {
		return nil
	}
	reply := l.read()
	// The reply is 80 bytes: a full 64-byte packet and then the rest.
	if len(reply) == 64 {
		reply = append(reply, l.read()...)
	}
	return switch2FlashPayload(reply)
}

// handshake reads the calibration and starts the pad's input reports.
func (d *sw2Driver) handshake(service uint32, slot int) (switch2Calibration, bool) {
	l, ok := d.openUSB(service)
	if !ok {
		return switch2Calibration{}, false
	}
	defer l.close()
	cal := switch2Calibration{
		left:  switch2StickFrom(l.flash(switch2FlashLeftFactory), l.flash(switch2FlashLeftUser)),
		right: switch2StickFrom(l.flash(switch2FlashRightFactory), l.flash(switch2FlashRightUser)),
	}
	for _, cmd := range switch2Init(slot) {
		if !l.write(cmd) {
			return cal, false
		}
		l.read()
	}
	return cal, true
}

// openHID opens the pad's HID device, matched by its kind and its location,
// and finds the elements a poll reads. On success the pad owns the retained
// objects.
func (d *sw2Driver) openHID(p *sw2Pad) bool {
	var alloc uintptr
	var opts uint32
	d.hidManagerCreate.call(unsafe.Pointer(&p.manager), unsafe.Pointer(&alloc), unsafe.Pointer(&opts))
	if p.manager == 0 {
		return false
	}
	// Match this pad only: a manager that matched keyboards would need the
	// user's permission to monitor input.
	dict := d.send(uintptr(d.clsDict), d.selDictionary)
	set := func(key string, v int64) {
		num := d.sendU64(uintptr(d.clsNumber), d.selNumberInt, uint64(v))
		k := d.str(key)
		cmd := d.selSetObject
		d.msgSet.call(nil, unsafe.Pointer(&dict), unsafe.Pointer(&cmd), unsafe.Pointer(&num), unsafe.Pointer(&k))
	}
	set("VendorID", p.kind.vendor)
	set("ProductID", p.kind.product)
	set("LocationID", int64(p.location))
	d.hidManagerSetDeviceMatching.call(nil, unsafe.Pointer(&p.manager), unsafe.Pointer(&dict))
	var kr uint64
	d.hidManagerOpen.call(unsafe.Pointer(&kr), unsafe.Pointer(&p.manager), unsafe.Pointer(&opts))
	if low32(&kr) != 0 {
		d.closeHID(p)
		return false
	}
	d.hidManagerCopyDevices.call(unsafe.Pointer(&p.devices), unsafe.Pointer(&p.manager))
	devs := d.send(p.devices, d.selAllObjects)
	if d.send(devs, d.selCount) == 0 {
		d.closeHID(p)
		return false
	}
	p.dev = d.sendU64(devs, d.selObjectAtIdx, 0)
	var match uintptr
	d.hidDeviceCopyMatchingElements.call(unsafe.Pointer(&p.elements), unsafe.Pointer(&p.dev), unsafe.Pointer(&match), unsafe.Pointer(&opts))
	count := uint64(d.send(p.elements, d.selCount))
	for i := range count {
		el := d.sendU64(p.elements, d.selObjectAtIdx, i)
		var page, usage, kind, report uint64
		d.hidElementGetType.call(unsafe.Pointer(&kind), unsafe.Pointer(&el))
		if t := low32(&kind); t < 1 || t > 3 {
			continue // not an input element
		}
		d.hidElementGetUsagePage.call(unsafe.Pointer(&page), unsafe.Pointer(&el))
		d.hidElementGetUsage.call(unsafe.Pointer(&usage), unsafe.Pointer(&el))
		d.hidElementGetReportID.call(unsafe.Pointer(&report), unsafe.Pointer(&el))
		switch u := low32(&usage); low32(&page) {
		case hidPageButton:
			if u >= 1 && int(u) <= len(p.buttons) && p.buttons[u-1] == 0 {
				p.buttons[u-1] = el
			}
		case hidPageDesktop:
			switch {
			case u >= hidUsageX && u <= hidUsageRz && p.axis[u-hidUsageX] == 0:
				p.axis[u-hidUsageX] = el
			case u == hidUsageHat && p.hat == 0:
				p.hat = el
			}
		default:
			if p.kind == kindDualSense && low32(&report) == dualSenseEnhancedReport && p.enhanced == 0 {
				p.enhanced = el
			}
		}
	}
	if !p.complete() {
		d.closeHID(p)
		return false
	}
	return true
}

// complete reports whether openHID found the elements the pad's kind needs.
func (p *sw2Pad) complete() bool {
	have := func(els ...uintptr) bool {
		for _, el := range els {
			if el == 0 {
				return false
			}
		}
		return true
	}
	switch p.kind {
	case kindSwitch2:
		return have(p.buttons[:]...) && have(p.axis[axisX], p.axis[axisY], p.axis[axisRx], p.axis[axisRz])
	case kindDualSense:
		return have(p.buttons[:dualSenseButtonCount]...) && have(p.axis[:]...) && have(p.hat)
	}
	return false
}

// closeHID frees what openHID retained.
func (d *sw2Driver) closeHID(p *sw2Pad) {
	if p.manager != 0 {
		var opts uint32
		var kr uint64
		d.hidManagerClose.call(unsafe.Pointer(&kr), unsafe.Pointer(&p.manager), unsafe.Pointer(&opts))
	}
	d.release(p.elements)
	d.release(p.devices)
	d.release(p.manager)
	p.manager, p.devices, p.elements, p.dev = 0, 0, 0, 0
}

// valueRef is an element's current value object, owned by the system, and
// whether the device answered.
func (d *sw2Driver) valueRef(p *sw2Pad, el uintptr) (uintptr, bool) {
	var val uintptr
	valp := unsafe.Pointer(&val)
	var kr uint64
	d.hidDeviceGetValue.call(unsafe.Pointer(&kr), unsafe.Pointer(&p.dev), unsafe.Pointer(&el), unsafe.Pointer(&valp))
	return val, low32(&kr) == 0 && val != 0
}

// value is an element's current integer value, when it was last reported
// (in the system's clock ticks), and whether the device answered.
func (d *sw2Driver) value(p *sw2Pad, el uintptr) (v int64, stamp uint64, ok bool) {
	val, ok := d.valueRef(p, el)
	if !ok {
		return 0, 0, false
	}
	d.hidValueGetIntegerValue.call(unsafe.Pointer(&v), unsafe.Pointer(&val))
	d.hidValueGetTimeStamp.call(unsafe.Pointer(&stamp), unsafe.Pointer(&val))
	return v, stamp, true
}

// readSwitch2 is a Switch 2 pad's input, and whether the pad is still there.
func (d *sw2Driver) readSwitch2(p *sw2Pad) (switch2Input, bool) {
	var in switch2Input
	for i, el := range p.buttons {
		v, _, ok := d.value(p, el)
		if !ok {
			return in, false
		}
		if v != 0 {
			in.buttons |= 1 << i
		}
	}
	var axes [4]uint16
	for i, a := range [4]int{axisX, axisY, axisRx, axisRz} {
		v, _, ok := d.value(p, p.axis[a])
		if !ok {
			return in, false
		}
		axes[i] = uint16(max(0, min(0xfff, v)))
	}
	in.lx, in.ly, in.rx, in.ry = axes[0], axes[1], axes[2], axes[3]
	return in, true
}

// readDualSense is a DualSense's input, and whether the pad is still there.
// It reads the pad's plain report and, when the device has one, its enhanced
// report, and answers from whichever was reported last.
func (d *sw2Driver) readDualSense(p *sw2Pad) (dualSenseInput, bool) {
	var in dualSenseInput
	var newest uint64
	byte8 := func(v int64) uint8 { return uint8(max(0, min(255, v))) }
	for i, el := range p.buttons[:dualSenseButtonCount] {
		v, stamp, ok := d.value(p, el)
		if !ok {
			return in, false
		}
		if v != 0 {
			in.buttons |= 1 << i
		}
		newest = max(newest, stamp)
	}
	var axes [axisCount]uint8
	for i, el := range p.axis {
		v, stamp, ok := d.value(p, el)
		if !ok {
			return in, false
		}
		axes[i], newest = byte8(v), max(newest, stamp)
	}
	in.lx, in.ly, in.rx, in.ry = axes[axisX], axes[axisY], axes[axisZ], axes[axisRz]
	in.l2, in.r2 = axes[axisRx], axes[axisRy]
	hat, stamp, ok := d.value(p, p.hat)
	if !ok {
		return in, false
	}
	in.hat, newest = byte8(hat), max(newest, stamp)

	if p.enhanced != 0 {
		if val, ok := d.valueRef(p, p.enhanced); ok {
			var at uint64
			var n int64
			var bytes unsafe.Pointer
			d.hidValueGetTimeStamp.call(unsafe.Pointer(&at), unsafe.Pointer(&val))
			d.hidValueGetLength.call(unsafe.Pointer(&n), unsafe.Pointer(&val))
			d.hidValueGetBytePtr.call(unsafe.Pointer(&bytes), unsafe.Pointer(&val))
			if at >= newest && bytes != nil && n > 0 && n <= 4096 {
				if e, ok := dualSenseParseEnhanced(unsafe.Slice((*byte)(bytes), int(n))); ok {
					return e, true
				}
			}
		}
	}
	return in, true
}

// poll appends the State of every live pad to dst. The caller holds one OS
// thread and an autorelease pool. A pad that does not answer is marked dead
// for the goroutine to free. When the system framework is listing the
// DualSense to this program, the driver's own DualSense is let go, so the
// pad is shown once.
func (d *sw2Driver) poll(dst []State, frameworkHasDualSense bool) []State {
	if !d.ok {
		return dst
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range d.pads {
		if p.dead {
			continue
		}
		switch p.kind {
		case kindSwitch2:
			in, ok := d.readSwitch2(p)
			if !ok {
				p.dead = true
				continue
			}
			dst = append(dst, switch2State(p.slot, in, p.cal))
		case kindDualSense:
			in, ok := d.readDualSense(p)
			if !ok || frameworkHasDualSense {
				p.dead = true
				continue
			}
			dst = append(dst, dualSenseState(p.slot, in))
		}
	}
	return dst
}

// run is the background goroutine: once a second it frees pads that died
// and brings up pads that appeared.
func (d *sw2Driver) run() {
	runtime.LockOSThread()
	for {
		pool := bridge.pushPool()
		d.reap()
		d.discover()
		bridge.popPool(pool)
		time.Sleep(time.Second)
	}
}

// reap frees the pads a poll found dead.
func (d *sw2Driver) reap() {
	d.mu.Lock()
	var dead []*sw2Pad
	live := d.pads[:0:0]
	for _, p := range d.pads {
		if p.dead {
			dead = append(dead, p)
		} else {
			live = append(live, p)
		}
	}
	d.pads = live
	d.mu.Unlock()
	for _, p := range dead {
		d.closeHID(p)
		darwinSlots.release(slotKey{kind: p.kind.slotKind, id: p.location})
	}
}

// discover brings up every attached pad that is not already live.
func (d *sw2Driver) discover() {
	for _, u := range d.findUSB() {
		d.bringUp(kindSwitch2, u.location, u.service)
		d.objectRelease(u.service)
	}
	bridge.mu.Lock()
	framework := bridge.serves(dualSenseName)
	bridge.mu.Unlock()
	if framework {
		d.seenHID = nil
		return // the framework is showing this program the DualSense
	}
	// A pad is taken up only on the second pass that finds it, a second after
	// the first: a program that polls from the main thread is shown the pad
	// by the framework well within that, and the driver then stands aside.
	found := d.findHID(kindDualSense)
	for _, location := range found {
		if slices.Contains(d.seenHID, location) {
			d.bringUp(kindDualSense, location, 0)
		}
	}
	d.seenHID = found
}

// findHID lists the locations of the attached HID devices of a kind.
func (d *sw2Driver) findHID(kind *hidKind) []uint64 {
	name := append([]byte("IOHIDDevice"), 0)
	np := unsafe.Pointer(&name[0])
	var matching uintptr
	d.ioServiceMatching.call(unsafe.Pointer(&matching), unsafe.Pointer(&np))
	runtime.KeepAlive(name)
	if matching == 0 {
		return nil
	}
	var port uint32
	var iter uint32
	iterp := unsafe.Pointer(&iter)
	var kr uint64
	// The call consumes the matching dictionary.
	d.ioServiceGetMatchingServices.call(unsafe.Pointer(&kr), unsafe.Pointer(&port), unsafe.Pointer(&matching), unsafe.Pointer(&iterp))
	if low32(&kr) != 0 || iter == 0 {
		return nil
	}
	defer d.objectRelease(iter)
	var out []uint64
	for {
		var next uint64
		d.ioIteratorNext.call(unsafe.Pointer(&next), unsafe.Pointer(&iter))
		svc := low32(&next)
		if svc == 0 {
			return out
		}
		vendor, _ := d.registryNumber(svc, "VendorID")
		product, _ := d.registryNumber(svc, "ProductID")
		location, ok := d.registryNumber(svc, "LocationID")
		d.objectRelease(svc)
		if vendor != kind.vendor || product != kind.product || !ok {
			continue
		}
		seen := false
		for _, l := range out {
			seen = seen || l == uint64(location)
		}
		if !seen {
			out = append(out, uint64(location))
		}
	}
}

// bringUp makes the pad of a kind at a location live, unless it already is.
// service is the Switch 2 pad's command interface, to start it through.
func (d *sw2Driver) bringUp(kind *hidKind, location uint64, service uint32) {
	d.mu.Lock()
	for _, p := range d.pads {
		if p.kind == kind && p.location == location {
			d.mu.Unlock()
			return
		}
	}
	d.mu.Unlock()
	key := slotKey{kind: kind.slotKind, id: location}
	slot, ok := darwinSlots.claim(key)
	if !ok {
		return
	}
	p := &sw2Pad{kind: kind, location: location, slot: slot}
	if kind.usb {
		p.cal, ok = d.handshake(service, slot)
	}
	if !ok || !d.openHID(p) {
		// Another program may hold the interface, or the pad left; try
		// again on the next pass.
		darwinSlots.release(key)
		return
	}
	d.mu.Lock()
	d.pads = append(d.pads, p)
	d.mu.Unlock()
}
