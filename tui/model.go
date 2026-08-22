package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type section int

const (
	secOverview section = iota
	secExperience
	secProjects
	secStack
	secBlog
	secContact
	secCount
)

var sectionNames = []string{"Overview", "Experience", "Projects", "Stack", "Blog", "Contact"}

// animMsg drives everything that moves: spinner, blinking cursor, clock,
// typewriter, equalizer and the screensaver. One ticker for all of it.
type animMsg time.Time

type modalKind int

const (
	modalNone modalKind = iota
	modalHelp
	modalTheme
)

const sidebarW = 26

type model struct {
	sec           section
	width, height int
	session       int

	expSel, projSel int

	blogVP    viewport.Model
	blogReady bool
	blogMD    string

	// animation state
	tick   int
	typed  int
	typing bool

	// opencode-style dialog: dimmed app behind a floating rounded box.
	// Theme selection live-previews; committedTheme is the one in effect
	// when the dialog opened (restored on esc).
	modal          modalKind
	mFilter        string
	mSel           int
	committedTheme string

	// blog markdown renders async so glamour can never block the first paint
	blogRendering  bool
	blogRequestedW int

	// screensaver easter egg ("z"): a DVD-logo style bouncing banner
	ssOn             bool
	ssX, ssY         int
	ssVX, ssVY       int
	ssText           string
}

// blogRenderedMsg carries asynchronously-rendered markdown back to the model.
type blogRenderedMsg struct {
	content string
	width   int
}

func newModel(md string, session int) model {
	vp := viewport.New(78, 20)
	return model{
		blogMD: md, blogVP: vp, typing: true, session: session,
		committedTheme: activeTheme,
		ssText: "ssh arpitmishra.dev", ssVX: 2, ssVY: 1,
	}
}

func newProgram(md string, session int) *tea.Program {
	return tea.NewProgram(newModel(md, session), tea.WithAltScreen())
}

func animTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return animMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(animTick(), tea.SetWindowTitle("arpit mishra — portfolio"))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.sizeBlogVP()
		return m, m.maybeRequestBlog()
	case blogRenderedMsg:
		m.blogRendering = false
		if msg.content != "" {
			m.blogVP.SetContent(msg.content)
			m.blogReady = true
		}
		// terminal resized while rendering — try again at the new width
		if m.blogWidth() != m.blogRequestedW {
			return m, m.maybeRequestBlog()
		}
	case animMsg:
		m.tick++
		if m.typing {
			m.typed += 2
			if m.typed >= len([]rune(bio)) {
				m.typed = len([]rune(bio))
				m.typing = false
			}
		}
		if m.ssOn {
			m.stepScreensaver()
		}
		return m, animTick()
	case tea.KeyMsg:
		if m.ssOn {
			// any key leaves the screensaver (q still quits)
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.ssOn = false
			return m, nil
		}
		if m.modal != modalNone {
			return m.updateModal(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.modal = modalHelp
		case "t":
			m.modal = modalTheme
			m.mFilter, m.mSel = "", 0
			m.committedTheme = activeTheme
		case "z":
			m.ssOn = true
			m.ssX, m.ssY = 4, 2
		case "tab", "right", "l":
			m.sec = (m.sec + 1) % secCount
		case "shift+tab", "left", "h":
			m.sec = (m.sec + secCount - 1) % secCount
		case "1", "2", "3", "4", "5", "6":
			if n := int(msg.String()[0] - '1'); n < int(secCount) {
				m.sec = section(n)
			}
		case "up", "k", "down", "j":
			down := msg.String() == "down" || msg.String() == "j"
			delta := 1
			if !down {
				delta = -1
			}
			switch m.sec {
			case secExperience:
				m.expSel = clamp(m.expSel+delta, 0, len(experiences)-1)
			case secProjects:
				m.projSel = clamp(m.projSel+delta, 0, len(projects)-1)
			case secBlog:
				if down {
					m.blogVP.LineDown(1)
				} else {
					m.blogVP.LineUp(1)
				}
			}
		case "pgdown", "pgup", "g", "G":
			if m.sec == secBlog {
				switch msg.String() {
				case "pgdown":
					m.blogVP.HalfViewDown()
				case "pgup":
					m.blogVP.HalfViewUp()
				case "g":
					m.blogVP.GotoTop()
				case "G":
					m.blogVP.GotoBottom()
				}
			}
		}
	}
	if m.sec == secBlog {
		var cmd tea.Cmd
		m.blogVP, cmd = m.blogVP.Update(msg)
		return m, cmd
	}
	return m, nil
}

