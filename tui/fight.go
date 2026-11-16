package main

import (
	"math"
	"math/rand/v2"
	"time"
)

// The promenade is also a very small arena: visitors jump (w), strike (f)
// with a weapon they pick (e), and someone knocked to zero lies there for a
// few seconds before getting back up. The hub keeps all of it, stamped with
// wall-clock times, and each session draws the same state on its own frame —
// so a strike lands for everyone at once with nothing broadcast.

// weapon is one choice in the weapon picker. Glyphs are drawn facing right
// and mirrored for a figure facing left.
type weapon struct {
	name   string
	idle   rune          // held up beside the head; 0 draws nothing
	strike string        // out from the forward hand, on the arm row, mid-swing
	reach  int           // columns from the body's centre a strike lands within
	dmg    int           // hit points it takes
	cool   time.Duration // between strikes
	knock  int           // columns it pushes the one hit
	glow   bool          // drawn in the accent colour, not as steel
}

var weapons = []weapon{
	{name: "fists", strike: "o", reach: 2, dmg: 12, cool: 280 * time.Millisecond, knock: 1},
	{name: "sword", idle: '/', strike: "──>", reach: 4, dmg: 25, cool: 450 * time.Millisecond, knock: 2},
	{name: "spear", idle: '|', strike: "────>", reach: 6, dmg: 18, cool: 650 * time.Millisecond, knock: 3},
	{name: "axe", idle: 'P', strike: "─D", reach: 3, dmg: 34, cool: 750 * time.Millisecond, knock: 3},
	{name: "hammer", idle: 'T', strike: "─█", reach: 3, dmg: 45, cool: 1000 * time.Millisecond, knock: 5},
	{name: "saber", idle: '|', strike: "━━━━", reach: 5, dmg: 30, cool: 500 * time.Millisecond, knock: 2, glow: true},
}

const defaultWeapon = 1 // sword

const (
	maxHP      = 100
	jumpDur    = 650 * time.Millisecond
	jumpPeak   = 2.0                    // rows at the top of a jump
	swingShow  = 220 * time.Millisecond // how long a strike stays drawn
	flashDur   = 180 * time.Millisecond // the one hit glows this long
	staggerDur = 300 * time.Millisecond // … and flails its arms this long
	barShow    = 4 * time.Second        // the health bar stays up this long after a hit
	spawnGuard = 1200 * time.Millisecond
	killShow   = 3 * time.Second // "x took out y" stays in the corner this long
)

// respawnDelay is how long someone stays down: three to five seconds.
var respawnDelay = func() time.Duration {
	return 3*time.Second + time.Duration(rand.Float64()*float64(2*time.Second))
}

// jumpLift is how many rows a jump started at jumpAt has someone off the
// ground at t: a parabola over jumpDur that peaks at jumpPeak rows.
func jumpLift(jumpAt, t time.Time) int {
	if jumpAt.IsZero() {
		return 0
	}
	u := float64(t.Sub(jumpAt)) / float64(jumpDur)
	if u < 0 || u >= 1 {
		return 0
	}
	return int(math.Round(jumpPeak * 4 * u * (1 - u)))
}

func (v visitor) dead() bool { return !v.respawnAt.IsZero() }

// guarded is the moment after getting back up when nothing lands.
func (v visitor) guarded(t time.Time) bool {
	return !v.spawnAt.IsZero() && t.Sub(v.spawnAt) < spawnGuard
}

func (v visitor) arms() weapon { return weapons[clamp(v.weapon, 0, len(weapons)-1)] }

// settle gets the fallen back up once their time is out. Every hub method
// calls it on what it touches, so nobody has to tick the hub.
func settle(v *visitor, t time.Time) {
	if v.dead() && !t.Before(v.respawnAt) {
		v.respawnAt, v.spawnAt, v.hp = time.Time{}, t, maxHP
	}
}

func (h *visitorHub) jump(id int, t time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if v, ok := h.vs[id]; ok {
		settle(v, t)
		if !v.dead() && jumpLift(v.jumpAt, t) == 0 && t.Sub(v.jumpAt) >= jumpDur {
			v.jumpAt = t
			h.actedAt = t
		}
	}
}

