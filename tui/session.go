package main

import (
	"io"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
)

// Watchdog limits. Vars, not consts, so a throwaway build can shorten them.
var (
	tick       = 5 * time.Second        // how often the watchdog checks a session
	writeStall = 60 * time.Second       // a write pending this long: the client stopped reading
	inputIdle  = idleQuit + time.Minute // no client bytes this long: the model's own quit has missed
)

// activity is what the watchdog knows about a session's client. The model's
// idle quit runs inside bubbletea's event loop, which a stalled write blocks,
// so the watchdog works from here instead.
type activity struct {
	mu        sync.Mutex
	lastInput time.Time // last bytes read from the client
	pending   int       // writes in flight
	writeFrom time.Time // when the oldest in-flight write began
}

func newActivity(t time.Time) *activity {
	return &activity{lastInput: t}
}

func (a *activity) beginWrite(t time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending == 0 {
		a.writeFrom = t
	}
	a.pending++
}

func (a *activity) endWrite() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending--
}

func (a *activity) gotInput(t time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastInput = t
}

// verdict reports why the session at time t should be dropped, or "" to keep it.
func (a *activity) verdict(t time.Time) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending > 0 && t.Sub(a.writeFrom) > writeStall {
		return "output stalled"
	}
	if t.Sub(a.lastInput) > inputIdle {
		return "no input"
	}
	return ""
}

// scopedSession is the ssh.Session a visitor's app runs on. It records the
// client's input and output for the watchdog. bubbletea reads and writes the
// session itself because wish uses an emulated pty, so these wrappers see all
// the traffic. If pty allocation (ssh.AllocatePty) is ever turned on, the io
// goes through the pty instead and these wrappers see nothing.
type scopedSession struct {
	ssh.Session
	id  int
	act *activity
}

func (s *scopedSession) Read(p []byte) (int, error) {
	n, err := s.Session.Read(p)
	if n > 0 {
		s.act.gotInput(now())
	}
	return n, err
}

func (s *scopedSession) Write(p []byte) (int, error) {
	s.act.beginWrite(now())
	defer s.act.endWrite()
	return s.Session.Write(p)
}

// sessionID is the id sessionScope gave s. A session that didn't pass through
// sessionScope gets a fresh id, which the hub doesn't know about.
func sessionID(s ssh.Session) int {
	if sc, ok := s.(*scopedSession); ok {
		return sc.id
	}
	return int(sessionsServed.Add(1))
}

// closeConn drops the whole SSH connection behind s, which ends every session on it.
func closeConn(s ssh.Session) {
	c, ok := s.Context().Value(ssh.ContextKeyConn).(io.Closer)
	if !ok {
		return
	}
	log.Debug("closing connection", "remote", remoteHost(s.RemoteAddr()))
	_ = c.Close()
}

// watch closes s's connection when act says the client has stalled or gone
// quiet. It returns once done is closed.
func watch(s ssh.Session, act *activity, done <-chan struct{}) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			if why := act.verdict(now()); why != "" {
				log.Info("dropping session", "reason", why, "remote", remoteHost(s.RemoteAddr()))
				closeConn(s)
				return
			}
		}
	}
}

// sessionScope gives each admitted session an id and a watchdog, and puts pty
// sessions on the hub for as long as the session runs. It sits inside
// sessionLimiter, so refused sessions never get here.
func sessionScope() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			id := int(sessionsServed.Add(1))
			scoped := &scopedSession{Session: s, id: id, act: newActivity(now())}
			// only a pty gets a model, so only a pty stands on the promenade
			if _, _, ok := s.Pty(); ok {
				hub.join(id, visitorName(s.User()))
				defer hub.leave(id)
			}
			done := make(chan struct{})
			stopped := make(chan struct{})
			go func() {
				defer close(stopped)
				watch(s, scoped.act, done)
			}()
			// runs before hub.leave, and waits for the watchdog, so it never outlives the session
			defer func() {
				close(done)
				<-stopped
			}()
			next(scoped)
		}
	}
}
