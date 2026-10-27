package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type section int

const (
	secOverview section = iota
	secExperience
	secProjects
	secStack
	secContact
	secCount
)

var sectionNames = []string{"Overview", "Experience", "Projects", "Stack", "Contact"}

// animMsg drives everything that moves: scenes, type effects, spinner,
// blinking cursor, clock and the screensaver. One ticker for all of it.
type animMsg time.Time

type modalKind int

const (
	modalNone modalKind = iota
	modalHelp
	modalTheme
	modalNote
)

const sidebarW = 26

const (
	tickDur    = 120 * time.Millisecond // the resting frame rate
	fastTick   = 50 * time.Millisecond  // while a transition is playing
	bioCPS     = 32                     // bio typewriter, characters a second
	idleSaver  = 90 * time.Second       // screensaver after this long without input
	idleFreeze = 6 * time.Minute        // and everything stops after this
	idleQuit   = 20 * time.Minute       // and the session ends; ssh's own idle timeout never fires
)

// Terminal sizes are clamped to these. Every frame is drawn at the full size,
// so an absurd size costs memory and CPU on every tick, and View holds renderMu.
const (
	maxCols = 600
	maxRows = 200
)

// screensaver effects, cycled with z
var ssModes = []string{"doom fire", "marine drive", "matrix", "starfield", "dvd"}

var taglineTW = newTypewriter([]string{
	"software engineer", "building ai agents", "contributing to zed", "shipping from mumbai",
}, 31)

const logoHold = 3.5 // seconds each language logo shows on Stack

type model struct {
	sec           section
	width, height int
	session       int
	ip            string // visitor's host address, for guestbook rate limits; empty locally

	// Per-session look: each visitor gets their own theme and the colour
	// profile their terminal supports. View applies both to the shared
	// palette globals while it renders (see renderMu).
	theme   string
	profile termenv.Profile

	expSel, projSel   int
	prevExp, prevProj int       // the selection before the last change, for the flaps
	logoSel           int       // the Stack logo picked last …
	logoAt            time.Time // … and when: they advance on their own from there

	// Time. at is this frame's wall clock and everything that plays once
	// (title decode, flaps, intro) measures from it. st is scene time: it only
	// advances while things may move — motion on, no dialog open, visitor not
	// long idle — so freezing a scene is "stop counting". tick counts frames.
	at, startAt, bootEndAt time.Time
	st                     float64
	tick                   int
	still, rain            bool
	lastInputAt            time.Time
	saverFired             bool
	secAt, selAt           time.Time // last section / selection change
	headY                  float64   // experience HEAD marker, gliding toward the selection

	booting bool
	walked  bool // has walked the promenade, so the key hint beside them can go

	// opencode-style dialog: dimmed app behind a floating rounded box.
	// Theme selection live-previews; committedTheme is the one in effect
	// when the dialog opened (restored on esc).
	modal          modalKind
	mFilter        string
	mSel           int
	committedTheme string

	// guestbook note being typed in the note dialog
	note     string
	noteSent bool
	noteErr  bool
	noteAt   time.Time

	// screensaver ("z", or after a while idle): full-screen effects
	ssOn       bool
	ssMode     int
	ssX, ssY   int
	ssVX, ssVY int
	ssText     string
}

func newModel(session int, profile termenv.Profile) model {
	now := time.Now()
	return model{
		session: session,
		theme:   themes[0].name, profile: profile,
		committedTheme: themes[0].name,
		ssText:         "ssh arpitmishra.dev", ssVX: 2, ssVY: 1,
		booting: true, rain: rainyFor(session),
		at: now, startAt: now, lastInputAt: now,
	}
}

func newProgram(session int) *tea.Program {
	return tea.NewProgram(newModel(session, lipgloss.ColorProfile()), tea.WithAltScreen())
}

func animTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return animMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(animTick(fastTick), tea.SetWindowTitle("arpit mishra — portfolio"))
}

// since is seconds from t to this frame.
func (m model) since(t time.Time) float64 { return m.at.Sub(t).Seconds() }

// blink is the shared cursor blink.
func (m model) blink() bool { return m.at.UnixMilli()/480%2 == 0 }

// animating says whether scene time should advance this frame.
func (m model) animating() bool {
	return !m.still && m.modal == modalNone && m.at.Sub(m.lastInputAt) < idleFreeze
}

