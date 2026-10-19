package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/muesli/termenv"
)

// The canvas is a tiny character framebuffer for the animated scenes: every
// cell holds one glyph and one colour, scenes draw into it in layers, and it
// is turned into escape codes exactly once per frame.

// rgb is a plain colour; scenes mix these freely and only quantize them when
// the canvas is rendered.
type rgb struct{ r, g, b float64 }

func hexRGB(s string) rgb {
	var r, g, b int
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return rgb{float64(r), float64(g), float64(b)}
}

func (c rgb) mix(o rgb, t float64) rgb {
	t = clamp01(t)
	return rgb{c.r + (o.r-c.r)*t, c.g + (o.g-c.g)*t, c.b + (o.b-c.b)*t}
}

func (c rgb) luma() float64 { return (0.2126*c.r + 0.7152*c.g + 0.0722*c.b) / 255 }

// palette is a theme as mixable colours, plus a background estimate — the
// colour things fade into. Scenes never paint a background, so "fading" a
// glyph means pulling its colour toward the terminal's own.
type palette struct {
	accent, accent2, green, text, dim, faint, sel, bg rgb
	warm                                              rgb // dawn/dusk glow, window light
}

func paletteOf(t theme) palette {
	p := palette{
		accent: hexRGB(t.accent), accent2: hexRGB(t.accent2), green: hexRGB(t.green),
		text: hexRGB(t.text), dim: hexRGB(t.dim), faint: hexRGB(t.faint), sel: hexRGB(t.sel),
	}
	if p.sel.luma() > 0.5 { // light theme: fade toward the light background
		p.bg = p.sel.mix(rgb{255, 255, 255}, 0.5)
	} else {
		p.bg = p.sel.mix(rgb{}, 0.45)
	}
	p.warm = p.accent.mix(rgb{245, 169, 127}, 0.55)
	return p
}

type cell struct {
	ch rune
	fg rgb
}

type canvas struct {
	w, h  int
	cells []cell
}

func newCanvas(w, h int) *canvas {
	c := &canvas{w: max(w, 0), h: max(h, 0)}
	c.cells = make([]cell, c.w*c.h)
	for i := range c.cells {
		c.cells[i].ch = ' '
	}
	return c
}

func (c *canvas) in(x, y int) bool { return x >= 0 && y >= 0 && x < c.w && y < c.h }

func (c *canvas) set(x, y int, ch rune, fg rgb) {
	if c.in(x, y) {
		c.cells[y*c.w+x] = cell{ch, fg}
	}
}

func (c *canvas) at(x, y int) cell {
	if !c.in(x, y) {
		return cell{ch: ' '}
	}
	return c.cells[y*c.w+x]
}

func (c *canvas) text(x, y int, s string, fg rgb) {
	for i, r := range []rune(s) {
		c.set(x+i, y, r, fg)
	}
}

// seqCache maps a quantized colour to its escape sequence per colour profile.
// Guarded by renderMu, like everything else touched while rendering.
var seqCache = map[seqKey]string{}

type seqKey struct {
	p       termenv.Profile
	r, g, b uint8
}

// fgSeq quantizes to 32 levels per channel: invisible in a glyph, but it lets
// neighbouring cells share one escape code, which keeps frames small on the wire.
func fgSeq(p termenv.Profile, c rgb) string {
	q := func(v float64) uint8 { return uint8(clampf(v, 0, 255)) &^ 7 }
	k := seqKey{p, q(c.r), q(c.g), q(c.b)}
	if s, ok := seqCache[k]; ok {
		return s
	}
	s := ""
	if col := p.Color(fmt.Sprintf("#%02x%02x%02x", k.r, k.g, k.b)); col != nil {
		if seq := col.Sequence(false); seq != "" {
			s = "\x1b[" + seq + "m"
		}
	}
	seqCache[k] = s
	return s
}

// render turns the canvas into styled lines. Runs of one colour share a
// single escape; spaces never change colour.
func (c *canvas) render(p termenv.Profile) string {
	var b strings.Builder
	b.Grow(c.w * c.h * 3)
	for y := 0; y < c.h; y++ {
		cur := ""
		for x := 0; x < c.w; x++ {
			cl := c.cells[y*c.w+x]
			if cl.ch != ' ' {
				if s := fgSeq(p, cl.fg); s != cur {
					b.WriteString(s)
					cur = s
				}
			}
			b.WriteRune(cl.ch)
		}
		if cur != "" {
			b.WriteString("\x1b[0m")
		}
		if y < c.h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// --- noise & maths ---

// hash3 is a cheap integer hash; every "random" thing in the scenes is a pure
// function of position and time, so nothing needs per-session state.
func hash3(x, y, z int) uint32 {
	h := uint32(x)*374761393 + uint32(y)*668265263 + uint32(z)*2246822519
	h = (h ^ (h >> 13)) * 1274126177
	return h ^ (h >> 16)
}

func rand3(x, y, z int) float64 { return float64(hash3(x, y, z)&0xffffff) / float64(0x1000000) }

// vnoise is smooth 2D value noise in [0,1).
func vnoise(x, y float64, seed int) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	ix, iy := int(x0), int(y0)
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a, b := rand3(ix, iy, seed), rand3(ix+1, iy, seed)
	c, d := rand3(ix, iy+1, seed), rand3(ix+1, iy+1, seed)
	return lerpf(lerpf(a, b, sx), lerpf(c, d, sx), sy)
}

func lerpf(a, b, t float64) float64 { return a + (b-a)*t }

func frac(x float64) float64 { return x - math.Floor(x) }

func clampf(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func clamp01(v float64) float64 { return clampf(v, 0, 1) }

// rampAt picks a glyph from a density ramp for v in [0,1].
func rampAt(ramp []rune, v float64) rune {
	return ramp[int(clamp01(v)*float64(len(ramp)-1)+0.5)]
}
