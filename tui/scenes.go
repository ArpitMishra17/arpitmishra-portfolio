package main

import (
	"math"
	"os"
	"strconv"
	"time"
)

// sceneOpts is everything a scene needs; scenes are pure functions of it.
type sceneOpts struct {
	t        float64 // animation time, seconds
	hour     float64 // Mumbai local hour, 0–24
	moon     float64 // lunar phase, 0 new → 0.5 full → 1 new
	rain     bool
	hint     bool // show the walking keys beside this visitor until they've walked
	visitors []visitor
	self     int
}

var ist = time.FixedZone("IST", 5*3600+1800)

// mumbaiHour is the hour of day the scenes are lit for. TUI_HOUR pins it,
// handy for previewing dawn or dusk at any time: TUI_HOUR=18.5 go run . -local
func mumbaiHour() float64 {
	if v, err := strconv.ParseFloat(os.Getenv("TUI_HOUR"), 64); err == nil {
		return math.Mod(math.Abs(v), 24)
	}
	now := time.Now().In(ist)
	return float64(now.Hour()) + float64(now.Minute())/60
}

// moonPhase counts synodic months from a known new moon — the moon in the
// scene is the real one.
func moonPhase(t time.Time) float64 {
	ref := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)
	return frac(t.Sub(ref).Hours() / 24 / 29.530588853)
}

// rainyFor decides the weather a session starts with: monsoon months are
// usually wet, the rest of the year rarely. TUI_RAIN=1/0 forces it.
func rainyFor(session int) bool {
	switch os.Getenv("TUI_RAIN") {
	case "1":
		return true
	case "0":
		return false
	}
	odds := 0.08
	if m := time.Now().In(ist).Month(); m >= time.June && m <= time.September {
		odds = 0.6
	}
	return rand3(session, time.Now().YearDay(), 7) < odds
}

// skyLight is how lit the world is at a given hour.
type skyLight struct {
	dark  float64 // 0 noon … 1 deep night
	warm  float64 // glow near the horizon at dawn and dusk
	sunUp float64 // sun elevation, -1 … 1
}

func skyAt(hour float64, rain bool) skyLight {
	sunUp := math.Sin((hour - 6) / 12 * math.Pi)
	s := skyLight{dark: clamp01(0.5 - sunUp*1.2), warm: clamp01(1 - math.Abs(sunUp)*3.5), sunUp: sunUp}
	if rain {
		s.dark = clamp01(s.dark + 0.25)
		s.warm *= 0.3
	}
	return s
}

func moonLit(phase float64) float64 { return (1 - math.Cos(phase*2*math.Pi)) / 2 }

