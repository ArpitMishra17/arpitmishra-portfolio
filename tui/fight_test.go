package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

// arena puts two visitors on a fresh hub, a facing b, gap columns apart on a
// promenade cols wide.
func arena(gap, cols int) (*visitorHub, *visitor, *visitor) {
	h := &visitorHub{vs: map[int]*visitor{}}
	h.join(1, "a")
	h.join(2, "b")
	a, b := h.vs[1], h.vs[2]
	a.x, a.face = 0.5, 1
	b.x, b.face = 0.5+float64(gap)/float64(cols-1), -1
	return h, a, b
}

func TestStrikeReach(t *testing.T) {
	const cols = 81
	at := time.Now()
	sword := weapons[defaultWeapon]
	for _, c := range []struct {
		gap  int
		hits int
	}{{1, 1}, {sword.reach, 1}, {sword.reach + 1, 0}, {-2, 0}} {
		h, _, b := arena(c.gap, cols)
		if n := h.strike(1, cols, at); n != c.hits {
			t.Errorf("gap %d: %d hits, want %d", c.gap, n, c.hits)
		}
		if c.hits > 0 && b.hp != maxHP-sword.dmg {
			t.Errorf("gap %d: hp %d, want %d", c.gap, b.hp, maxHP-sword.dmg)
		}
	}
}

func TestStrikeCooldownAndKnockback(t *testing.T) {
	const cols = 81
	h, _, b := arena(1, cols)
	at := time.Now()
	x0 := b.x
	h.strike(1, cols, at)
	if b.x <= x0 {
		t.Errorf("not knocked back: x %v → %v", x0, b.x)
	}
	b.x = x0
	if n := h.strike(1, cols, at.Add(weapons[defaultWeapon].cool/2)); n != 0 {
		t.Errorf("struck again inside the cooldown")
	}
	if n := h.strike(1, cols, at.Add(weapons[defaultWeapon].cool)); n != 1 {
		t.Errorf("no strike after the cooldown")
	}
}

func TestJumpDodges(t *testing.T) {
	const cols = 81
	h, _, b := arena(2, cols)
	at := time.Now()
	h.jump(2, at)
	if n := h.strike(1, cols, at.Add(jumpDur/2)); n != 0 || b.hp != maxHP {
		t.Errorf("a strike under the top of a jump landed")
	}
	// jumping with them reaches them again
	h, _, _ = arena(2, cols)
	h.jump(2, at)
	h.jump(1, at)
	if n := h.strike(1, cols, at.Add(jumpDur/2)); n != 1 {
		t.Errorf("a strike at the same height missed")
	}
}

func TestKnockoutAndRespawn(t *testing.T) {
	const cols = 81
	old := respawnDelay
	respawnDelay = func() time.Duration { return 4 * time.Second }
	t.Cleanup(func() { respawnDelay = old })

	h, _, b := arena(1, cols)
	at := time.Now()
	for i := 0; !b.dead(); i++ {
		if i > 20 {
			t.Fatal("never knocked out")
		}
		b.x = 0.5 + 1/float64(cols-1)
		h.strike(1, cols, at)
		at = at.Add(time.Second)
	}
	if k := h.lastKill(); k.by != "a" || k.who != "b" {
		t.Errorf("kill note %+v", k)
	}
	down := b.respawnAt
	// can't walk, jump or be hit while down
	if n := h.strike(1, cols, at); n != 0 {
		t.Error("hit while knocked out")
	}
	h.jump(2, at)
	if !b.jumpAt.IsZero() {
		t.Error("jumped while knocked out")
	}
	// up again, full health, and guarded for a moment
	h.jump(2, down)
	if b.dead() || b.hp != maxHP {
		t.Fatalf("not back up at the respawn time: dead %v hp %d", b.dead(), b.hp)
	}
	b.x = 0.5 + 1/float64(cols-1)
	if n := h.strike(1, cols, down.Add(spawnGuard/2)); n != 0 {
		t.Error("hit straight after getting up")
	}
	if n := h.strike(1, cols, down.Add(spawnGuard+time.Second)); n != 1 {
		t.Error("still unhittable after the guard")
	}
}

func TestJumpLiftShape(t *testing.T) {
	at := time.Now()
	if l := jumpLift(at, at); l != 0 {
		t.Errorf("lift at take-off %d", l)
	}
	if l := jumpLift(at, at.Add(jumpDur/2)); l != int(jumpPeak) {
		t.Errorf("lift at the top %d, want %d", l, int(jumpPeak))
	}
	if l := jumpLift(at, at.Add(jumpDur)); l != 0 {
		t.Errorf("lift after landing %d", l)
	}
	if l := jumpLift(time.Time{}, at); l != 0 {
		t.Errorf("lift without a jump %d", l)
	}
}

func TestWeaponPicker(t *testing.T) {
	hub.join(7, "picker")
	t.Cleanup(func() { hub.leave(7) })
	m := newModel(7, termenv.Ascii)
	m.booting = false
	m.width, m.height = 120, 40
	press := func(k string) {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		got, _ := m.Update(msg)
		m = got.(model)
	}
	press("e")
	if m.modal != modalWeapon {
		t.Fatal("e didn't open the weapon picker")
	}
	if out := m.View(); !strings.Contains(out, "hammer") {
		t.Error("picker doesn't list the weapons")
	}
	press("down")
	press("enter")
	if w := hub.weaponOf(7); w != defaultWeapon+1 {
		t.Errorf("armed with %d, want %d", w, defaultWeapon+1)
	}
	press("e")
	press("5")
	if w := hub.weaponOf(7); w != 4 || m.modal != modalNone {
		t.Errorf("number key: armed with %d, modal %v", w, m.modal)
	}
}

func TestRoadDrawsFight(t *testing.T) {
	h, a, b := arena(2, 60)
	at := time.Now()
	h.strike(1, 60, at)
	o := sceneOpts{now: at.Add(50 * time.Millisecond), self: 1, kill: h.lastKill()}
	o.visitors = []visitor{*a, *b}
	out := road(60, 9, o, paletteOf(themes[0])).render(termenv.Ascii)
	for _, want := range []string{"|──", "━", "-25"} {
		if !strings.Contains(out, want) {
			t.Errorf("road missing %q:\n%s", want, out)
		}
	}
}
