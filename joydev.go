// Copyright (c) 2026 Neomantra Corp.

package ntgamepad

import "encoding/binary"

// Linux joystick interface (/dev/input/js*): each read yields whole
// struct js_event records of joydevEventSize bytes:
//
//	u32 time (ms) · s16 value · u8 type · u8 number
//
// On open the kernel replays the current value of every button and axis with
// joydevInit set in the type.
const (
	joydevEventSize = 8
	joydevButton    = 0x01
	joydevAxis      = 0x02
	joydevInit      = 0x80
)

// joydev accumulates one device's joystick events into the standard layout.
// It assumes the numbering the kernel's xpad driver gives (Xbox controllers,
// and others that present as one):
//
//	axes    0,1 left stick · 2 left trigger · 3,4 right stick ·
//	        5 right trigger · 6,7 d-pad hat
//	buttons 0 A · 1 B · 2 X · 3 Y · 4 LB · 5 RB · 6 back · 7 start ·
//	        8 guide · 9 left stick · 10 right stick
//
// Other drivers number differently; see the README.
type joydev struct {
	axes     [8]float64
	axisSeen [8]bool
	buttons  [16]bool
}

// joydevButtons maps xpad button numbers to bits; zero has no bit (guide).
var joydevButtons = [11]Button{
	ButtonSouth, ButtonEast, ButtonWest, ButtonNorth,
	ButtonL1, ButtonR1, ButtonSelect, ButtonStart,
	0, ButtonL3, ButtonR3,
}

// feed applies the whole events in buf and returns how many bytes it used; a
// trailing partial event is left for the caller to keep.
func (j *joydev) feed(buf []byte) int {
	used := 0
	for ; used+joydevEventSize <= len(buf); used += joydevEventSize {
		ev := buf[used : used+joydevEventSize]
		val := int16(binary.LittleEndian.Uint16(ev[4:6]))
		num := int(ev[7])
		switch ev[6] &^ joydevInit {
		case joydevButton:
			if num < len(j.buttons) {
				j.buttons[num] = val != 0
			}
		case joydevAxis:
			if num < len(j.axes) {
				j.axes[num] = clamp(float64(val)/32767.0, -1, 1)
				j.axisSeen[num] = true
			}
		}
	}
	return used
}

// state is the device's current State.
func (j *joydev) state(index int, name string) State {
	s := State{
		Connected: true,
		Index:     index,
		Name:      name,
		LeftX:     j.axes[0],
		LeftY:     -j.axes[1],
		RightX:    j.axes[3],
		RightY:    -j.axes[4],
	}
	// A trigger axis rests at -1 and is fully pulled at +1. Until the device
	// has reported it, it reads released, not half pulled.
	if j.axisSeen[2] {
		s.LeftTrigger = clamp((j.axes[2]+1)*0.5, 0, 1)
	}
	if j.axisSeen[5] {
		s.RightTrigger = clamp((j.axes[5]+1)*0.5, 0, 1)
	}
	switch {
	case j.axes[7] < -0.5:
		s.Buttons |= ButtonDPadUp
	case j.axes[7] > 0.5:
		s.Buttons |= ButtonDPadDown
	}
	switch {
	case j.axes[6] < -0.5:
		s.Buttons |= ButtonDPadLeft
	case j.axes[6] > 0.5:
		s.Buttons |= ButtonDPadRight
	}
	for i, b := range joydevButtons {
		if b != 0 && j.buttons[i] {
			s.Buttons |= b
		}
	}
	if s.LeftTrigger > 0.5 {
		s.Buttons |= ButtonL2
	}
	if s.RightTrigger > 0.5 {
		s.Buttons |= ButtonR2
	}
	return s
}
