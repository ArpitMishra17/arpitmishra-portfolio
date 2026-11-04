package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
)

// The kitty graphics protocol, for the parts the TUI uses: a PNG sent in one
// go, shown through Unicode placeholders so it moves with the text around it.
// See https://sw.kovidgoyal.net/kitty/graphics-protocol/

// kittyPlaceholder marks a cell that shows part of an image.
const kittyPlaceholder = '\U0010EEEE'

// kittyDiacritics[i] encodes row or column index i in a placeholder cell. The
// list is the rowcolumn-diacritics.txt table from the kitty source.
var kittyDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
	0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
	0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F, 0x0483, 0x0484,
	0x0485, 0x0486, 0x0487, 0x0592, 0x0593, 0x0594, 0x0595, 0x0597,
	0x0598, 0x0599, 0x059C, 0x059D, 0x059E, 0x059F, 0x05A0, 0x05A1,
	0x05A8, 0x05A9, 0x05AB, 0x05AC, 0x05AF, 0x05C4, 0x0610, 0x0611,
	0x0612, 0x0613, 0x0614, 0x0615, 0x0616, 0x0617, 0x0657, 0x0658,
	0x0659, 0x065A, 0x065B, 0x065D, 0x065E, 0x06D6, 0x06D7, 0x06D8,
	0x06D9, 0x06DA, 0x06DB, 0x06DC, 0x06DF, 0x06E0, 0x06E1, 0x06E2,
	0x06E4, 0x06E7, 0x06E8, 0x06EB, 0x06EC, 0x0730, 0x0732, 0x0733,
	0x0735, 0x0736, 0x073A, 0x073D, 0x073F, 0x0740, 0x0741, 0x0743,
	0x0745, 0x0747, 0x0749, 0x074A, 0x07EB, 0x07EC, 0x07ED, 0x07EE,
	0x07EF, 0x07F0, 0x07F1, 0x07F3, 0x0816, 0x0817, 0x0818, 0x0819,
	0x081B, 0x081C, 0x081D, 0x081E, 0x081F, 0x0820, 0x0821, 0x0822,
	0x0823, 0x0825, 0x0826, 0x0827, 0x0829, 0x082A, 0x082B, 0x082C,
	0x082D, 0x0951, 0x0953, 0x0954, 0x0F82, 0x0F83, 0x0F86, 0x0F87,
	0x135D, 0x135E, 0x135F, 0x17DD, 0x193A, 0x1A17, 0x1A75, 0x1A76,
	0x1A77, 0x1A78, 0x1A79, 0x1A7A, 0x1A7B, 0x1A7C, 0x1B6B, 0x1B6D,
	0x1B6E, 0x1B6F, 0x1B70, 0x1B71, 0x1B72, 0x1B73, 0x1CD0, 0x1CD1,
	0x1CD2, 0x1CDA, 0x1CDB, 0x1CE0, 0x1DC0, 0x1DC1, 0x1DC3, 0x1DC4,
	0x1DC5, 0x1DC6, 0x1DC7, 0x1DC8, 0x1DC9, 0x1DCB, 0x1DCC, 0x1DD1,
	0x1DD2, 0x1DD3, 0x1DD4, 0x1DD5, 0x1DD6, 0x1DD7, 0x1DD8, 0x1DD9,
	0x1DDA, 0x1DDB, 0x1DDC, 0x1DDD, 0x1DDE, 0x1DDF, 0x1DE0, 0x1DE1,
	0x1DE2, 0x1DE3, 0x1DE4, 0x1DE5, 0x1DE6, 0x1DFE, 0x20D0, 0x20D1,
	0x20D4, 0x20D5, 0x20D6, 0x20D7, 0x20DB, 0x20DC, 0x20E1, 0x20E7,
	0x20E9, 0x20F0, 0x2CEF, 0x2CF0, 0x2CF1, 0x2DE0, 0x2DE1, 0x2DE2,
	0x2DE3, 0x2DE4, 0x2DE5, 0x2DE6, 0x2DE7, 0x2DE8, 0x2DE9, 0x2DEA,
	0x2DEB, 0x2DEC, 0x2DED, 0x2DEE, 0x2DEF, 0x2DF0, 0x2DF1, 0x2DF2,
	0x2DF3, 0x2DF4, 0x2DF5, 0x2DF6, 0x2DF7, 0x2DF8, 0x2DF9, 0x2DFA,
	0x2DFB, 0x2DFC, 0x2DFD, 0x2DFE, 0x2DFF, 0xA66F, 0xA67C, 0xA67D,
	0xA6F0, 0xA6F1, 0xA8E0, 0xA8E1, 0xA8E2, 0xA8E3, 0xA8E4, 0xA8E5,
	0xA8E6, 0xA8E7, 0xA8E8, 0xA8E9, 0xA8EA, 0xA8EB, 0xA8EC, 0xA8ED,
	0xA8EE, 0xA8EF, 0xA8F0, 0xA8F1, 0xAAB0, 0xAAB2, 0xAAB3, 0xAAB7,
	0xAAB8, 0xAABE, 0xAABF, 0xAAC1, 0xFE20, 0xFE21, 0xFE22, 0xFE23,
	0xFE24, 0xFE25, 0xFE26, 0x10A0F, 0x10A38, 0x1D185, 0x1D186, 0x1D187,
	0x1D188, 0x1D189, 0x1D1AA, 0x1D1AB, 0x1D1AC, 0x1D1AD, 0x1D242, 0x1D243,
	0x1D244,
}

