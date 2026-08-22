package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
)

//go:embed content/styles/dark.json
var darkStyleJSON []byte

// theme is a named palette; the active one is applied to the cXxx/stXxx vars.
type theme struct {
	name    string
	accent  string // primary — headings, name, selection
	accent2 string // secondary — section labels, bullets
	green   string // cursor, live dot
	text    string
	dim     string
	faint   string // borders, hairlines
	sel     string // selected row background
}

var themes = []theme{
	{name: "arpit/site", accent: "#66D0BC", accent2: "#3A8B95", green: "#4ade80", text: "#c8c8cc", dim: "#8b8b93", faint: "#3f3f47", sel: "#1e1e24"},
	{name: "catppuccin/frappe", accent: "#ca9ee6", accent2: "#99d1db", green: "#a6d189", text: "#c6d0f5", dim: "#838ba7", faint: "#51576d", sel: "#292c3c"},
	{name: "catppuccin/latte", accent: "#8839ef", accent2: "#04a5e5", green: "#40a02b", text: "#4c4f69", dim: "#7c7f93", faint: "#9ca0b5", sel: "#e6e9ef"},
	{name: "catppuccin/macchiato", accent: "#c6a0f6", accent2: "#8aadf4", green: "#a6da95", text: "#cad3f5", dim: "#8087a2", faint: "#494d64", sel: "#363a4f"},
	{name: "catppuccin/mocha", accent: "#cba6f7", accent2: "#89b4fa", green: "#a6e3a1", text: "#cdd6f4", dim: "#7f849c", faint: "#45475a", sel: "#313244"},
	{name: "dracula", accent: "#bd93f9", accent2: "#8be9fd", green: "#50fa7b", text: "#f8f8f2", dim: "#a9b1d6", faint: "#44475a", sel: "#3b3d51"},
	{name: "gruvbox/dark", accent: "#fabd2f", accent2: "#83a598", green: "#b8bb26", text: "#ebdbb2", dim: "#928374", faint: "#504945", sel: "#3c3836"},
	{name: "everforest", accent: "#a7c080", accent2: "#7fbbb3", green: "#83c092", text: "#d3c6aa", dim: "#9da9a0", faint: "#4f585e", sel: "#3a464c"},
	{name: "kanagawa", accent: "#7e9cd8", accent2: "#957fb8", green: "#98bb6c", text: "#dcd7ba", dim: "#9cabca", faint: "#363646", sel: "#2a2a37"},
	{name: "monokai/pro", accent: "#66d9ef", accent2: "#fd971f", green: "#a6e22e", text: "#f8f8f2", dim: "#75715e", faint: "#413f35", sel: "#3e3d32"},
	{name: "nord", accent: "#88c0d0", accent2: "#81a1c1", green: "#a3be8c", text: "#eceff4", dim: "#7b88a1", faint: "#4c566a", sel: "#3b4252"},
	{name: "one/dark", accent: "#61afef", accent2: "#c678dd", green: "#98c379", text: "#abb2bf", dim: "#5c6370", faint: "#3e4451", sel: "#2c313c"},
	{name: "rose/pine", accent: "#ebbcba", accent2: "#c4a7e7", green: "#9ccfd8", text: "#e0def4", dim: "#908caa", faint: "#403d52", sel: "#26233a"},
	{name: "tokyo/night", accent: "#7aa2f7", accent2: "#7dcfff", green: "#9ece6a", text: "#c0caf5", dim: "#9aa5ce", faint: "#3b4261", sel: "#292e42"},
}

// active theme name, applied at startup and when the picker changes it.
var activeTheme = themes[0].name

// activeColors mirrors the applied theme as raw hex strings so the glamour
// markdown style can be derived from it.
var activeColors = themes[0]

// Palette vars — everything reads these at render time, so applyTheme()
// only needs to reassign them plus the derived styles below.
var (
	cAccent  = lipgloss.Color(themes[0].accent)
	cAccent2 = lipgloss.Color(themes[0].accent2)
	cGreen   = lipgloss.Color(themes[0].green)
	cText    = lipgloss.Color(themes[0].text)
	cDim     = lipgloss.Color(themes[0].dim)
	cFaint   = lipgloss.Color(themes[0].faint)
	cSel     = lipgloss.Color(themes[0].sel)
)

var (
	stTagline = lipgloss.NewStyle().Foreground(cDim)
	stNavItem = lipgloss.NewStyle().Foreground(cDim)
	stNavNum  = lipgloss.NewStyle().Foreground(cFaint)
	stCursor  = lipgloss.NewStyle().Foreground(cGreen)
	stName    = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	stTitle   = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	stSection = lipgloss.NewStyle().Foreground(cAccent2)
	stBody    = lipgloss.NewStyle().Foreground(cText)
	stDim     = lipgloss.NewStyle().Foreground(cDim)
	stFaint   = lipgloss.NewStyle().Foreground(cFaint)
	stFooter  = lipgloss.NewStyle().Foreground(cDim)
	stKeycap  = lipgloss.NewStyle().Foreground(cText)
	stOnline  = lipgloss.NewStyle().Foreground(cGreen)
	stClock   = lipgloss.NewStyle().Foreground(cDim)
	stSelRow  = lipgloss.NewStyle().Background(cSel).Foreground(cAccent).Bold(true)
	stBullet  = lipgloss.NewStyle().Foreground(cAccent2)
)

// applyTheme swaps the global palette by name.
func applyTheme(name string) {
	for _, t := range themes {
		if t.name == name {
			activeTheme = t.name
			activeColors = t
			setColors(t)
			return
		}
	}
}

