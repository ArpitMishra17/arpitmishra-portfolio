package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newGuestbookTest points the guestbook at a temp file, resets the note rate
// limits (package-level state) and takes over the clock.
func newGuestbookTest(t *testing.T) (path string, clock *time.Time) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "guestbook.jsonl")
	t.Setenv("GUESTBOOK_PATH", path)
	noteAllLimit = newHitLimiter(noteHourMax, time.Hour)
	noteIPLimit = newHitLimiter(noteIPHourMax, time.Hour)
	return path, useFakeClock(t)
}

func lineCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "\n")
}

func TestSaveNoteWritesLine(t *testing.T) {
	path, _ := newGuestbookTest(t)
	if err := saveNote("ada", 7, "10.0.0.1", "hello there"); err != nil {
		t.Fatalf("saveNote: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"note":"hello there"`) || !strings.Contains(string(b), `"name":"ada"`) {
		t.Errorf("unexpected guestbook line: %s", b)
	}
}

func TestSaveNoteSizeCap(t *testing.T) {
	path, _ := newGuestbookTest(t)
	if err := os.WriteFile(path, make([]byte, guestbookMaxBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveNote("ada", 1, "10.0.0.1", "one more"); err != errGuestbookFull {
		t.Fatalf("saveNote at the cap: err = %v, want errGuestbookFull", err)
	}
	if fi, _ := os.Stat(path); fi.Size() != guestbookMaxBytes {
		t.Errorf("file grew past the cap: %d bytes", fi.Size())
	}

	// one byte under the cap, a note still lands (the file may pass the cap by one note)
	if err := os.WriteFile(path, make([]byte, guestbookMaxBytes-1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveNote("ada", 1, "10.0.0.1", "just under"); err != nil {
		t.Errorf("saveNote just under the cap: %v", err)
	}
}

func TestSaveNoteGlobalRate(t *testing.T) {
	path, clock := newGuestbookTest(t)
	for i := 0; i < noteHourMax; i++ {
		// a new address each time, so only the global cap can refuse
		ip := fmt.Sprintf("10.1.%d.%d", i/256, i%256)
		if err := saveNote("guest", i, ip, "hi"); err != nil {
			t.Fatalf("note %d refused under the global cap: %v", i, err)
		}
	}
	if err := saveNote("guest", 0, "10.2.0.1", "hi"); err != errNoteLimited {
		t.Fatalf("note over the global cap: err = %v, want errNoteLimited", err)
	}
	if n := lineCount(t, path); n != noteHourMax {
		t.Errorf("refused note was written: %d lines, want %d", n, noteHourMax)
	}
	*clock = clock.Add(time.Hour + time.Second)
	if err := saveNote("guest", 0, "10.2.0.1", "hi"); err != nil {
		t.Errorf("note refused after the hour passed: %v", err)
	}
}

func TestSaveNoteRefusedByGlobalCapSpendsNoIPAllowance(t *testing.T) {
	_, clock := newGuestbookTest(t)
	for i := 0; i < noteHourMax; i++ {
		ip := fmt.Sprintf("10.3.%d.%d", i/256, i%256)
		if err := saveNote("guest", i, ip, "hi"); err != nil {
			t.Fatalf("note %d refused under the global cap: %v", i, err)
		}
	}
	// refused by the global cap, so the address must not be charged for them
	for i := 0; i < noteIPHourMax+2; i++ {
		if err := saveNote("ada", 9, "10.4.0.1", "hi"); err != errNoteLimited {
			t.Fatalf("note %d over the global cap: err = %v, want errNoteLimited", i, err)
		}
	}
	*clock = clock.Add(time.Hour + time.Second)
	for i := 0; i < noteIPHourMax; i++ {
		if err := saveNote("ada", i, "10.4.0.1", "hi"); err != nil {
			t.Fatalf("note %d from the address refused after refusals it didn't cause: %v", i, err)
		}
	}
}

func TestSaveNotePerIPRate(t *testing.T) {
	path, _ := newGuestbookTest(t)
	for i := 0; i < noteIPHourMax; i++ {
		if err := saveNote("ada", i, "10.0.0.1", "hi"); err != nil {
			t.Fatalf("note %d from one IP refused: %v", i, err)
		}
	}
	if err := saveNote("ada", 9, "10.0.0.1", "hi"); err != errNoteLimited {
		t.Fatalf("note over the per-IP cap: err = %v, want errNoteLimited", err)
	}
	if err := saveNote("bob", 10, "10.0.0.2", "hi"); err != nil {
		t.Errorf("other IP refused because of the first: %v", err)
	}
	if n := lineCount(t, path); n != noteIPHourMax+1 {
		t.Errorf("file has %d lines, want %d", n, noteIPHourMax+1)
	}
}