// frameDur ticks faster while something short is playing, so flaps and
// decodes move smoothly, and drops back to the resting rate after.
func (m model) frameDur() time.Duration {
	if m.booting || m.since(m.secAt) < 1.2 || m.since(m.selAt) < 1.2 ||
		(m.noteSent && m.since(m.noteAt) < 3) ||
		(m.sec == secStack && math.Mod(m.since(m.logoAt), logoHold) < 0.5) {
		return fastTick
	}
	return tickDur
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = clamp(msg.Width, 0, maxCols), clamp(msg.Height, 0, maxRows)
	case animMsg:
		now := time.Time(msg)
		// no key for this long: end the session so abandoned clients free their slot
		if now.Sub(m.lastInputAt) > idleQuit {
			return m, tea.Quit
		}
		dt := clampf(now.Sub(m.at).Seconds(), 0, 0.25)
		m.at = now
		m.tick++
		if m.animating() {
			m.st += dt
		}
		if m.booting && m.since(m.startAt) >= bootDur {
			m.endBoot()
		}
		// the HEAD marker eases toward the selected commit
		target := float64(m.expSel * 2)
		m.headY += (target - m.headY) * (1 - math.Exp(-dt*9))
		if m.still || math.Abs(target-m.headY) < 0.3 {
			m.headY = target
		}
		if !m.ssOn && !m.booting && m.modal == modalNone && !m.saverFired && m.at.Sub(m.lastInputAt) >= idleSaver {
			m.ssOn, m.ssMode, m.saverFired = true, 0, true
		}
		if m.ssOn && ssModes[m.ssMode] == "dvd" && m.animating() {
			m.stepScreensaver()
		}
		return m, animTick(m.frameDur())
	case tea.KeyMsg:
		m.at = time.Now()
		m.lastInputAt, m.saverFired = m.at, false
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.booting {
			// any key skips the intro (q still quits)
			if msg.String() == "q" {
				return m, tea.Quit
			}
			m.endBoot()
			return m, nil
		}
		if m.ssOn {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "z":
				m.ssMode = (m.ssMode + 1) % len(ssModes)
				m.ssX, m.ssY = 4, 2
				return m, nil
			}
			m.ssOn = false // any other key leaves the screensaver
			return m, nil
		}
		if m.modal != modalNone {
			return m.updateModal(msg)
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "?":
			m.modal = modalHelp
		case "t":
			m.modal = modalTheme
			m.mFilter, m.mSel = "", 0
			m.committedTheme = m.theme
		case "z":
			m.ssOn, m.ssMode = true, 0
			m.ssX, m.ssY = 4, 2
		case "m":
			m.still = !m.still
		case "r":
			m.rain = !m.rain
		case "a", "d":
			if m.sec == secOverview {
				step := 1 / float64(max(m.contentWidth()-5, 1))
				if msg.String() == "a" {
					step = -step
				}
				hub.walk(m.session, step)
				m.walked = true
			}
		case "n":
			if m.sec == secContact && !m.noteSent {
				m.modal, m.note, m.noteErr = modalNote, "", false
			}
		case "tab", "right", "l":
			m.goTo((m.sec + 1) % secCount)
		case "shift+tab", "left", "h":
			m.goTo((m.sec + secCount - 1) % secCount)
		case "1", "2", "3", "4", "5":
			if n := int(msg.String()[0] - '1'); n < int(secCount) {
				m.goTo(section(n))
			}
		case "up", "k", "down", "j":
			down := msg.String() == "down" || msg.String() == "j"
			delta := 1
			if !down {
				delta = -1
			}
			switch m.sec {
			case secExperience:
				if n := clamp(m.expSel+delta, 0, len(experiences)-1); n != m.expSel {
					m.prevExp, m.expSel, m.selAt = m.expSel, n, m.at
				}
			case secProjects:
				if n := clamp(m.projSel+delta, 0, len(projects)-1); n != m.projSel {
					m.prevProj, m.projSel, m.selAt = m.projSel, n, m.at
				}
			case secStack:
				m.logoSel = (m.logoShown() + delta + len(logoArt)) % len(logoArt)
				m.logoAt = m.at
			}
		}
	}
	return m, nil
}

func (m *model) goTo(s section) {
	if s != m.sec {
		m.sec, m.secAt = s, m.at
		// arriving on a list flaps its current entry in from blank
		m.prevExp, m.prevProj, m.selAt = -1, -1, m.at
		if s == secStack {
			m.logoSel, m.logoAt = m.logoShown(), m.at
		}
	}
}

func (m *model) endBoot() {
	m.booting = false
	m.bootEndAt, m.secAt, m.selAt = m.at, m.at, m.at
	m.prevExp, m.prevProj = -1, -1
}

// updateModal handles keys while a dialog is open. Like opencode: typing
// filters, arrows move, esc closes, enter picks. Moving through the theme
// grid live-previews each theme behind the dialog.
func (m model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal == modalNote {
		return m.updateNote(msg)
	}
	items := m.filteredThemes()
	switch msg.String() {
	case "esc":
		if m.modal == modalTheme {
			m.theme = m.committedTheme // revert preview
		}
		m.modal = modalNone
	case "enter":
		if m.modal == modalTheme && len(items) > 0 {
			m.committedTheme = items[clamp(m.mSel, 0, len(items)-1)]
			m.theme = m.committedTheme
		}
		m.modal = modalNone
	case "backspace":
		if r := []rune(m.mFilter); len(r) > 0 {
			m.mFilter = string(r[:len(r)-1])
			m.mSel = 0
			m.previewTheme()
		}
	case "up", "k":
		if m.modal == modalTheme {
			m.mSel = clamp(m.mSel-2, 0, max(len(items)-1, 0))
			m.previewTheme()
		}
	case "down", "j":
		if m.modal == modalTheme {
			m.mSel = clamp(m.mSel+2, 0, max(len(items)-1, 0))
			m.previewTheme()
		}
	case "left", "h":
		if m.modal == modalTheme {
			m.mSel = clamp(m.mSel-1, 0, max(len(items)-1, 0))
			m.previewTheme()
		}
	case "right", "l":
		if m.modal == modalTheme {
			m.mSel = clamp(m.mSel+1, 0, max(len(items)-1, 0))
			m.previewTheme()
		}
	case "?":
		if m.modal == modalHelp {
			m.modal = modalNone
		}
	default:
		// printable runes go into the filter (theme picker only)
		if m.modal == modalTheme && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			m.mFilter += string(msg.Runes)
			m.mSel = 0
			m.previewTheme()
		}
	}
	return m, nil
}

// updateNote is a one-line text box: type, backspace, enter sends, esc cancels.
func (m model) updateNote(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.modal = modalNone
	case tea.KeyEnter:
		if note := strings.TrimSpace(m.note); note != "" {
			err := saveNote(hub.name(m.session), m.session, m.ip, note)
			m.noteSent, m.noteErr, m.noteAt = err == nil, err != nil, m.at
		}
		m.modal = modalNone
	case tea.KeyBackspace:
		if r := []rune(m.note); len(r) > 0 {
			m.note = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.note += " "
	case tea.KeyRunes:
		m.note += cleanNoteRunes(msg.Runes)
	}
	if r := []rune(m.note); len(r) > noteMax {
		m.note = string(r[:noteMax])
	}
	return m, nil
}