// arm sets the weapon someone carries.
func (h *visitorHub) arm(id, w int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if v, ok := h.vs[id]; ok {
		v.weapon = clamp(w, 0, len(weapons)-1)
	}
}

func (h *visitorHub) weaponOf(id int) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if v, ok := h.vs[id]; ok {
		return v.weapon
	}
	return defaultWeapon
}

// strike swings id's weapon at t, judged on the striker's own promenade,
// cols wide: everyone standing in front of them within reach, and not so high
// in a jump that the swing passes under them, is hit. It returns how many were.
func (h *visitorHub) strike(id, cols int, t time.Time) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	a, ok := h.vs[id]
	if !ok {
		return 0
	}
	settle(a, t)
	wp := a.arms()
	if a.dead() || (!a.swingAt.IsZero() && t.Sub(a.swingAt) < wp.cool) {
		return 0
	}
	a.swingAt, h.actedAt = t, t
	span := float64(max(cols-1, 1))
	face := float64(a.face)
	aLift := jumpLift(a.jumpAt, t)
	hits := 0
	for _, o := range h.vs {
		if o.id == id {
			continue
		}
		settle(o, t)
		if o.dead() || o.guarded(t) {
			continue
		}
		dx := (o.x - a.x) * span * face // columns ahead of the striker
		if dx < -0.5 || dx > float64(wp.reach)+0.5 {
			continue
		}
		// the swing is on the striker's arm row; it lands on any of the
		// three rows the other's body covers
		if d := aLift - jumpLift(o.jumpAt, t); d < -1 || d > 1 {
			continue
		}
		hits++
		o.hp -= wp.dmg
		o.hitAt, o.hitDir, o.lastDmg = t, a.face, wp.dmg
		o.x = clamp01(o.x + face*float64(wp.knock)/span)
		if o.hp <= 0 {
			o.hp = 0
			o.respawnAt = t.Add(respawnDelay())
			h.kill = killNote{by: a.name, who: o.name, at: t}
		}
	}
	return hits
}

// killNote is the latest knockout, for the corner of the promenade.
type killNote struct {
	by, who string
	at      time.Time
}

func (h *visitorHub) lastKill() killNote {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.kill
}

// busy reports whether anyone jumped or struck in the last moment, so the
// sessions watching can switch to the fast frame rate while it plays out.
func (h *visitorHub) busy(t time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return !h.actedAt.IsZero() && t.Sub(h.actedAt) < 1500*time.Millisecond
}

// mirrorRune flips a glyph for a figure facing left.
func mirrorRune(r rune) rune {
	switch r {
	case '/':
		return '\\'
	case '\\':
		return '/'
	case '>':
		return '<'
	case '<':
		return '>'
	case '(':
		return ')'
	case ')':
		return '('
	}
	return r
}

// stageFight sets up a still of a fight for -render: arpit (1) and the first
// fake visitor (100) face each other a step apart, as of a moment before at.
func stageFight(kind string, at time.Time, cols int) {
	hub.mu.Lock()
	a, b := hub.vs[1], hub.vs[100]
	if a == nil || b == nil {
		hub.mu.Unlock()
		return
	}
	a.x, a.face = 0.45, 1
	b.x, b.face = 0.45+3/float64(cols-1), -1
	hub.mu.Unlock()
	t := at.Add(-80 * time.Millisecond)
	switch kind {
	case "jump":
		hub.jump(100, at.Add(-jumpDur/2))
		hub.strike(1, cols, t)
	case "dead":
		hub.arm(1, 4) // hammer
		for i := 3; i >= 1; i-- {
			hub.mu.Lock()
			b.x = 0.45 + 3/float64(cols-1) // back in reach after the knockback
			hub.mu.Unlock()
			hub.strike(1, cols, at.Add(-time.Duration(i)*time.Second-1500*time.Millisecond))
		}
	case "hit":
		hub.strike(1, cols, at.Add(-time.Second))
		hub.strike(1, cols, t)
	default: // swing
		hub.arm(100, 2) // spear
		hub.strike(1, cols, t)
	}
}