// setColors assigns the color vars and rebuilds the derived styles.
func setColors(t theme) {
	cAccent = lipgloss.Color(t.accent)
	cAccent2 = lipgloss.Color(t.accent2)
	cGreen = lipgloss.Color(t.green)
	cText = lipgloss.Color(t.text)
	cDim = lipgloss.Color(t.dim)
	cFaint = lipgloss.Color(t.faint)
	cSel = lipgloss.Color(t.sel)
	rebuildStyles()
}

// dimmedTheme mixes every color ~60% toward near-black, approximating the
// dimmed-backdrop look while keeping the theme's hue visible behind dialogs.
func dimmedTheme(t theme) theme {
	d := func(hex string) string { return hexLerp(hex, "#0b0b10", 0.6) }
	return theme{
		name: t.name, accent: d(t.accent), accent2: d(t.accent2), green: d(t.green),
		text: d(t.text), dim: d(t.dim), faint: d(t.faint), sel: d(t.sel),
	}
}

// hexLerp interpolates between two hex colors; t=0 → a, t=1 → b.
func hexLerp(a, b string, t float64) string {
	var ar, ag, ab, br, bg, bb int
	fmt.Sscanf(a, "#%02x%02x%02x", &ar, &ag, &ab)
	fmt.Sscanf(b, "#%02x%02x%02x", &br, &bg, &bb)
	mix := func(x, y int) int { return int(float64(x) + t*float64(y-x)) }
	return fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

// rebuildStyles re-derives every style var after the color vars change.
func rebuildStyles() {
	stTagline = lipgloss.NewStyle().Foreground(cDim)
	stNavItem = lipgloss.NewStyle().Foreground(cDim)
	stNavNum = lipgloss.NewStyle().Foreground(cFaint)
	stCursor = lipgloss.NewStyle().Foreground(cGreen)
	stName = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	stTitle = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	stSection = lipgloss.NewStyle().Foreground(cAccent2)
	stBody = lipgloss.NewStyle().Foreground(cText)
	stDim = lipgloss.NewStyle().Foreground(cDim)
	stFaint = lipgloss.NewStyle().Foreground(cFaint)
	stFooter = lipgloss.NewStyle().Foreground(cDim)
	stKeycap = lipgloss.NewStyle().Foreground(cText)
	stOnline = lipgloss.NewStyle().Foreground(cGreen)
	stClock = lipgloss.NewStyle().Foreground(cDim)
	stSelRow = lipgloss.NewStyle().Background(cSel).Foreground(cAccent).Bold(true)
	stBullet = lipgloss.NewStyle().Foreground(cAccent2)
}

// link wraps text in an OSC-8 hyperlink — clickable in Ghostty, iTerm, Kitty…
// and degrades to plain text everywhere else.
func link(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

var spinnerFrames = []string{"◉", "◎", "○", "◎"}

// eqBars renders a tiny animated equalizer, phase driven by the app ticker.
func eqBars(tick, width int) string {
	levels := []int{3, 6, 2, 5, 1, 4}
	blocks := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	var b string
	for i := 0; i < width; i++ {
		h := levels[(i+tick/2)%len(levels)]
		b += lipgloss.NewStyle().Foreground(cAccent).Render(blocks[clamp(h, 0, len(blocks)-1)])
	}
	return b
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]|\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)")

// blogStyle derives a glamour markdown style from the active palette so the
// blog post follows the theme like every other surface. It starts from the
// stock dark.json (for margins/formatting) and recolors the key elements.
func blogStyle() ansi.StyleConfig {
	c := activeColors
	sp := func(s string) *string { return &s }

	var cfg ansi.StyleConfig
	if err := json.Unmarshal(darkStyleJSON, &cfg); err != nil {
		cfg = ansi.StyleConfig{}
	}
	cfg.Document.Color = sp(c.text)
	cfg.Text.Color = sp(c.text)
	cfg.H1.Color = sp(c.accent)
	cfg.H2.Color = sp(c.accent)
	cfg.H3.Color = sp(c.accent)
	cfg.H4.Color = sp(c.accent)
	cfg.Link.Color = sp(c.accent2)
	cfg.LinkText.Color = sp(c.accent2)
	cfg.Code.Color = sp(c.accent)
	cfg.CodeBlock.Color = sp(c.text)
	cfg.BlockQuote.Color = sp(c.dim)
	cfg.Emph.Color = sp(c.dim)
	cfg.Table.Color = sp(c.text)
	cfg.Item.Color = sp(c.text)
	cfg.HorizontalRule.Color = sp(c.faint)
	return cfg
}

// stripANSI removes color/hyperlink escapes, leaving plain text — used to
// dim the app behind an open dialog.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// dialog renders an opencode-style modal: rounded border with an inline
// title, content lines (pre-styled), and a keycap footer bar below the box.
// Total width is innerW + 4.
func dialog(title string, content []string, footer string, innerW int) string {
	// top border with the title inlaid: ╭─ title ──────╮
	tLen := len([]rune(title))
	top := "╭─ " + title + " " + strings.Repeat("─", max(innerW-tLen-1, 1)) + "╮"

	var lines []string
	side := lipgloss.NewStyle().Foreground(cFaint).Render
	lines = append(lines, side(top))
	for _, c := range content {
		lines = append(lines, side("│")+" "+c+strings.Repeat(" ", max(innerW-stripWidth(c), 0))+" "+side("│"))
	}
	lines = append(lines,
		side("╰"+strings.Repeat("─", innerW+2)+"╯"),
		"",
		footer)
	return strings.Join(lines, "\n")
}

// keycaps renders a footer hint like "↑↓←→ navigate  esc close".
func keycaps(pairs ...[2]string) string {
	var parts []string
	for _, p := range pairs {
		parts = append(parts, stKeycap.Render(p[0])+stFooter.Render(" "+p[1]))
	}
	return strings.Join(parts, stFooter.Render("   "))
}