// kittyChunk is the most base64 bytes one graphics escape may carry.
const kittyChunk = 4096

// kittyHeldMax caps how many bytes of an unfinished reply are held back while
// waiting for the rest of it. Past this, the bytes are treated as typed keys.
const kittyHeldMax = 256

// kittyQuery asks whether the terminal speaks the graphics protocol. Its APC
// reply comes back only if it does, so the device attributes request after it
// marks the end: every terminal answers that.
const kittyQuery = "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\\x1b[c"

// kittyBlock returns rows lines of cols placeholder cells for image id. Each
// line sets the foreground to the id, so the placeholders show that image, and
// each cell names its row and column with diacritics. Sizes past the table
// are cut to the table's length.
func kittyBlock(id uint8, cols, rows int) []string {
	rows = min(max(rows, 0), len(kittyDiacritics))
	cols = min(max(cols, 0), len(kittyDiacritics))
	lines := make([]string, 0, rows)
	for r := range rows {
		var b strings.Builder
		fmt.Fprintf(&b, "\x1b[38;5;%dm", id)
		for c := range cols {
			b.WriteRune(kittyPlaceholder)
			b.WriteRune(kittyDiacritics[r])
			b.WriteRune(kittyDiacritics[c])
		}
		b.WriteString("\x1b[39m")
		lines = append(lines, b.String())
	}
	return lines
}

// kittyCell is one placeholder cell for row 0, column 0, without colour codes.
// Callers that batch runs of one image id add those codes themselves.
func kittyCell() string {
	return string(kittyPlaceholder) + string(kittyDiacritics[0]) + string(kittyDiacritics[0])
}

// kittyUpload returns the escapes that send png (PNG bytes) as image id, and
// make a virtual placement of cols x rows cells for the placeholders to show.
// The image is sent in chunks of at most kittyChunk base64 bytes. Only the
// first escape carries the full set of keys; each later one carries m and q.
func kittyUpload(id uint8, png []byte, cols, rows int) []byte {
	data := base64.StdEncoding.EncodeToString(png)
	var out []byte
	for start := 0; ; start += kittyChunk {
		end := min(start+kittyChunk, len(data))
		more := 0 // m=1 means more chunks follow
		if end < len(data) {
			more = 1
		}
		keys := fmt.Sprintf("m=%d,q=2", more)
		if start == 0 {
			// a=T transmits and displays, U=1 makes the placement virtual,
			// q=2 suppresses the reply
			keys = fmt.Sprintf("a=T,U=1,f=100,t=d,i=%d,c=%d,r=%d,q=2,m=%d", id, cols, rows, more)
		}
		out = append(out, "\x1b_G"...)
		out = append(out, keys...)
		out = append(out, ';')
		out = append(out, data[start:end]...)
		out = append(out, "\x1b\\"...)
		if more == 0 {
			return out
		}
	}
}

