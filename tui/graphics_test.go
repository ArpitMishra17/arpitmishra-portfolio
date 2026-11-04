package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/rivo/uniseg"
)

var (
	sgrRE   = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	clockRE = regexp.MustCompile(`\d\d:\d\d:\d\d`)
	// process uptime and the hub count depend on when and after what the test runs
	uptimeRE = regexp.MustCompile(`up [0-9hms]+`)
	hereRE   = regexp.MustCompile(`\d+ here`)
)

// stripSGR drops colour codes, leaving the text.
func stripSGR(s string) string { return sgrRE.ReplaceAllString(s, "") }

// view is m's view with the wall clock, the uptime and the visitor count
// masked, which can change between two renders.
func view(m model) string {
	v := clockRE.ReplaceAllString(m.View(), "HH:MM:SS")
	v = uptimeRE.ReplaceAllString(v, "up N")
	return hereRE.ReplaceAllString(v, "N here")
}

// expModel is the Experience section at 120x40, settled: nothing is mid-flap or
// fading in.
func expModel(sel int) model {
	m := newModel(1, termenv.TrueColor)
	m.booting = false
	m.width, m.height = 120, 40
	m.sec = secExperience
	m.expSel, m.prevExp = sel, sel
	m.at = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	past := m.at.Add(-100 * time.Second)
	m.startAt, m.bootEndAt, m.secAt, m.selAt = past, past, past, past
	m.lastInputAt = m.at
	return m
}

// hasID reports whether the uploads up include image id, in its first chunk.
func hasID(up []byte, id int) bool {
	return bytes.Contains(up, []byte(fmt.Sprintf("i=%d,", id)))
}

func TestImageIDs(t *testing.T) {
	seen := map[int]string{}
	add := func(id int, what string) {
		if id < 1 || id > 255 {
			t.Fatalf("%s: id %d is outside 1..255", what, id)
		}
		if prev, ok := seen[id]; ok {
			t.Fatalf("%s and %s both use id %d", what, prev, id)
		}
		seen[id] = what
	}
	for i, e := range experiences {
		add(expImageID(i), "experience "+e.company)
	}
	for c := range pipeColors {
		for p := pipePiece(0); p < pipePieces; p++ {
			add(pipeSpriteID(c, p), fmt.Sprintf("pipe colour %d piece %d", c, p))
		}
	}
	for f := range bhFrameN {
		add(bhFrameID(f), fmt.Sprintf("black hole frame %d", f))
	}
}

// every image is uploaded once, under its own id.
func TestImageUploads(t *testing.T) {
	up := imageUploads()
	images := len(experiences) + len(pipeColors)*int(pipePieces)
	t.Logf("kitty uploads: %d images, %d bytes", images, len(up))
	if !bytes.HasPrefix(up, []byte("\x1b_G")) {
		t.Fatal("uploads do not start with a graphics escape")
	}
	if n := bytes.Count(up, []byte("\x1b_Ga=T")); n != images {
		t.Fatalf("%d transmissions, want %d", n, images)
	}
	for i := range experiences {
		if !hasID(up, expImageID(i)) {
			t.Fatalf("experience %d (id %d) is not uploaded", i, expImageID(i))
		}
	}
	for c := range pipeColors {
		for p := pipePiece(0); p < pipePieces; p++ {
			if !hasID(up, pipeSpriteID(c, p)) {
				t.Fatalf("pipe colour %d piece %d (id %d) is not uploaded", c, p, pipeSpriteID(c, p))
			}
		}
	}
}

func TestExperienceLogoWidth(t *testing.T) {
	for i := range experiences {
		for _, l := range kittyBlock(uint8(expImageID(i)), expLogoCols, expLogoRows) {
			if got := lipgloss.Width(l); got != expLogoCols {
				t.Fatalf("experience %d logo row is %d cells wide, want %d", i, got, expLogoCols)
			}
		}
	}
}

