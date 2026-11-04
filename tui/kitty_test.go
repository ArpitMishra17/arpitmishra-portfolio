package main

import (
	"bytes"
	"encoding/base64"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode"
)

func TestKittyDiacritics(t *testing.T) {
	// the spec's example: U+0305 is row or column 0, U+030D is 1
	want := []rune{0x0305, 0x030D, 0x030E, 0x0310, 0x0312}
	if !slices.Equal(kittyDiacritics[:len(want)], want) {
		t.Errorf("first diacritics = %X, want %X", kittyDiacritics[:len(want)], want)
	}
	if len(kittyDiacritics) != 297 {
		t.Errorf("table has %d entries, want 297", len(kittyDiacritics))
	}
	seen := map[rune]bool{}
	for i, r := range kittyDiacritics {
		if seen[r] {
			t.Errorf("entry %d (%U) repeats", i, r)
		}
		seen[r] = true
		if !unicode.Is(unicode.Mn, r) {
			t.Errorf("entry %d (%U) is not a combining mark", i, r)
		}
	}
}

func TestKittyBlock(t *testing.T) {
	const id, cols, rows = 7, 3, 2
	lines := kittyBlock(id, cols, rows)
	if len(lines) != rows {
		t.Fatalf("got %d lines, want %d", len(lines), rows)
	}
	for r, line := range lines {
		body, ok := strings.CutPrefix(line, "\x1b[38;5;7m")
		if !ok {
			t.Fatalf("line %d lacks the colour code for id %d: %q", r, id, line)
		}
		body, ok = strings.CutSuffix(body, "\x1b[39m")
		if !ok {
			t.Fatalf("line %d lacks the colour reset: %q", r, line)
		}
		runes := []rune(body)
		if len(runes) != cols*3 {
			t.Fatalf("line %d has %d runes, want %d", r, len(runes), cols*3)
		}
		for c := range cols {
			want := []rune{kittyPlaceholder, kittyDiacritics[r], kittyDiacritics[c]}
			if got := runes[c*3 : c*3+3]; !slices.Equal(got, want) {
				t.Errorf("row %d col %d cell = %U, want %U", r, c, got, want)
			}
		}
	}
}

func TestKittyBlockSizes(t *testing.T) {
	if lines := kittyBlock(1, 2, -1); len(lines) != 0 {
		t.Errorf("negative rows gave %d lines", len(lines))
	}
	if lines := kittyBlock(1, 0, 3); len(lines) != 3 || lines[0] != "\x1b[38;5;1m\x1b[39m" {
		t.Errorf("zero cols gave %q", lines)
	}
	if lines := kittyBlock(1, 1, len(kittyDiacritics)+10); len(lines) != len(kittyDiacritics) {
		t.Errorf("rows past the table gave %d lines, want %d", len(lines), len(kittyDiacritics))
	}
}

func TestKittyCell(t *testing.T) {
	if got, want := kittyCell(), "\U0010EEEE̅̅"; got != want {
		t.Errorf("kittyCell() = %q, want %q", got, want)
	}
}

// apcs splits escape output into the keys and payload of each APC in it.
func apcs(t *testing.T, out []byte) (keys, payloads []string) {
	t.Helper()
	s := string(out)
	for s != "" {
		if !strings.HasPrefix(s, "\x1b_G") {
			t.Fatalf("output does not start an APC: %q", s[:min(len(s), 20)])
		}
		end := strings.Index(s, "\x1b\\")
		if end < 0 {
			t.Fatal("APC is not terminated")
		}
		k, p, _ := strings.Cut(s[len("\x1b_G"):end], ";")
		keys = append(keys, k)
		payloads = append(payloads, p)
		s = s[end+len("\x1b\\"):]
	}
	return keys, payloads
}

func TestKittyUploadSingleChunk(t *testing.T) {
	png := []byte("small png bytes")
	keys, payloads := apcs(t, kittyUpload(9, png, 4, 2))
	if len(keys) != 1 {
		t.Fatalf("got %d chunks, want 1", len(keys))
	}
	if want := "a=T,U=1,f=100,t=d,i=9,c=4,r=2,q=2,m=0"; keys[0] != want {
		t.Errorf("keys = %q, want %q", keys[0], want)
	}
	if got, _ := base64.StdEncoding.DecodeString(payloads[0]); !bytes.Equal(got, png) {
		t.Errorf("payload decodes to %q, want %q", got, png)
	}
}

func TestKittyUploadChunks(t *testing.T) {
	// the chunk count changes at 4096 base64 bytes, which is 3072 PNG bytes
	for _, tt := range []struct{ size, chunks int }{
		{20, 1},
		{3072, 1},
		{3075, 2},
		{10 * 1024, 4},
	} {
		png := make([]byte, tt.size)
		for i := range png {
			png[i] = byte(i*7 + i/251)
		}
		keys, payloads := apcs(t, kittyUpload(9, png, 4, 2))
		if len(keys) != tt.chunks {
			t.Fatalf("%d bytes: got %d chunks, want %d", tt.size, len(keys), tt.chunks)
		}
		first := "a=T,U=1,f=100,t=d,i=9,c=4,r=2,q=2,"
		if !strings.HasPrefix(keys[0], first) {
			t.Errorf("%d bytes: first chunk keys %q lack %q", tt.size, keys[0], first)
		}
		var joined strings.Builder
		for i, p := range payloads {
			if len(p) > kittyChunk {
				t.Errorf("%d bytes: chunk %d is %d bytes, more than %d", tt.size, i, len(p), kittyChunk)
			}
			joined.WriteString(p)
			m := "1" // more chunks follow
			if i == len(payloads)-1 {
				m = "0"
			}
			if i == 0 {
				if !strings.HasSuffix(keys[0], ",m="+m) {
					t.Errorf("%d bytes: first chunk keys %q, want m=%s last", tt.size, keys[0], m)
				}
			} else if want := "m=" + m + ",q=2"; keys[i] != want {
				t.Errorf("%d bytes: chunk %d keys = %q, want %q", tt.size, i, keys[i], want)
			}
		}
		got, err := base64.StdEncoding.DecodeString(joined.String())
		if err != nil {
			t.Fatalf("%d bytes: joined payload is not base64: %v", tt.size, err)
		}
		if !bytes.Equal(got, png) {
			t.Errorf("%d bytes: joined payload does not decode to the input", tt.size)
		}
	}
}

