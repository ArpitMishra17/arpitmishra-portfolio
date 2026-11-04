package main

// The 3D pipes screensaver: one pipe grows across a grid of cells, one cell a
// step, and when the grid fills up it clears and starts again. The grid is a
// pure function of (size, seed, steps). Frames are cheap because each
// (size, seed) keeps its simulation and only runs the steps it has not yet
// run; see simulatePipes.

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
)

type pipeCell struct {
	piece pipePiece
	color int
	on    bool
}

type pipeGrid struct {
	w, h  int
	cells []pipeCell
}

// at is the cell at (x, y), or an empty cell outside the grid.
func (g *pipeGrid) at(x, y int) pipeCell {
	if x < 0 || y < 0 || x >= g.w || y >= g.h {
		return pipeCell{}
	}
	return g.cells[y*g.w+x]
}

const (
	pipesPerSecond = 28  // growth speed, cells a second
	pipeTurn       = 0.1 // chance a pipe turns at a cell, once it has run straight
	pipeRun        = 3   // straight cells a pipe runs before it may turn again
	pipeFill       = 0.6 // the grid clears once this share of cells is on
	pipeSeed       = 7   // the ASCII screensaver's seed
	pipeMemoSize   = 4   // simulations kept between frames
)

// pipeSteps converts screensaver time to steps.
func pipeSteps(t float64) int {
	if t <= 0 || math.IsNaN(t) {
		return 0
	}
	return int(math.Min(t*pipesPerSecond, 1<<31))
}

// directions: 0 up, 1 right, 2 down, 3 left. Opposite is d+2, a turn is d±1.
var (
	pipeDX = [4]int{0, 1, 0, -1}
	pipeDY = [4]int{-1, 0, 1, 0}
)

// pieceForArms is the piece that joins the given edges.
func pieceForArms(arms uint8) pipePiece {
	switch arms {
	case armE | armW:
		return pipeH
	case armN | armS:
		return pipeV
	case armN | armE:
		return pipeNE
	case armN | armW:
		return pipeNW
	case armS | armE:
		return pipeSE
	default:
		return pipeSW
	}
}

// pipeSim is a simulation in progress. Its rng is a splitmix-style counter,
// so the whole run is a function of the seed.
type pipeSim struct {
	g      pipeGrid
	rng    uint32
	steps  int  // steps run so far
	active bool // a pipe is growing
	head   int  // index of the growing pipe's head cell
	dir    int  // the direction the head last moved
	run    int  // cells the head has moved straight on since it last turned
	count  int  // pipes started since the grid was last cleared, for colours
	filled int  // cells on
}

// endPieces are the pipe ends by the direction they reach.
var endPieces = [4]pipePiece{pipeEndN, pipeEndE, pipeEndS, pipeEndW}

func newPipeSim(w, h int, seed uint32) *pipeSim {
	return &pipeSim{g: pipeGrid{w: w, h: h, cells: make([]pipeCell, w*h)}, rng: seed}
}

func (s *pipeSim) next() uint32 {
	s.rng += 0x9e3779b9
	z := s.rng
	z = (z ^ (z >> 16)) * 0x85ebca6b
	z = (z ^ (z >> 13)) * 0xc2b2ae35
	return z ^ (z >> 16)
}

func (s *pipeSim) chance(p float64) bool { return float64(s.next()>>8)/(1<<24) < p }

// emptyCell is a random empty cell, or -1 if the grid is full.
func (s *pipeSim) emptyCell() int {
	n := len(s.g.cells)
	for i := 0; i < 32; i++ {
		if k := int(s.next() % uint32(n)); !s.g.cells[k].on {
			return k
		}
	}
	start := int(s.next() % uint32(n))
	for i := 0; i < n; i++ {
		if k := (start + i) % n; !s.g.cells[k].on {
			return k
		}
	}
	return -1
}

func (s *pipeSim) reset() {
	clear(s.g.cells)
	s.filled, s.active = 0, false
}

// neighbour is the cell next to k in direction d, or -1 at the edge.
func (s *pipeSim) neighbour(k, d int) int {
	w, h := s.g.w, s.g.h
	x, y := k%w+pipeDX[d], k/w+pipeDY[d]
	if x < 0 || y < 0 || x >= w || y >= h {
		return -1
	}
	return y*w + x
}

// pickNext is the direction a pipe leaves cell k in, given the way it is
// heading and how many cells it has run straight: straight on, or a turn. A
// voluntary turn needs pipeRun straight cells first and then has chance
// pipeTurn. A blocked way is turned past whatever the run, and a pipe never
// goes back. It is -1 when every way is blocked.
func (s *pipeSim) pickNext(k, heading, run int) int {
	left, right := (heading+1)%4, (heading+3)%4
	if s.next()&1 == 1 {
		left, right = right, left
	}
	cands := [3]int{heading, left, right}
	if run >= pipeRun && s.chance(pipeTurn) {
		cands = [3]int{left, right, heading}
	}
	for _, d := range cands {
		if n := s.neighbour(k, d); n >= 0 && !s.g.cells[n].on {
			return d
		}
	}
	return -1
}

// enter puts the growing pipe's head on cell k, having moved in direction d.
// The head is an end: a ball on a stub back to the cell it came from.
func (s *pipeSim) enter(k, d, color int) {
	s.g.cells[k] = pipeCell{piece: endPieces[(d+2)%4], color: color, on: true}
	s.filled++
	if s.active && d == s.dir {
		s.run++
	} else {
		s.run = 0 // a turn, or the first cell of a pipe, starts a new run
	}
	s.head, s.dir, s.active = k, d, true
}

