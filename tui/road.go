package main

import (
	"math"
	"time"
	"unicode/utf8"
)

// road is the Overview's strip: a faint ground line with a few pebbles and
// cacti, in the spirit of the offline dinosaur game, and a stick figure on it
// for everyone connected right now. They walk with a / d, and that is all
// they do — it only says "someone else is here". The scene stays low-contrast
// and pinned to the bottom so the text above it keeps the focus.
func road(w, h int, o sceneOpts, p palette) *canvas {
	cv := newCanvas(w, h)
	if w < 30 || h < 4 {
		return cv
	}
	t := o.t

	// a row of pebbles under the line when there's room; feet stand on it
	ground := h - 1
	if h >= 6 {
		ground = h - 2
	}
	feet := ground - 1

	line := p.faint.mix(p.bg, 0.1)
	speck := p.faint.mix(p.bg, 0.35)
	for x := 0; x < w; x++ {
		cv.set(x, ground, '─', line)
		if ground+1 < h {
			if r := rand3(x, 0, 61); r < 0.07 {
				cv.set(x, ground+1, []rune(".,`-.")[int(r*100)%5], speck)
			}
		}
	}

	// a few cacti and a rock, placed once by hash and kept clear of the edges
	plant := p.faint.mix(p.bg, 0.15)
	for i := 0; i < clamp(w/45+1, 1, 3); i++ {
		x := 3 + int(rand3(i, 1, 62)*float64(w-6))
		if hash3(i, 2, 62)%3 == 0 {
			cv.set(x, feet, '▂', plant)
			continue
		}
		cv.set(x, feet, '|', plant)
		cv.set(x, feet-1, 'Y', plant)
	}

	// the odd cloud, drifting far too slowly to notice, when there's sky to spare
	if h >= 8 {
		for i := 0; i < 2; i++ {
			span := w + 12
			x := int(math.Mod(rand3(i, 3, 63)*float64(span)+t*0.35, float64(span))) - 6
			y := i
			cv.text(x, y, ".--.", speck)
			cv.text(x-1, y+1, "(____)", speck)
		}
	}

	// everyone currently connected; you are drawn last so you're never hidden
	var self *visitor
	for i, v := range o.visitors {
		if v.id == o.self {
			self = &o.visitors[i]
			continue
		}
		drawWalker(cv, v, feet, false, o, p)
	}
	if self != nil {
		drawWalker(cv, *self, feet, true, o, p)
	}

	// how to join in: a line in the corner (it beats name tags for the spot),
	// and the keys beside you until you walk
	if o.hint {
		hintCol := p.faint.mix(p.dim, 0.4)
		tip := "a / d to walk · you're in green"
		if w < utf8.RuneCountInString(tip)+8 {
			tip = "a / d to walk"
		}
		n := utf8.RuneCountInString(tip)
		if x := w - n - 1; blank(cv, x, x+n-1, 0) {
			cv.text(x, 0, tip, hintCol)
		}
		if self != nil {
			x := walkerX(*self, w)
			keys := p.green.mix(p.bg, 0.35)
			cv.text(x-5, feet-1, "a ←", keys)
			cv.text(x+3, feet-1, "→ d", keys)
		}
	}

	// name tags above the heads: yours first, so it wins when two collide
	if self != nil {
		drawTag(cv, *self, feet, o, p, p.green.mix(p.bg, 0.25))
	}
	tag := p.faint.mix(p.dim, 0.3)
	for _, v := range o.visitors {
		if v.id != o.self {
			drawTag(cv, v, feet, o, p, tag)
		}
	}
	return cv
}

// walkerX is a visitor's centre column, kept clear of the edges so the
// three-wide figure always fits.
func walkerX(v visitor, w int) int {
	return clamp(int(math.Round(v.x*float64(w-1))), 1, w-2)
}

// hopLift is 1 for a moment every so often: an idle figure's little jump, on
// a period of 8–15 s that differs per visitor and skips about half its turns.
func hopLift(id int, t float64) int {
	period := 8 + float64(hash3(id, 0, 64)%8)
	k := t + rand3(id, 1, 64)*period
	if math.Mod(k, period) < 0.4 && hash3(id, int(k/period), 65)%2 == 0 {
		return 1
	}
	return 0
}

// drawWalker draws one stick figure standing on feet; the legs swap frames
// while its owner is walking.
func drawWalker(cv *canvas, v visitor, feet int, self bool, o sceneOpts, p palette) {
	x := walkerX(v, cv.w)
	col := p.dim.mix(p.bg, 0.25)
	if self {
		col = p.green
	}
	legs := "/ \\"
	lift := hopLift(v.id, o.t)
	if time.Since(v.movedAt) < 300*time.Millisecond {
		lift = 0
		legs = " |\\"
		if (x+int(o.t*8))%2 == 0 {
			legs = "/| "
		}
	}
	for i, r := range "/|\\" {
		cv.set(x-1+i, feet-1-lift, r, col)
	}
	cv.set(x, feet-2-lift, 'o', col)
	for i, r := range legs {
		cv.set(x-1+i, feet-lift, r, col)
	}
}

// drawTag puts a visitor's name centred above their head, unless that would
// run into something already there.
func drawTag(cv *canvas, v visitor, feet int, o sceneOpts, p palette, col rgb) {
	n := utf8.RuneCountInString(v.name)
	x := clamp(walkerX(v, cv.w)-n/2, 0, max(cv.w-n, 0))
	y := feet - 3 - hopLift(v.id, o.t)
	if time.Since(v.movedAt) < 300*time.Millisecond {
		y = feet - 3
	}
	if blank(cv, x-1, x+n, y) {
		cv.text(x, y, v.name, col)
	}
}

// blank reports whether cells x0..x1 of row y are empty (off-canvas counts as
// not empty, so nothing is placed half outside).
func blank(cv *canvas, x0, x1, y int) bool {
	if y < 0 || y >= cv.h {
		return false
	}
	for x := max(x0, 0); x <= min(x1, cv.w-1); x++ {
		if cv.at(x, y).ch != ' ' {
			return false
		}
	}
	return true
}