// previewTheme applies whatever the cursor sits on, without committing it.
func (m *model) previewTheme() {
	items := m.filteredThemes()
	if len(items) > 0 {
		m.theme = items[clamp(m.mSel, 0, len(items)-1)]
	}
}

func (m model) filteredThemes() []string {
	if m.mFilter == "" {
		return themeNames()
	}
	var out []string
	for _, n := range themeNames() {
		if strings.Contains(n, m.mFilter) {
			out = append(out, n)
		}
	}
	return out
}

// renderMu guards the package-level palette and styles, which every session
// shares: a session holds it while its own theme and colour profile are
// applied and its frame is rendered.
var renderMu sync.Mutex

func (m model) View() string {
	renderMu.Lock()
	defer renderMu.Unlock()
	lipgloss.SetColorProfile(m.profile)
	applyTheme(m.theme)
	return m.view()
}

func (m model) view() string {
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cFaint)

	if m.width == 0 {
		// no resize event yet — paint a default-size frame immediately
		// instead of sitting on a loading screen
		m.width, m.height = 100, 30
	}

	switch {
	case m.booting:
		return frame.Render(m.viewBoot())
	case m.ssOn:
		return frame.Render(m.viewScreensaver())
	}

	app := m.viewApp(frame)
	if m.modal != modalNone {
		return m.overlay(frame, app)
	}
	return app
}

// contentWidth is the width of the right-hand pane, padding included.
func (m model) contentWidth() int {
	innerW := clamp(m.width-2, 40, m.width)
	return max(innerW-sidebarW-5, 20)
}

// sceneOpts snapshots the world for this frame's scenes.
func (m model) sceneOpts() sceneOpts {
	return sceneOpts{
		t: m.st, hour: mumbaiHour(), moon: moonPhase(time.Now()),
		rain: m.rain, visitors: hub.list(), self: m.session, hint: !m.walked,
	}
}

func (m model) viewApp(frame lipgloss.Style) string {
	innerW := clamp(m.width-2, 40, m.width)
	innerH := m.height - 2
	contentW := m.contentWidth()

	// --- sidebar ---
	nav := make([]string, 0, secCount)
	for i, n := range sectionNames {
		num := stNavNum.Render(fmt.Sprintf("%d ", i+1))
		if section(i) == m.sec {
			nav = append(nav, stCursor.Render("❯ ")+num+lipgloss.NewStyle().Foreground(cText).Bold(true).Render(n))
		} else {
			nav = append(nav, "  "+num+stNavItem.Render(n))
		}
	}

	// the tagline types itself out, a few phrases in turn
	tagline, lit := taglineTW.at(m.st)
	if m.still {
		tagline, lit = "software engineer", m.blink()
	}
	cursor := " "
	if lit {
		cursor = stCursor.Render("▍")
	}

	// a black hole turning its disk, when the sidebar has the room
	art := ""
	if innerH >= 30 {
		art = blackHole(sidebarW-2, 9, m.st, paletteOf(activeColors)).render(m.profile) + "\n"
	}

	side := lipgloss.JoinVertical(lipgloss.Left,
		"",
		stName.Render(name),
		stTagline.Render(tagline)+cursor,
		"",
		strings.Join(nav, "\n"),
		"",
		"",
		art,
		stDim.Render("now"),
		stFaint.Render("interning at vyazen"),
		"",
		stFaint.Render("mumbai, india"),
	)
	side = lipgloss.NewStyle().Width(sidebarW).MaxHeight(innerH - 2).PaddingLeft(1).Render(side)

	// --- content pane ---
	h := innerH - 2
	var content string
	switch m.sec {
	case secOverview:
		content = m.viewOverview(contentW, h)
	case secExperience:
		content = m.viewExperience(contentW)
	case secProjects:
		content = m.viewProjects(contentW)
	case secStack:
		content = m.viewStack(contentW, h)
	case secContact:
		content = m.viewContact(contentW, h)
	}
	content = lipgloss.NewStyle().
		Width(contentW).Height(h).MaxHeight(h).
		Padding(0, 2).
		Render(content)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		side,
		stFaint.Render(vline(h)), // match content height, or the frame overflows by a row
		content,
	)

	// --- status bar ---
	spinner := spinnerFrames[int(m.at.UnixMilli()/240)%len(spinnerFrames)]
	here := stDim.Render(fmt.Sprintf("%d here", hub.count()))
	left := "  " + m.footerHint()
	right := stOnline.Render(spinner) + " " + stOnline.Render("live") +
		"  " + here +
		"  " + stDim.Render(fmt.Sprintf("session #%d", m.session)) +
		"  " + stDim.Render(uptime()) +
		"  " + stClock.Render(clockNow()) + " "
	// narrow terminals (80 cols): shed the right-hand extras before the
	// key hints, so the bar never wraps and pushes the frame off-screen
	if stripWidth(left)+stripWidth(right) > innerW {
		right = stOnline.Render(spinner) + " " + here + "  " + stClock.Render(clockNow()) + " "
	}
	if stripWidth(left)+stripWidth(right) > innerW {
		right = ""
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Bottom,
		lipgloss.NewStyle().Width(innerW-stripWidth(right)).MaxWidth(innerW).MaxHeight(1).Render(left),
		right,
	)

	return frame.Render(lipgloss.JoinVertical(lipgloss.Left,
		body,
		stFaint.Render(hline(innerW)),
		bar,
	))
}

