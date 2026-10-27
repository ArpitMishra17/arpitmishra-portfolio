package main

import (
	"strings"
	"testing"
	"time"

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

// quits runs cmd and reports whether it produced tea.Quit's message.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestIdleQuit(t *testing.T) {
	m := newModel(1, termenv.Ascii)
	m.booting = false
	m.lastInputAt = time.Now().Add(-(idleQuit + time.Minute))
	_, cmd := m.Update(animMsg(time.Now()))
	if !quits(cmd) {
		t.Error("model idle past idleQuit did not quit")
	}
}

func TestFreshModelDoesNotQuit(t *testing.T) {
	m := newModel(1, termenv.Ascii)
	m.booting = false
	_, cmd := m.Update(animMsg(time.Now()))
	if quits(cmd) {
		t.Error("fresh model quit on its first frame")
	}
}

func TestKeyResetsIdleQuit(t *testing.T) {
	m := newModel(1, termenv.Ascii)
	m.booting = false
	m.lastInputAt = time.Now().Add(-(idleQuit + time.Minute))
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	_, cmd := got.(model).Update(animMsg(time.Now()))
	if quits(cmd) {
		t.Error("model quit right after a key press")
	}
}

func TestScreensaverStillStartsBeforeIdleQuit(t *testing.T) {
	m := newModel(1, termenv.Ascii)
	m.booting = false
	m.lastInputAt = time.Now().Add(-(idleSaver + time.Minute))
	got, cmd := m.Update(animMsg(time.Now()))
	if quits(cmd) {
		t.Fatal("quit before idleQuit")
	}
	if !got.(model).ssOn {
		t.Error("screensaver did not start after idleSaver")
	}
	if idleQuit <= idleSaver || idleQuit <= idleFreeze {
		t.Error("idleQuit must be longer than idleSaver and idleFreeze")
	}
}

func TestNoteUsesVisitorIP(t *testing.T) {
	newGuestbookTest(t)
	m := newModel(1, termenv.Ascii)
	m.booting = false
	m.ip = "10.9.9.9"
	m.modal, m.note = modalNote, "hi"
	// use up this address's notes, so the send must be refused for its IP
	for i := 0; i < noteIPHourMax; i++ {
		noteIPLimit.allow("10.9.9.9", now())
	}
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := got.(model)
	if mm.noteSent || !mm.noteErr {
		t.Errorf("noteSent=%v noteErr=%v, want refused by the IP limit", mm.noteSent, mm.noteErr)
	}
}
