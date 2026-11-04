package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func countOn(g *pipeGrid) int {
	n := 0
	for _, c := range g.cells {
		if c.on {
			n++
		}
	}
	return n
}

func TestPipesDeterministic(t *testing.T) {
	a := runPipes(40, 20, 800, 3)
	b := runPipes(40, 20, 800, 3)
	if !slices.Equal(a.cells, b.cells) {
		t.Fatal("same seed gave different grids")
	}
	if slices.Equal(a.cells, runPipes(40, 20, 800, 4).cells) {
		t.Fatal("different seeds gave the same grid")
	}
}

// the memo must give exactly what a fresh run gives, including when steps go back.
func TestSimulatePipesMatchesFresh(t *testing.T) {
	for _, steps := range []int{0, 1, 50, 300, 120, 900, 900} {
		got := simulatePipes(40, 20, steps, 3)
		want := runPipes(40, 20, steps, 3)
		if got.w != want.w || got.h != want.h || !slices.Equal(got.cells, want.cells) {
			t.Fatalf("steps %d: memoised grid differs from a fresh run", steps)
		}
	}
}

func TestPipesZeroSteps(t *testing.T) {
	for _, g := range []*pipeGrid{runPipes(10, 5, 0, 1), simulatePipes(10, 5, 0, 1)} {
		if n := countOn(g); n != 0 {
			t.Fatalf("0 steps: %d cells on", n)
		}
		if len(g.cells) != 50 {
			t.Fatalf("0 steps: %d cells, want 50", len(g.cells))
		}
	}
}

func TestPipesGridBounds(t *testing.T) {
	g := runPipes(7, 3, 200, 2)
	if g.w != 7 || g.h != 3 || len(g.cells) != 21 {
		t.Fatalf("grid %dx%d with %d cells", g.w, g.h, len(g.cells))
	}
	if g.at(-1, 0).on || g.at(7, 0).on || g.at(0, 3).on || g.at(0, -1).on {
		t.Fatal("cells outside the grid should be empty")
	}
	if g := runPipes(-4, 3, 10, 1); len(g.cells) != 0 {
		t.Fatal("negative size should be empty")
	}
}

// every arm of an on cell must reach an on neighbour that points back, or a cap.
func TestPipesConnect(t *testing.T) {
	cases := []struct {
		w, h, steps int
		seed        uint32
	}{{30, 12, 600, 1}, {60, 20, 3000, 2}, {10, 6, 400, 9}, {3, 3, 200, 5}}
	for _, tc := range cases {
		g := runPipes(tc.w, tc.h, tc.steps, tc.seed)
		for y := 0; y < g.h; y++ {
			for x := 0; x < g.w; x++ {
				c := g.at(x, y)
				if !c.on || c.piece == pipeCap {
					continue
				}
				for d := 0; d < 4; d++ {
					if pipeArms[c.piece]&(1<<d) == 0 {
						continue
					}
					nb := g.at(x+pipeDX[d], y+pipeDY[d])
					if !nb.on {
						t.Fatalf("%dx%d@%d seed %d: arm at (%d,%d) dir %d reaches an empty or outside cell", tc.w, tc.h, tc.steps, tc.seed, x, y, d)
					}
					back := uint8(1) << ((d + 2) % 4)
					if nb.piece != pipeCap && pipeArms[nb.piece]&back == 0 {
						t.Fatalf("%dx%d@%d seed %d: arm at (%d,%d) dir %d meets a neighbour that does not join back", tc.w, tc.h, tc.steps, tc.seed, x, y, d)
					}
				}
			}
		}
	}
}

// the grid fills, then clears: the count rises, and drops back to one new pipe.
func TestPipesFillClears(t *testing.T) {
	const w, h = 10, 8
	s := newPipeSim(w, h, 5)
	prev, drops := 0, 0
	for i := 0; i < 3000; i++ {
		s.step()
		s.steps++
		n := countOn(&s.g)
		if n > w*h {
			t.Fatalf("%d cells on in a %d grid", n, w*h)
		}
		if n < prev {
			drops++
			if n > 2 { // a clear starts one pipe: its first cell and the next
				t.Fatalf("after a clear %d cells are on, want at most 2", n)
			}
		}
		prev = n
	}
	if drops == 0 {
		t.Fatal("the grid never cleared")
	}
}

func TestPipesASCIIRenders(t *testing.T) {
	p := paletteOf(themes[0])
	check := func(w, h int, t2 float64) {
		cv := pipesASCII(w, h, t2, p)
		if cv.w != max(w, 0) || cv.h != max(h, 0) {
			t.Fatalf("%dx%d: canvas %dx%d", w, h, cv.w, cv.h)
		}
		cv.render(termenv.TrueColor)
	}
	for w := 0; w <= 300; w += 7 {
		for h := 0; h <= 100; h += 5 {
			check(w, h, 3)
		}
	}
	check(0, 0, 0)
	check(300, 100, 120)
	check(-3, -3, 1)
}

