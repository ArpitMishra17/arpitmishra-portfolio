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
	// quit is closed when the session's handler ends, so the input pump
	// doesn't wait forever to hand over a chunk nobody will read.
	quit chan struct{}

	wmu sync.Mutex // one write to the client at a time, so output never interleaves

	// the input pump reads the client in its own goroutine, so a probe can
	// watch for replies while Read is idle
	pumpOnce sync.Once
	input    chan []byte // chunks from the pump, in order
	inputErr error       // why input closed; set before it is closed

	// reader state, touched only by Read and probeKitty (bubbletea's one reader)
	pending []byte      // filtered input not yet returned by Read
	probed  bool        // a kitty query was sent, so late replies are stripped
	filter  replyFilter // holds a reply cut off at the end of a chunk
}

// startPump starts the goroutine that reads the client's input. It is the only
// reader of the underlying session.
func (s *scopedSession) startPump() {
	s.pumpOnce.Do(func() {
		s.input = make(chan []byte, 16)
		go s.pump()
	})
}

func (s *scopedSession) pump() {
	defer close(s.input)
	buf := make([]byte, 4096)
	for {
		n, err := s.Session.Read(buf)
		if n > 0 {
			s.act.gotInput(now())
			chunk := append([]byte(nil), buf[:n]...)
			select {
			case s.input <- chunk:
			case <-s.quit:
				return
			}
		}
		if err != nil {
			s.inputErr = err
			return
		}
	}
}

// closed is what Read returns once the client's input is over.
func (s *scopedSession) closed() error {
	if s.inputErr != nil {
		return s.inputErr
	}
	return io.EOF
}

// filtered returns the part of an input chunk that Read should pass on.
func (s *scopedSession) filtered(chunk []byte) []byte {
	if !s.probed {
		return chunk
	}
	return s.filter.strip(chunk)
}

func (s *scopedSession) Read(p []byte) (int, error) {
	s.startPump()
	if len(p) == 0 {
		return 0, nil
	}
	for len(s.pending) == 0 {
		chunk, open := <-s.input
		if !open {
			return 0, s.closed()
		}
		s.pending = s.filtered(chunk)
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

func (s *scopedSession) Write(p []byte) (int, error) {
	// the write counts as pending before the lock, so a write queued behind
	// a stalled one is still seen by the watchdog
	s.act.beginWrite(now())
	defer s.act.endWrite()
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return s.Session.Write(p)
}

// probeMaxBytes caps the input held while waiting for a probe's answer.
const probeMaxBytes = 16 << 10

// probeKitty asks the terminal whether it speaks the kitty graphics protocol,
// and waits up to timeout for the answer. Input the client types meanwhile is
// kept for Read. From here on, late replies are stripped from the input.
func (s *scopedSession) probeKitty(timeout time.Duration) bool {
	s.startPump()
	s.probed = true
	if _, err := s.Write([]byte(kittyQuery)); err != nil {
		return false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	buf := s.pending
	s.pending = nil
	var supported, done bool
wait:
	for {
		var ok, d bool
		ok, d, buf = parseKittyProbe(buf)
		supported = supported || ok
		done = done || d
		// a real terminal answers in a few bytes; a client flooding input
		// during the probe gets no graphics instead of an ever-growing buffer
		if done || len(buf) > probeMaxBytes {
			break
		}
		select {
		case chunk, open := <-s.input:
			if !open {
				break wait
			}
			buf = append(buf, chunk...)
		case <-timer.C:
			break wait
		}
	}
	// a reply cut off at the end is held by the filter for its last bytes
	s.pending = s.filter.strip(buf)
	return supported
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
			scoped := &scopedSession{Session: s, id: id, act: newActivity(now()), quit: make(chan struct{})}
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
				close(scoped.quit)
				close(done)
				<-stopped
			}()
			next(scoped)
		}
	}
}
