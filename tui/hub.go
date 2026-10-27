package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

// visitor is someone connected right now, standing on the promenade in the
// Overview scene. Every session reads the same hub on its own frame tick, so
// people see each other move within one frame — no broadcasting needed.
type visitor struct {
	id      int
	name    string
	x       float64 // 0–1 across the promenade, so any terminal width works
	movedAt time.Time
}

type visitorHub struct {
	mu sync.Mutex
	vs map[int]*visitor
}

var hub = &visitorHub{vs: map[int]*visitor{}}

func (h *visitorHub) join(id int, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.vs[id] = &visitor{id: id, name: name, x: 0.12 + rand3(id, 0, 141)*0.76}
}

func (h *visitorHub) leave(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.vs, id)
}

func (h *visitorHub) walk(id int, dx float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if v, ok := h.vs[id]; ok {
		v.x = clamp01(v.x + dx)
		v.movedAt = time.Now()
	}
}

func (h *visitorHub) list() []visitor {
	h.mu.Lock()
	out := make([]visitor, 0, len(h.vs))
	for _, v := range h.vs {
		out = append(out, *v)
	}
	h.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func (h *visitorHub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.vs)
}

func (h *visitorHub) name(id int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if v, ok := h.vs[id]; ok {
		return v.name
	}
	return "guest"
}

// visitorName turns an ssh username into a safe name tag: other visitors see
// it, so only plain ASCII survives (no escape sequences, no wide glyphs).
func visitorName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.') {
			b.WriteRune(r)
		}
		if b.Len() >= 12 {
			break
		}
	}
	if b.Len() == 0 {
		return "guest"
	}
	return b.String()
}

// --- guestbook ---

// Notes are appended to a private JSONL file for me to read; they are never
// shown to other visitors, so there's nothing to moderate.
var (
	guestbookMu    sync.Mutex
	guestbookCount atomic.Int64

	// notes are capped overall and per visitor, so the file can't be flooded
	noteAllLimit = newHitLimiter(noteHourMax, time.Hour)
	noteIPLimit  = newHitLimiter(noteIPHourMax, time.Hour)
	noteRateMu   sync.Mutex // makes the two caps one check, see allowNote
)

const (
	noteMax           = 200
	guestbookMaxBytes = 2 << 20 // the file stops growing at about this size
	noteHourMax       = 100     // notes an hour from everyone together
	noteIPHourMax     = 3       // notes an hour from one address
)

var (
	errGuestbookFull = errors.New("guestbook is full")
	errNoteLimited   = errors.New("too many notes right now")
)

func guestbookPath() string {
	if p := os.Getenv("GUESTBOOK_PATH"); p != "" {
		return p
	}
	return "guestbook.jsonl"
}

// loadGuestbookCount counts existing notes once at startup.
func loadGuestbookCount() {
	f, err := os.Open(guestbookPath())
	if err != nil {
		return
	}
	defer f.Close()
	n := int64(0)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		n++
	}
	guestbookCount.Store(n)
}

// allowNote charges a note from ip to both caps, or to neither if either is full.
func allowNote(ip string, t time.Time) bool {
	noteRateMu.Lock()
	defer noteRateMu.Unlock()
	if !noteIPLimit.peek(ip, t) || !noteAllLimit.peek("", t) {
		return false
	}
	noteIPLimit.allow(ip, t)
	noteAllLimit.allow("", t)
	return true
}

// saveNote appends a note to the guestbook file, subject to the rate caps.
func saveNote(name string, session int, ip, note string) error {
	t := now()
	if !allowNote(ip, t) {
		return errNoteLimited
	}
	guestbookMu.Lock()
	defer guestbookMu.Unlock()
	f, err := os.OpenFile(guestbookPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() >= guestbookMaxBytes {
		return errGuestbookFull
	}
	b, _ := json.Marshal(map[string]any{
		"time": t.UTC().Format(time.RFC3339), "name": name, "session": session, "note": note,
	})
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	guestbookCount.Add(1)
	return nil
}

// cleanNoteRunes keeps printable characters only — pasted text can carry
// escape sequences.
func cleanNoteRunes(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		if r == '\n' || r == '\t' {
			r = ' '
		}
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
