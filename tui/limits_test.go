package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/ssh"
)

// useFakeClock replaces now with a clock that only moves when the test sets
// the returned value.
func useFakeClock(t *testing.T) *time.Time {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	old := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
	return &at
}

func TestRemoteHost(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7:5522":         "203.0.113.7",
		"[::ffff:203.0.113.7]:22":  "203.0.113.7", // IPv4-mapped shares the IPv4 key
		"[2001:db8:1:2:aaaa::1]:2": "2001:db8:1:2::/64",
		"[2001:db8:1:2:ffff::9]:2": "2001:db8:1:2::/64", // same /64, same key
		"[2001:db8:1:3::1]:22":     "2001:db8:1:3::/64",
		"[fe80::1%eth0]:22":        "fe80::/64", // zone dropped
		"not-a-host-port":          "not-a-host-port",
	}
	for in, want := range cases {
		if got := remoteHost(strAddr(in)); got != want {
			t.Errorf("remoteHost(%q) = %q, want %q", in, got, want)
		}
	}
	if got := remoteHost(nil); got != "unknown" {
		t.Errorf("remoteHost(nil) = %q, want unknown", got)
	}
}

func TestRemoteHostIPv6SharesLimitWithin64(t *testing.T) {
	g := newSessionGate(100, 2, 1000)
	at := time.Now()
	// one visitor with a whole /64 to pick addresses from gets the same per-IP cap
	for i, a := range []string{"[2001:db8:5:6::1]:1", "[2001:db8:5:6::2]:1"} {
		if _, ok := g.admit(remoteHost(strAddr(a)), at); !ok {
			t.Fatalf("address %d refused under the per-IP cap", i)
		}
	}
	if _, ok := g.admit(remoteHost(strAddr("[2001:db8:5:6:ffff::3]:1")), at); ok {
		t.Fatal("third address in the same /64 admitted over the per-IP cap")
	}
	if _, ok := g.admit(remoteHost(strAddr("[2001:db8:5:7::1]:1")), at); !ok {
		t.Fatal("address in another /64 refused")
	}
}

// strAddr is a net.Addr whose String is fixed, to feed remoteHost raw strings.
type strAddr string

func (a strAddr) Network() string { return "tcp" }
func (a strAddr) String() string  { return string(a) }

func TestSessionGateTotalCap(t *testing.T) {
	g := newSessionGate(3, 10, 1000)
	at := time.Now()
	var releases []func()
	for i := 0; i < 3; i++ {
		rel, ok := g.admit(fmt.Sprintf("10.0.0.%d", i), at)
		if !ok {
			t.Fatalf("session %d refused under the total cap", i)
		}
		releases = append(releases, rel)
	}
	if _, ok := g.admit("10.0.0.9", at); ok {
		t.Fatal("4th session admitted over the total cap")
	}
	releases[0]()
	if _, ok := g.admit("10.0.0.9", at); !ok {
		t.Fatal("session refused after a slot was released")
	}
}

func TestSessionGatePerIPCap(t *testing.T) {
	g := newSessionGate(100, 2, 1000)
	at := time.Now()
	r1, _ := g.admit("10.0.0.1", at)
	if _, ok := g.admit("10.0.0.1", at); !ok {
		t.Fatal("2nd session from one IP refused")
	}
	if _, ok := g.admit("10.0.0.1", at); ok {
		t.Fatal("3rd session from one IP admitted over the per-IP cap")
	}
	if _, ok := g.admit("10.0.0.2", at); !ok {
		t.Fatal("other IP refused because of the first IP's count")
	}
	r1()
	if _, ok := g.admit("10.0.0.1", at); !ok {
		t.Fatal("IP still refused after one of its sessions ended")
	}
}

func TestSessionGateRateWindow(t *testing.T) {
	g := newSessionGate(100, 100, 3)
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if _, ok := g.admit("10.0.0.1", start.Add(time.Duration(i)*10*time.Second)); !ok {
			t.Fatalf("connection %d refused under the rate cap", i)
		}
	}
	if _, ok := g.admit("10.0.0.1", start.Add(40*time.Second)); ok {
		t.Fatal("4th connection in a minute admitted")
	}
	if _, ok := g.admit("10.0.0.2", start.Add(40*time.Second)); !ok {
		t.Fatal("rate cap leaked across IPs")
	}
	// the first hit leaves the window a minute after it was made
	if _, ok := g.admit("10.0.0.1", start.Add(61*time.Second)); !ok {
		t.Fatal("connection refused after the window moved on")
	}
}

