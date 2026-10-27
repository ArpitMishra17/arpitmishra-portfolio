package main

import (
	"net"
	"testing"
	"time"

	"github.com/charmbracelet/ssh"
)

func TestVerdict(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("healthy", func(t *testing.T) {
		a := newActivity(t0)
		if v := a.verdict(t0.Add(time.Second)); v != "" {
			t.Errorf("healthy session dropped: %q", v)
		}
	})

	t.Run("write stalled", func(t *testing.T) {
		a := newActivity(t0)
		a.gotInput(t0.Add(writeStall)) // keep input fresh so only the write can trip
		a.beginWrite(t0.Add(writeStall))
		now := t0.Add(writeStall + writeStall + time.Second)
		if v := a.verdict(now); v != "output stalled" {
			t.Errorf("verdict = %q, want output stalled", v)
		}
		if v := a.verdict(t0.Add(writeStall + writeStall - time.Second)); v != "" {
			t.Errorf("write dropped before writeStall: %q", v)
		}
	})

	t.Run("write finished", func(t *testing.T) {
		a := newActivity(t0)
		a.gotInput(t0.Add(writeStall))
		a.beginWrite(t0)
		a.endWrite()
		if v := a.verdict(t0.Add(writeStall + time.Hour)); v == "output stalled" {
			t.Errorf("finished write still counted as stalled")
		}
	})

	t.Run("overlapping writes keep the oldest start", func(t *testing.T) {
		a := newActivity(t0)
		a.gotInput(t0.Add(writeStall))
		a.beginWrite(t0)
		a.beginWrite(t0.Add(writeStall / 2))
		a.endWrite() // the older write is still in flight
		if v := a.verdict(t0.Add(writeStall + time.Second)); v != "output stalled" {
			t.Errorf("verdict = %q, want output stalled while the older write is pending", v)
		}
	})

	t.Run("input idle", func(t *testing.T) {
		a := newActivity(t0)
		if v := a.verdict(t0.Add(inputIdle + time.Second)); v != "no input" {
			t.Errorf("verdict = %q, want no input", v)
		}
		a.gotInput(t0.Add(inputIdle))
		if v := a.verdict(t0.Add(inputIdle + time.Second)); v != "" {
			t.Errorf("session dropped after recent input: %q", v)
		}
	})
}

// fakeIO is a session whose Read and Write are plain buffers, with a look at
// the activity record during Write.
type fakeIO struct {
	ssh.Session
	in       []byte
	seen     *activity
	wrote    []byte
	seenPend int
}

func (f *fakeIO) Read(p []byte) (int, error) {
	if len(f.in) == 0 {
		return 0, nil
	}
	n := copy(p, f.in)
	f.in = f.in[n:]
	return n, nil
}

func (f *fakeIO) Write(p []byte) (int, error) {
	if f.seen != nil {
		f.seen.mu.Lock()
		f.seenPend = f.seen.pending
		f.seen.mu.Unlock()
	}
	f.wrote = append(f.wrote, p...)
	return len(p), nil
}

func TestScopedSessionRecordsIO(t *testing.T) {
	clock := useFakeClock(t)
	t0 := *clock
	f := &fakeIO{in: []byte("abc")}
	a := newActivity(t0)
	s := &scopedSession{Session: f, id: 3, act: a}
	var _ ssh.Session = s // still a session for bubbletea and wish

	*clock = t0.Add(5 * time.Second)
	buf := make([]byte, 8)
	if n, _ := s.Read(buf); n != 3 {
		t.Fatalf("Read = %d bytes, want 3", n)
	}
	if !a.lastInput.Equal(*clock) {
		t.Errorf("lastInput = %v after input, want %v", a.lastInput, *clock)
	}

	// an empty read is not input
	*clock = t0.Add(10 * time.Second)
	f.in = nil
	s.Read(buf)
	if !a.lastInput.Equal(t0.Add(5 * time.Second)) {
		t.Errorf("empty read moved lastInput to %v", a.lastInput)
	}

	// a write is pending while it runs, and finished after
	f.seen = a
	if _, err := s.Write([]byte("frame")); err != nil {
		t.Fatal(err)
	}
	if f.seenPend != 1 {
		t.Errorf("pending during Write = %d, want 1", f.seenPend)
	}
	if string(f.wrote) != "frame" {
		t.Errorf("Write reached the session as %q", f.wrote)
	}
	if a.pending != 0 {
		t.Errorf("pending after Write = %d, want 0", a.pending)
	}
	if s.id != 3 {
		t.Errorf("id = %d", s.id)
	}
}

func TestWatchdogClosesStalledWriter(t *testing.T) {
	oldTick, oldStall := tick, writeStall
	tick, writeStall = 2*time.Millisecond, 30*time.Millisecond
	t.Cleanup(func() { tick, writeStall = oldTick, oldStall })

	// a client that has stopped reading: Write blocks until the connection closes
	conn := newCloseRecorder()
	sw := &fakeSession{pty: true, stuck: true, addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 1}, conn: conn}

	before := hub.count()
	handlerDone := make(chan struct{})
	sessionScope()(func(s ssh.Session) {
		defer close(handlerDone)
		s.Write([]byte("frame")) // blocks until the watchdog closes the connection
	})(sw)
	select {
	case <-handlerDone:
	default:
		t.Fatal("handler still blocked after the watchdog ran")
	}
	select {
	case <-conn.closed:
	default:
		t.Fatal("watchdog did not close the connection")
	}
	if n := hub.count(); n != before {
		t.Errorf("hub holds %d visitors after the session, want %d", n, before)
	}
}

func TestSessionScopeJoinsAndLeavesHub(t *testing.T) {
	useFakeClock(t)
	before := hub.count()
	var inside int
	var gotID int
	h := sessionScope()(func(s ssh.Session) {
		inside = hub.count()
		gotID = sessionID(s)
	})
	h(&fakeSession{pty: true, addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.6"), Port: 1}})
	if inside != before+1 {
		t.Errorf("hub count during the session = %d, want %d", inside, before+1)
	}
	if gotID == 0 {
		t.Error("handler got session id 0")
	}
	if n := hub.count(); n != before {
		t.Errorf("hub count after the session = %d, want %d", n, before)
	}
}

func TestSessionScopeLeavesHubOnPanic(t *testing.T) {
	useFakeClock(t)
	before := hub.count()
	h := sessionScope()(func(ssh.Session) { panic("boom") })
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("panic was swallowed by sessionScope")
			}
		}()
		h(&fakeSession{pty: true, addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.7"), Port: 1}})
	}()
	if n := hub.count(); n != before {
		t.Errorf("hub count after a panicking session = %d, want %d", n, before)
	}
}

func TestSessionScopeKeepsNonPtyOffHub(t *testing.T) {
	useFakeClock(t)
	before := hub.count()
	var inside int
	h := sessionScope()(func(ssh.Session) { inside = hub.count() })
	h(&fakeSession{pty: false, addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.8"), Port: 1}})
	if inside != before {
		t.Errorf("non-pty session joined the hub: count %d, want %d", inside, before)
	}
}

func TestSessionIDFallsBackWithoutScope(t *testing.T) {
	a := sessionID(&fakeSession{})
	b := sessionID(&fakeSession{})
	if a == 0 || b == 0 || a == b {
		t.Errorf("fallback ids = %d, %d; want two distinct non-zero ids", a, b)
	}
}
