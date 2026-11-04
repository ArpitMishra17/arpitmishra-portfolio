package main

import (
	"math"
	"slices"
	"testing"
)

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// the frames are bhFrameN PNGs of the image size, within the byte budget.
func TestBlackHoleFrames(t *testing.T) {
	const budget = 300 * 1024
	frames := bhFrames()
	if len(frames) != bhFrameN {
		t.Fatalf("%d frames, want %d", len(frames), bhFrameN)
	}
	total := 0
	for f, p := range frames {
		total += len(p)
		img := decodePNG(t, p)
		if b := img.Bounds(); b.Dx() != bhPxW || b.Dy() != bhPxH {
			t.Fatalf("frame %d: bounds %v, want %dx%d", f, b, bhPxW, bhPxH)
		}
	}
	t.Logf("black hole: %d frames, %d bytes in all, %d bytes for frame 0", len(frames), total, len(frames[0]))
	if total > budget {
		t.Fatalf("frames total %d bytes, want at most %d", total, budget)
	}
}

// frame bhFrameN is frame 0 again: the gas comes round exactly, so the loop
// has no jump. Each band turns less than 10 degrees a frame at the inner edge.
func TestBlackHoleLoop(t *testing.T) {
	art := traceBlackHolePixels()
	first, again := bhShade(art, 0), bhShade(art, bhFrameN)
	// float rounding at a table sample can move one pixel one level, so allow that
	differ, worst := 0, 0
	for i := range first {
		for ch := range len(px{}) {
			if d := absInt(int(first[i][ch]) - int(again[i][ch])); d > 0 {
				differ++
				worst = max(worst, d)
			}
		}
	}
	if worst > 1 || differ > len(first)/1000 {
		t.Fatalf("frame bhFrameN differs from frame 0: %d channels, worst by %d", differ, worst)
	}
	if slices.Equal(first, bhShade(art, 1)) {
		t.Fatal("frame 1 is frame 0: the gas does not turn")
	}
	if deg := art.turns[0] * 2 * math.Pi / bhFrameN * 180 / math.Pi; deg >= 10 {
		t.Fatalf("the inner band turns %.1f degrees a frame", deg)
	}
	for b, n := range art.turns {
		if k := n * bhHarm; k != math.Trunc(k) {
			t.Fatalf("band %d turns %v of a loop, not a whole number of steps", b, n)
		}
	}
}

// the picture reads as the lensed disk: a black shadow, the far side of the
// disk arching over and under it, and the approaching side brighter. Round it
// the image is clear, so the terminal's background shows.
func TestBlackHoleLight(t *testing.T) {
	art := traceBlackHolePixels()
	pix := bhShade(art, 0)
	at := func(x, y int) px { return pix[y*bhPxW+x] }
	// the light over black
	luma := func(c px) float64 {
		return (0.2126*float64(c[0]) + 0.7152*float64(c[1]) + 0.0722*float64(c[2])) * float64(c[3]) / 255
	}
	cx, cy := bhPxW/2, bhPxH/2
	if c := at(cx, cy); c != (px{0, 0, 0, 255}) {
		t.Fatalf("the shadow's centre is %v, want opaque black", c)
	}
	for _, p := range [][2]int{{0, 0}, {bhPxW - 1, 0}, {0, bhPxH - 1}, {bhPxW - 1, bhPxH - 1}} {
		if c := at(p[0], p[1]); c[3] != 0 {
			t.Fatalf("corner %v is %v, want clear", p, c)
		}
	}
	if luma(at(cx, cy-60)) < 30 || luma(at(cx, cy+60)) < 30 {
		t.Fatal("the disk does not arch over and under the shadow")
	}
	left, right := 0.0, 0.0
	for y := 0; y < bhPxH; y++ {
		for x := 0; x < bhPxW; x++ {
			if x < cx {
				left += luma(at(x, y))
			} else {
				right += luma(at(x, y))
			}
		}
	}
	if left < 1.3*right {
		t.Fatalf("approaching side %.0f, receding %.0f: no Doppler asymmetry", left, right)
	}
}

// bhFrameAt maps scene time to a frame, and wraps at the loop.
func TestBlackHoleFrameAt(t *testing.T) {
	cases := []struct {
		st   float64
		want int
	}{
		{0, 0},
		{bhFrameDur * 3.5, 3},
		{bhFrameDur * bhFrameN, 0},
		{bhFrameDur*bhFrameN + bhFrameDur*2.2, 2},
		{-0.01, bhFrameN - 1},
	}
	for _, c := range cases {
		if got := bhFrameAt(c.st); got != c.want {
			t.Errorf("bhFrameAt(%v) = %d, want %d", c.st, got, c.want)
		}
	}
}
