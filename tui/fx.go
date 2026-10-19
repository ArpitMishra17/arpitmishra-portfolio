package main

// Full-screen effects for the screensaver.

// fireColor runs ember red → orange → pale yellow, nudged toward the theme.
func fireColor(v float64, p palette) rgb {
	var c rgb
	if v < 0.5 {
		c = rgb{190, 50, 30}.mix(rgb{255, 140, 40}, v*2)
	} else {
		c = rgb{255, 140, 40}.mix(rgb{255, 235, 170}, (v-0.5)*2)
	}
	return c.mix(p.accent, 0.2).mix(p.bg, clamp01(0.35-v))
}

// --- screensaver effects ---

// doomFire is the classic demoscene fire, from noise rather than a
// propagation buffer so it needs no state.
func doomFire(w, h int, t float64, p palette) *canvas {
	cv := newCanvas(w, h)
	for y := 0; y < h; y++ {
		up := float64(h-1-y) / float64(h)
		for x := 0; x < w; x++ {
			n := vnoise(float64(x)*0.12, float64(y)*0.22+t*3.2, 151)*0.7 +
				vnoise(float64(x)*0.35, float64(y)*0.5+t*5, 152)*0.3
			v := (1 - up*1.6) + (n-0.5)*1.3
			if v < 0.08 {
				continue
			}
			cv.set(x, y, rampAt([]rune(" .:-=+*#%@"), v), fireColor(v, p))
		}
	}
	return cv
}

// matrixRain drops columns of half-width katakana in the theme's green.
func matrixRain(w, h int, t float64, p palette) *canvas {
	cv := newCanvas(w, h)
	glyphs := []rune("ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎ0123456789:=*+<>")
	for x := 0; x < w; x += 2 {
		speed := 6 + rand3(x, 0, 111)*14
		trail := 6 + int(rand3(x, 1, 111)*14)
		period := float64(h + trail + int(rand3(x, 2, 111)*float64(h)))
		head := int(frac(t*speed/period+rand3(x, 3, 111)) * period)
		for k := 0; k < trail; k++ {
			y := head - k
			if y < 0 || y >= h {
				continue
			}
			g := glyphs[hash3(x, y, int(t*5+rand3(x, y, 113)*5))%uint32(len(glyphs))]
			col := p.bg.mix(p.green, 1-float64(k)/float64(trail))
			if k == 0 {
				col = p.text.mix(rgb{255, 255, 255}, 0.4)
			}
			cv.set(x, y, g, col)
		}
	}
	return cv
}

// starfield flies through stars toward the viewer.
func starfield(w, h int, t float64, p palette) *canvas {
	cv := newCanvas(w, h)
	for i := 0; i < 320; i++ {
		sx, sy := rand3(i, 0, 121)*2-1, rand3(i, 1, 121)*2-1
		z := 1 - frac(t*0.22+rand3(i, 2, 121))
		if z < 0.03 {
			continue
		}
		x := int(float64(w)/2 + sx/z*float64(w)*0.22)
		y := int(float64(h)/2 + sy/z*float64(h)*0.22)
		b := 1 - z
		col := p.bg.mix(p.text, b)
		if i%7 == 0 {
			col = p.bg.mix(p.accent, b)
		}
		cv.set(x, y, rampAt([]rune(".·+*@"), b), col)
	}
	return cv
}
