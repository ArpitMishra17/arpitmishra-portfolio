package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/muesli/termenv"
)

const defaultPort = "23234"

var (
	sessionsServed atomic.Int64
	processStart   = time.Now()
)

// uptime renders server uptime like "2h13m" for the status bar.
func uptime() string {
	d := time.Since(processStart)
	if d < time.Minute {
		return "up " + fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return "up " + time.Since(processStart).Round(time.Minute).String()[2:]
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-render" {
		// Print a single frame at a fixed size — useful for testing layout
		// without a terminal: go run . -render 120 40
		w, h := 120, 40
		if len(os.Args) > 3 {
			fmt.Sscan(os.Args[2], &w)
			fmt.Sscan(os.Args[3], &h)
		}
		mdl := newModel(mustLoadBlog(), 1, lipgloss.ColorProfile())
		mdl.typed = len([]rune(bio)) // skip the typewriter for static renders
		mdl.typing = false
		if len(os.Args) > 4 {
			for i, n := range sectionNames {
				if strings.EqualFold(n, os.Args[4]) {
					mdl.sec = section(i)
				}
			}
			if strings.EqualFold(os.Args[4], "help") {
				mdl.modal = modalHelp
			}
			if strings.EqualFold(os.Args[4], "theme") {
				mdl.modal = modalTheme
			}
		}
		mdl2, _ := mdl.Update(tea.WindowSizeMsg{Width: w, Height: h})
		fmt.Println(mdl2.View())
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "-local" {
		// Renders the TUI straight in the current terminal, no SSH involved.
		// Useful for fast iteration: go run . -local
		if _, err := newProgram(mustLoadBlog(), 1).Run(); err != nil {
			log.Error("local run failed", "error", err)
			os.Exit(1)
		}
		return
	}

	port := defaultPort
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	s, err := wish.NewServer(
		wish.WithAddress("0.0.0.0:"+port),
		wish.WithHostKeyPath(".ssh/host_ed25519"),
		// Public on :22, so drop abandoned sessions instead of holding them forever.
		wish.WithIdleTimeout(15*time.Minute),
		wish.WithMiddleware(
			bm.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				m := newModel(mustLoadBlog(), int(sessionsServed.Add(1)), sessionProfile(s))
				return m, []tea.ProgramOption{tea.WithAltScreen()}
			}),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("could not start server", "error", err)
		os.Exit(1)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	}()

	log.Info("listening", "addr", "0.0.0.0:"+port, "connect", "ssh -p "+port+" localhost")
	if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("could not serve", "error", err)
		os.Exit(1)
	}
}

// sessionProfile works out what colours the visitor's terminal supports from
// the TERM and env their ssh client sent. The server process itself has no
// terminal, so lipgloss's own detection would always answer "no colour".
// Anything that asks for a terminal today handles 256 colours, so that's the
// floor (plain TERM=xterm would otherwise drop to 16).
func sessionProfile(s ssh.Session) termenv.Profile {
	pty, _, _ := s.Pty()
	env := sshEnv(append(s.Environ(), "TERM="+pty.Term))
	p := termenv.NewOutput(s, termenv.WithEnvironment(env), termenv.WithUnsafe()).EnvColorProfile()
	if p > termenv.ANSI256 { // higher profile = fewer colours
		p = termenv.ANSI256
	}
	return p
}

// sshEnv adapts an ssh session's environment for termenv.
type sshEnv []string

func (e sshEnv) Environ() []string { return e }

func (e sshEnv) Getenv(key string) string {
	for _, kv := range e {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v
		}
	}
	return ""
}