func TestPipesASCIIGlyphs(t *testing.T) {
	cv := pipesASCII(60, 20, 30, paletteOf(themes[0]))
	seen := map[rune]bool{}
	for _, c := range cv.cells {
		if c.ch != ' ' {
			seen[c.ch] = true
		}
	}
	if len(seen) == 0 {
		t.Fatal("no pipes drawn after 30 seconds")
	}
	for ch := range seen {
		if !slices.Contains(pipeGlyphs[:], ch) {
			t.Fatalf("unexpected glyph %q", ch)
		}
	}
}

// a pipe runs straight for pipeRun cells before a voluntary turn, and a blocked
// straight on is turned past at once, whatever the run.
func TestPipesStraightRun(t *testing.T) {
	s := newPipeSim(9, 9, 1)
	const k = 4*9 + 4 // the centre, heading right
	for run := 0; run < pipeRun; run++ {
		for i := 0; i < 200; i++ {
			if d := s.pickNext(k, 1, run); d != 1 {
				t.Fatalf("run %d: turned to %d with straight on free", run, d)
			}
		}
	}
	turned := false
	for i := 0; i < 2000 && !turned; i++ {
		turned = s.pickNext(k, 1, pipeRun) != 1
	}
	if !turned {
		t.Fatal("after a straight run the pipe never turns")
	}
	s.g.cells[k+1].on = true // block straight on
	for i := 0; i < 200; i++ {
		if d := s.pickNext(k, 1, 0); d != 0 && d != 2 {
			t.Fatalf("blocked straight on: picked %d, want a turn", d)
		}
	}
}

func TestPipeStepsScale(t *testing.T) {
	if pipeSteps(-1) != 0 || pipeSteps(0) != 0 {
		t.Fatal("non-positive time should be zero steps")
	}
	if got := pipeSteps(10); got != 10*pipesPerSecond {
		t.Fatalf("pipeSteps(10) = %d", got)
	}
}

func BenchmarkSimulatePipes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		pipeMu.Lock()
		clear(pipeMemo) // a cold run: no kept simulation
		pipeMu.Unlock()
		b.StartTimer()
		simulatePipes(300, 100, 5000, 1)
	}
}

func BenchmarkSimulatePipesWarm(b *testing.B) {
	simulatePipes(300, 100, 5000, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		simulatePipes(300, 100, 5000, 1)
	}
}

// the kitty frame is exactly w cells in each of h rows, whatever the size.
func TestPipesKittyWidths(t *testing.T) {
	const label = " pipes · z next · any key exits "
	const dim = "\x1b[38;2;1;2;3m"
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {2, 1}, {10, 1}, {80, 24}, {600, 200}, {-3, -3}} {
		w, h := max(sz[0], 0), max(sz[1], 0)
		for _, steps := range []int{0, 40, 3000} {
			out := pipesKitty(sz[0], sz[1], steps, 7, label, dim)
			if h == 0 {
				if out != "" {
					t.Fatalf("%dx%d: got %q, want nothing", sz[0], sz[1], out)
				}
				continue
			}
			lines := strings.Split(out, "\n")
			if len(lines) != h {
				t.Fatalf("%dx%d: %d rows, want %d", sz[0], sz[1], len(lines), h)
			}
			for y, l := range lines {
				if got := lipgloss.Width(l); got != w {
					t.Fatalf("%dx%d steps %d row %d: width %d, want %d", sz[0], sz[1], steps, y, got, w)
				}
			}
			if w >= len(label)+1 && !strings.Contains(stripSGR(lines[h-1]), label) {
				t.Fatalf("%dx%d: label missing", sz[0], sz[1])
			}
		}
	}
}

// with no label, each on cell is one placeholder under its sprite's id, and
// each empty cell is a space in the default colour.
func TestPipesKittyCells(t *testing.T) {
	const w, h, steps, seed = 30, 10, 400, 3
	g := simulatePipes(w, h, steps, seed)
	out := pipesKitty(w, h, steps, seed, "", "")
	for y, line := range strings.Split(out, "\n") {
		id, x := 0, 0 // the colour in effect, and the column
		rs := []rune(line)
		for i := 0; i < len(rs); i++ {
			switch r := rs[i]; {
			case r == 0x1b:
				end := i
				for rs[end] != 'm' {
					end++
				}
				next := 0
				if _, err := fmt.Sscanf(string(rs[i:end+1]), "\x1b[38;5;%dm", &next); err != nil {
					next = 0 // the reset, 39m
				}
				if next == id {
					t.Fatalf("row %d col %d: colour code %d repeats", y, x, next)
				}
				id, i = next, end
			case r == kittyPlaceholder:
				c := g.at(x, y)
				if !c.on || id != pipeSpriteID(c.color, c.piece) {
					t.Fatalf("row %d col %d: placeholder under id %d, cell %+v", y, x, id, c)
				}
				i += 2 // the row and column diacritics
				x++
			case r == ' ':
				if id != 0 || g.at(x, y).on {
					t.Fatalf("row %d col %d: space under id %d, on %v", y, x, id, g.at(x, y).on)
				}
				x++
			default:
				t.Fatalf("row %d col %d: unexpected rune %U", y, x, r)
			}
		}
		if x != w {
			t.Fatalf("row %d has %d cells, want %d", y, x, w)
		}
	}
}
