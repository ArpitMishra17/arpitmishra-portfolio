package main

// The sidebar's black hole for kitty terminals, in pixels. The rays are the
// ones blackhole.go traces, sampled several times per pixel and lit as light
// instead of glyphs. The gas turns in a short loop of frames; the frames are
// built once per process and sent once per visit. There is no sky: the gas is
// light over a clear image, so the terminal's own background shows round it,
// and only the hole's shadow is painted black.

import (
	"bytes"
	"cmp"
	"image"
	"image/color"
	"image/png"
	"math"
	"slices"
	"sync"
)

const (
	bhPxW, bhPxH = 24 * 12, 9 * 24 // the image: 24x9 cells of 12x24 px
	bhPxSS       = 3               // rays per pixel, across and down
	bhHarm       = 8               // the gas's harmonics are multiples of this; a band turns whole steps of 1/bhHarm a loop
	bhPxSpin     = 3               // steps of 1/bhHarm the inner band turns a loop; the outer bands turn fewer
	bhPxTab      = 1024            // samples in each band's ring of gas
	bhPxDim      = 0.6             // light kept by each disk crossing after the first
	bhPxRing     = 2.5             // extra gain on those lensed crossings, which makes the photon ring
	bhPxFade     = 0.2             // the inner edge's fade, sharper than the text version's so the ring is thin
	bhBeam       = 0.25            // Doppler beaming: the approaching side is (1+bhBeam)^3 as bright, the receding (1-bhBeam)^3
	bhPxGain     = 2.0             // exposure before the tone curve
	bhPxEdge     = 24              // the gas fades out over this many pixels at the image's sides, so the disk isn't cut off
	bhFrameN     = 16              // frames in a loop
	bhFrameStep  = 0.09            // scene seconds before the next frame: a resting tick (0.12 s) or two fast ones (0.05 s)
)

// px is one pixel's colour and opacity, eight bits a channel, not premultiplied.
type px [4]uint8

// traceBlackHolePixels traces the rays of the image. Each band's gas has
// harmonics that are multiples of bhHarm, and band b turns n_b/bhHarm times a
// loop with n_b a whole number, so a loop of bhFrameN frames closes.
func traceBlackHolePixels() *blackHoleArt {
	rnd := mulberry32(17)
	art := &blackHoleArt{cols: bhPxW, rows: bhPxH}
	for b := 0; b < bhBands; b++ {
		r := bhIn + float64(bhOut-bhIn)*float64(b)/(bhBands-1)
		n := math.Max(1, math.Round(bhPxSpin*math.Pow(bhIn/r, 1.5)))
		art.turns = append(art.turns, n/bhHarm)
		art.tabs = append(art.tabs, bandGas(&rnd, []float64{8, 16, 24, 32, 40, 48}, bhPxTab))
	}

	cells, _, hole := traceRays(bhRaster{
		w: bhPxW, h: bhPxH, sx: bhPxSS, sy: bhPxSS, aspect: 1, dim: bhPxDim, ring: bhPxRing, fade: bhPxFade,
		beam: func(phi float64) float64 {
			g := 1 - bhBeam*math.Cos(phi) // above 1 on the side coming toward us
			return g * g * g
		},
	})
	art.cells, art.hole = cells, hole
	return art
}

// bhHeat is the colour of the disk at brightness v in [0, 1]: black, then dark
// red, orange and pale yellow-white.
func bhHeat(v float64) rgb {
	stops := [...]struct {
		v float64
		c rgb
	}{{0, rgb{}}, {0.25, rgb{120, 30, 22}}, {0.5, rgb{225, 85, 35}}, {0.75, rgb{255, 170, 70}}, {1, rgb{255, 240, 205}}}
	for i := 1; i < len(stops); i++ {
		if v <= stops[i].v {
			return stops[i-1].c.mix(stops[i].c, (v-stops[i-1].v)/(stops[i].v-stops[i-1].v))
		}
	}
	return stops[len(stops)-1].c
}

// bhShade lights frame f. Each pixel's light is its crossings' light at that
// spin, toned so the inner disk does not clip; the colour is the disk's heat,
// bluer on the approaching side.
//
// The gas glows: over black it would add its light, and over the terminal's
// background it should too. So a pixel is as opaque as its brightest channel,
// with its colour scaled up to match, which over black gives back the light
// exactly. The shadow is opaque black for the share of the pixel's rays that
// fell in, which softens its edge. The disk runs past the image's sides, so
// its light fades out toward them.
func bhShade(art *blackHoleArt, f int) []px {
	turn := 2 * math.Pi * float64(f) / bhFrameN
	out := make([]px, len(art.cells))
	for k, h := range art.cells {
		var c rgb
		if len(h) > 0 {
			sum, toward, wsum := art.emission(h, turn)
			v := 1 - math.Exp(-bhPxGain*sum)
			s := 0.0
			if wsum > 0 {
				s = toward / wsum
			}
			c = bhHeat(v).mix(rgb{120, 170, 255}, clamp01(s)*0.7*v)
			e := clamp01(float64(min(k%bhPxW, bhPxW-1-k%bhPxW)) / bhPxEdge)
			e = e * e * (3 - 2*e)
			c = rgb{c.r * e, c.g * e, c.b * e}
		}
		a := math.Max(art.hole[k], math.Max(c.r, math.Max(c.g, c.b))/255)
		if a < 0.5/255 {
			continue // clear
		}
		out[k] = px{byteOf(c.r / a), byteOf(c.g / a), byteOf(c.b / a), byteOf(a * 255)}
	}
	return out
}