func TestParseKittyProbe(t *testing.T) {
	tests := []struct {
		name            string
		in              string
		supported, done bool
		rest            string
	}{
		{"ok and device attributes", "\x1b_Gi=31;OK\x1b\\\x1b[?62;22c", true, true, ""},
		{"device attributes only", "\x1b[?62c", false, true, ""},
		{"error reply", "\x1b_Gi=31;ENOENT:no such file\x1b\\\x1b[?62c", false, true, ""},
		{"typed keys around the replies", "ab\x1b_Gi=31;OK\x1b\\cd\x1b[?62cef", true, true, "abcdef"},
		{"partial reply kept", "\x1b_Gi=31;O", false, false, "\x1b_Gi=31;O"},
		{"partial device attributes kept", "\x1b[?62;", false, false, "\x1b[?62;"},
		{"lone esc is a key", "\x1b", false, false, "\x1b"},
		{"reply for another image", "\x1b_Gi=7;OK\x1b\\", false, false, ""},
		{"not a reply", "\x1b[A", false, false, "\x1b[A"},
		{"no replies", "xyz", false, false, "xyz"},
		{"empty", "", false, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			supported, done, rest := parseKittyProbe([]byte(tt.in))
			if supported != tt.supported || done != tt.done {
				t.Errorf("supported, done = %v, %v; want %v, %v", supported, done, tt.supported, tt.done)
			}
			if string(rest) != tt.rest {
				t.Errorf("rest = %q, want %q", rest, tt.rest)
			}
		})
	}
}

func TestParseKittyProbeResumes(t *testing.T) {
	// a reply that arrives in two parts is read once the second part comes
	_, done, rest := parseKittyProbe([]byte("\x1b_Gi=31;O"))
	if done {
		t.Fatal("done before the reply ended")
	}
	supported, done, rest := parseKittyProbe(append(rest, "K\x1b\\\x1b[?62c"...))
	if !supported || !done || len(rest) != 0 {
		t.Errorf("resumed probe = %v, %v, %q; want true, true, empty", supported, done, rest)
	}
}

func TestReplyFilterHoldsCap(t *testing.T) {
	var f replyFilter
	// a short unfinished reply is held for its end
	short := "\x1b_G" + strings.Repeat("A", 100)
	if got := f.strip([]byte(short)); len(got) != 0 || len(f.held) != len(short) {
		t.Errorf("short partial reply: out %q, held %d bytes", got, len(f.held))
	}
	// one past the cap is released as ordinary input
	var g replyFilter
	long := "\x1b_G" + strings.Repeat("A", kittyHeldMax)
	if got := g.strip([]byte(long)); string(got) != long || len(g.held) != 0 {
		t.Errorf("over-cap partial reply: out %d bytes, held %d", len(got), len(g.held))
	}
}

func TestKittyFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"forced on", map[string]string{"TUI_KITTY": "1", "TERM": "dumb"}, true},
		{"forced off", map[string]string{"TUI_KITTY": "0", "TERM": "xterm-kitty"}, false},
		{"kitty term", map[string]string{"TERM": "xterm-kitty"}, true},
		{"ghostty term", map[string]string{"TERM": "xterm-ghostty"}, true},
		{"ghostty program", map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "Ghostty"}, true},
		{"rex program", map[string]string{"TERM_PROGRAM": "rex"}, true},
		{"other terminal", map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "Apple_Terminal"}, false},
		{"nothing set", map[string]string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kittyFromEnv(env(tt.env)); got != tt.want {
				t.Errorf("kittyFromEnv = %v, want %v", got, tt.want)
			}
		})
	}
}

// slowSink takes each write one byte at a time, yielding between bytes, so
// writes that overlap show up in its output.
type slowSink struct {
	out []byte
}

func (s *slowSink) Write(p []byte) (int, error) {
	for _, b := range p {
		s.out = append(s.out, b)
		runtime.Gosched()
	}
	return len(p), nil
}

func TestLockedWriterDoesNotInterleave(t *testing.T) {
	sink := &slowSink{}
	lw := &lockedWriter{w: sink}
	const writers, writes, size = 8, 20, 32
	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := bytes.Repeat([]byte{'a' + byte(w)}, size)
			for range writes {
				if n, err := lw.Write(msg); err != nil || n != size {
					t.Errorf("Write = %d, %v", n, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if len(sink.out) != writers*writes*size {
		t.Fatalf("wrote %d bytes, want %d", len(sink.out), writers*writes*size)
	}
	for i := 0; i < len(sink.out); i += size {
		block := sink.out[i : i+size]
		if bytes.Count(block, block[:1]) != size {
			t.Fatalf("write at offset %d interleaved: %q", i, block)
		}
	}
}
