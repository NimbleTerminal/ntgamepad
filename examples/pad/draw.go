// Copyright (c) 2026 Neomantra Corp.

package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
	"nimbleterminal.dev/ntgamepad"
)

// The graphical rendition: the controller drawn as a picture, in pixels.

const (
	picW = 720
	picH = 400
)

var (
	colBack    = color.RGBA{0x14, 0x17, 0x1c, 0xff}
	colBody    = color.RGBA{0x2a, 0x30, 0x3a, 0xff}
	colEdge    = color.RGBA{0x4a, 0x53, 0x63, 0xff}
	colWell    = color.RGBA{0x1a, 0x1e, 0x25, 0xff}
	colOff     = color.RGBA{0x3b, 0x43, 0x50, 0xff}
	colText    = color.RGBA{0xc9, 0xd1, 0xdc, 0xff}
	colMute    = color.RGBA{0x7d, 0x88, 0x99, 0xff}
	colOn      = color.RGBA{0x3b, 0xc9, 0xdb, 0xff}
	colOnText  = color.RGBA{0x0b, 0x16, 0x1a, 0xff}
	colStick   = color.RGBA{0x8c, 0x96, 0xa6, 0xff}
	colMissing = color.RGBA{0x20, 0x24, 0x2b, 0xff}
)

// canvas draws antialiased shapes and small labels onto an image. Each
// shape is rasterized in its own bounding box, so its cost follows its size
// and not the picture's.
type canvas struct {
	img *image.RGBA
	ras *vector.Rasterizer
}

func newCanvas(img *image.RGBA) canvas { return canvas{img: img, ras: &vector.Rasterizer{}} }

// fill rasterizes a path inside the box x0..x1, y0..y1. build is given the
// box's origin to subtract from its coordinates.
func (c canvas) fill(x0, y0, x1, y1 float64, col color.Color, build func(r *vector.Rasterizer, ox, oy float64)) {
	box := image.Rect(int(math.Floor(x0)), int(math.Floor(y0)), int(math.Ceil(x1))+1, int(math.Ceil(y1))+1).Intersect(c.img.Bounds())
	if box.Empty() {
		return
	}
	c.ras.Reset(box.Dx(), box.Dy())
	build(c.ras, float64(box.Min.X), float64(box.Min.Y))
	c.ras.Draw(c.img, box, image.NewUniform(col), image.Point{})
}

func (c canvas) circle(cx, cy, radius float64, col color.Color) {
	c.fill(cx-radius, cy-radius, cx+radius, cy+radius, col, func(r *vector.Rasterizer, ox, oy float64) {
		const n = 40
		for i := range n {
			a := 2 * math.Pi * float64(i) / n
			x, y := float32(cx-ox+radius*math.Cos(a)), float32(cy-oy+radius*math.Sin(a))
			if i == 0 {
				r.MoveTo(x, y)
			} else {
				r.LineTo(x, y)
			}
		}
		r.ClosePath()
	})
}

// rect fills a rectangle with rounded corners.
func (c canvas) rect(x0, y0, x1, y1, radius float64, col color.Color) {
	c.fill(x0, y0, x1, y1, col, func(r *vector.Rasterizer, ox, oy float64) {
		const n = 8
		corner := func(cx, cy, from float64) {
			for i := 0; i <= n; i++ {
				a := from + (math.Pi/2)*float64(i)/n
				r.LineTo(float32(cx-ox+radius*math.Cos(a)), float32(cy-oy+radius*math.Sin(a)))
			}
		}
		r.MoveTo(float32(x0-ox+radius), float32(y0-oy))
		corner(x1-radius, y0+radius, -math.Pi/2)
		corner(x1-radius, y1-radius, 0)
		corner(x0+radius, y1-radius, math.Pi/2)
		corner(x0+radius, y0+radius, math.Pi)
		r.ClosePath()
	})
}

// label draws text centred on (cx, cy), enlarged by a whole factor.
func (c canvas) label(cx, cy float64, text string, scale int, col color.Color) {
	face := basicfont.Face7x13
	w := font.MeasureString(face, text).Ceil()
	h := face.Metrics().Ascent.Ceil()
	small := image.NewAlpha(image.Rect(0, 0, max(1, w), h+face.Metrics().Descent.Ceil()))
	d := font.Drawer{Dst: small, Src: image.Opaque, Face: face, Dot: fixed.P(0, h)}
	d.DrawString(text)
	ox := int(cx) - w*scale/2
	oy := int(cy) - h*scale/2
	src := image.NewUniform(col)
	for y := range small.Bounds().Dy() {
		for x := range small.Bounds().Dx() {
			if small.AlphaAt(x, y).A == 0 {
				continue
			}
			cell := image.Rect(ox+x*scale, oy+y*scale, ox+(x+1)*scale, oy+(y+1)*scale)
			draw.Draw(c.img, cell, src, image.Point{}, draw.Src)
		}
	}
}

