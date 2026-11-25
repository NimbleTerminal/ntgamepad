// Copyright (c) 2026 Neomantra Corp.

//go:build linux && !js

package ntgamepad

import (
	"errors"
	"strconv"
	"sync/atomic"
	"syscall"
)

// linuxDenied is set when a joystick device is there but may not be opened.
var linuxDenied atomic.Bool

// linuxReader reads /dev/input/js0 through js3 without blocking. Slot i is
// jsI: a device that is unplugged frees its slot, and one plugged in later is
// picked up by the next poll.
type linuxReader struct {
	closed bool
	slots  [MaxPads]linuxPad
}

type linuxPad struct {
	fd   int
	open bool
	name string
	dev  joydev
	// buf holds what a read returned; pending bytes of a partial event stay
	// at its start.
	buf     [joydevEventSize * 32]byte
	pending int
}

func openPlatform() Reader { return &linuxReader{} }

func (r *linuxReader) Close() error {
	r.closed = true
	var first error
	for i := range r.slots {
		if err := r.slots[i].close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (p *linuxPad) close() error {
	if !p.open {
		return nil
	}
	p.open = false
	return syscall.Close(p.fd)
}

func (r *linuxReader) Poll() State { return first(r.PollAll(nil)) }

func (r *linuxReader) PollAll(dst []State) []State {
	dst = dst[:0]
	if r.closed {
		return dst
	}
	for i := range r.slots {
		p := &r.slots[i]
		if !p.open {
			path := "/dev/input/js" + strconv.Itoa(i)
			fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
			if err != nil {
				if errors.Is(err, syscall.EACCES) {
					linuxDenied.Store(true)
				}
				continue
			}
			*p = linuxPad{fd: fd, open: true, name: path}
		}
		if !p.drain() {
			_ = p.close()
			continue
		}
		dst = append(dst, p.dev.state(i, p.name))
	}
	return dst
}

// drain reads every event waiting on the device. It reports false when the
// device is gone.
func (p *linuxPad) drain() bool {
	for {
		n, err := syscall.Read(p.fd, p.buf[p.pending:])
		if n > 0 {
			total := p.pending + n
			used := p.dev.feed(p.buf[:total])
			p.pending = copy(p.buf[:], p.buf[used:total])
		}
		switch {
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EAGAIN):
			return true
		case err != nil || n == 0:
			// ENODEV when unplugged; a zero read is end of file.
			return false
		}
	}
}

// platformNotes is what Linux has to say to the developer.
func platformNotes() []string {
	if linuxDenied.Load() {
		return []string{noteLinuxDenied}
	}
	return nil
}
