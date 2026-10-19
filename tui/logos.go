package main

// Technology logos from ascii.rest (github.com/bas3line/ascii, MIT): each is
// ASCII art traced from the devicon SVG, a map of which brand colour each
// cell takes, and a glint that crosses it every few seconds. The art lives in
// logos_data.go (the library's own) and logos_traced.go (traced with its kit
// by tools/logos). Logos are trademarks of their owners.

import (
	"math"
	"strings"
)

type logoData struct {
	name       string
	cols, rows int
	small      bool     // traced at the size it shows, so it is never halved
	palette    []string // the colours for a light page, then a dark one; each plain, lifted, and glinting
	art, ink   []string
}

// logoArt is every logo — the five from the ascii.rest library, the rest
// traced with its kit (logos_traced.go) — in the order the stack lists them.
var logoArt = func() []logoData {
	all := append(append([]logoData{}, libraryLogos...), tracedLogos...)
	var out []logoData
	for _, g := range techGroups {
		for _, it := range g.items {
			for _, l := range all {
				if l.name == it {
					out = append(out, l)
				}
			}
		}
	}
	return out
}()

// logoFor finds the logo for a stack item, if ascii.rest has one.
func logoFor(item string) int {
	for i, l := range logoArt {
		if l.name == item {
			return i
		}
	}
	return -1
}

// logoBox is the logo's inked area, so blank margins don't eat the pane.
func (l logoData) box() (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = l.cols, l.rows, -1, -1
	for y, line := range l.art {
		for x, ch := range []rune(line) {
			if ch != ' ' {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	return
}

// logoSize is how big the logo draws at full size or halved.
func logoSize(l logoData, half bool) (int, int) {
	x0, y0, x1, y1 := l.box()
	w, h := x1-x0+1, y1-y0+1
	if half && !l.small {
		return (w + 1) / 2, (h + 1) / 2
	}
	return w, h
}

const (
	glintPass  = 2.0 // seconds a glint takes to cross
	glintEvery = 6.0
	glintHalf  = 5.0 // half its width, in cells
	glintLean  = 0.9 // cells it shifts left a row down, so it leans like a slash
	glintSolid = "8dbqpPYOo0"
)

// drawLogo draws logo l, full size or halved, with the glint at time t.
// reveal (0–1) decodes it in out of noise along a diagonal, for a change of logo.
func drawLogo(l logoData, half bool, t, reveal float64, p palette) *canvas {
	half = half && !l.small
	x0, y0, x1, y1 := l.box()
	art := make([][]rune, l.rows)
	for y := range art {
		art[y] = []rune(l.art[y])
	}
	ink := func(x, y int) int {
		rs := []rune(l.ink[y])
		if x >= len(rs) || rs[x] == ' ' {
			return 0
		}
		v := rs[x]
		switch {
		case v >= '0' && v <= '9':
			return int(v - '0')
		case v >= 'a' && v <= 'z':
			return int(v-'a') + 10
		}
		return 0
	}
	n := len(l.palette) / 6
	base := 3 * n // the dark-page half
	if p.sel.luma() > 0.5 {
		base = 0
	}
	colour := func(level, i int) rgb {
		return hexRGB(l.palette[min(base+level*n+i, len(l.palette)-1)])
	}

	// the glint's position, in full-size cells along x + lean·y
	lo, hi := float64(x0)+glintLean*float64(y0), float64(x1)+glintLean*float64(y1)
	span := hi - lo + 2*glintHalf
	at := -1e9
	if since := t - 0.5; since >= 0 {
		at = lo - glintHalf + span*math.Mod(since, glintEvery)/glintPass
	}
	glint := func(x, y float64) float64 {
		d := math.Abs(x + glintLean*y - at)
		k := clamp01(1 - d/glintHalf)
		return k * k * (3 - 2*k)
	}
	level := func(k float64) int {
		switch {
		case k > 0.6:
			return 2
		case k > 0.25:
			return 1
		}
		return 0
	}
	// cells decode in along a diagonal sweep
	hidden := func(x, y int) bool {
		if reveal >= 1 {
			return false
		}
		f := (float64(x-x0) + 0.9*float64(y-y0)) / (float64(x1-x0) + 0.9*float64(y1-y0) + 1)
		return reveal < f*0.75+rand3(x, y, 241)*0.25
	}

	w, h := logoSize(l, half)
	cv := newCanvas(w, h)
	if !half {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				ch := art[y][x]
				if ch == ' ' {
					continue
				}
				if hidden(x, y) {
					cv.set(x-x0, y-y0, scrambleGlyphs[hash3(x, y, int(t*20))%uint32(len(scrambleGlyphs))], p.faint)
					continue
				}
				k := glint(float64(x), float64(y))
				if k > 0.55 && strings.ContainsRune(glintSolid, ch) {
					ch = '/'
				}
				cv.set(x-x0, y-y0, ch, colour(level(k), ink(x, y)))
			}
		}
		return cv
	}

	// Halved: each cell stands for a 2×2 block of the art. Every glyph is
	// read as how much ink it puts in the top and bottom of its cell, and the
	// block's ink picks the glyph whose shape matches best.
	for ty := 0; ty < h; ty++ {
		for tx := 0; tx < w; tx++ {
			sx, sy := x0+tx*2, y0+ty*2
			var top, bot, left, right float64
			counts := map[int]float64{}
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					x, y := sx+dx, sy+dy
					if x > x1 || y > y1 {
						continue
					}
					a, b := coverage(art[y][x])
					if dy == 0 {
						top += (a + b) / 4
					} else {
						bot += (a + b) / 4
					}
					if dx == 0 {
						left += (a + b) / 4
					} else {
						right += (a + b) / 4
					}
					if a+b > 0 {
						counts[ink(x, y)] += a + b
					}
				}
			}
			ch := halfGlyph(top, bot, left, right)
			if ch == ' ' {
				continue
			}
			if hidden(sx, sy) {
				cv.set(tx, ty, scrambleGlyphs[hash3(sx, sy, int(t*20))%uint32(len(scrambleGlyphs))], p.faint)
				continue
			}
			best, most := 0, -1.0
			for i, c := range counts {
				if c > most || (c == most && i < best) {
					best, most = i, c
				}
			}
			k := glint(float64(sx)+0.5, float64(sy)+0.5)
			if k > 0.55 && strings.ContainsRune(glintSolid, ch) {
				ch = '/'
			}
			cv.set(tx, ty, ch, colour(level(k), best))
		}
	}
	return cv
}