// overlay floats the dialog over a dimmed — but still colored, so theme
// previews show through — render of the app, opencode-style.
func (m model) overlay(frame lipgloss.Style, _ string) string {
	// build the dialog at full color first
	var dlgStr string
	switch m.modal {
	case modalHelp:
		dlgStr = m.helpDialog()
	case modalTheme:
		dlgStr = m.themeDialog()
	case modalNote:
		dlgStr = m.noteDialog()
	}
	if dlgStr == "" {
		return m.viewApp(frame)
	}

	// render the backdrop with dimmed theme colors, then restore
	full := activeColors
	setColors(dimmedTheme(full))
	activeColors = dimmedTheme(full) // scenes read their palette from here
	app := m.viewApp(frame)
	activeColors = full
	setColors(full)

	dlg := strings.Split(dlgStr, "\n")
	dw := 0
	for _, l := range dlg {
		if w := stripWidth(l); w > dw {
			dw = w
		}
	}
	dw = min(dw, m.width)
	x0 := max((m.width-dw)/2, 0)
	y0 := max((m.height-len(dlg))/2, 0)

	lines := strings.Split(app, "\n")
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}

	for i, dl := range dlg {
		y := y0 + i
		if y >= len(lines) {
			break
		}
		seg := padRight(dl, dw)
		left, rightRest := splitANSIAt(lines[y], x0)
		_, rightTail := splitANSIAt(rightRest, dw)
		lines[y] = left + seg + rightTail
	}
	return strings.Join(lines, "\n")
}

// splitANSIAt splits an ANSI-styled string at a display column, carrying any
// in-effect SGR styling into the right half (and resetting the left).
func splitANSIAt(s string, col int) (string, string) {
	if col <= 0 {
		return "", s
	}
	var left, right strings.Builder
	var codes []string // SGR sequences currently in effect
	cut := false
	width := 0
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) {
			j := i + 1
			switch s[j] {
			case '[': // CSI: parameters until a final byte 0x40–0x7e
				j++
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				if j < len(s) {
					j++
				}
			case ']': // OSC: until BEL or ST (ESC \)
				j++
				for j < len(s) && s[j] != 0x07 && !(s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\') {
					j++
				}
				if j < len(s) && s[j] == 0x07 {
					j++
				} else if j+1 < len(s) {
					j += 2
				}
			default: // two-byte escape
				j++
			}
			seq := s[i:j]
			if cut {
				right.WriteString(seq)
			} else {
				left.WriteString(seq)
				if isReset(seq) {
					codes = codes[:0]
				} else if strings.HasSuffix(seq, "m") {
					codes = append(codes, seq)
				}
			}
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if width >= col && !cut {
			cut = true
			if len(codes) > 0 {
				left.WriteString("\x1b[0m")
				for _, c := range codes {
					right.WriteString(c)
				}
			}
		}
		if cut {
			right.WriteString(string(r))
		} else {
			left.WriteString(string(r))
			width++
		}
		i += size
	}
	return left.String(), right.String()
}

func isReset(seq string) bool {
	return seq == "\x1b[0m" || seq == "\x1b[m"
}

func (m model) themeDialog() string {
	const innerW = 56
	items := m.filteredThemes()

	// filter row: "› filter▏" with the match count right-aligned
	cur := " "
	if m.blink() {
		cur = stCursor.Render("▍")
	}
	filterRow := stTitle.Render("› ") + stBody.Render(m.mFilter) + cur +
		strings.Repeat(" ", max(innerW-stripWidth(m.mFilter)-24, 1)) +
		stDim.Render(fmt.Sprintf("%d/%d themes", len(items), len(themes)))

	var content []string
	content = append(content, filterRow, "")

	// two-column grid of theme names
	const colW = 27
	for i := 0; i < len(items); i += 2 {
		var row string
		for c := 0; c < 2; c++ {
			idx := i + c
			if idx >= len(items) {
				break
			}
			if idx == m.mSel {
				// same 2-col gutter as the marker column so text doesn't shift
				label := "  " + items[idx] + " "
				row += stSelRow.Render(label) + strings.Repeat(" ", colW-stripWidth(label))
			} else {
				mark := "  "
				if items[idx] == m.committedTheme {
					mark = stOnline.Render("● ")
				}
				row += mark + stBody.Render(items[idx]) +
					strings.Repeat(" ", colW-2-stripWidth(items[idx]))
			}
		}
		content = append(content, row)
	}

	return dialog("theme", content,
		keycaps(
			[2]string{"↑↓←→", "navigate"},
			[2]string{"type", "filter"},
			[2]string{"enter", "select"},
			[2]string{"esc", "close"},
		), innerW+4)
}

func (m model) helpDialog() string {
	const innerW = 46
	rows := [][2]string{
		{"↑ / k", "move up"},
		{"↓ / j", "move down"},
		{"← → / tab", "switch section"},
		{"1 – 5", "jump to section"},
		{"a / d", "walk the promenade"},
		{"n", "leave a note (contact)"},
		{"r", "toggle rain"},
		{"m", "pause motion"},
		{"t", "change theme"},
		{"z", "???"},
		{"q", "quit"},
	}
	var content []string
	content = append(content, "")
	for _, r := range rows {
		content = append(content, "  "+stKeycap.Render(fmt.Sprintf("%-11s", r[0]))+stDim.Render(r[1]))
	}
	content = append(content, "", "  "+stFaint.Render("psst — try z, then z again"))
	return dialog("keys", content,
		keycaps(
			[2]string{"esc", "close"},
			[2]string{"?", "toggle"},
		), innerW+4)
}

func (m model) noteDialog() string {
	const innerW = 56
	cur := " "
	if m.blink() {
		cur = stCursor.Render("▍")
	}
	content := []string{
		stDim.Render("say hi, leave feedback, tell me what to build."),
		stFaint.Render("only I can read these."),
		"",
	}
	lines := chunkWidth(m.note, innerW-2)
	for i, l := range lines {
		prefix := "  "
		if i == 0 {
			prefix = stTitle.Render("› ")
		}
		if i == len(lines)-1 {
			l = stBody.Render(l) + cur
		} else {
			l = stBody.Render(l)
		}
		content = append(content, prefix+l)
	}
	count := fmt.Sprintf("%d/%d", utf8.RuneCountInString(m.note), noteMax)
	content = append(content, "", strings.Repeat(" ", innerW-len(count))+stFaint.Render(count))
	return dialog("leave a note", content,
		keycaps(
			[2]string{"enter", "send"},
			[2]string{"esc", "cancel"},
		), innerW+4)
}