// drawSky paints stars, the sun or moon, clouds and horizon glow into rows
// [0, horizon). It returns the x the moon/sun sits at (for the water's
// glitter path) and whether one is visible.
func drawSky(cv *canvas, horizon int, o sceneOpts, p palette, sk skyLight, celestialX float64) (float64, bool) {
	w := cv.w
	t := o.t

	// stars, twinkling; fixed to absolute columns so resizing doesn't reshuffle them
	if sk.dark > 0.35 && !o.rain {
		for y := 0; y < horizon*3/5; y++ { // high up, clear of the rooftops
			for x := 0; x < w; x++ {
				r := rand3(x, y, 7)
				if r > 0.03 {
					continue
				}
				tw := 0.5 + 0.5*math.Sin(t*(1.2+r*60)+r*400)
				b := sk.dark * (0.3 + 0.7*tw) * (1 - float64(y)/float64(horizon)*0.45)
				cv.set(x, y, rampAt([]rune(".·+*"), b), p.bg.mix(p.text, b))
			}
		}
	}

	// horizon glow at dawn and dusk
	if sk.warm > 0.05 {
		band := max(horizon*2/3, 3)
		for y := max(horizon-band, 0); y < horizon; y++ {
			near := 1 - float64(horizon-1-y)/float64(band)
			for x := 0; x < w; x++ {
				n := vnoise(float64(x)*0.12+t*0.05, float64(y)*0.8, 21)
				if n*near*sk.warm > 0.22 {
					cv.set(x, y, rampAt([]rune(".-~="), n*near), p.bg.mix(p.warm, sk.warm*(0.35+0.5*near)))
				}
			}
		}
	}

	cx, shown := float64(w)*celestialX, false
	switch {
	case sk.dark > 0.3 && !o.rain && moonLit(o.moon) > 0.04:
		// the moon, shaded as a lit sphere for the real phase
		my := math.Max(1.4, float64(horizon)*0.3)
		ry := clampf(float64(horizon)*0.2, 1.0, 2.6)
		rx := ry * 2.1
		ang := o.moon * 2 * math.Pi
		lx, lz := math.Sin(ang), -math.Cos(ang)
		for y := int(my - ry - 1); y <= int(my+ry+1); y++ {
			for x := int(cx - rx - 1); x <= int(cx+rx+1); x++ {
				dx, dy := (float64(x)+0.5-cx)/rx, (float64(y)+0.5-my)/ry
				d := dx*dx + dy*dy
				if d > 1 {
					continue
				}
				z := math.Sqrt(1 - d)
				lit := dx*lx + z*lz
				if lit <= 0 {
					cv.set(x, y, '.', p.bg.mix(p.dim, 0.3)) // earthshine
					continue
				}
				b := 0.3 + 0.7*lit - vnoise(dx*3+10, dy*3, 3)*0.3
				cv.set(x, y, rampAt([]rune(".:-=+*#%@"), b), p.dim.mix(p.text, 0.35+0.65*b).mix(p.bg, 1-sk.dark))
			}
		}
		shown = true
	case sk.sunUp > -0.08:
		// the sun, sliding along an arc from east to west
		cx = float64(w) * clampf((o.hour-5.5)/13, 0.05, 0.95)
		sy := float64(horizon-1) - sk.sunUp*float64(horizon-2)
		col := p.text.mix(p.warm, 0.25+sk.warm*0.7)
		if o.rain {
			col = col.mix(p.bg, 0.6)
		}
		for y := int(sy - 2); y <= int(sy+2); y++ {
			for x := int(cx - 4); x <= int(cx+4); x++ {
				dx, dy := (float64(x)+0.5-cx)/2.6, (float64(y)+0.5-sy)/1.3
				if d := dx*dx + dy*dy; d <= 1 {
					cv.set(x, y, rampAt([]rune("o#@"), 1-d), col)
				}
			}
		}
		shown = true
	}

	// clouds only when it rains: on a clear night the sky stays clean
	th := 0.42
	for y := 0; o.rain && y < horizon-2; y++ {
		for x := 0; x < w; x++ {
			n := vnoise(float64(x)*0.06+t*0.06, float64(y)*0.5, 11)*0.75 +
				vnoise(float64(x)*0.2+t*0.1, float64(y)*0.9, 12)*0.25
			if n <= th {
				continue
			}
			v := (n - th) / (1 - th)
			col := p.bg.mix(p.dim, 0.25+v*0.5).mix(p.text, (1-sk.dark)*0.4)
			col = col.mix(p.warm, sk.warm*0.8)
			cv.set(x, y, rampAt([]rune(".-="), v*1.4), col)
		}
	}
	return cx, shown
}

// drawRain adds slanted streaks over everything plus ripple rings on the water.
func drawRain(cv *canvas, waterTop int, t float64, p palette) {
	w, h := cv.w, cv.h
	streak := p.bg.mix(p.dim, 0.55)
	for i := 0; i < w*h/16; i++ {
		speed := 11 + rand3(i, 0, 91)*7
		span := float64(h + 4)
		y := frac(rand3(i, 1, 91)+t*speed/span)*span - 2
		x := rand3(i, 2, 91)*float64(w+h/2) - y*0.5
		cv.set(int(x), int(y), '/', streak)
	}
	water := h - waterTop
	if water <= 0 {
		return
	}
	for i := 0; i < w*water/30; i++ {
		ph := t*0.9 + rand3(i, 3, 93)
		cyc := int(ph)
		ph = frac(ph)
		x := int(rand3(i, cyc, 95) * float64(w))
		y := waterTop + int(rand3(i, cyc, 97)*float64(water))
		col := p.text.mix(p.bg, 0.35+ph*0.5)
		switch {
		case ph < 0.2:
			cv.set(x, y, '.', col)
		case ph < 0.45:
			cv.set(x, y, 'o', col)
		case ph < 0.75:
			cv.set(x-1, y, '(', col)
			cv.set(x+1, y, ')', col)
		}
	}
}

// light is a bright point above the water — a lamp, a lit window — that the
// water reflects. Lamps at the waterline throw long streaks; the rest a dash.
type light struct {
	x, y   int
	col    rgb
	streak bool
}