// countWriter counts the writes it gets.
type countWriter struct {
	bytes.Buffer
	writes int
}

func (c *countWriter) Write(p []byte) (int, error) {
	c.writes++
	return c.Buffer.Write(p)
}

// cmdMsgs runs cmd, and every command in the batch it returns, and gives the
// messages they return.
func cmdMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, c := range batch {
		out = append(out, cmdMsgs(c)...)
	}
	return out
}

// Init sends the uploads only when the terminal speaks kitty and there is a
// writer for them. The images go in one write, then the black hole's frames a
// write each, which reports bhReadyMsg.
func TestInitUploads(t *testing.T) {
	cases := []struct {
		name       string
		kitty, gfx bool
		writes     int
	}{
		{"no kitty", false, true, 0},
		{"kitty without a writer", true, false, 0},
		{"kitty", true, true, 1 + bhFrameN},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := expModel(0)
			w := &countWriter{}
			m.kitty = c.kitty
			if c.gfx {
				m.gfx = w
			}
			msgs := cmdMsgs(m.Init())
			if w.writes != c.writes {
				t.Fatalf("%d writes, want %d", w.writes, c.writes)
			}
			ready := false
			for _, msg := range msgs {
				_, ok := msg.(bhReadyMsg)
				ready = ready || ok
			}
			if ready != (c.writes > 0) {
				t.Fatalf("bhReadyMsg sent %v with %d writes", ready, c.writes)
			}
			if c.writes > 0 {
				want := append([]byte(nil), imageUploads()...)
				for _, up := range bhUploads() {
					want = append(want, up...)
				}
				if !bytes.Equal(w.Bytes(), want) {
					t.Fatal("the writes are not the image uploads, then the black hole's frames")
				}
			}
		})
	}
}

// the sidebar's black hole is the kitty frame only when the terminal speaks
// kitty and the frames are in. Before that, and without kitty, it is the text
// version, and the rows keep their width either way.
func TestSidebarBlackHoleFrame(t *testing.T) {
	plain := expModel(0)
	plain.sec = secOverview
	kit := plain
	kit.kitty = true
	if view(kit) != view(plain) {
		t.Fatal("kitty without the frames changed the sidebar")
	}
	ready := kit
	ready.bhReady = true
	for f := 0; f < bhFrameN; f++ {
		m := ready
		m.st = (float64(f) + 0.5) * bhFrameDur
		out := view(m)
		want := fmt.Sprintf("\x1b[38;5;%dm", bhFrameID(f))
		if !strings.Contains(out, want) {
			t.Fatalf("frame %d: the view does not show image %d", f, bhFrameID(f))
		}
		for g := 0; g < bhFrameN; g++ {
			if g != f && strings.Contains(out, fmt.Sprintf("\x1b[38;5;%dm", bhFrameID(g))) {
				t.Fatalf("frame %d: the view also shows image %d", f, bhFrameID(g))
			}
		}
		for _, row := range kittyBlock(uint8(bhFrameID(f)), sidebarW-2, 9) {
			if !strings.Contains(out, row) {
				t.Fatalf("frame %d: a sidebar row is missing", f)
			}
		}
	}
	if !strings.Contains(view(ready), "38;5;100m") {
		t.Fatal("the frame at scene start is not image 100")
	}
	if strings.ContainsRune(view(plain), kittyPlaceholder) {
		t.Fatal("placeholders in the sidebar without kitty")
	}
}