func (m model) footerHint() string {
	key := func(k, s string) string {
		return stKeycap.Render(k) + stFooter.Render(" "+s+"  ")
	}
	switch {
	case m.modal != modalNone:
		return key("esc", "close")
	case m.sec == secOverview:
		return key("a/d", "walk") + key("⇤⇥", "section") + key("t", "themes") + key("?", "help") + key("q", "quit")
	case m.sec == secStack:
		return key("↑↓", "logo") + key("⇤⇥", "section") + key("t", "themes") + key("?", "help") + key("q", "quit")
	case m.sec == secContact && !m.noteSent:
		return key("n", "note") + key("⇤⇥", "section") + key("t", "themes") + key("?", "help") + key("q", "quit")
	default:
		return key("↑↓", "select") + key("⇤⇥", "section") + key("t", "themes") + key("?", "help") + key("q", "quit")
	}
}

// title renders a section heading, decoding out of flickering glyphs right
// after the section changes.
func (m model) title(s string) string {
	since := m.since(m.secAt)
	if m.still || since > 1.5 {
		return stTitle.Render(s)
	}
	rs, settled := scrambleIn(s, since)
	return styleRuns(rs, settled, stTitle, stFaint)
}

// flapped renders a line as split-flap tiles turning from what was shown
// before; tiles still turning are drawn faint.
func (m model) flapped(from, to string, st lipgloss.Style) string {
	since := m.since(m.selAt)
	if m.still || since > 1.5 {
		return st.Render(to)
	}
	rs, turning := flap(from, to, since)
	settled := make([]bool, len(turning))
	for i, t := range turning {
		settled[i] = !t
	}
	return strings.TrimRight(styleRuns(rs, settled, st, stFaint), " ")
}

// styleRuns styles each run of runes by whether its flag is set.
func styleRuns(rs []rune, flag []bool, on, off lipgloss.Style) string {
	var b strings.Builder
	for i := 0; i < len(rs); {
		j := i
		for j < len(rs) && flag[j] == flag[i] {
			j++
		}
		if flag[i] {
			b.WriteString(on.Render(string(rs[i:j])))
		} else {
			b.WriteString(off.Render(string(rs[i:j])))
		}
		i = j
	}
	return b.String()
}

// staggered shows bullet i a beat after bullet i-1 when a selection changes;
// hidden bullets keep their height so nothing below jumps.
func (m model) staggered(i int, s string) string {
	if m.still || m.since(m.selAt) >= 0.25+float64(i)*0.09 {
		return s
	}
	return strings.Repeat("\n", countLines(s)-1)
}

// sceneRoom is how many rows a scene gets under some text, 0 if it won't fit.
func sceneRoom(text string, w, h, minH, maxH int) int {
	sh := min(h-countLines(text)-1, maxH)
	if sh < minH || w < 30 {
		return 0
	}
	return sh
}

// bottomScene places a scene under some text, pinned to the bottom of the
// pane, if there's room for at least minH rows of it.
func bottomScene(text string, w, h, minH, maxH int, draw func(w, h int) *canvas, p termenv.Profile) string {
	sh := sceneRoom(text, w, h, minH, maxH)
	if sh == 0 {
		return text
	}
	return text + strings.Repeat("\n", h-countLines(text)-sh+1) + draw(w, sh).render(p)
}

func (m model) viewOverview(w, h int) string {
	tw := w - 4
	n := len([]rune(bio))
	if !m.still {
		n = clamp(int(m.since(m.bootEndAt)*bioCPS), 0, n)
	}
	bioText := wrap(string([]rune(bio)[:n]), tw)
	// reserve the finished bio's height so the scene doesn't shift while typing
	if pad := countLines(wrap(bio, tw)) - countLines(bioText); pad > 0 {
		bioText += strings.Repeat("\n", pad)
	}
	var b strings.Builder
	b.WriteString(m.greeting() + "\n\n")
	b.WriteString(stBody.Render(bioText) + "\n\n")
	b.WriteString(stSection.Render("links") + "\n")
	b.WriteString(linksBlock(2, tw) + "\n")
	b.WriteString(stFaint.Render("  ⌘/ctrl-click a link to open it"))

	o, pal := m.sceneOpts(), paletteOf(activeColors)
	return bottomScene(b.String(), tw, h, 4, 6, func(w, h int) *canvas {
		return road(w, h, o, pal)
	}, m.profile)
}

// greeting is the Overview's heading in plain type, the name in the accent.
// It decodes in on arrival, then a soft glint runs over the name now and then.
func (m model) greeting() string {
	const greet, name = "Hey, I'm ", "Arpit."
	full := []rune(greet + name)
	g := len([]rune(greet))
	rs, settled := scrambleIn(string(full), m.since(m.secAt))
	if m.still || m.since(m.secAt) > 1.5 {
		rs, settled = full, nil
	}
	const every, pass = 6.0, 0.9
	pos := -99.0
	if u := math.Mod(m.st+every-1, every) / pass; !m.still && u < 1 {
		pos = -2 + u*float64(len(name)+4)
	}
	nameStyle := stTitle
	var b strings.Builder
	for i, r := range rs {
		switch {
		case settled != nil && !settled[i]:
			b.WriteString(stFaint.Render(string(r)))
		case i < g:
			b.WriteString(stBody.Render(string(r)))
		default:
			k := clamp01(1 - math.Abs(float64(i-g)-pos)/2)
			st := nameStyle
			if k > 0 {
				st = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(
					hexLerp(activeColors.accent, hexLerp(activeColors.text, "#ffffff", 0.5), math.Round(k*3)/3)))
			}
			b.WriteString(st.Render(string(r)))
		}
	}
	return b.String()
}