// updateModal handles keys while a dialog is open. Like opencode: typing
// filters, arrows move, esc closes, enter picks. Moving through the theme
// grid live-previews each theme behind the dialog.
func (m model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.filteredThemes()
	switch msg.String() {
	case "esc":
		if m.modal == modalTheme {
			applyTheme(m.committedTheme) // revert preview
		}
		m.modal = modalNone
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		if m.modal == modalTheme && len(items) > 0 {
			m.committedTheme = items[clamp(m.mSel, 0, len(items)-1)]
			applyTheme(m.committedTheme)
			m.modal = modalNone
			// re-render the blog markdown in the new palette
			m.blogReady = false
			m.blogRequestedW = 0
			return m, m.maybeRequestBlog()
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

// previewTheme applies whatever the cursor sits on, without committing it.
func (m *model) previewTheme() {
	items := m.filteredThemes()
	if len(items) > 0 {
		applyTheme(items[clamp(m.mSel, 0, len(items)-1)])
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

func (m model) View() string {
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cFaint)

	if m.width == 0 {
		// no resize event yet — paint a default-size frame immediately
		// instead of sitting on a loading screen
		vm := m
		vm.width, vm.height = 100, 30
		vm.sizeBlogVP()
		if m.ssOn {
			return frame.Render(vm.viewScreensaver())
		}
		return vm.viewApp(frame)
	}

	if m.ssOn {
		return frame.Render(m.viewScreensaver())
	}

	app := m.viewApp(frame)
	if m.modal != modalNone {
		return m.overlay(frame, app)
	}
	return app
}

func (m model) viewApp(frame lipgloss.Style) string {
	innerW := clamp(m.width-2, 40, m.width)
	innerH := m.height - 2

	contentW := innerW - sidebarW - 5
	if contentW < 20 {
		contentW = 20
	}

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

	cursor := " "
	if m.tick%8 < 4 {
		cursor = stCursor.Render("▍")
	}

	side := lipgloss.JoinVertical(lipgloss.Left,
		"",
		stName.Render(name),
		stTagline.Render(tagline)+cursor,
		"",
		strings.Join(nav, "\n"),
		"",
		"",
		eqBars(m.tick, 14),
		stDim.Render("now coding"),
		stFaint.Render("arpit/portfolio-tui"),
		"",
		stFaint.Render("mumbai, india"),
	)
	side = lipgloss.NewStyle().Width(sidebarW).PaddingLeft(1).Render(side)

	// --- content pane ---
	var content string
	switch m.sec {
	case secOverview:
		content = m.viewOverview(contentW)
	case secExperience:
		content = m.viewExperience(contentW)
	case secProjects:
		content = m.viewProjects(contentW)
	case secStack:
		content = m.viewStack(contentW)
	case secBlog:
		content = m.viewBlog(contentW)
	case secContact:
		content = m.viewContact(contentW)
	}
	content = lipgloss.NewStyle().
		Width(contentW).Height(innerH - 2).
		Padding(0, 2).
		Render(content)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		side,
		stFaint.Render(vline(innerH-1)),
		content,
	)

	// --- status bar ---
	spinner := spinnerFrames[(m.tick/2)%len(spinnerFrames)]
	left := "  " + m.footerHint()
	right := stOnline.Render(spinner) + " " + stOnline.Render("live") +
		"  " + stDim.Render(fmt.Sprintf("session #%d", m.session)) +
		"  " + stDim.Render(uptime()) +
		"  " + stClock.Render(clockNow()) + " "
	bar := lipgloss.JoinHorizontal(lipgloss.Bottom,
		lipgloss.NewStyle().Width(innerW-stripWidth(right)).MaxWidth(innerW).Render(left),
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
	}
	if dlgStr == "" {
		return m.viewApp(frame)
	}

	// render the backdrop with dimmed theme colors, then restore
	full := activeColors
	setColors(dimmedTheme(full))
	app := m.viewApp(frame)
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
	if m.tick%8 < 4 {
		cur = stCursor.Render("▍")
	}
	filterRow := stTitle.Render("› ")+stBody.Render(m.mFilter)+cur +
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
		{"1 – 6", "jump to section"},
		{"g / G", "blog top / bottom"},
		{"t", "change theme"},
		{"z", "???"},
		{"q", "quit"},
	}
	var content []string
	content = append(content, "")
	for _, r := range rows {
		content = append(content, "  "+stKeycap.Render(fmt.Sprintf("%-11s", r[0]))+stDim.Render(r[1]))
	}
	content = append(content, "", "  "+stFaint.Render("psst — try z"))
	return dialog("keys", content,
		keycaps(
			[2]string{"esc", "close"},
			[2]string{"?", "toggle"},
		), innerW+4)
}

func (m model) footerHint() string {
	key := func(k, s string) string {
		return stKeycap.Render(k) + stFooter.Render(" "+s+"  ")
	}
	switch {
	case m.modal != modalNone:
		return key("esc", "close")
	case m.sec == secBlog:
		return key("↑↓", "scroll") + key("g/G", "top/bot") + key("⇤⇥", "section") + key("t", "themes") + key("?", "help") + key("q", "quit")
	default:
		return key("↑↓", "select") + key("⇤⇥", "section") + key("t", "themes") + key("?", "help") + key("q", "quit")
	}
}

func (m model) viewOverview(w int) string {
	shown := string([]rune(bio)[:clamp(m.typed, 0, len([]rune(bio)))])
	var b strings.Builder
	b.WriteString(stTitle.Render("Hey, I'm Arpit.") + "\n\n")
	b.WriteString(wrap(stBody.Render(shown), w-2) + "\n\n")
	b.WriteString(stSection.Render("links") + "\n")
	b.WriteString(linksBlock(2) + "\n")
	b.WriteString(stFaint.Render("  ⌘/ctrl-click a link to open it"))
	return b.String()
}

func (m model) viewExperience(w int) string {
	var b strings.Builder
	b.WriteString(stTitle.Render("Experience") + stDim.Render("  — 8 roles") + "\n\n")

	if w >= 76 {
		const listW = 30
		list := m.expList(listW - 2)
		e := experiences[m.expSel]
		det := new(strings.Builder)
		det.WriteString(stName.Render(e.role) + "\n")
		det.WriteString(stSection.Render(e.company) + "  " + stDim.Render(e.period) + "\n\n")
		for _, p := range e.points {
			det.WriteString(bullet(p, w-listW-8) + "\n\n")
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(listW).Render(list),
			stFaint.Render(vline(countLines(det.String()))),
			lipgloss.NewStyle().Width(w - listW - 2).PaddingLeft(2).Render(det.String()),
		))
		return b.String()
	}

	for i, e := range experiences {
		if i == m.expSel {
			b.WriteString("  " + stCursor.Render("▍") +
				stSelRow.Render(" "+e.role+" ") +
				stDim.Render("  ·  " + e.company) + "\n")
			b.WriteString("    " + stDim.Render(e.period) + "\n")
		} else {
			b.WriteString("    " + stBody.Render(e.role) + stDim.Render("  ·  " + e.company) + "\n")
		}
	}
	return b.String()
}

func (m model) expList(w int) string {
	var lines []string
	for i, e := range experiences {
		if i == m.expSel {
			lines = append(lines, stCursor.Render("▍")+stSelRow.Render(" "+e.company+" "))
		} else {
			lines = append(lines, "  "+stBody.Render(e.company))
		}
	}
	return lipgloss.NewStyle().Width(w).Render(strings.Join(lines, "\n"))
}

func (m model) viewProjects(w int) string {
	var b strings.Builder
	b.WriteString(stTitle.Render("Projects") + stDim.Render("  — 4 builds") + "\n\n")

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
		det := new(strings.Builder)
		det.WriteString(stName.Render(p.name) + "\n")
		det.WriteString(stSection.Render(p.stack) + "\n\n")
		for _, pt := range p.points {
			det.WriteString(bullet(pt, w-listW-8) + "\n\n")
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(listW).Render(strings.Join(list, "\n")),
			stFaint.Render(vline(countLines(det.String()))),
			lipgloss.NewStyle().Width(w - listW - 2).PaddingLeft(2).Render(det.String()),
		))
		return b.String()
	}

	for i, p := range projects {
		if i == m.projSel {
			b.WriteString("  " + stCursor.Render("▍") +
				stSelRow.Render(" "+p.name+" ") +
				stDim.Render("  " + p.stack) + "\n")
			for _, pt := range p.points {
				b.WriteString(bullet(pt, w-6) + "\n")
			}
		} else {
			b.WriteString("    " + stBody.Render(p.name) + stDim.Render("  " + p.stack) + "\n")
		}
	}
	return b.String()
}