// button draws a round button, lit when down.
func (c canvas) button(cx, cy, radius float64, text string, down bool) {
	fillCol, textCol := color.Color(colOff), color.Color(colText)
	if down {
		fillCol, textCol = colOn, colOnText
	}
	c.circle(cx, cy, radius+2, colEdge)
	c.circle(cx, cy, radius, fillCol)
	if text != "" {
		c.label(cx, cy, text, 2, textCol)
	}
}

// pill draws a wide button, lit when down.
func (c canvas) pill(x0, y0, x1, y1 float64, text string, down bool) {
	fillCol, textCol := color.Color(colOff), color.Color(colText)
	if down {
		fillCol, textCol = colOn, colOnText
	}
	c.rect(x0-2, y0-2, x1+2, y1+2, (y1-y0)/2+2, colEdge)
	c.rect(x0, y0, x1, y1, (y1-y0)/2, fillCol)
	c.label((x0+x1)/2, (y0+y1)/2, text, 2, textCol)
}

// Stick geometry: the well's radius, how far the cap travels, the cap's
// radius.
const (
	stickWell   = 46.0
	stickTravel = 24.0
	stickCap    = 26.0
)

// well draws a stick's well, without its cap.
func (c canvas) well(cx, cy float64) {
	c.circle(cx, cy, stickWell+3, colEdge)
	c.circle(cx, cy, stickWell, colWell)
}

// cap draws a stick's cap, moved by the stick's deflection; +Y is up. It is
// lit when the stick is clicked.
func (c canvas) cap(cx, cy float64, x, y float64, clicked bool, body color.Color) {
	capCol := color.Color(colStick)
	if clicked {
		capCol = colOn
	}
	c.circle(cx+x*stickTravel, cy-y*stickTravel, stickCap, capCol)
	c.circle(cx+x*stickTravel, cy-y*stickTravel, stickCap-8, body)
}

// Where things are on the picture.
const (
	leftStickX, leftStickY   = 170.0, 170.0
	rightStickX, rightStickY = 450.0, 262.0
	dpadX, dpadY             = 270.0, 262.0
	dpadArm, dpadHalf        = 34.0, 13.0
	faceX, faceY, faceSpread = 550.0, 170.0, 40.0
)

// roundKey is a round button; wideKey a wide one; dpadKey one arm of the
// d-pad.
type roundKey struct {
	b            ntgamepad.Button
	x, y, radius float64
	text         string
}

type wideKey struct {
	b              ntgamepad.Button
	x0, y0, x1, y1 float64
	text           string
}

type dpadKey struct {
	b              ntgamepad.Button
	x0, y0, x1, y1 float64
}

var (
	roundKeys = []roundKey{
		{ntgamepad.ButtonSelect, 318, 150, 13, ""},
		{ntgamepad.ButtonStart, 402, 150, 13, ""},
		{ntgamepad.ButtonSouth, faceX, faceY + faceSpread, 19, "S"},
		{ntgamepad.ButtonEast, faceX + faceSpread, faceY, 19, "E"},
		{ntgamepad.ButtonWest, faceX - faceSpread, faceY, 19, "W"},
		{ntgamepad.ButtonNorth, faceX, faceY - faceSpread, 19, "N"},
	}
	wideKeys = []wideKey{
		{ntgamepad.ButtonL2, 96, 22, 216, 50, "L2"},
		{ntgamepad.ButtonR2, 504, 22, 624, 50, "R2"},
		{ntgamepad.ButtonL1, 80, 60, 240, 88, "L1"},
		{ntgamepad.ButtonR1, 480, 60, 640, 88, "R1"},
	}
	dpadKeys = []dpadKey{
		{ntgamepad.ButtonDPadUp, dpadX - dpadHalf, dpadY - dpadArm, dpadX + dpadHalf, dpadY - dpadHalf},
		{ntgamepad.ButtonDPadDown, dpadX - dpadHalf, dpadY + dpadHalf, dpadX + dpadHalf, dpadY + dpadArm},
		{ntgamepad.ButtonDPadLeft, dpadX - dpadArm, dpadY - dpadHalf, dpadX - dpadHalf, dpadY + dpadHalf},
		{ntgamepad.ButtonDPadRight, dpadX + dpadHalf, dpadY - dpadHalf, dpadX + dpadArm, dpadY + dpadHalf},
	}
	triggerBars = [2]float64{96, 504} // left edge of each trigger's travel bar
)

