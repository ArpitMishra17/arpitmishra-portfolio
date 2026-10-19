package main

// The sidebar's black hole, after ascii.rest's black-hole piece by @bas3line
// (github.com/bas3line/ascii, MIT): a Schwarzschild black hole and its disk,
// seen almost edge on. Rays are traced once through the bent space; then only
// the gas turns, faster near the hole, the side coming toward us brighter.
// The original is one ink; here the disk is coloured by how hot it looks.

import (
	"math"
	"sync"
)

const (
	bhSX, bhSY        = 3, 4      // rays per cell across and down
	bhD, bhIn, bhOut  = 30, 3, 9  // camera distance, the disk's inner and outer edge, in Schwarzschild radii
	bhTilt            = 0.15      // the camera's height above the disk plane, in radians
	bhSpan            = 0.3       // half the view's width, as a tangent (a little tighter than the original, to fill a small frame)
	bhGain, bhFloor   = 2.1, 0.03 // exposure (brighter than the original, for a small frame), and the faintest light that shows
	bhLoop            = 60.0      // seconds until the gas comes round to where it started
	bhBands           = 12        // rings of gas, each turning a whole number of times a loop
	bhSpin            = 7         // turns a loop at the inner edge; Kepler sets the rest
	bhBMax, bhNB, bhH = 12.0, 1500, 0.02
	bhTab             = 256
)

type bhCrossing struct {
	band          int
	frac, phi, wt float64
}

type blackHoleArt struct {
	cols, rows int
	cells      [][]bhCrossing
	stars      []bool
	tabs       [][]float64
	turns      []int
}

var (
	bhMu    sync.Mutex
	bhCache = map[[2]int]*blackHoleArt{}
)

// blackHoleSized traces the rays for one frame size, once.
func blackHoleSized(cols, rows int) *blackHoleArt {
	bhMu.Lock()
	defer bhMu.Unlock()
	if b, ok := bhCache[[2]int{cols, rows}]; ok {
		return b
	}
	b := traceBlackHole(cols, rows)
	bhCache[[2]int{cols, rows}] = b
	return b
}

// bhPaths: a ray stays in a plane through the hole, and its path there
// depends only on its impact parameter b. In that plane u = 1/r obeys
// u” = 1.5u² − u in the angle swept from the camera; each path is stepped
// with RK4 until it falls in or leaves, and kept as u at every step.
var bhPaths = sync.OnceValue(func() [][]float64 {
	f := func(u float64) float64 { return 1.5*u*u - u }
	table := make([][]float64, bhNB)
	for n := range table {
		b := (float64(n) + 0.5) / bhNB * bhBMax
		u, w := 1.0/bhD, math.Sqrt(1-math.Pow(b/bhD, 2))/b
		us := []float64{u}
		for u < 1 && (u > 1.0/(bhOut+1) || w > 0) {
			k1, l1 := w, f(u)
			k2, l2 := w+bhH/2*l1, f(u+bhH/2*k1)
			k3, l3 := w+bhH/2*l2, f(u+bhH/2*k2)
			k4, l4 := w+bhH*l3, f(u+bhH*k3)
			u += bhH / 6 * (k1 + 2*k2 + 2*k3 + k4)
			w += bhH / 6 * (l1 + 2*l2 + 2*l3 + l4)
			us = append(us, u)
		}
		table[n] = us
	}
	return table
})