// startPipe starts a pipe at k. Its first cell is an end facing the cell the
// pipe moves into; a cell with no free way out is a lone ball.
func (s *pipeSim) startPipe(k int) {
	color := s.count % len(pipeColors)
	s.count++
	d := s.pickNext(k, int(s.next()%4), 0)
	if d < 0 {
		s.g.cells[k] = pipeCell{piece: pipeCap, color: color, on: true}
		s.filled++
		return
	}
	s.g.cells[k] = pipeCell{piece: endPieces[d], color: color, on: true}
	s.filled++
	s.enter(s.neighbour(k, d), d, color)
}

// grow moves the head one cell. The cell it leaves becomes the corner or
// straight joining where the pipe came from and where it goes. A head that
// cannot move ends the pipe; it stays an end, facing back.
func (s *pipeSim) grow() {
	d := s.pickNext(s.head, s.dir, s.run)
	if d < 0 {
		s.active = false
		return
	}
	color := s.g.cells[s.head].color
	s.g.cells[s.head].piece = pieceForArms(1<<((s.dir+2)%4) | 1<<d)
	s.enter(s.neighbour(s.head, d), d, color)
}

// step runs one step: grow the pipe, or when none is growing, start one. A
// grid at the fill mark, or with no empty cell left, clears first.
func (s *pipeSim) step() {
	n := len(s.g.cells)
	if n == 0 {
		return
	}
	if s.active {
		s.grow()
		return
	}
	if s.filled >= int(pipeFill*float64(n)) {
		s.reset()
	}
	k := s.emptyCell()
	if k < 0 {
		s.reset()
		k = s.emptyCell()
	}
	s.startPipe(k)
}

func (s *pipeSim) advance(steps int) {
	for s.steps < steps {
		s.step()
		s.steps++
	}
}

// runPipes simulates from nothing, with no memo.
func runPipes(w, h, steps int, seed uint32) *pipeGrid {
	s := newPipeSim(max(w, 0), max(h, 0), seed)
	s.advance(steps)
	return &s.g
}

type pipeKey struct {
	w, h int
	seed uint32
}

var (
	pipeMu   sync.Mutex
	pipeMemo = map[pipeKey]*pipeSim{}
)

// simulatePipes is the grid after the given number of steps from seed.
//
// It is the same as runPipes, but it keeps the simulation of recent sizes and
// seeds, so a frame only runs the steps since the last one. A step count that
// goes backwards starts that simulation again. The result is a copy, so the
// caller may keep it.
func simulatePipes(w, h, steps int, seed uint32) *pipeGrid {
	w, h = max(w, 0), max(h, 0)
	if w*h == 0 || steps <= 0 {
		return &pipeGrid{w: w, h: h, cells: make([]pipeCell, w*h)}
	}
	key := pipeKey{w, h, seed}
	pipeMu.Lock()
	defer pipeMu.Unlock()
	s := pipeMemo[key]
	if s == nil || s.steps > steps {
		if len(pipeMemo) >= pipeMemoSize {
			clear(pipeMemo)
		}
		s = newPipeSim(w, h, seed)
		pipeMemo[key] = s
	}
	s.advance(steps)
	return &pipeGrid{w: w, h: h, cells: slices.Clone(s.g.cells)}
}

// pipeGlyphs are the heavy box-drawing pieces, and a dot for a cap.
var pipeGlyphs = [pipePieces]rune{
	pipeH: '━', pipeV: '┃', pipeNE: '┗', pipeNW: '┛', pipeSE: '┏', pipeSW: '┓',
	pipeCap: '●', pipeEndN: '●', pipeEndE: '●', pipeEndS: '●', pipeEndW: '●',
}

// pipesASCII is the screensaver for terminals without graphics: each pipe in
// its colour, mixed a little toward the theme's background.
func pipesASCII(w, h int, t float64, p palette) *canvas {
	cv := newCanvas(w, h)
	g := simulatePipes(cv.w, cv.h, pipeSteps(t), pipeSeed)
	for y := 0; y < cv.h; y++ {
		for x := 0; x < cv.w; x++ {
			if c := g.at(x, y); c.on {
				cv.set(x, y, pipeGlyphs[c.piece], pipeColors[c.color].mix(p.bg, 0.15))
			}
		}
	}
	return cv
}

// pipesKitty is the pipes screensaver for a terminal with kitty graphics. Each
// on cell is a placeholder for its piece's sprite, under that sprite's image
// id. The label goes on the last row, over its end, in the dim colour dim.
func pipesKitty(w, h, steps int, seed uint32, label, dim string) string {
	w, h = max(w, 0), max(h, 0)
	g := simulatePipes(w, h, steps, seed)
	runes := []rune(label)
	lx := w - len(runes) - 1 // the label's first column
	lines := make([]string, h)
	for y := range lines {
		var b strings.Builder
		cur := 0 // the colour in effect: an image id, 0 for the default, -1 for dim
		for x := range w {
			want, text := 0, " "
			switch c := g.at(x, y); {
			case y == h-1 && x >= lx && x < lx+len(runes):
				want, text = -1, string(runes[x-lx])
			case c.on:
				want, text = pipeSpriteID(c.color, c.piece), kittyCell()
			}
			if want != cur {
				switch want {
				case 0:
					b.WriteString("\x1b[39m")
				case -1:
					b.WriteString(dim)
				default:
					fmt.Fprintf(&b, "\x1b[38;5;%dm", want)
				}
				cur = want
			}
			b.WriteString(text)
		}
		if cur != 0 {
			b.WriteString("\x1b[39m")
		}
		lines[y] = b.String()
	}
	return strings.Join(lines, "\n")
}
