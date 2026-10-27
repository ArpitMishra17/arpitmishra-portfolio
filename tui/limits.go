package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
)

// now is the clock for the limits. Tests replace it to control time.
var now = time.Now

const (
	maxSessions       = 40    // concurrent sessions, all visitors together
	maxPerIP          = 4     // concurrent sessions from one address
	maxConnsPerMinute = 10    // new sessions one address may open a minute
	maxRateKeys       = 10000 // addresses a rate window tracks before it prunes
)

var gate = newSessionGate(maxSessions, maxPerIP, maxConnsPerMinute)

// refuseWait bounds how long the refusal text may take to reach the client.
// refuseLinger keeps the connection open after it, so the client can read it.
var (
	refuseWait   = 3 * time.Second
	refuseLinger = 250 * time.Millisecond
)

// remoteHost is the key a visitor's connections share a limit under. IPv4 and
// IPv4-mapped IPv6 use the IPv4 address. Other IPv6 uses its /64, since a
// visitor can usually pick any address in one. Addresses that don't parse
// (non-TCP) are used raw.
func remoteHost(a net.Addr) string {
	if a == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(a.String())
	if err != nil {
		host = a.String()
	}
	ip := net.ParseIP(strings.Split(host, "%")[0]) // drop an IPv6 zone
	if ip == nil {
		return host
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// sessionGate decides whether a new session may start: first a per-IP rate
// cap, then the total and per-IP caps on sessions running at once.
type sessionGate struct {
	rate     *hitLimiter
	maxTotal int
	maxPerIP int

	mu    sync.Mutex
	total int
	perIP map[string]int // entries are deleted at zero, so the map stays small
}

func newSessionGate(maxTotal, maxPerIP, perMinute int) *sessionGate {
	return &sessionGate{
		rate:     newHitLimiter(perMinute, time.Minute),
		maxTotal: maxTotal,
		maxPerIP: maxPerIP,
		perIP:    map[string]int{},
	}
}

// admit reports whether a session from ip may start at time t. When it may,
// the returned func must be called once the session ends.
func (g *sessionGate) admit(ip string, t time.Time) (release func(), ok bool) {
	if !g.rate.allow(ip, t) {
		return nil, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.total >= g.maxTotal || g.perIP[ip] >= g.maxPerIP {
		return nil, false
	}
	g.total++
	g.perIP[ip]++
	return func() { g.done(ip) }, true
}

func (g *sessionGate) done(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.total--
	if g.perIP[ip] <= 1 {
		delete(g.perIP, ip)
	} else {
		g.perIP[ip]--
	}
}

// sessionLimiter turns away sessions the gate refuses, with a short message,
// and otherwise holds a slot for as long as next runs.
func sessionLimiter(g *sessionGate) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			release, ok := g.admit(remoteHost(s.RemoteAddr()), now())
			if !ok {
				refuse(s)
				return
			}
			defer release()
			next(s)
		}
	}
}

// refuse sends the client a short message, ends the session, then closes the
// whole connection, so the client can't open another session on it. A client
// that won't read is cut off after refuseWait.
func refuse(s ssh.Session) {
	guard := time.AfterFunc(refuseWait, func() { closeConn(s) })
	fmt.Fprint(s, "too many connections right now, try again in a minute\r\n")
	_ = s.Exit(1)
	guard.Stop()
	time.AfterFunc(refuseLinger, func() { closeConn(s) })
}

// hitLimiter allows at most limit hits per key in any span of window: a sliding
// window, so old hits fall out one at a time. Refused hits are not recorded.
type hitLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time // hit times, oldest first; keys never hold an empty slice
}

func newHitLimiter(limit int, window time.Duration) *hitLimiter {
	return &hitLimiter{limit: limit, window: window, hits: map[string][]time.Time{}}
}

// allow records a hit for key at time t and reports whether it is within the limit.
func (l *hitLimiter) allow(key string, t time.Time) bool {
	return l.hit(key, t, true)
}

// peek reports what allow would return for key at time t, without recording a hit.
func (l *hitLimiter) peek(key string, t time.Time) bool {
	return l.hit(key, t, false)
}

func (l *hitLimiter) hit(key string, t time.Time, record bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, seen := l.hits[key]; !seen && len(l.hits) >= maxRateKeys {
		l.prune(t)
	}
	ts := l.recent(key, t)
	ok := len(ts) < l.limit
	if ok && record {
		ts = append(ts, t)
	}
	if len(ts) == 0 {
		delete(l.hits, key)
	} else {
		l.hits[key] = ts
	}
	return ok
}

// recent drops the hits for key that have left the window and returns the rest.
func (l *hitLimiter) recent(key string, t time.Time) []time.Time {
	ts := l.hits[key]
	cut := t.Add(-l.window)
	i := 0
	for i < len(ts) && !ts[i].After(cut) {
		i++
	}
	return ts[i:]
}

// prune forgets keys whose hits have all left the window. If the map is still
// full, it is cleared, so a flood of distinct addresses can't grow it without
// bound. The cost is that current counts reset, which needs that many addresses.
func (l *hitLimiter) prune(t time.Time) {
	cut := t.Add(-l.window)
	for key, ts := range l.hits {
		if !ts[len(ts)-1].After(cut) {
			delete(l.hits, key)
		}
	}
	if len(l.hits) >= maxRateKeys {
		l.hits = map[string][]time.Time{}
	}
}