func (m model) viewStack(w int) string {
	var b strings.Builder
	b.WriteString(stTitle.Render("Stack") + "\n\n")
	for _, g := range techGroups {
		b.WriteString(stSection.Render(g.name) + "\n")
		b.WriteString(wrap(stBody.Render(strings.Join(g.items, "  ·  ")), w-2) + "\n\n")
	}
	return b.String()
}

func (m model) viewBlog(w int) string {
	if !m.blogReady {
		return stDim.Render("loading post…")
	}
	p := posts[0]
	header := stTitle.Render(p.title) + "\n" + stDim.Render(p.date) + "\n\n"
	return header + m.blogVP.View()
}

func (m model) viewContact(w int) string {
	var b strings.Builder
	b.WriteString(stTitle.Render("Contact") + "\n\n")
	b.WriteString(wrap(stBody.Render("Best way to reach me is email — I usually reply within a day."), w-2) + "\n\n")
	b.WriteString(linksBlock(1) + "\n")
	b.WriteString(stFaint.Render("  ⌘/ctrl-click a link to open it"))
	return b.String()
}

func (m model) viewScreensaver() string {
	w, h := m.width-2, m.height-2
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

func (m *model) sizeBlogVP() {
	if m.width == 0 {
		return
	}
	m.blogVP.Width = max(m.blogWidth(), 20)
	m.blogVP.Height = max(m.height-10, 5)
}

func (m model) blogWidth() int {
	return max(m.width-sidebarW-7, 20)
}

// maybeRequestBlog kicks off an async glamour render if the markdown hasn't
// been rendered at the current width yet. Rendering off the update loop is
// what keeps startup instant — glamour's style setup can probe the terminal
// and block, which used to freeze the first frame.
func (m *model) maybeRequestBlog() tea.Cmd {
	if m.blogRendering || m.blogMD == "" {
		return nil
	}
	w := m.blogWidth()
	if w == m.blogRequestedW && m.blogReady {
		return nil
	}
	m.blogRequestedW = w
	m.blogRendering = true
	md := m.blogMD
	return func() tea.Msg {
		// styled from the active palette — the blog follows the theme
		if r, err := glamour.NewTermRenderer(
			glamour.WithWordWrap(w),
			glamour.WithStyles(blogStyle()),
		); err == nil {
			if out, err := r.Render(md); err == nil {
				return blogRenderedMsg{content: out, width: w}
			}
		}
		return blogRenderedMsg{width: w}
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

func mustLoadBlog() string {
	if len(posts) == 0 {
		return ""
	}
	return loadPost(posts[0])
}

func linksBlock(githubFirst int) string {
	if githubFirst == 1 {
		return "  " + strings.Join([]string{
			"email     " + link("mailto:"+email, stBody.Render(email)),
			"github    " + link("https://"+github, stBody.Render(github)),
			"linkedin  " + link("https://"+linkedin, stBody.Render(linkedin)),
			"x         " + link("https://"+xHandle, stBody.Render(xHandle)),
		}, "\n  ")
	}
	return "  " + strings.Join([]string{
		"github    " + link("https://"+github, stBody.Render(github)),
		"linkedin  " + link("https://"+linkedin, stBody.Render(linkedin)),
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