func TestSidebarBlackHoleWidth(t *testing.T) {
	for _, sz := range [][2]int{{80, 40}, {120, 40}, {160, 50}} {
		plain := expModel(0)
		plain.sec = secOverview
		plain.width, plain.height = sz[0], sz[1]
		ready := plain
		ready.kitty, ready.bhReady = true, true
		ready.st = 2
		pl := strings.Split(view(plain), "\n")
		kl := strings.Split(view(ready), "\n")
		if len(kl) != len(pl) {
			t.Fatalf("%dx%d: %d rows with the frame, %d without", sz[0], sz[1], len(kl), len(pl))
		}
		for i := range pl {
			if a, b := lipgloss.Width(kl[i]), lipgloss.Width(pl[i]); a != b {
				t.Fatalf("%dx%d row %d: %d cells with the frame, %d without", sz[0], sz[1], i, a, b)
			}
		}
	}
	for _, l := range kittyBlock(uint8(bhFrameID(0)), sidebarW-2, 9) {
		if got := lipgloss.Width(l); got != sidebarW-2 {
			t.Fatalf("black hole row is %d cells wide, want %d", got, sidebarW-2)
		}
	}
}

// the frames' upload reports ready, and the model takes it.
func TestBHReady(t *testing.T) {
	m := expModel(0)
	m.kitty = true
	m.gfx = &countWriter{}
	if m.bhReady {
		t.Fatal("ready before the frames are sent")
	}
	got, _ := m.Update(bhReadyMsg{})
	if !got.(model).bhReady {
		t.Fatal("bhReadyMsg did not set bhReady")
	}
}

// the Experience view without kitty is what it was before the logos. The footer
// clock is wall time, so it is masked in both.
func TestExperienceViewPlain(t *testing.T) {
	var b strings.Builder
	for _, sel := range []int{0, 5} {
		fmt.Fprintf(&b, "== expSel %d\n", sel)
		b.WriteString(expModel(sel).View())
		b.WriteByte('\n')
	}
	got := clockRE.ReplaceAllString(b.String(), "HH:MM:SS")
	got = uptimeRE.ReplaceAllString(got, "up N")
	got = hereRE.ReplaceAllString(got, "N here")
	want, err := os.ReadFile("testdata/experience.golden")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatal("experience view changed without kitty")
	}
}

// the wide layout starts at a terminal width of about 109 (its pane is 76 cells
// wide). There each role has its logo beside it, and every row keeps the width
// and count it has without one. The narrow layout has no logos, so it is the same.
func TestExperienceViewKitty(t *testing.T) {
	for _, w := range []int{110, 120, 160} {
		for sel := range experiences {
			plain := expModel(sel)
			plain.width = w
			kit := plain
			kit.kitty = true
			pl := strings.Split(view(plain), "\n")
			kl := strings.Split(view(kit), "\n")
			if len(kl) != len(pl) {
				t.Fatalf("width %d, sel %d: %d rows with kitty, %d without", w, sel, len(kl), len(pl))
			}
			for i := range pl {
				if a, b := lipgloss.Width(kl[i]), lipgloss.Width(pl[i]); a != b {
					t.Fatalf("width %d, sel %d, row %d: %d cells with kitty, %d without", w, sel, i, a, b)
				}
			}
			joined := strings.Join(kl, "\n")
			for _, r := range kittyBlock(uint8(expImageID(sel)), expLogoCols, expLogoRows) {
				if !strings.Contains(joined, r+"  ") {
					t.Fatalf("width %d, sel %d: logo row missing", w, sel)
				}
			}
		}
	}
	plain := expModel(3)
	plain.width = 100
	kit := plain
	kit.kitty = true
	if view(kit) != view(plain) {
		t.Fatal("the narrow layout changed with kitty")
	}
	if strings.ContainsRune(view(plain), kittyPlaceholder) {
		t.Fatal("placeholders in the view without kitty")
	}
}

// the help dialog over the Experience view with logos keeps every row's width.
func TestOverlayOverKittyExperience(t *testing.T) {
	plain := expModel(2)
	plain.modal = modalHelp
	kit := plain
	kit.kitty = true
	pl := strings.Split(view(plain), "\n")
	kl := strings.Split(view(kit), "\n")
	if len(kl) != len(pl) {
		t.Fatalf("%d rows with kitty, %d without", len(kl), len(pl))
	}
	for i := range pl {
		if a, b := lipgloss.Width(kl[i]), lipgloss.Width(pl[i]); a != b {
			t.Fatalf("row %d: %d cells with kitty, %d without", i, a, b)
		}
	}
}