// drawWater fills rows [top, h) with a calm swell and reflects only the
// lights: a shimmering streak under each streetlight, a broken dash under
// each window, and the moon's (or sun's) glitter path.
func drawWater(cv *canvas, top int, o sceneOpts, p palette, lights []light, glitterX, glitter float64) {
	w, h := cv.w, cv.h
	t := o.t
	water := h - top
	tint := p.faint.mix(p.accent2, 0.3)
	// each row sways sideways a little, so its reflections wobble together
	sway := make([]int, water)
	for k := range sway {
		sway[k] = int(math.Round(math.Sin(float64(k)*1.3+t*2.1)*(0.4+float64(k)*0.15) +
			(vnoise(float64(k)*0.7, t*0.8, 71)-0.5)*1.2))
	}
	for k := 0; k < water; k++ {
		depth := float64(k) / float64(max(water, 1))
		for x := 0; x < w; x++ {
			n := vnoise(float64(x)*0.14-t*0.8, float64(k)*0.9+t*0.25, 73)
			switch {
			case n > 0.8:
				cv.set(x, top+k, '~', p.bg.mix(tint, 0.7-depth*0.3))
			case n > 0.74:
				cv.set(x, top+k, '-', p.bg.mix(tint, 0.55-depth*0.3))
			}
		}
	}
	for _, l := range lights {
		if l.streak {
			// lengths and breaks wander, so the streaks never line up like a fence
			n := 1 + int(vnoise(float64(l.x)*0.37, t*0.35, 81)*float64(min(water, 5)))
			for k := 0; k < n && k < water; k++ {
				f := vnoise(float64(l.x)*0.53, float64(k)*1.1-t*2.5, 83)
				if f < 0.45+float64(k)*0.06 {
					continue
				}
				ch := ':'
				if f > 0.7 && k < 2 {
					ch = '|'
				} else if k >= 2 {
					ch = '.'
				}
				cv.set(l.x+sway[k], top+k, ch, l.col.mix(p.bg, 0.35+float64(k)*0.14))
			}
			continue
		}
		k := top - 1 - l.y // mirrored: as far below the surface as it is above
		if k < 0 || k >= water || vnoise(float64(l.x)*0.6, t*1.5+float64(k), 85) < 0.4 {
			continue
		}
		cv.set(l.x+sway[k], top+k, '-', l.col.mix(p.bg, 0.45+float64(k)/float64(water)*0.3))
	}
	if glitterX >= 0 && glitter > 0.05 {
		for k := 0; k < water; k++ {
			depth := float64(k) / float64(water)
			half := 1.5 + float64(k)*0.9
			for x := int(glitterX - half); x <= int(glitterX+half); x++ {
				g := vnoise(float64(x)*0.7+t*1.6, float64(k)*1.3-t*0.5, 89)
				edge := math.Abs(float64(x)-glitterX) / half
				if g > 0.5+edge*0.25 {
					ch := '-'
					if g > 0.72 {
						ch = '='
					}
					cv.set(x+sway[k], top+k, ch, p.text.mix(p.bg, 1-glitter*(1-depth*0.5)*(1-edge*0.6)))
				}
			}
		}
	}
}