func (m model) viewExperience(w int) string {
	var b strings.Builder
	b.WriteString(m.title("Experience") + stDim.Render("  — 8 roles") + "\n\n")

	if w >= 76 {
		const listW = 32
		list := m.expList(listW - 2)
		e := experiences[m.expSel]
		var prev experience
		if m.prevExp >= 0 {
			prev = experiences[m.prevExp]
		}
		det := new(strings.Builder)
		det.WriteString(m.flapped(prev.role, e.role, stName) + "\n")
		det.WriteString(m.flapped(prev.company, e.company, stSection) + "  " + m.flapped(prev.period, e.period, stDim) + "\n\n")
		for i, p := range e.points {
			det.WriteString(m.staggered(i, bullet(p, w-listW-8)) + "\n\n")
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(listW).Render(list),
			stFaint.Render(vline(max(countLines(det.String()), countLines(list)))),
			lipgloss.NewStyle().Width(w-listW-2).PaddingLeft(2).Render(det.String()),
		))
		return b.String()
	}

	for i, e := range experiences {
		if i == m.expSel {
			b.WriteString("  " + stCursor.Render("▍") +
				stSelRow.Render(" "+e.role+" ") +
				stDim.Render("  ·  "+e.company) + "\n")
			b.WriteString("    " + stDim.Render(e.period) + "\n")
		} else {
			b.WriteString("    " + stBody.Render(e.role) + stDim.Render("  ·  "+e.company) + "\n")
		}
	}
	return b.String()
}

// expList draws the roles like a git log: one per role on a rail, dated, with
// a marker that glides along the rail to the selected commit.
func (m model) expList(w int) string {
	head := int(math.Round(m.headY))
	moving := math.Abs(m.headY-float64(m.expSel*2)) > 0.01
	var lines []string
	for i, e := range experiences {
		row := i * 2
		name := e.company
		if r := []rune(name); len(r) > w-13 {
			name = string(r[:w-14]) + "…"
		}
		start := stFaint.Render(startDate(e.period))
		switch {
		case i == m.expSel && !moving:
			lines = append(lines, stCursor.Render("●")+" "+start+" "+stSelRow.Render(" "+name+" "))
		case row == head:
			lines = append(lines, stCursor.Render("◉")+" "+start+" "+stBody.Render(name))
		default:
			lines = append(lines, stFaint.Render("○")+" "+start+" "+stBody.Render(name))
		}
		if i < len(experiences)-1 {
			if row+1 == head {
				lines = append(lines, stCursor.Render("┃"))
			} else {
				lines = append(lines, stFaint.Render("│"))
			}
		}
	}
	return lipgloss.NewStyle().Width(w).Render(strings.Join(lines, "\n"))
}

// startDate shortens when a role began to the width of a short hash:
// "May 2026 — Present" becomes "May '26".
func startDate(period string) string {
	f := strings.Fields(period)
	if len(f) < 2 || len(f[1]) != 4 {
		return fmt.Sprintf("%-7.7s", period)
	}
	return f[0] + " '" + f[1][2:]
}

func (m model) viewProjects(w int) string {
	var b strings.Builder
	b.WriteString(m.title("Projects") + stDim.Render("  — 4 builds") + "\n\n")

	if w >= 76 {
		// same list-detail layout as Experience
		const listW = 24
		var list []string
		for i, p := range projects {
			if i == m.projSel {
				list = append(list, stCursor.Render("▍")+stSelRow.Render(" "+p.name+" "))
			} else {
				list = append(list, "  "+stBody.Render(p.name))
			}
		}
		p := projects[m.projSel]
		var prev project
		if m.prevProj >= 0 {
			prev = projects[m.prevProj]
		}
		det := new(strings.Builder)
		det.WriteString(m.flapped(prev.name, p.name, stName) + "\n")
		det.WriteString(m.flapped(prev.stack, p.stack, stSection) + "\n\n")
		for i, pt := range p.points {
			det.WriteString(m.staggered(i, bullet(pt, w-listW-8)) + "\n\n")
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(listW).Render(strings.Join(list, "\n")),
			stFaint.Render(vline(countLines(det.String()))),
			lipgloss.NewStyle().Width(w-listW-2).PaddingLeft(2).Render(det.String()),
		))
		return b.String()
	}

	for i, p := range projects {
		if i == m.projSel {
			b.WriteString("  " + stCursor.Render("▍") +
				stSelRow.Render(" "+p.name+" ") +
				stDim.Render("  "+p.stack) + "\n")
			for _, pt := range p.points {
				b.WriteString(bullet(pt, w-6) + "\n")
			}
		} else {
			b.WriteString("    " + stBody.Render(p.name) + stDim.Render("  "+p.stack) + "\n")
		}
	}
	return b.String()
}

// logoShown is the language logo on show: the one picked last, then each
// in turn every few seconds.
func (m model) logoShown() int {
	n := m.logoSel
	if !m.still && !m.logoAt.IsZero() {
		n += int(m.since(m.logoAt) / logoHold)
	}
	return n % len(logoArt)
}