// basePad draws everything that does not change from frame to frame: the
// body, every button unlit, the stick wells. The body is dimmed for a
// controller that is not connected.
func basePad(connected bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, picW, picH))
	draw.Draw(img, img.Bounds(), image.NewUniform(colBack), image.Point{}, draw.Src)
	c := newCanvas(img)

	// Triggers and shoulders sit behind the body's top edge.
	for _, k := range wideKeys {
		c.pill(k.x0, k.y0, k.x1, k.y1, k.text, false)
	}
	for _, x := range triggerBars {
		c.rect(x, 8, x+120, 14, 3, colOff)
	}

	// Body: a wide shell with two grips.
	body := bodyColor(connected)
	c.rect(48, 96, 672, 300, 70, colEdge)
	c.rect(40, 200, 220, 388, 80, colEdge)
	c.rect(500, 200, 680, 388, 80, colEdge)
	c.rect(52, 100, 668, 296, 66, body)
	c.rect(44, 204, 216, 384, 76, body)
	c.rect(504, 204, 676, 384, 76, body)

	c.well(leftStickX, leftStickY)
	c.well(rightStickX, rightStickY)
	c.rect(dpadX-dpadArm-2, dpadY-dpadHalf-2, dpadX+dpadArm+2, dpadY+dpadHalf+2, 6, colEdge)
	c.rect(dpadX-dpadHalf-2, dpadY-dpadArm-2, dpadX+dpadHalf+2, dpadY+dpadArm+2, 6, colEdge)
	c.rect(dpadX-dpadArm, dpadY-dpadHalf, dpadX+dpadArm, dpadY+dpadHalf, 5, colOff)
	c.rect(dpadX-dpadHalf, dpadY-dpadArm, dpadX+dpadHalf, dpadY+dpadArm, 5, colOff)
	for _, k := range roundKeys {
		c.button(k.x, k.y, k.radius, k.text, false)
	}
	c.label(318, 124, "Select", 1, colMute)
	c.label(402, 124, "Start", 1, colMute)
	return img
}

func bodyColor(connected bool) color.Color {
	if connected {
		return colBody
	}
	return colMissing
}

// bases holds basePad for a controller that is missing (0) and connected (1),
// drawn on first use.
var bases [2]*image.RGBA

// drawPad renders one controller: a copy of the unchanging base with what is
// held lit, the stick caps where the sticks are, and the trigger travel. A
// State that is not connected draws the outline alone.
func drawPad(p ntgamepad.State) *image.RGBA {
	which := 0
	if p.Connected {
		which = 1
	}
	if bases[which] == nil {
		bases[which] = basePad(p.Connected)
	}
	img := image.NewRGBA(bases[which].Rect)
	copy(img.Pix, bases[which].Pix)
	c := newCanvas(img)
	down := func(b ntgamepad.Button) bool { return p.Connected && p.Buttons&b != 0 }

	for _, k := range wideKeys {
		if down(k.b) {
			c.pill(k.x0, k.y0, k.x1, k.y1, k.text, true)
		}
	}
	for _, k := range roundKeys {
		if down(k.b) {
			c.button(k.x, k.y, k.radius, k.text, true)
		}
	}
	for _, k := range dpadKeys {
		if down(k.b) {
			c.rect(k.x0, k.y0, k.x1, k.y1, 4, colOn)
		}
	}
	body := bodyColor(p.Connected)
	c.cap(leftStickX, leftStickY, p.LeftX, p.LeftY, down(ntgamepad.ButtonL3), body)
	c.cap(rightStickX, rightStickY, p.RightX, p.RightY, down(ntgamepad.ButtonR3), body)
	if p.Connected {
		for i, v := range [2]float64{p.LeftTrigger, p.RightTrigger} {
			if v > 0 {
				c.rect(triggerBars[i], 8, triggerBars[i]+max(6, 120*v), 14, 3, colOn)
			}
		}
	}
	return img
}