// coverage is how much ink a glyph of the art puts in the top and the bottom
// half of its cell, roughly.
func coverage(r rune) (top, bottom float64) {
	switch {
	case r == ' ':
		return 0, 0
	case strings.ContainsRune("8#@0O%&$MWBNHKRDEGd b", r):
		return 1, 1
	case strings.ContainsRune("qpg", r):
		return 0.45, 1
	case strings.ContainsRune("PYT^", r):
		return 1, 0.45
	case r == '"':
		return 0.55, 0
	case strings.ContainsRune("'`", r):
		return 0.3, 0
	case r == '_':
		return 0, 0.45
	case strings.ContainsRune(".,", r):
		return 0, 0.3
	case strings.ContainsRune("|()[]\\/!lI1", r):
		return 0.55, 0.55
	case strings.ContainsRune(":;=-~+*", r):
		return 0.35, 0.35
	case strings.ContainsRune("acemnorsuvwxz", r):
		return 0.25, 0.75
	}
	return 0.7, 0.7
}

// halfGlyph picks the glyph for a halved block from the ink in its halves.
func halfGlyph(top, bot, left, right float64) rune {
	all := (top + bot) / 2
	switch {
	case all < 0.1:
		return ' '
	case top > 0.7 && bot > 0.7:
		switch {
		case left < 0.5:
			return 'd'
		case right < 0.5:
			return 'b'
		}
		return '8'
	case bot > top+0.3:
		switch {
		case bot > 0.65:
			if left >= right {
				return 'p'
			}
			return 'q'
		case bot > 0.3:
			return '_'
		}
		return '.'
	case top > bot+0.3:
		switch {
		case top > 0.65:
			return 'P'
		case top > 0.3:
			return '"'
		}
		return '\''
	case all > 0.55:
		return 'O'
	case all > 0.3:
		return ':'
	}
	return '.'
}