// marineDrive is Mumbai's seafront at the real local hour: the skyline with
// its lit windows, the Queen's Necklace of streetlights along the sea wall,
// and the lights mirrored in the bay. It plays as a screensaver.
func marineDrive(w, h int, o sceneOpts, p palette) *canvas {
	cv := newCanvas(w, h)
	if w < 10 || h < 6 {
		return cv
	}
	t := o.t
	sk := skyAt(o.hour, o.rain)

	water := max(3, int(math.Round(float64(h)*0.38)))
	top := h - water   // first water row
	wall := top - 1    // the sea wall — visitors' feet
	road := top - 2    // streetlights, and visitors' heads
	ground := road - 1 // the buildings' bottom row

	moonX, celestial := drawSky(cv, ground+1, o, p, sk, 0.22)

	// the skyline: outlines with lit windows — mid-rise along the bay,
	// towers clustering to the right like Nariman Point
	var lights []light
	edge := p.faint.mix(p.dim, 0.45+0.3*(1-sk.dark))
	window := p.accent2.mix(p.warm, 0.5).mix(p.text, 0.35)
	red := rgb{240, 100, 100}.mix(p.accent, 0.25)
	litOdds := 0.35 * sk.dark
	for i, x := 0, 0; x < w; i++ {
		gap := 1 + int(rand3(i, 1, 41)*4)
		bw := 4 + int(rand3(i, 2, 41)*5)
		bx := x + gap
		region := float64(bx) / float64(w)
		tall := 0.5 * smoothstep(0.55, 0.9, region) * (0.4 + 0.6*rand3(i, 4, 41))
		prof := 0.3 + 0.25*rand3(i, 3, 41) + tall
		bh := max(int(prof*float64(ground+1)), 2)
		btop := ground - bh + 1
		for y := btop; y <= ground; y++ {
			for xx := bx; xx < bx+bw; xx++ {
				switch {
				case xx == bx || xx == bx+bw-1:
					cv.set(xx, y, '|', edge)
				case (xx-bx)%2 == 1 && (y-btop)%2 == 0 &&
					// windows switch on and off every few seconds
					rand3(xx, y, int(t/6+rand3(xx, y, 43)*6)) < litOdds:
					cv.set(xx, y, '▪', window)
					lights = append(lights, light{x: xx, y: y, col: window})
				default:
					cv.set(xx, y, ' ', edge) // stars stay behind the buildings
				}
			}
		}
		for xx := bx + 1; xx < bx+bw-1; xx++ {
			cv.set(xx, btop-1, '_', edge)
		}
		if prof > 0.75 && btop >= 3 {
			ax := bx + bw/2
			cv.set(ax, btop-2, '|', edge)
			if sk.dark > 0.3 && int(t*1.1+float64(i))%2 == 0 {
				cv.set(ax, btop-3, '•', red)
			}
		}
		x = bx + bw
	}

	// the Queen's Necklace along the sea wall
	lamp := p.accent.mix(p.text, 0.15).mix(p.faint, 1-sk.dark)
	for x := 2; x < w; x += 4 {
		cv.set(x, road, '•', lamp)
		if sk.dark > 0.3 {
			lights = append(lights, light{x: x, y: road, col: lamp, streak: true})
		}
	}
	for x := 0; x < w; x++ {
		cv.set(x, wall, '─', p.faint.mix(p.dim, 0.3))
	}

	// everyone currently connected, strolling, with a name tag
	for _, v := range o.visitors {
		x := int(math.Round(v.x * float64(w-1)))
		body, name := p.accent2, p.text.mix(p.bg, 0.3)
		if v.id == o.self {
			body, name = p.green, p.green
		}
		ch := 'A'
		if time.Since(v.movedAt) < 300*time.Millisecond && int(t*8)%2 == 0 {
			ch = 'λ'
		}
		cv.set(x, wall, ch, body)
		cv.set(x, road, 'o', p.text)
		if ground >= 0 {
			nx := clamp(x-len(v.name)/2, 0, max(w-len(v.name), 0))
			cv.text(nx, ground, v.name, name)
		}
	}

	glitterX, glitter := -1.0, 0.0
	if celestial {
		glitterX, glitter = moonX, 0.75
		if sk.dark > 0.3 {
			glitter = 0.45 + 0.5*moonLit(o.moon)
		}
	}
	drawWater(cv, top, o, p, lights, glitterX, glitter)
	if o.rain {
		drawRain(cv, top, t, p)
	}
	return cv
}

