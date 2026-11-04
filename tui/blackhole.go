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
	hole       []float64   // share of each cell's rays that fall in
	tabs       [][]float64 // each band's gas, a ring of samples
	turns      []float64   // turns each band takes per spin unit (a whole loop, for the text version)
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

// bhRaster says how the rays land on a grid of cells.
type bhRaster struct {
	w, h   int                       // cells across and down
	sx, sy int                       // rays per cell across and down
	aspect float64                   // a cell's height over its width, as tangents
	dim    float64                   // light kept by each disk crossing after the first
	ring   float64                   // gain on those later crossings, the lensed images
	fade   float64                   // how far in from the disk's inner edge the light fades, in Schwarzschild radii
	beam   func(phi float64) float64 // Doppler beaming of a crossing at angle phi
}

// bandGas is one band's gas: a ring of n samples of noise from the harmonics
// ks, each with a random phase and amplitude, scaled to [0.45, 1].
func bandGas(rnd *mulberry32, ks []float64, n int) []float64 {
	type wave struct{ k, p, a float64 }
	var waves []wave
	for _, k := range ks {
		waves = append(waves, wave{k, rnd.next() * 6.283, (0.5 + rnd.next()) / math.Sqrt(k)})
	}
	tab := make([]float64, n)
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := range tab {
		for _, wv := range waves {
			tab[i] += wv.a * math.Sin(wv.k*2*math.Pi*float64(i)/float64(n)+wv.p)
		}
		lo, hi = math.Min(lo, tab[i]), math.Max(hi, tab[i])
	}
	for i := range tab {
		tab[i] = 0.45 + 0.55*(tab[i]-lo)/(hi-lo)
	}
	return tab
}

// traceRays fires the rays of g. Each ray: its plane, then the angles where it
// crosses the disk plane, a half turn apart. It returns each cell's crossings,
// which cells no ray touches the disk or falls into the hole, and the share of
// each cell's rays that fall in.
func traceRays(g bhRaster) ([][]bhCrossing, []bool, []float64) {
	cw := 2 * bhSpan / float64(g.w) // one cell's width as a tangent
	s, c := math.Sin(bhTilt), math.Cos(bhTilt)
	table := bhPaths()

	cells := make([][]bhCrossing, g.w*g.h)
	empty := make([]bool, g.w*g.h)
	hole := make([]float64, g.w*g.h)
	for i := range empty {
		empty[i] = true
	}
	for j := 0; j < g.h*g.sy; j++ {
		y := (float64(g.h)/2 + 0.5 - (float64(j)+0.5)/float64(g.sy)) * g.aspect * cw
		for i := 0; i < g.w*g.sx; i++ {
			x := ((float64(i)+0.5)/float64(g.sx) - float64(g.w)/2) * cw
			k := (j/g.sy)*g.w + i/g.sx
			n := math.Sqrt(x*x + y*y + 1)
			d := [3]float64{x / n, (y*c - s) / n, (y*s + c) / n}
			out := d[1]*s - d[2]*c
			side := math.Sqrt(1 - out*out)
			e2 := [3]float64{d[0] / side, (d[1] - out*s) / side, (d[2] + out*c) / side}
			us := table[min(bhNB-1, int(bhD*side*bhNB/bhBMax))]
			if us[len(us)-1] >= 1 {
				empty[k] = false
				hole[k] += 1 / float64(g.sx*g.sy)
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
				edge := math.Min(1, (r-bhIn)/g.fade) * (1 - math.Pow((r-bhIn)/(bhOut-bhIn), 2))
				pos := (r - bhIn) / (bhOut - bhIn) * (bhBands - 1)
				b0 := min(bhBands-2, int(pos))
				empty[k] = false
				gain := 1.0
				if light < 1 {
					gain = g.ring
				}
				cells[k] = append(cells[k], bhCrossing{b0, pos - float64(b0), phi,
					light * edge * g.beam(phi) * math.Pow(bhIn/r, 1.5) / float64(g.sx*g.sy) * gain})
				light *= g.dim // a crossing seen through the disk is dimmer
			}
		}
	}
	return cells, empty, hole
}

func traceBlackHole(cols, rows int) *blackHoleArt {
	// each band's gas is a ring of noise: a few whole harmonics, so it closes
	rnd := mulberry32(17)
	art := &blackHoleArt{cols: cols, rows: rows}
	for b := 0; b < bhBands; b++ {
		r := bhIn + float64(bhOut-bhIn)*float64(b)/(bhBands-1)
		art.turns = append(art.turns, float64(max(1, int(math.Round(bhSpin*math.Pow(bhIn/r, 1.5))))))
		art.tabs = append(art.tabs, bandGas(&rnd, []float64{2, 3, 5, 8, 13}, bhTab))
	}

	cells, empty, _ := traceRays(bhRaster{
		w: cols, h: rows, sx: bhSX, sy: bhSY, aspect: 2, dim: 0.45, ring: 1, fade: 0.7,
		beam: func(phi float64) float64 { return 1 - 0.5*math.Cos(phi) },
	})
	art.cells = cells
	art.stars = make([]bool, cols*rows)
	for k := range art.stars {
		art.stars[k] = empty[k] && rnd.next() < 0.03
	}
	return art
}

// gas is band b's brightness at angle a, from the nearest sample of its ring.
func (art *blackHoleArt) gas(b int, a float64) float64 {
	tab := art.tabs[b]
	n := len(tab)
	i := int(math.Floor(a/(2*math.Pi)*float64(n))) % n
	if i < 0 {
		i += n
	}
	return tab[i]
}

// emission sums a cell's crossings at spin angle turn: the light, the weight
// of the light toward us, and the total weight. Band b is turned turns[b] times
// over the spin.
func (art *blackHoleArt) emission(h []bhCrossing, turn float64) (sum, toward, wsum float64) {
	for _, x := range h {
		g0 := art.gas(x.band, x.phi-art.turns[x.band]*turn)
		g1 := art.gas(x.band+1, x.phi-art.turns[x.band+1]*turn)
		sum += x.wt * (g0 + (g1-g0)*x.frac)
		toward += x.wt * -math.Cos(x.phi)
		wsum += x.wt
	}
	return sum, toward, wsum
}

// blackHole draws the frame at time t: glyph density from the light, colour
// from ember red through orange to white-hot, the approaching side bluer.
func blackHole(cols, rows int, t float64, p palette) *canvas {
	art := blackHoleSized(cols, rows)
	cv := newCanvas(cols, rows)
	ramp := []rune(" .:-=+*#%@")
	turn := 2 * math.Pi * math.Mod(t, bhLoop) / bhLoop
	for k, h := range art.cells {
		sum, toward, wsum := art.emission(h, turn)
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
