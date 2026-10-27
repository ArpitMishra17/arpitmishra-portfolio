package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

func TestWindowSizeClamp(t *testing.T) {
	cases := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"default-ish", 120, 40, 120, 40},
		{"ultra-wide", 250, 70, 250, 70},
		{"huge", 20000, 20000, maxCols, maxRows},
		{"bigger", 60000, 60000, maxCols, maxRows},
		{"negative", -5, -1, 0, 0},
		{"wide only", 1 << 30, 30, maxCols, 30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newModel(1, termenv.Ascii)
			got, _ := m.Update(tea.WindowSizeMsg{Width: c.w, Height: c.h})
			mm := got.(model)
			if mm.width != c.wantW || mm.height != c.wantH {
				t.Errorf("size %dx%d -> %dx%d, want %dx%d", c.w, c.h, mm.width, mm.height, c.wantW, c.wantH)
			}
		})
	}
}

func TestHugeWindowRendersAtClampedSize(t *testing.T) {
	m := newModel(1, termenv.Ascii)
	m.booting = false
	got, _ := m.Update(tea.WindowSizeMsg{Width: 20000, Height: 20000})
	out := got.(model).View()
	if lines := strings.Count(out, "\n") + 1; lines > maxRows {
		t.Errorf("frame has %d lines at a huge size, want at most %d", lines, maxRows)
	}
}
