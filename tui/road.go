package main

import (
	"fmt"
	"math"
	"time"
	"unicode/utf8"
)

// road is the Overview's strip: a faint ground line with a few pebbles, in
// the spirit of the offline dinosaur game, and a stick figure on it for
// everyone connected right now. They walk with a / d, jump with w, and strike
// whoever is in front of them with f (see fight.go). The scene stays
// low-contrast and pinned to the bottom so the text above it keeps the focus.
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
	for _, v := range o.visitors {
		drawHit(cv, v, feet, o, p)
	}

	// the corner: the latest knockout for a moment, otherwise how to join in
	// (it beats name tags for the spot); and the keys beside you until you walk
	corner, cornerCol := "", p.faint.mix(p.dim, 0.4)
	if k := o.kill; !k.at.IsZero() && o.now.Sub(k.at) < killShow {
		corner, cornerCol = k.by+" took out "+k.who, p.warm
	} else if o.hint {
		corner = "a/d walk · w jump · f strike · e weapon"
		if w < utf8.RuneCountInString(corner)+8 {
			corner = "a/d walk · w jump · f strike"
		}
		if w < utf8.RuneCountInString(corner)+8 {
			corner = "a / d to walk"
		}
	}
	if n := utf8.RuneCountInString(corner); n > 0 {
		if x := w - n - 1; blank(cv, x, x+n-1, 0) {
			cv.text(x, 0, corner, cornerCol)
		}
	}
	if o.hint && self != nil && !self.dead() {
		x := walkerX(*self, w)
		y := feet - 1 - lift(*self, feet, o)
		keys := p.green.mix(p.bg, 0.35)
		cv.text(x-5, y, "a ←", keys)
		cv.text(x+3, y, "→ d", keys)
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

// walking is whether v moved a moment ago.
func walking(v visitor, o sceneOpts) bool {
	return o.now.Sub(v.movedAt) < 300*time.Millisecond
}

// lift is how many rows v's figure is off the ground this frame: a jump, or
// the idle hop when they're just standing there, never so high that the head
// leaves the canvas.
func lift(v visitor, feet int, o sceneOpts) int {
	if v.dead() {
		return 0
	}
	l := jumpLift(v.jumpAt, o.now)
	if l == 0 && !walking(v, o) && o.now.Sub(v.swingAt) > swingShow && o.now.Sub(v.hitAt) > staggerDur {
		l = hopLift(v.id, o.t)
	}
	return clamp(l, 0, max(feet-2, 0))
}

// hurt is the colour of a hit: a red pulled a little toward the theme.
func hurt(p palette) rgb { return rgb{235, 80, 80}.mix(p.text, 0.15) }

// drawWalker draws one stick figure standing on feet, with its weapon: the
// legs swap frames while its owner walks, the weapon goes out on a strike,
// the figure flashes and flails when hit, and lies flat while knocked out.
func drawWalker(cv *canvas, v visitor, feet int, self bool, o sceneOpts, p palette) {
	x := walkerX(v, cv.w)
	col := p.dim.mix(p.bg, 0.25)
	if self {
		col = p.green
	}
	face := v.face
	if face == 0 {
		face = 1
	}

	if v.dead() {
		// fallen backwards, away from the blow, fading as they wait
		body := "<─x"
		if v.hitDir < 0 {
			body = "x─>"
		}
		k := 0.3
		if left := v.respawnAt.Sub(o.now); left < time.Second {
			k = 0.3 + 0.4*(1-left.Seconds())
		}
		cv.text(x-1, feet, body, col.mix(p.bg, k))
		return
	}
	// just back up: blinking, and nothing lands yet
	if v.guarded(o.now) && int(o.now.Sub(v.spawnAt)/(90*time.Millisecond))%2 == 1 {
		return
	}
	if o.now.Sub(v.hitAt) < flashDur {
		col = hurt(p)
	}

	l := lift(v, feet, o)
	hy, ay, ly := feet-2-l, feet-1-l, feet-l
	wp := v.arms()
	steel := p.text.mix(col, 0.45)
	if wp.glow {
		steel = p.accent
	}
	if self {
		steel = steel.mix(p.green, 0.3)
	}
	put := func(dx, y int, r rune, c rgb) { // dx is "ahead" in the facing direction
		if face < 0 {
			r = mirrorRune(r)
		}
		cv.set(x+dx*face, y, r, c)
	}

	legs := "/ \\"
	if walking(v, o) && l == 0 {
		legs = " |\\"
		if (x+int(o.t*8))%2 == 0 {
			legs = "/| "
		}
	}
	if jumpLift(v.jumpAt, o.now) > 0 {
		legs = "/ \\"
	}

	cv.set(x, hy, 'o', col)
	cv.set(x, ay, '|', col)
	switch {
	case o.now.Sub(v.hitAt) < staggerDur:
		// arms thrown up, weapon dropped out of sight for a beat
		put(-1, ay, '\\', col)
		put(1, ay, '/', col)
	case o.now.Sub(v.swingAt) < swingShow:
		put(-1, ay, '/', col)
		put(1, ay, '─', col)
		for i, r := range []rune(wp.strike) {
			put(2+i, ay, r, steel.mix(p.text, 0.3))
		}
	case wp.idle != 0:
		put(-1, ay, '/', col)
		put(1, ay, '/', col) // forward arm up, holding it
		put(2, hy, wp.idle, steel)
	default:
		put(-1, ay, '/', col)
		put(1, ay, '\\', col)
	}
	for i, r := range legs {
		cv.set(x-1+i, ly, r, col)
	}
}

// drawHit puts the marks of a fresh hit on top of everything: a spark where it
// landed and the damage floating up off the head.
func drawHit(cv *canvas, v visitor, feet int, o sceneOpts, p palette) {
	since := o.now.Sub(v.hitAt)
	if v.hitAt.IsZero() || since > 700*time.Millisecond {
		return
	}
	x := walkerX(v, cv.w)
	l := lift(v, feet, o)
	if since < 150*time.Millisecond && !v.dead() {
		cv.set(x-v.hitDir, feet-1-l, '*', p.warm)
	}
	// beside the head, away from the blow, rising; the other side if that's taken
	num := fmt.Sprintf("-%d", v.lastDmg)
	y := feet - 2 - l - int(since.Seconds()*3)
	c := hurt(p).mix(p.bg, since.Seconds())
	for _, nx := range []int{x + 2, x - 1 - len(num)} {
		if v.hitDir < 0 {
			nx = 2*x - nx - len(num) + 1 // mirrored
		}
		if blank(cv, nx, nx+len(num)-1, y) {
			cv.text(nx, y, num, c)
			break
		}
	}
}

// drawTag puts a visitor's name centred above their head, unless that would
// run into something already there. For a while after a hit a health bar goes
// above it, or in its place when there's no room above. While someone is
// knocked out the tag sits just over them with the seconds until they're up.
func drawTag(cv *canvas, v visitor, feet int, o sceneOpts, p palette, col rgb) {
	label, y := v.name, feet-3-lift(v, feet, o)
	if v.dead() {
		left := int(math.Ceil(v.respawnAt.Sub(o.now).Seconds()))
		label, y, col = fmt.Sprintf("%s %d", v.name, max(left, 1)), feet-1, col.mix(p.bg, 0.3)
	}
	cx := walkerX(v, cv.w)
	if !v.dead() && !v.hitAt.IsZero() && o.now.Sub(v.hitAt) < barShow {
		if y-1 < 0 {
			drawBar(cv, v, cx, y, p)
			return
		}
		drawBar(cv, v, cx, y-1, p)
	}
	n := utf8.RuneCountInString(label)
	x := clamp(cx-n/2, 0, max(cv.w-n, 0))
	if blank(cv, x-1, x+n, y) {
		cv.text(x, y, label, col)
	}
}

// drawBar is a seven-cell health bar centred on column cx, green going to
// amber going to red as it empties.
func drawBar(cv *canvas, v visitor, cx, y int, p palette) {
	const n = 7
	x := clamp(cx-n/2, 0, max(cv.w-n, 0))
	if !blank(cv, x, x+n-1, y) {
		return
	}
	frac := float64(v.hp) / maxHP
	fill := int(math.Ceil(frac * n))
	c := p.green
	switch {
	case frac <= 0.3:
		c = hurt(p)
	case frac <= 0.6:
		c = p.warm
	}
	for i := 0; i < n; i++ {
		if i < fill {
			cv.set(x+i, y, '━', c)
		} else {
			cv.set(x+i, y, '─', p.faint.mix(p.bg, 0.3))
		}
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