// bhPalette picks at most 256 colours for all the frames, by median cut: split
// the box of colours holding the most pixels along its widest channel, at the
// pixel median, until there are 256 boxes, then average each box. One palette
// serves every frame, so the colours do not shift between frames. It also maps
// each colour in the frames to its nearest entry. The palette runs dark to
// light, so neighbouring shades get neighbouring indices.
func bhPalette(frames [][]px) (color.Palette, map[px]uint8) {
	counts := map[px]int{}
	for _, f := range frames {
		for _, c := range f {
			counts[c]++
		}
	}
	type entry struct {
		c px
		n int
	}
	items := make([]entry, 0, len(counts))
	for c, n := range counts {
		items = append(items, entry{c, n})
	}
	// map order is random; sort so the palette is the same every run
	slices.SortFunc(items, func(a, b entry) int { return bytes.Compare(a.c[:], b.c[:]) })
	const chans = len(px{})

	type box struct{ lo, hi, n int } // items[lo:hi], holding n pixels
	boxes := []box{{0, len(items), 0}}
	for _, e := range items {
		boxes[0].n += e.n
	}
	for len(boxes) < 256 {
		best := -1
		for i, b := range boxes {
			if b.hi-b.lo > 1 && (best < 0 || b.n > boxes[best].n) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		b := boxes[best]
		lo, hi := [chans]int{255, 255, 255, 255}, [chans]int{}
		for _, e := range items[b.lo:b.hi] {
			for ch, v := range e.c {
				lo[ch], hi[ch] = min(lo[ch], int(v)), max(hi[ch], int(v))
			}
		}
		ch := 0
		for i := 1; i < chans; i++ {
			if hi[i]-lo[i] > hi[ch]-lo[ch] {
				ch = i
			}
		}
		slices.SortFunc(items[b.lo:b.hi], func(x, y entry) int { return cmp.Compare(x.c[ch], y.c[ch]) })
		mid, acc := b.lo, 0
		for mid < b.hi-1 && acc < b.n/2 {
			acc += items[mid].n
			mid++
		}
		boxes[best] = box{b.lo, mid, acc}
		boxes = append(boxes, box{mid, b.hi, b.n - acc})
	}

	cols := make([]px, 0, len(boxes))
	for _, b := range boxes {
		var sum [chans]float64
		n := 0.0
		for _, e := range items[b.lo:b.hi] {
			for ch, v := range e.c {
				sum[ch] += float64(e.n) * float64(v)
			}
			n += float64(e.n)
		}
		cols = append(cols, px{byteOf(sum[0] / n), byteOf(sum[1] / n), byteOf(sum[2] / n), byteOf(sum[3] / n)})
	}
	// the light a colour gives over black, which is its colour times its opacity
	luma := func(c px) float64 {
		return (0.2126*float64(c[0]) + 0.7152*float64(c[1]) + 0.0722*float64(c[2])) * float64(c[3]) / 255
	}
	slices.SortFunc(cols, func(a, b px) int { return cmp.Compare(luma(a), luma(b)) })

	pal := make(color.Palette, len(cols))
	for i, c := range cols {
		pal[i] = color.NRGBA{c[0], c[1], c[2], c[3]}
	}
	index := make(map[px]uint8, len(items))
	for _, e := range items {
		best, bestD := 0, math.MaxInt
		for i, p := range cols {
			d := 0
			for ch := range p {
				diff := int(p[ch]) - int(e.c[ch])
				d += diff * diff
			}
			if d < bestD {
				best, bestD = i, d
			}
		}
		index[e.c] = uint8(best)
	}
	return pal, index
}

// bhEncode is one frame as an indexed PNG.
func bhEncode(pix []px, pal color.Palette, index map[px]uint8) []byte {
	img := image.NewPaletted(image.Rect(0, 0, bhPxW, bhPxH), pal)
	for i, c := range pix {
		img.Pix[i] = index[c]
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// bhFrames is the PNG of every frame, built once per process.
var bhFrames = sync.OnceValue(func() [][]byte {
	art := traceBlackHolePixels()
	shades := make([][]px, bhFrameN)
	for f := range shades {
		shades[f] = bhShade(art, f)
	}
	pal, index := bhPalette(shades)
	out := make([][]byte, bhFrameN)
	for f, pix := range shades {
		out[f] = bhEncode(pix, pal, index)
	}
	return out
})

// bhUploads is each frame as kitty upload escapes, one write apiece: a frame
// is whole within its write, and frames of the TUI can go out between them,
// so a slow link doesn't freeze the screen behind all the frames.
var bhUploads = sync.OnceValue(func() [][]byte {
	frames := bhFrames()
	out := make([][]byte, len(frames))
	for f, png := range frames {
		out[f] = kittyUpload(uint8(bhFrameID(f)), png, sidebarW-2, 9)
	}
	return out
})

// bhFrameID is the kitty image id of frame f. It sits past the pipe sprites.
func bhFrameID(f int) int { return 100 + f }