func TestSessionGateCountersReturnToZero(t *testing.T) {
	g := newSessionGate(10, 10, 1000)
	at := time.Now()
	var releases []func()
	for i := 0; i < 5; i++ {
		rel, ok := g.admit(fmt.Sprintf("10.0.0.%d", i%2), at)
		if !ok {
			t.Fatalf("session %d refused", i)
		}
		releases = append(releases, rel)
	}
	for _, rel := range releases {
		rel()
	}
	if g.total != 0 {
		t.Errorf("total = %d after all sessions ended, want 0", g.total)
	}
	if len(g.perIP) != 0 {
		t.Errorf("perIP holds %d entries after all sessions ended, want 0", len(g.perIP))
	}
}

// fakeSession is the part of ssh.Session the limiter and scope touch. Any other
// method panics on the nil embedded interface, which is what we want in a test.
type fakeSession struct {
	ssh.Session
	addr   net.Addr
	pty    bool      // whether the client asked for a pty
	in     io.Reader // client input; nil reads as EOF
	conn   *closeRecorder
	stuck  bool // Write blocks until the connection is closed
	out    bytes.Buffer
	exit   int
	exited bool
}

func (f *fakeSession) RemoteAddr() net.Addr { return f.addr }
func (f *fakeSession) User() string         { return "ada" }
func (f *fakeSession) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	return ssh.Pty{}, nil, f.pty
}
func (f *fakeSession) Context() ssh.Context { return fakeCtx{conn: f.conn} }
func (f *fakeSession) Exit(code int) error  { f.exit, f.exited = code, true; return nil }

func (f *fakeSession) Read(p []byte) (int, error) {
	if f.in == nil {
		return 0, io.EOF
	}
	return f.in.Read(p)
}

func (f *fakeSession) Write(p []byte) (int, error) {
	if f.stuck {
		<-f.conn.closed
		return 0, io.ErrClosedPipe
	}
	return f.out.Write(p)
}

// closeRecorder stands in for the SSH connection. closed is closed on the first Close.
type closeRecorder struct {
	once   sync.Once
	closed chan struct{}
}

func newCloseRecorder() *closeRecorder { return &closeRecorder{closed: make(chan struct{})} }

func (c *closeRecorder) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

// fakeCtx is the part of ssh.Context closeConn reads: the connection value.
type fakeCtx struct {
	ssh.Context
	conn *closeRecorder
}

func (c fakeCtx) Value(key any) any {
	if key == ssh.ContextKeyConn && c.conn != nil {
		return c.conn
	}
	return nil
}

func TestSessionLimiterMiddleware(t *testing.T) {
	clock := useFakeClock(t)
	g := newSessionGate(100, 1, 1000)
	ip := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 5000}

	var ran []int
	h := sessionLimiter(g)(func(s ssh.Session) {
		ran = append(ran, 1)
		// while the first session runs, a second from the same IP is refused
		second := &fakeSession{addr: ip}
		sessionLimiter(g)(func(ssh.Session) { ran = append(ran, 2) })(second)
		if !second.exited || second.exit != 1 {
			t.Errorf("second session: exited=%v code=%d, want exit 1", second.exited, second.exit)
		}
		if second.out.Len() == 0 {
			t.Error("refused session got no message")
		}
	})

	first := &fakeSession{addr: ip}
	h(first)
	if len(ran) != 1 || ran[0] != 1 {
		t.Fatalf("handler calls = %v, want [1]", ran)
	}
	if first.exited {
		t.Error("admitted session was exited by the limiter")
	}
	if g.total != 0 || len(g.perIP) != 0 {
		t.Errorf("slot not released after the session: total=%d perIP=%v", g.total, g.perIP)
	}

	// the rate window also runs off the package clock
	*clock = clock.Add(time.Second)
	h(&fakeSession{addr: ip})
	if len(ran) != 2 {
		t.Errorf("session after the first one ended was not admitted: %v", ran)
	}
}

func TestSessionLimiterRefusesWithoutNext(t *testing.T) {
	useFakeClock(t)
	g := newSessionGate(100, 100, 2)
	ip := &net.TCPAddr{IP: net.ParseIP("10.0.0.3"), Port: 1}
	called := 0
	h := sessionLimiter(g)(func(ssh.Session) { called++ })
	for i := 0; i < 3; i++ {
		s := &fakeSession{addr: ip}
		h(s)
		if i == 2 {
			if !s.exited || s.exit != 1 {
				t.Errorf("3rd connection in a minute: exited=%v code=%d", s.exited, s.exit)
			}
			if !bytes.Contains(s.out.Bytes(), []byte("too many connections")) {
				t.Errorf("refusal message = %q", s.out.String())
			}
		}
	}
	if called != 2 {
		t.Errorf("next ran %d times, want 2", called)
	}
}