// cols is the text of display columns [a, b) of a row, without styling.
func cols(row string, a, b int) string {
	var out strings.Builder
	x := 0
	rest := stripSGR(row)
	for rest != "" {
		var g string
		var w int
		g, rest, w, _ = uniseg.FirstGraphemeClusterInString(rest, -1)
		if x >= a && x+w <= b {
			out.WriteString(g)
		}
		x += w
	}
	return out.String()
}

// every dialog sits in the same columns with kitty images beside it as
// without: an image cell is one column but several runes.
func TestOverlayColumnsBesideKittyImages(t *testing.T) {
	for _, mk := range []modalKind{modalHelp, modalTheme, modalNote} {
		plain := expModel(2)
		plain.modal = mk
		kit := plain
		kit.kitty, kit.bhReady = true, true
		var dlg string
		switch mk {
		case modalHelp:
			dlg = plain.helpDialog()
		case modalTheme:
			dlg = plain.themeDialog()
		case modalNote:
			dlg = plain.noteDialog()
		}
		dl := strings.Split(dlg, "\n")
		dw := 0
		for _, l := range dl {
			dw = max(dw, lipgloss.Width(l))
		}
		x0, y0 := (plain.width-dw)/2, (plain.height-len(dl))/2
		pl := strings.Split(view(plain), "\n")
		kl := strings.Split(view(kit), "\n")
		beside := false
		for y := y0; y < y0+len(dl); y++ {
			if a, b := cols(kl[y], x0, x0+dw), cols(pl[y], x0, x0+dw); a != b {
				t.Fatalf("modal %d row %d: with kitty %q, without %q", mk, y, a, b)
			}
			beside = beside || strings.ContainsRune(cols(kl[y], 0, x0), kittyPlaceholder)
		}
		if !beside {
			t.Fatalf("modal %d: no image beside the dialog, so the test proves nothing", mk)
		}
	}
}

// the idle screensaver opens on pipes.
func TestIdleSaverOpensOnPipes(t *testing.T) {
	m := expModel(0)
	m.lastInputAt = m.at.Add(-(idleSaver + time.Second))
	got, _ := m.Update(animMsg(m.at))
	mm := got.(model)
	if !mm.ssOn || ssModes[mm.ssMode] != "pipes" {
		t.Fatalf("idle saver on %v, mode %q", mm.ssOn, ssModes[mm.ssMode])
	}
}

// the pipes screensaver has the frame's inner size in both forms, and the
// label where the other modes put it.
func TestScreensaverPipesWidths(t *testing.T) {
	const label = " pipes · z next · any key exits "
	for _, kitty := range []bool{false, true} {
		for _, sz := range [][2]int{{80, 24}, {120, 40}, {3, 3}} {
			m := expModel(0)
			m.ssOn, m.ssMode, m.kitty = true, 0, kitty
			m.width, m.height = sz[0], sz[1]
			m.st = 60
			out := m.viewScreensaver()
			w, h := sz[0]-2, sz[1]-2
			lines := strings.Split(out, "\n")
			if len(lines) != h {
				t.Fatalf("kitty %v %dx%d: %d rows, want %d", kitty, sz[0], sz[1], len(lines), h)
			}
			for y, l := range lines {
				if got := lipgloss.Width(l); got != w {
					t.Fatalf("kitty %v %dx%d row %d: width %d, want %d", kitty, sz[0], sz[1], y, got, w)
				}
			}
			if has := strings.ContainsRune(out, kittyPlaceholder); has != kitty {
				t.Fatalf("kitty %v: placeholders present %v", kitty, has)
			}
			if w >= len(label)+1 && !strings.Contains(stripSGR(lines[h-1]), label) {
				t.Fatalf("kitty %v: label missing from the last row", kitty)
			}
		}
	}
}