func traceBlackHole(cols, rows int) *blackHoleArt {
	cw := 2 * bhSpan / float64(cols) // one cell's width as a tangent; it stands twice as tall
	s, c := math.Sin(bhTilt), math.Cos(bhTilt)
	table := bhPaths()

	// each band's gas is a ring of noise: a few whole harmonics, so it closes
	rnd := mulberry32(17)
	art := &blackHoleArt{cols: cols, rows: rows, cells: make([][]bhCrossing, cols*rows), stars: make([]bool, cols*rows)}
	for b := 0; b < bhBands; b++ {
		r := bhIn + float64(bhOut-bhIn)*float64(b)/(bhBands-1)
		art.turns = append(art.turns, max(1, int(math.Round(bhSpin*math.Pow(bhIn/r, 1.5)))))
		type wave struct{ k, p, a float64 }
		var waves []wave
		for _, k := range []float64{2, 3, 5, 8, 13} {
			waves = append(waves, wave{k, rnd.next() * 6.283, (0.5 + rnd.next()) / math.Sqrt(k)})
		}
		tab := make([]float64, bhTab)
		lo, hi := math.Inf(1), math.Inf(-1)
		for i := range tab {
			for _, wv := range waves {
				tab[i] += wv.a * math.Sin(wv.k*2*math.Pi*float64(i)/bhTab+wv.p)
			}
			lo, hi = math.Min(lo, tab[i]), math.Max(hi, tab[i])
		}
		for i := range tab {
			tab[i] = 0.45 + 0.55*(tab[i]-lo)/(hi-lo)
		}
		art.tabs = append(art.tabs, tab)
	}

	// Each ray: its plane, then the angles where it crosses the disk plane, a
	// half turn apart. A crossing inside the disk keeps what never changes.
	empty := make([]bool, cols*rows)
	for i := range empty {
		empty[i] = true
	}
	for j := 0; j < rows*bhSY; j++ {
		y := (float64(rows)/2 + 0.5 - (float64(j)+0.5)/bhSY) * 2 * cw
		for i := 0; i < cols*bhSX; i++ {
			x := ((float64(i)+0.5)/bhSX - float64(cols)/2) * cw
			k := (j/bhSY)*cols + i/bhSX
			n := math.Sqrt(x*x + y*y + 1)
			d := [3]float64{x / n, (y*c - s) / n, (y*s + c) / n}
			out := d[1]*s - d[2]*c
			side := math.Sqrt(1 - out*out)
			e2 := [3]float64{d[0] / side, (d[1] - out*s) / side, (d[2] + out*c) / side}
			us := table[min(bhNB-1, int(bhD*side*bhNB/bhBMax))]
			if us[len(us)-1] >= 1 {
				empty[k] = false
			}
			// images winding more than once round the hole only speckle its shadow
			end := math.Min(float64(len(us)-1)*bhH, 5.6)
			light := 1.0
			for a := math.Mod(math.Atan2(-s, e2[1])+2*math.Pi, math.Pi); a < end; a += math.Pi {
				p := a / bhH
				q := int(p)
				r := 1 / (us[q] + (us[q+1]-us[q])*(p-float64(q)))
				if r <= bhIn || r >= bhOut {
					continue
				}
				phi := math.Atan2(math.Sin(a)*e2[2]-math.Cos(a)*c, math.Sin(a)*e2[0])
				edge := math.Min(1, (r-bhIn)/0.7) * (1 - math.Pow((r-bhIn)/(bhOut-bhIn), 2))
				doppler := 1 - 0.5*math.Cos(phi)
				pos := (r - bhIn) / (bhOut - bhIn) * (bhBands - 1)
				b0 := min(bhBands-2, int(pos))
				empty[k] = false
				art.cells[k] = append(art.cells[k], bhCrossing{b0, pos - float64(b0), phi,
					light * edge * doppler * math.Pow(bhIn/r, 1.5) / (bhSX * bhSY)})
				light *= 0.45 // a crossing seen through the disk is dimmer
			}
		}
	}
	for k := range art.stars {
		art.stars[k] = empty[k] && rnd.next() < 0.03
	}
	return art
}

// blackHole draws the frame at time t: glyph density from the light, colour
// from ember red through orange to white-hot, the approaching side bluer.
func blackHole(cols, rows int, t float64, p palette) *canvas {
	art := blackHoleSized(cols, rows)
	cv := newCanvas(cols, rows)
	ramp := []rune(" .:-=+*#%@")
	turn := 2 * math.Pi * math.Mod(t, bhLoop) / bhLoop
	at := func(b int, a float64) float64 {
		i := int(math.Floor(a/(2*math.Pi)*bhTab)) % bhTab
		if i < 0 {
			i += bhTab
		}
		return art.tabs[b][i]
	}
	for k, h := range art.cells {
		sum, toward, wsum := 0.0, 0.0, 0.0
		for _, x := range h {
			g0 := at(x.band, x.phi-float64(art.turns[x.band])*turn)
			g1 := at(x.band+1, x.phi-float64(art.turns[x.band+1])*turn)
			sum += x.wt * (g0 + (g1-g0)*x.frac)
			toward += x.wt * -math.Cos(x.phi)
			wsum += x.wt
		}
		v := math.Pow(clamp01((sum*bhGain-bhFloor)/(1-bhFloor)), 0.75)
		ch := ramp[int(math.Round(v*float64(len(ramp)-1)))]
		if ch == ' ' {
			if art.stars[k] {
				cv.set(k%cols, k/cols, '.', p.dim.mix(p.bg, 0.3))
			}
			continue
		}
		var col rgb
		switch {
		case v < 0.35:
			col = rgb{110, 30, 25}.mix(rgb{225, 85, 35}, v/0.35)
		case v < 0.7:
			col = rgb{225, 85, 35}.mix(rgb{255, 175, 70}, (v-0.35)/0.35)
		default:
			col = rgb{255, 175, 70}.mix(rgb{255, 242, 210}, (v-0.7)/0.3)
		}
		if wsum > 0 {
			col = col.mix(rgb{205, 220, 255}, math.Max(0, toward/wsum)*0.45*v)
		}
		cv.set(k%cols, k/cols, ch, col)
	}
	return cv
}
