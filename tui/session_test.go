package main

import (
	"bytes"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
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

// fakeIO is a session whose input is a pipe the test types into, and whose
// output is a buffer, with a look at the activity record during Write. onWrite,
// if set, runs after each Write with the pipe's write end, so a test can script
// the terminal's replies.
type fakeIO struct {
	ssh.Session
	in       *io.PipeReader // nil reads as EOF
	term     *io.PipeWriter
	seen     *activity
	seenPend int
	wrote    []byte
	onWrite  func(p []byte, term *io.PipeWriter)
}

func (f *fakeIO) Read(p []byte) (int, error) {
	if f.in == nil {
		return 0, io.EOF
	}
	return f.in.Read(p)
}

func (f *fakeIO) Write(p []byte) (int, error) {
	if f.seen != nil {
		f.seen.mu.Lock()
		f.seenPend = f.seen.pending
		f.seen.mu.Unlock()
	}
	f.wrote = append(f.wrote, p...)
	if f.onWrite != nil {
		f.onWrite(p, f.term)
	}
	return len(p), nil
}

// pipeSession returns a scoped session whose input is a pipe, and the pipe's
// write end, which stands for the client. Call useFakeClock first if the test
// cares about activity times.
func pipeSession(t *testing.T, onWrite func([]byte, *io.PipeWriter)) (*scopedSession, *io.PipeWriter) {
	t.Helper()
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	f := &fakeIO{in: pr, term: pw, onWrite: onWrite}
	return &scopedSession{Session: f, id: 1, act: newActivity(now())}, pw
}

// answering returns a fake terminal that replies with reply to the kitty query.
func answering(reply string) func([]byte, *io.PipeWriter) {
	return func(p []byte, term *io.PipeWriter) {
		if string(p) == kittyQuery {
			term.Write([]byte(reply))
		}
	}
}

// readUntil reads from s until want bytes have arrived or Read fails, and
// returns what came back with the error. It fails the test if Read blocks.
func readUntil(t *testing.T, s *scopedSession, want int) (string, error) {
	t.Helper()
	type result struct {
		got []byte
		err error
	}
	res := make(chan result, 1)
	go func() {
		var acc []byte
		var err error
		buf := make([]byte, 64)
		for len(acc) < want && err == nil {
			var n int
			n, err = s.Read(buf)
			acc = append(acc, buf[:n]...)
		}
		res <- result{acc, err}
	}()
	select {
	case r := <-res:
		return string(r.got), r.err
	case <-time.After(2 * time.Second):
		t.Fatal("Read blocked")
		return "", nil
	}
}

func writeClient(t *testing.T, pw *io.PipeWriter, s string) {
	t.Helper()
	if _, err := pw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
}

const kittyOKAndDA1 = "\x1b_Gi=31;OK\x1b\\\x1b[?62;22c"

func TestScopedSessionRecordsIO(t *testing.T) {
	clock := useFakeClock(t)
	t0 := *clock
	a := newActivity(t0)
	f := &fakeIO{}
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	f.in = pr
	s := &scopedSession{Session: f, id: 3, act: a}
	var _ ssh.Session = s // still a session for bubbletea and wish

	*clock = t0.Add(5 * time.Second)
	go pw.Write([]byte("abc"))
	buf := make([]byte, 8)
	if n, _ := s.Read(buf); n != 3 {
		t.Fatalf("Read = %d bytes, want 3", n)
	}
	if !a.lastInput.Equal(*clock) {
		t.Errorf("lastInput = %v after input, want %v", a.lastInput, *clock)
	}

	// an empty read is not input
	*clock = t0.Add(10 * time.Second)
	if _, err := pw.Write(nil); err != nil {
		t.Fatal(err)
	}
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

func TestProbeKittySupported(t *testing.T) {
	useFakeClock(t)
	s, _ := pipeSession(t, answering(kittyOKAndDA1))
	if !s.probeKitty(time.Second) {
		t.Fatal("supported terminal reported as unsupported")
	}
	if got := s.Session.(*fakeIO).wrote; string(got) != kittyQuery {
		t.Errorf("probe wrote %q, want the kitty query", got)
	}
}

func TestProbeKittySplitReply(t *testing.T) {
	useFakeClock(t)
	s, _ := pipeSession(t, func(p []byte, term *io.PipeWriter) {
		if string(p) == kittyQuery {
			term.Write([]byte("\x1b_Gi=31;O"))
			term.Write([]byte("K\x1b\\\x1b[?62"))
			term.Write([]byte(";22c"))
		}
	})
	if !s.probeKitty(time.Second) {
		t.Error("reply split over three reads not read as OK")
	}
}

func TestProbeKittyUnsupported(t *testing.T) {
	useFakeClock(t)
	s, _ := pipeSession(t, answering("\x1b[?62c")) // device attributes only
	start := time.Now()
	if s.probeKitty(5 * time.Second) {
		t.Error("terminal without the protocol reported as supported")
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("probe waited %v after the device attributes reply", took)
	}
}

func TestProbeKittyErrorReply(t *testing.T) {
	useFakeClock(t)
	s, _ := pipeSession(t, answering("\x1b_Gi=31;ENOENT:no such file\x1b\\\x1b[?62c"))
	if s.probeKitty(time.Second) {
		t.Error("error reply reported as supported")
	}
}

func TestProbeKittyTimeout(t *testing.T) {
	useFakeClock(t)
	s, _ := pipeSession(t, nil) // the terminal never answers
	start := time.Now()
	if s.probeKitty(30 * time.Millisecond) {
		t.Error("silent terminal reported as supported")
	}
	if took := time.Since(start); took < 30*time.Millisecond {
		t.Errorf("probe returned after %v, before its timeout", took)
	}
}

func TestProbeKittyFloodIsCapped(t *testing.T) {
	useFakeClock(t)
	s, pw := pipeSession(t, func(p []byte, term *io.PipeWriter) {
		if string(p) != kittyQuery {
			return
		}
		// keep sending junk and never answer; stops once the session closes
		go func() {
			junk := bytes.Repeat([]byte("x"), 1024)
			for {
				if _, err := term.Write(junk); err != nil {
					return
				}
			}
		}()
	})
	start := time.Now()
	if s.probeKitty(10 * time.Second) {
		t.Error("flooding client reported as supported")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("probe waited %v under a flood, want it to give up at the cap", took)
	}
	if n := len(s.pending); n > probeMaxBytes+4096 {
		t.Errorf("probe kept %d bytes, want at most about %d", n, probeMaxBytes)
	}
	// end the flood and let the pump finish before the fake clock is restored
	pw.Close()
	for range s.input {
	}
}

func TestProbeKittyKeepsTypedKeys(t *testing.T) {
	useFakeClock(t)
	s, pw := pipeSession(t, func(p []byte, term *io.PipeWriter) {
		if string(p) != kittyQuery {
			return
		}
		term.Write([]byte("a")) // typed before the reply
		term.Write([]byte(kittyOKAndDA1))
		term.Write([]byte("b")) // typed after it
	})
	if !s.probeKitty(time.Second) {
		t.Fatal("probe failed")
	}
	got, _ := readUntil(t, s, 2)
	if got != "ab" {
		t.Errorf("typed keys read as %q, want %q", got, "ab")
	}
	writeClient(t, pw, "c")
	if got, _ := readUntil(t, s, 1); got != "c" {
		t.Errorf("key typed later read as %q, want c", got)
	}
}

func TestLateRepliesAreFiltered(t *testing.T) {
	useFakeClock(t)
	s, pw := pipeSession(t, answering(kittyOKAndDA1))
	if !s.probeKitty(time.Second) {
		t.Fatal("probe failed")
	}

	// a late kitty reply and a late device attributes reply, between keys
	writeClient(t, pw, "x\x1b_Gi=31;OK\x1b\\y\x1b[?62;22cz")
	if got, _ := readUntil(t, s, 3); got != "xyz" {
		t.Errorf("late replies leaked into input: got %q, want xyz", got)
	}

	// a reply cut off between two writes is held until its end arrives
	writeClient(t, pw, "\x1b_Gi=31;")
	writeClient(t, pw, "OK\x1b\\w")
	writeClient(t, pw, "\x1b[?62")
	writeClient(t, pw, ";22cv")
	if got, _ := readUntil(t, s, 2); got != "wv" {
		t.Errorf("split replies not filtered: got %q, want wv", got)
	}
}

func TestEscAloneIsNotHeld(t *testing.T) {
	useFakeClock(t)
	s, pw := pipeSession(t, answering(kittyOKAndDA1))
	if !s.probeKitty(time.Second) {
		t.Fatal("probe failed")
	}
	writeClient(t, pw, "\x1b")
	if got, _ := readUntil(t, s, 1); got != "\x1b" {
		t.Errorf("Esc alone read as %q, want the Esc byte at once", got)
	}
}

func TestFilterOnlyAfterProbe(t *testing.T) {
	useFakeClock(t)
	s, pw := pipeSession(t, nil)
	s.startPump() // the pipe needs a reader before the client can type
	// without a probe, input is passed through untouched
	writeClient(t, pw, "\x1b[?62c")
	if got, _ := readUntil(t, s, len("\x1b[?62c")); got != "\x1b[?62c" {
		t.Errorf("unprobed input altered: got %q", got)
	}
}

func TestLateReplyStillRecordsInput(t *testing.T) {
	clock := useFakeClock(t)
	t0 := *clock
	s, pw := pipeSession(t, answering(kittyOKAndDA1))
	if !s.probeKitty(time.Second) {
		t.Fatal("probe failed")
	}
	*clock = t0.Add(7 * time.Second)
	writeClient(t, pw, "\x1b_Gi=31;OK\x1b\\k")
	if got, _ := readUntil(t, s, 1); got != "k" {
		t.Fatalf("read %q, want k", got)
	}
	if !s.act.lastInput.Equal(t0.Add(7 * time.Second)) {
		t.Errorf("lastInput = %v, want the time of the late reply's bytes", s.act.lastInput)
	}
}

func TestInputPumpEndsWithSession(t *testing.T) {
	useFakeClock(t)
	s, pw := pipeSession(t, nil)
	s.quit = make(chan struct{})
	s.startPump()
	// more chunks than the pump can queue, so it is blocked handing one over
	go func() {
		for range 20 {
			if _, err := pw.Write([]byte("k")); err != nil {
				return
			}
		}
	}()
	time.Sleep(10 * time.Millisecond)
	close(s.quit)
	if _, err := readUntil(t, s, 1<<20); err != io.EOF {
		t.Errorf("Read after the handler ended = %v, want io.EOF", err)
	}
}

// chunkedIO takes each write one byte at a time, yielding between bytes, so
// writes that overlap show up in its output.
type chunkedIO struct {
	ssh.Session
	out []byte
}

func (c *chunkedIO) Write(p []byte) (int, error) {
	for _, b := range p {
		c.out = append(c.out, b)
		runtime.Gosched()
	}
	return len(p), nil
}

func TestScopedSessionWritesDoNotInterleave(t *testing.T) {
	useFakeClock(t)
	f := &chunkedIO{}
	s := &scopedSession{Session: f, id: 1, act: newActivity(now())}
	const writers, writes, size = 8, 20, 32
	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := []byte(strings.Repeat(string(rune('a'+w)), size))
			for range writes {
				s.Write(msg)
			}
		}()
	}
	wg.Wait()
	if len(f.out) != writers*writes*size {
		t.Fatalf("wrote %d bytes, want %d", len(f.out), writers*writes*size)
	}
	for i := 0; i < len(f.out); i += size {
		block := f.out[i : i+size]
		if bytes.Count(block, block[:1]) != size {
			t.Fatalf("write at offset %d interleaved: %q", i, block)
		}
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
