package main

// Type effects, adapted from the "type" pieces of ascii.rest by @bas3line
// (https://github.com/bas3line/ascii): scramble, typewriter, split-flap and
// wave text. MIT licensed — see LICENSE-ascii.rest.

import (
	"math"
	"strings"
	"unicode/utf8"
)

// mulberry32 is the small seeded generator the pieces use, so a typo lands
// the same way every time.
type mulberry32 uint32

func (r *mulberry32) next() float64 {
	*r += 0x6d2b79f5
	t := uint32(*r)
	t = (t ^ (t >> 15)) * (1 | t)
	t = (t + (t^(t>>7))*(61|t)) ^ t
	return float64(t^(t>>14)) / 4294967296
}

// --- scramble ---

var scrambleGlyphs = []rune("!<>-_\\/[]{}=+*^?#%&$@0123456789abcdefxyz")

// scrambleIn decodes s out of flickering glyphs, a ragged front moving left
// to right. It returns, per rune, the glyph to show and whether it has settled.
func scrambleIn(s string, since float64) ([]rune, []bool) {
	rs := []rune(s)
	in := math.Min(0.03, 0.55/float64(max(len(rs), 1))) // long titles settle faster per letter
	const lead = 0.1
	tick := int(since * 20)
	out, ok := make([]rune, len(rs)), make([]bool, len(rs))
	for i, r := range rs {
		settle := lead + float64(i)*in + rand3(i, len(rs), 211)*0.12
		if r == ' ' || since >= settle {
			out[i], ok[i] = r, true
			continue
		}
		out[i] = scrambleGlyphs[hash3(i, tick, 213)%uint32(len(scrambleGlyphs))]
	}
	return out, ok
}

// --- typewriter ---

type typeEvent struct {
	at   float64
	text string
}

// typewriter types phrases one after another like a person: uneven key
// timing, the odd slip onto a neighbouring key and back, held backspace.
type typewriter struct {
	events         []typeEvent
	period, offset float64
}

func newTypewriter(phrases []string, seed uint32) typewriter {
	rnd := mulberry32(seed)
	rows := []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"}
	slip := func(ch rune) (rune, bool) {
		for _, row := range rows {
			if i := strings.IndexRune(row, ch); i >= 0 {
				switch {
				case i == 0:
					return rune(row[1]), true
				case i == len(row)-1:
					return rune(row[i-1]), true
				case rnd.next() < 0.5:
					return rune(row[i-1]), true
				default:
					return rune(row[i+1]), true
				}
			}
		}
		return 0, false
	}
	var tw typewriter
	at, text := 0.0, ""
	key := func(next string, wait float64) {
		at += wait
		text = next
		tw.events = append(tw.events, typeEvent{at, text})
	}
	delay := func(ch rune) float64 {
		d := 0.06 + rnd.next()*0.09
		if ch == ' ' {
			d += 0.05 + rnd.next()*0.08
		}
		if rnd.next() < 0.05 {
			d += 0.3
		}
		return d
	}
	var done []float64
	for _, phrase := range phrases {
		ph := []rune(phrase)
		typo := -1
		if rnd.next() < 0.5 && len(ph) > 5 {
			typo = 3 + int(rnd.next()*float64(len(ph)-4))
		}
		for i := 0; i < len(ph); i++ {
			if i == typo {
				if wrong, ok := slip(ph[i]); ok {
					more := min(int(rnd.next()*3), len(ph)-i-1)
					key(text+string(wrong), delay(ph[i]))
					for k := 1; k <= more; k++ {
						key(text+string(ph[i+k]), delay(ph[i+k]))
					}
					at += 0.25 + rnd.next()*0.2
					for k := 0; k <= more; k++ {
						w := 0.07
						if k == 0 {
							w = 0.1
						}
						key(trimLast(text), w)
					}
				}
			}
			d := delay(ph[i])
			if i > 0 && strings.ContainsRune(".,!?", ph[i-1]) {
				d += 0.15
			}
			key(text+string(ph[i]), d)
		}
		done = append(done, at)
		at += 2.6 // hold the finished phrase
		for k := range ph {
			w := 0.045
			switch k {
			case 0:
				w = 0
			case 1:
				w = 0.18
			}
			key(trimLast(text), w)
		}
		at += 0.5
	}
	tw.period = at
	// time 0 shows the first phrase finished, just before it's deleted
	tw.offset = done[0] + 1.8
	return tw
}

func trimLast(s string) string {
	_, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n]
}

// at gives the line at time t, and whether the cursor is lit: solid while
// keys land, blinking once it sits still.
func (tw typewriter) at(t float64) (string, bool) {
	now := math.Mod(t+tw.offset, tw.period)
	line, last := "", 0.0
	for _, e := range tw.events {
		if e.at > now {
			break
		}
		line, last = e.text, e.at
	}
	idle := now - last
	return line, idle < 0.45 || math.Mod(idle-0.45, 1.06) > 0.53
}

// --- split-flap ---

const flapDrum = " ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789.,-/'&()—·@"

// flap shows `to` as a row of split-flap tiles `since` seconds after it
// replaced `from`: each tile riffles forward through its drum a card at a
// time, a touch after its neighbour. turning marks tiles still moving.
func flap(from, to string, since float64) (shown []rune, turning []bool) {
	drum := []rune(flapDrum)
	fr, tr := []rune(from), []rune(to)
	n := max(len(fr), len(tr))
	shown, turning = make([]rune, n), make([]bool, n)
	idx := func(rs []rune, i int) int {
		if i >= len(rs) {
			return 0
		}
		for k, d := range drum {
			if d == rs[i] {
				return k
			}
		}
		return -1
	}
	const rate = 45.0 // cards a second
	const most = 9    // a tile riffles through at most this many cards
	for i := 0; i < n; i++ {
		final := ' '
		if i < len(tr) {
			final = tr[i]
		}
		shown[i] = final
		a, b := idx(fr, i), idx(tr, i)
		begin := float64(i)*0.012 + rand3(i, n, 223)*0.08
		if a < 0 || b < 0 {
			// not on the drum: it just drops in when its turn comes
			if since < begin+0.1 && i < len(fr) {
				shown[i], turning[i] = fr[i], true
			}
			continue
		}
		need := (b - a + len(drum)) % len(drum)
		if need > most {
			a, need = (b-most+len(drum))%len(drum), most
		}
		if flips := int(math.Max(0, since-begin) * rate); flips < need {
			shown[i], turning[i] = drum[(a+flips)%len(drum)], true
		}
	}
	return shown, turning
}

// --- wave text ---

// waveText runs one swell through s from left to right and lets it settle;
// it returns three rows, the letters lifted a row up or down at the crests.
func waveText(s string, since float64) [3][]rune {
	const amp, wavelength, speed = 1.2, 10.0, 20.0
	rs := []rune(s)
	var g [3][]rune
	for r := range g {
		g[r] = []rune(strings.Repeat(" ", len(rs)))
	}
	rise, top := wavelength/2, wavelength
	pass := 2*rise + top
	envelope := func(u float64) float64 {
		switch {
		case u < 0:
			return 0
		case u < rise:
			return math.Pow(math.Sin(math.Pi/2*(u/rise)), 2)
		case u < rise+top:
			return 1
		case u < pass:
			return math.Pow(math.Cos(math.Pi/2*((u-rise-top)/rise)), 2)
		}
		return 0
	}
	for i, r := range rs {
		u := since*speed - float64(i)
		h := amp * envelope(u) * math.Sin(2*math.Pi*u/wavelength)
		lift := clamp(int(math.Round(h)), -1, 1)
		g[1-lift][i] = r
	}
	return g
}