// viewStack lists the tech beside (or above) the logo of one language,
// which decodes in, catches a glint, and gives way to the next.
func (m model) viewStack(w, h int) string {
	tw := w - 4
	shown := m.logoShown()
	l := logoArt[shown]

	// logos draw at half their original size, beside the list when it fits,
	// else under it
	const listW, half = 40, true
	maxHW, maxHH := 0, 0
	for _, lg := range logoArt {
		hw, hh := logoSize(lg, half)
		maxHW, maxHH = max(maxHW, hw), max(maxHH, hh)
	}
	side := tw >= listW+2+maxHW && h >= maxHH+4
	textW := tw
	if side {
		textW = listW
	}

	var b strings.Builder
	b.WriteString(m.title("Stack") + "\n\n")
	for _, g := range techGroups {
		b.WriteString(stSection.Render(g.name) + "\n")
		var styled []string
		var widths []int
		for _, it := range g.items {
			st := stBody
			if logoFor(it) == shown {
				st = stTitle
			}
			styled = append(styled, st.Render(it))
			widths = append(widths, stripWidth(it))
		}
		b.WriteString(joinWrapped(styled, widths, stFaint.Render("  ·  "), 5, textW) + "\n\n")
	}
	text := strings.TrimRight(b.String(), "\n")

	since := math.Mod(m.since(m.logoAt), logoHold)
	reveal := 1.0
	if !m.still && !m.logoAt.IsZero() {
		reveal = clamp01(since / 0.3)
	}
	logo := drawLogo(l, half, m.st, reveal, paletteOf(activeColors)).render(m.profile)
	caption := stDim.Render(l.name) + stFaint.Render("  ·  ↑ ↓ for more")
	// every logo sits in the same box, centred, so the caption never moves
	_, lh := logoSize(l, half)
	boxed := strings.Repeat("\n", (maxHH-lh)/2) + logo + strings.Repeat("\n", maxHH-lh-(maxHH-lh)/2)
	if side {
		block := lipgloss.NewStyle().Width(tw - listW - 2).Align(lipgloss.Center).Render(
			lipgloss.JoinVertical(lipgloss.Center, boxed, "", caption))
		// level with the list's middle rather than its top
		pad := max(2, (countLines(text)-countLines(block))/2)
		return lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(listW+2).Render(text), strings.Repeat("\n", pad)+block)
	}
	if h-countLines(text)-3 < maxHH {
		return text
	}
	block := lipgloss.NewStyle().Width(tw).Align(lipgloss.Center).Render(
		lipgloss.JoinVertical(lipgloss.Center, boxed, "", caption))
	return text + strings.Repeat("\n", max(h-countLines(text)-countLines(block)+1, 2)) + block
}

func (m model) viewContact(w, h int) string {
	tw := w - 4
	var b strings.Builder
	b.WriteString(m.title("Contact") + "\n\n")
	b.WriteString(wrap(stBody.Render("Best way to reach me is email — I usually reply within a day."), w-2) + "\n\n")
	b.WriteString(linksBlock(1, tw) + "\n")
	b.WriteString(stFaint.Render("  ⌘/ctrl-click a link to open it") + "\n\n")
	count := ""
	if c := guestbookCount.Load(); c > 0 {
		count = stFaint.Render(fmt.Sprintf("  ·  %d so far", c))
	}
	switch {
	case m.noteSent:
		// the thanks rides one swell, then settles
		rows := waveText("note received — thank you!", m.since(m.noteAt))
		if m.still {
			rows = waveText("note received — thank you!", 99)
		}
		b.WriteString("    " + stDim.Render(string(rows[0])) + "\n")
		b.WriteString("  " + stOnline.Render("✓ ") + stDim.Render(string(rows[1])) + count + "\n")
		b.WriteString("    " + stDim.Render(string(rows[2])))
	case m.noteErr:
		b.WriteString("  " + stDim.Render("couldn't save that note — try email instead"))
	default:
		b.WriteString("  " + stKeycap.Render("n") + stDim.Render(" leave me a note — only I can read it") + count)
	}
	text := b.String()

	o, pal := m.sceneOpts(), paletteOf(activeColors)
	return bottomScene(text, tw, h, 12, 19, func(w, h int) *canvas {
		return lighthouse(w, h, o, pal)
	}, m.profile)
}

// --- intro ---

var bootSteps = []struct{ text, detail string }{
	{"mounting /experience", fmt.Sprintf("%d roles", len(experiences))},
	{"loading /projects", fmt.Sprintf("%d builds", len(projects))},
	{"indexing /stack", fmt.Sprintf("%d tools", toolCount())},
	{"lighting marine drive", ""},
	{"finding other visitors", ""},
}

const bootStep = 0.12 // seconds per boot line

// one beat per boot line, then the welcome decodes and holds briefly
var bootDur = float64(len(bootSteps)+1)*bootStep + 1.2

func toolCount() int {
	n := 0
	for _, g := range techGroups {
		n += len(g.items)
	}
	return n
}

func (m model) viewBoot() string {
	w, h := m.width-2, m.height-2
	const bw = 46
	bt := m.since(m.startAt)
	var lines []string
	lines = append(lines, stTitle.Render("arpit-os 1.0")+stFaint.Render("  ·  tty/ssh"), "")
	for i, s := range bootSteps {
		if bt < float64(i)*bootStep {
			lines = append(lines, "")
			continue
		}
		detail := s.detail
		switch s.text {
		case "lighting marine drive":
			hr := mumbaiHour()
			detail = fmt.Sprintf("%02d:%02d ist", int(hr), int(math.Mod(hr*60, 60)))
		case "finding other visitors":
			detail = fmt.Sprintf("%d online", hub.count())
		}
		status := stDim.Render("[ ") + stOnline.Render("ok") + stDim.Render(" ]")
		if bt < float64(i+1)*bootStep {
			status = stDim.Render("[ ") + stFaint.Render(spinnerFrames[int(bt*20)%len(spinnerFrames)]+" ") + stDim.Render(" ]")
		}
		row := status + " " + stBody.Render(s.text)
		lines = append(lines, row+strings.Repeat(" ", max(bw-stripWidth(row)-len(detail), 1))+stDim.Render(detail))
	}

	done := clamp01(bt / (float64(len(bootSteps)) * bootStep))
	fill := int(done * float64(bw))
	lines = append(lines, "", stTitle.Render(strings.Repeat("━", fill))+stFaint.Render(strings.Repeat("━", bw-fill)), "")

	if d := bt - float64(len(bootSteps)+1)*bootStep; d >= 0 {
		rs, settled := scrambleIn("welcome, "+hub.name(m.session)+".", d)
		lines = append(lines, styleRuns(rs, settled, stName, stFaint))
	} else {
		lines = append(lines, "")
	}
	lines = append(lines, "", stFaint.Render("any key to skip"))

	block := lipgloss.NewStyle().Width(bw).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
}