// shortenRefuse makes the refusal timers fire within a test's lifetime.
func shortenRefuse(t *testing.T, wait, linger time.Duration) {
	t.Helper()
	oldWait, oldLinger := refuseWait, refuseLinger
	refuseWait, refuseLinger = wait, linger
	t.Cleanup(func() { refuseWait, refuseLinger = oldWait, oldLinger })
}

// refusedSession opens one more session from ip through a limiter that is
// already full for it, and returns the refused fake session.
func refusedSession(t *testing.T, conn *closeRecorder, stuck bool) *fakeSession {
	t.Helper()
	clock := useFakeClock(t)
	g := newSessionGate(100, 1, 1000)
	ip := &net.TCPAddr{IP: net.ParseIP("10.0.0.4"), Port: 1}
	hold, ok := g.admit(remoteHost(ip), *clock)
	if !ok {
		t.Fatal("first session refused")
	}
	defer hold()
	s := &fakeSession{addr: ip, conn: conn, stuck: stuck}
	sessionLimiter(g)(func(ssh.Session) { t.Error("refused session reached the handler") })(s)
	return s
}

func TestRefusalDeliversMessageThenClosesConn(t *testing.T) {
	shortenRefuse(t, 2*time.Second, 10*time.Millisecond)
	conn := newCloseRecorder()
	s := refusedSession(t, conn, false)
	if !bytes.Contains(s.out.Bytes(), []byte("too many connections")) {
		t.Errorf("refusal message = %q", s.out.String())
	}
	if !s.exited || s.exit != 1 {
		t.Errorf("refused session: exited=%v code=%d, want exit 1", s.exited, s.exit)
	}
	select {
	case <-conn.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("connection not closed after the refusal")
	}
}

func TestRefusalCutsOffClientThatWontRead(t *testing.T) {
	shortenRefuse(t, 20*time.Millisecond, 10*time.Millisecond)
	conn := newCloseRecorder()
	done := make(chan struct{})
	go func() {
		refusedSession(t, conn, true)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("refusal to a non-reading client never returned")
	}
	select {
	case <-conn.closed:
	default:
		t.Fatal("connection still open after the blocked write")
	}
}

func TestHitLimiterPeekRecordsNothing(t *testing.T) {
	l := newHitLimiter(1, time.Minute)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !l.peek("a", t0) || !l.peek("a", t0) {
		t.Fatal("peek refused under the limit")
	}
	if !l.allow("a", t0) {
		t.Fatal("allow refused after peeks: peek recorded a hit")
	}
	if l.peek("a", t0.Add(time.Second)) {
		t.Fatal("peek allowed over the limit")
	}
}

func TestHitLimiterSlidingWindow(t *testing.T) {
	l := newHitLimiter(2, time.Minute)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !l.allow("a", t0) || !l.allow("a", t0.Add(10*time.Second)) {
		t.Fatal("first two hits refused")
	}
	if l.allow("a", t0.Add(20*time.Second)) {
		t.Fatal("third hit in the window allowed")
	}
	// the refused hit at 20s was not recorded, so once the first hit leaves the
	// window only the 10s hit counts: one more fits, and then the limit is back
	if !l.allow("a", t0.Add(60*time.Second+time.Millisecond)) {
		t.Fatal("hit refused after the first one left the window")
	}
	if l.allow("a", t0.Add(60*time.Second+2*time.Millisecond)) {
		t.Fatal("limit not enforced after the window moved")
	}
	if !l.allow("a", t0.Add(time.Hour)) {
		t.Fatal("key refused after its window cleared")
	}
	if n := len(l.hits["a"]); n != 1 {
		t.Errorf("expired hits kept: %d recorded, want 1", n)
	}
}

func TestHitLimiterPrunesStaleKeys(t *testing.T) {
	l := newHitLimiter(1, time.Minute)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < maxRateKeys; i++ {
		l.allow(fmt.Sprintf("ip-%d", i), t0)
	}
	// every stored hit is over a minute old, so the map empties down to the new key
	if !l.allow("new", t0.Add(2*time.Minute)) {
		t.Fatal("new key refused")
	}
	if len(l.hits) != 1 {
		t.Errorf("map holds %d keys after pruning, want 1", len(l.hits))
	}
}

func TestHitLimiterCapsMapWhenAllKeysAreFresh(t *testing.T) {
	l := newHitLimiter(1, time.Hour)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < maxRateKeys+500; i++ {
		l.allow(fmt.Sprintf("ip-%d", i), t0)
		if len(l.hits) > maxRateKeys {
			t.Fatalf("map grew to %d keys, cap is %d", len(l.hits), maxRateKeys)
		}
	}
}