// lighthouse is the Contact scene, after ascii.rest's lighthouse piece by
// @bas3line (MIT): a banded tower on a heap of boulders at night. Its beam
// turns round the lantern — long as it crosses the frame, a flash as it
// faces us — lighting the haze and the swell, and surf bursts on the rocks.
// The lantern and gallery keep their size; the tower, sea and rocks fit the
// height given.
func lighthouse(w, h int, o sceneOpts, p palette) *canvas {
	cv := newCanvas(w, h)
	if w < 30 || h < 16 {
		return cv
	}
	t := o.t
	const (
		lamp   = 4        // the lamp's row
		top    = 8        // the shaft's first row
		wt, wb = 1.7, 2.5 // the shaft's half width at the top and the foot, in rows
		period = 8.0      // seconds a turn
		swell  = 3.2      // seconds between breakers
	)
	ramp := []rune(" .:-=+*#%@")
	beamRamp := []rune(" .:-=+*#")
	// most of the height goes to the tower, so it reads at a glance; the sea
	// is a band along the bottom and the rocks a small heap at its foot
	horizon := h - max(4, int(float64(h)*0.28))
	foot := horizon + 2
	band := 2
	tc := w / 2

	// boulders, back to front: x in rows from the tower, y and waterline from
	// the foot, half width, half height. The first two stand behind the tower.
	type rock struct{ x, y, rx, ry, wl float64 }
	rocks := []rock{
		{-3.9, -2.9, 2.3, 1.7, 99}, {3.9, -3.1, 2.4, 1.8, 99}, {-6.6, -1.1, 2.3, 1.6, -0.1}, {6.7, -1.3, 2.2, 1.5, -0.4},
		{-1.7, -0.5, 2.7, 1.8, 0.8}, {3.1, -0.2, 2.5, 1.7, 1.1}, {-9.3, 0.5, 1.5, 0.9, 1}, {9.2, 0.2, 1.3, 0.8, 0.7},
	}
	const rs = 0.65 // the heap, smaller than the original's so the tower leads
	for i := range rocks {
		b := &rocks[i]
		b.x, b.y, b.rx, b.ry = b.x*rs, b.y*rs+float64(foot), b.rx*rs, b.ry*rs
		if b.wl < 50 {
			b.wl = b.wl*rs + float64(foot)
		}
	}
	const behind = 2

	// what stands still: tower, gallery, roof and rocks, as light 0..1
	const (
		kNone = iota
		kBand
		kStripe
		kRock
		kMetal
	)
	n := w * h
	still := make([]float64, n)
	kind := make([]int, n)
	rockAt := make([]int, n)
	for i := range still {
		still[i], rockAt[i] = -1, -1
	}
	moon := func(u, v, nz float64) float64 { return math.Max(0, -0.5*u-0.45*v+0.75*nz) } // lit from the upper left
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			k, x, y := r*w+c, float64(c-tc)/2, float64(r)+0.5
			boulder := func(i int) {
				b := rocks[i]
				u, v := (x-b.x)/b.rx, (y-b.y)/b.ry
				q := u*u + v*v
				if q >= 1 || y > b.wl+0.5 {
					return
				}
				nz := math.Sqrt(1 - q)
				still[k] = (0.06+0.86*moon(u, v, nz))*math.Min(1, 0.15+nz*1.8) + 0.06*(rand3(c, r, 251)-0.5)
				kind[k], rockAt[k] = kRock, i
			}
			for i := 0; i < behind; i++ {
				boulder(i)
			}
			if r >= top && r <= foot {
				hw := wt + (wb-wt)*float64(r-top)/float64(foot-top)
				if math.Abs(x) <= hw {
					u := x / (hw + 0.25)
					nz := math.Sqrt(1 - u*u)
					dark := ((r-top)/band)%2 == 1
					v := 0.92
					kind[k] = kBand
					if dark {
						v, kind[k] = 0.24, kStripe
					}
					v *= 0.4 + 0.6*moon(u, 0, nz) // enough ambient that the shaded side still reads
					if r >= foot-2 && r < foot && math.Abs(x) < 0.6 {
						v = 0.03 // the door
					}
					if r == top+band+1 && x == 0 {
						v = 0.03 // a slit of a window
					}
					still[k], rockAt[k] = v, -1
				}
			}
			if r == top-2 && math.Abs(x) <= 2.9 { // the gallery rail
				switch {
				case math.Abs(x) > 2.6:
					still[k] = 0.55
				case (c-tc)%2 != 0:
					still[k] = 0.2
				default:
					still[k] = 0.6 - 0.25*(x/3)
				}
				kind[k] = kMetal
			}
			if r == top-1 && math.Abs(x) <= 3 { // the deck, in shadow
				still[k] = 0.45 * (0.25 + 0.75*moon(x/3.2, 0.3, math.Sqrt(1-(x/3.2)*(x/3.2))))
				kind[k] = kMetal
			}
			if r >= lamp-3 && r <= lamp-2 { // the dome
				rw := 1.15
				if r == lamp-2 {
					rw = 1.85
				}
				if math.Abs(x) <= rw {
					u := x / (rw + 0.3)
					still[k], kind[k] = 0.12+0.6*moon(u, -0.4, math.Sqrt(1-u*u)), kMetal
				}
			}
			if r == lamp-4 && x == 0 {
				still[k], kind[k] = 0.5, kMetal
			}
			for i := behind; i < len(rocks); i++ {
				boulder(i)
			}
		}
	}

	beamCol := rgb{255, 222, 160}.mix(p.text, 0.25) // lamplight is warm whatever the theme
	if p.sel.luma() > 0.5 {
		beamCol = rgb{200, 140, 40}.mix(p.text, 0.3) // and darker on a light page
	}
	sea := p.faint.mix(p.accent2, 0.4)
	stripe := rgb{215, 85, 85}.mix(p.accent2, 0.25)

	// the beam in the haze through a cell: a cone seen side on, so the more it
	// turns toward or away from us the shorter, wider and brighter it looks
	th := -0.35 + 2*math.Pi*t/period
	cs, sn := math.Cos(th), math.Sin(th)
	face := math.Pow(math.Max(0, sn), 10) // the lens square to us
	beamAt := func(dx, dy float64) float64 {
		fore := math.Max(math.Abs(cs), 0.1)
		ax := 1.0
		if cs < 0 {
			ax = -1
		}
		ay := math.Min(0.35, 0.02/fore)
		m := math.Hypot(ax, ay)
		a := (dx*ax + dy*ay) / m
		if a <= 0 {
			return 0
		}
		across := math.Abs(dx*ay-dy*ax) / m
		half := 0.5 + a*(0.11/fore)
		b := math.Exp(-math.Pow(across/half, 4)) * math.Exp(-a/(22*fore)) * math.Min(1.6, 1/math.Pow(fore, 0.3))
		if sn < 0 {
			b *= 0.75
		}
		return b
	}

	lit := make([]float64, w) // beam brightness at the lamp's row, for the sea
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			k, dx, dy := r*w+c, float64(c-tc)/2, float64(r-lamp)
			glare := math.Exp(-math.Pow(math.Hypot(dx, dy)/(1.2+2.2*face), 2)) * (0.5 + 1.2*face)
			if r < horizon {
				b := beamAt(dx, dy) + glare
				if r == lamp {
					lit[c] = b
				}
				if still[k] >= 0 {
					continue
				}
				if i := min(7, int(b*6+0.35*rand3(c, r, 253))); i > 0 {
					cv.set(c, r, beamRamp[i], p.bg.mix(beamCol, 0.3+0.7*math.Min(1, b)))
				}
				continue
			}
			// the sea: crests rolling in, finer toward the horizon, lit under the beam
			d := float64(r - horizon + 1)
			x := float64(c) / (3.2/d + 0.6)
			wave := math.Sin(x*0.9+d*1.7-t*2.2+math.Sin(x*0.31+d)*1.5) + 0.6*math.Sin(x*0.43-d*0.9+t*1.3)
			l := math.Min(1, lit[c]*0.9+glare*0.5) * math.Exp(-(d-1)/6)
			col := p.bg.mix(sea, 0.6+0.3*math.Exp(-(d-1)/4)).mix(beamCol, l)
			switch {
			case wave > 0.9-0.5*l:
				ch := '~'
				switch {
				case l > 0.5:
					ch = '='
				case l <= 0.2 && d < 3:
					ch = '-'
				}
				cv.set(c, r, ch, col)
			case wave > 0.4-0.4*l && (d < 3 || l > 0.15):
				ch := '-'
				if d < 3 {
					ch = '.'
				}
				cv.set(c, r, ch, col)
			case d == 1:
				cv.set(c, r, '_', col)
			}
		}
	}

	// a few faint stars, clear of the tower
	for i := 0; i < w*horizon/80; i++ {
		c, r := int(rand3(i, 1, 257)*float64(w)), int(rand3(i, 2, 257)*float64(horizon-2))
		if still[r*w+c] >= 0 || abs(c-tc) <= 4 || cv.at(c, r).ch != ' ' {
			continue
		}
		ch := '·'
		if v := rand3(i, 3, 257); v < 0.2 {
			ch = '*'
		} else if v < 0.6 {
			ch = '.'
		}
		cv.set(c, r, ch, p.dim.mix(p.bg, 0.3))
	}

	// the tower and rocks over the rest, each boulder washed by the swell at its own waterline
	sw := make([]float64, len(rocks))
	for i, b := range rocks {
		sw[i] = b.wl + 0.45*math.Sin(2*math.Pi*t/swell+float64(i)*1.9) + 0.25*math.Sin(t*1.3+float64(i))
	}
	for k := 0; k < n; k++ {
		v := still[k]
		if v < 0 || (rockAt[k] >= 0 && float64(k/w)+0.5 > sw[rockAt[k]]) {
			continue
		}
		var col rgb
		switch kind[k] {
		case kBand:
			col = p.dim.mix(p.text, v)
		case kStripe:
			col = stripe.mix(p.bg, 0.35-v)
		case kRock:
			col = p.faint.mix(p.dim, v)
		default:
			col = p.dim.mix(p.text, v*0.5)
		}
		g := v
		if kind[k] == kStripe && v > 0.05 {
			g = v + 0.45 // the red bands as solid as the white, so the stripes read
		}
		cv.set(k%w, k/w, ramp[clamp(int(math.Round(g*9)), 1, 9)], col)
	}

	// the lantern glows from within
	glow := 0.75 + 0.25*face
	for r := lamp - 1; r <= lamp+1; r++ {
		for x := -3; x <= 3; x++ {
			edge, bar := abs(x) == 3, abs(x) == 2
			v := glow * (1 - 0.1*math.Abs(float64(x)))
			if r != lamp {
				v *= 0.85
			}
			col := beamCol.mix(p.text, 0.3*face)
			if edge || bar {
				v, col = 0.34, p.dim
				if bar {
					v = 0.2
				}
			}
			cv.set(tc+x, r, ramp[clamp(int(math.Round(v*9)), 1, 9)], col)
		}
	}
	// the lamp itself, and rays when it faces us
	lampCh := 'o'
	switch {
	case face > 0.3:
		lampCh = '@'
	case sn > -0.2:
		lampCh = '*'
	}
	bright := beamCol.mix(rgb{255, 255, 255}, 0.5)
	cv.set(tc, lamp, lampCh, bright)
	if face > 0.5 {
		cv.set(tc, lamp-1, '|', bright)
		cv.set(tc, lamp+1, '|', bright)
		cv.set(tc-1, lamp, '=', bright)
		cv.set(tc+1, lamp, '=', bright)
	}

	// surf: a breaker bursts on a boulder, its spray thrown up in a fan that
	// falls back as drops. [boulder, side it breaks on, phase in the swell]
	foam := p.text.mix(p.bg, 0.25)
	for nn, s := range [][3]float64{{6, -1, 0}, {7, 1, 0.45}, {3, 1, 0.8}, {2, -1, 0.3}, {4, 0, 0.62}, {5, 0, 0.15}} {
		b := rocks[int(s[0])]
		side := s[1]
		x0, y0 := float64(tc)+2*(b.x+b.rx*side*0.95), b.wl-0.2
		tt := frac(t/swell+s[2]) * swell
		if tt > 1.6 {
			continue
		}
		fr := int(y0 + 0.7)
		for j := -4; j <= 4; j++ {
			cx := int(math.Round(x0)) + j
			if tt < 1.3 && math.Abs(float64(j)) <= 1+tt*3 {
				ch := '~'
				if tt < 0.5 && abs(j) < 2 {
					ch = '='
				}
				cv.set(cx, fr, ch, foam)
			}
		}
		if side == 0 {
			continue
		}
		for j := 0; j < 18; j++ {
			f := float64(j) / 17
			h1, h2 := rand3(j, nn+7, 259), rand3(j, nn+19, 259)
			vx := side*(1+13*f*f) + (h1-0.5)*3
			vy := (11 - 6*f) * (0.8 + 0.4*h2)
			up := vy-18*tt > 0
			for back := 0; back <= 1; back++ {
				if back == 1 && !up {
					break
				}
				ts := tt - float64(back)*0.06
				cx := int(math.Round(x0 + vx*ts))
				cy := int(math.Round(y0 - vy*ts + 9*ts*ts))
				if ts < 0 || !cv.in(cx, cy) || float64(cy) > y0+0.3 {
					continue
				}
				if rockAt[cy*w+cx] >= 0 && tt > 0.2 {
					continue
				}
				var ch rune
				switch {
				case back == 1:
					if cur := cv.at(cx, cy).ch; cur != ' ' && cur != '~' {
						continue
					}
					ch = '\''
				case tt < 0.22 && f < 0.4:
					ch = '#'
				case tt < 0.22 || up:
					ch = '*'
				case tt < 1.15:
					ch = ':'
				default:
					ch = '.'
				}
				cv.set(cx, cy, ch, foam)
			}
		}
	}
	return cv
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func smoothstep(a, b, x float64) float64 {
	t := clamp01((x - a) / (b - a))
	return t * t * (3 - 2*t)
}