// --- screensaver ---

func (m model) viewScreensaver() string {
	w, h := m.width-2, m.height-2
	if ssModes[m.ssMode] == "dvd" {
		return m.viewBounce(w, h)
	}
	t, pal := m.st, paletteOf(activeColors)
	var cv *canvas
	switch ssModes[m.ssMode] {
	case "marine drive":
		cv = marineDrive(w, h, m.sceneOpts(), pal)
	case "doom fire":
		cv = doomFire(w, h, t, pal)
	case "matrix":
		cv = matrixRain(w, h, t, pal)
	case "starfield":
		cv = starfield(w, h, t, pal)
	}
	label := fmt.Sprintf(" %s · z next · any key exits ", ssModes[m.ssMode])
	cv.text(w-utf8.RuneCountInString(label)-1, h-1, label, pal.dim)
	return cv.render(m.profile)
}

func (m model) viewBounce(w, h int) string {
	tw := stripWidth(m.ssText)

	canvas := lipgloss.NewStyle().Width(w).Height(h).Render("")
	lines := strings.Split(canvas, "\n")
	if m.ssY < len(lines) {
		row := lines[m.ssY]
		spaces := strings.Repeat(" ", clamp(m.ssX, 0, max(1, len(row)-tw)))
		lines[m.ssY] = spaces + stName.Render(m.ssText)
	}
	return strings.Join(lines, "\n")
}

func (m *model) stepScreensaver() {
	tw := stripWidth(m.ssText)
	maxX := m.width - 2 - tw - 2
	maxY := m.height - 2 - 2
	if maxX < 1 {
		maxX = 1
	}
	if maxY < 1 {
		maxY = 1
	}
	m.ssX += m.ssVX
	m.ssY += m.ssVY
	if m.ssX <= 0 || m.ssX >= maxX {
		m.ssVX = -m.ssVX
		m.ssX = clamp(m.ssX, 0, maxX)
	}
	if m.ssY <= 0 || m.ssY >= maxY {
		m.ssVY = -m.ssVY
		m.ssY = clamp(m.ssY, 0, maxY)
	}
}

// --- helpers ---

func themeNames() []string {
	out := make([]string, len(themes))
	for i, t := range themes {
		out[i] = t.name
	}
	return out
}

func linksBlock(githubFirst, w int) string {
	// the full linkedin address wraps on narrow panes; its path alone fits
	li := linkedin
	if 12+len(li) > w {
		li = strings.TrimPrefix(li, "linkedin.com/")
	}
	if githubFirst == 1 {
		return "  " + strings.Join([]string{
			"email     " + link("mailto:"+email, stBody.Render(email)),
			"github    " + link("https://"+github, stBody.Render(github)),
			"linkedin  " + link("https://"+linkedin, stBody.Render(li)),
			"x         " + link("https://"+xHandle, stBody.Render(xHandle)),
		}, "\n  ")
	}
	return "  " + strings.Join([]string{
		"github    " + link("https://"+github, stBody.Render(github)),
		"linkedin  " + link("https://"+linkedin, stBody.Render(li)),
		"x         " + link("https://"+xHandle, stBody.Render(xHandle)),
		"email     " + link("mailto:"+email, stBody.Render(email)),
	}, "\n  ")
}

func countLines(s string) int { return strings.Count(s, "\n") + 1 }

// bullet renders one bullet point with continuation lines hanging-indented
// under the text, not under the marker.
func bullet(text string, w int) string {
	lines := wrapLine(text, max(w-4, 10))
	var out []string
	for i, l := range lines {
		if i == 0 {
			out = append(out, "  "+stBullet.Render("▸ ")+stBody.Render(l))
		} else {
			out = append(out, "    "+stBody.Render(l))
		}
	}
	return strings.Join(out, "\n")
}

// joinWrapped lays pre-styled items out with a separator, wrapping by their
// plain widths.
func joinWrapped(items []string, widths []int, sep string, sepW, w int) string {
	var lines []string
	cur, curW := "", 0
	for i, it := range items {
		if curW > 0 && curW+sepW+widths[i] > w {
			lines = append(lines, cur)
			cur, curW = "", 0
		}
		if curW > 0 {
			cur += sep
			curW += sepW
		}
		cur += it
		curW += widths[i]
	}
	return strings.Join(append(lines, cur), "\n")
}

// chunkWidth hard-wraps s into lines of at most w columns.
func chunkWidth(s string, w int) []string {
	var lines []string
	cur, curW := "", 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if curW+rw > w {
			lines = append(lines, cur)
			cur, curW = "", 0
		}
		cur += string(r)
		curW += rw
	}
	return append(lines, cur)
}

func clockNow() string { return time.Now().Format("15:04:05") }

func stripWidth(s string) int { return lipgloss.Width(s) }

func padRight(s string, w int) string {
	d := w - stripWidth(s)
	if d <= 0 {
		return s
	}
	return s + strings.Repeat(" ", d)
}

func vline(h int) string { return strings.TrimRight(strings.Repeat("│\n", max(h, 1)), "\n") }

func hline(w int) string { return strings.Repeat("─", max(w, 1)) }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func wrap(s string, w int) string {
	if w < 10 {
		return s
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		out = append(out, wrapLine(para, w)...)
	}
	return strings.Join(out, "\n")
}

func wrapLine(s string, w int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := words[0]
	for _, word := range words[1:] {
		if lipgloss.Width(cur)+1+lipgloss.Width(word) > w {
			lines = append(lines, cur)
			cur = word
		} else {
			cur += " " + word
		}
	}
	return append(lines, cur)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