// parseKittyProbe reads input received after kittyQuery. supported is set by
// an APC reply to the query that says OK. done is set by the device attributes
// reply, which ends the probe. rest is b without the replies, so keys typed
// meanwhile are kept. A reply cut off at the end of b stays in rest, so a
// later call with the rest of it can finish it.
func parseKittyProbe(b []byte) (supported, done bool, rest []byte) {
	rest = make([]byte, 0, len(b))
	for i := 0; i < len(b); {
		j := bytes.IndexByte(b[i:], 0x1b)
		if j < 0 {
			return supported, done, append(rest, b[i:]...)
		}
		rest = append(rest, b[i:i+j]...)
		i += j
		n, incomplete := replyLen(b[i:])
		switch {
		case n > 0:
			reply := b[i : i+n]
			if bytes.HasPrefix(reply, []byte("\x1b_G")) {
				supported = supported || kittyOKReply(reply)
			} else {
				done = true
			}
			i += n
		case incomplete:
			return supported, done, append(rest, b[i:]...)
		default:
			rest = append(rest, b[i])
			i++
		}
	}
	return supported, done, rest
}

// kittyOKReply reports whether reply, an APC graphics reply, is "OK" for the
// probe's image id.
func kittyOKReply(reply []byte) bool {
	body := string(reply[len("\x1b_G") : len(reply)-len("\x1b\\")])
	keys, payload, ok := strings.Cut(body, ";")
	return ok && payload == "OK" && strings.Contains(","+keys+",", ",i=31,")
}

// replyLen finds a terminal reply at the start of b. That is a kitty graphics
// reply, "\x1b_G" up to "\x1b\\", or a device attributes reply, "\x1b[?" then
// digits and semicolons then "c". It returns the reply's length. If b holds
// only the start of a reply, it returns 0 and incomplete true, unless b is
// longer than kittyHeldMax, which makes it ordinary input.
func replyLen(b []byte) (n int, incomplete bool) {
	switch {
	case bytes.HasPrefix(b, []byte("\x1b_G")):
		end := bytes.Index(b[len("\x1b_G"):], []byte("\x1b\\"))
		if end >= 0 {
			return len("\x1b_G") + end + len("\x1b\\"), false
		}
	case bytes.HasPrefix(b, []byte("\x1b[?")):
		for i := len("\x1b[?"); i < len(b); i++ {
			switch {
			case b[i] == 'c':
				return i + 1, false
			case b[i] == ';' || ('0' <= b[i] && b[i] <= '9'):
			default:
				return 0, false
			}
		}
	default:
		return 0, false
	}
	return 0, len(b) <= kittyHeldMax
}

// replyFilter strips terminal replies from client input, so a late reply
// doesn't reach bubbletea as key presses. A reply cut off at the end of a
// chunk is held until the next chunk arrives.
type replyFilter struct {
	held []byte
}

// strip returns in without the complete replies in it.
func (f *replyFilter) strip(in []byte) []byte {
	if len(f.held) > 0 {
		in = append(f.held, in...)
		f.held = nil
	}
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); {
		j := bytes.IndexByte(in[i:], 0x1b)
		if j < 0 {
			return append(out, in[i:]...)
		}
		out = append(out, in[i:i+j]...)
		i += j
		n, incomplete := replyLen(in[i:])
		switch {
		case n > 0:
			i += n
		case incomplete:
			f.held = append([]byte(nil), in[i:]...)
			return out
		default:
			out = append(out, in[i])
			i++
		}
	}
	return out
}

// kittyFromEnv decides from the environment whether to use the protocol.
// TUI_KITTY=1 forces it on and TUI_KITTY=0 forces it off. Otherwise it is on
// for the terminals that speak it.
func kittyFromEnv(getenv func(string) string) bool {
	switch getenv("TUI_KITTY") {
	case "1":
		return true
	case "0":
		return false
	}
	term := getenv("TERM")
	prog := getenv("TERM_PROGRAM")
	return strings.EqualFold(term, "xterm-kitty") ||
		strings.EqualFold(term, "xterm-ghostty") ||
		strings.EqualFold(prog, "ghostty") ||
		strings.EqualFold(prog, "rex")
}

// lockedWriter serialises writes to w, so one write never interleaves with
// another, such as an image upload with a frame.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
