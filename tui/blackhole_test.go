package main

import (
	"os"
	"strings"
	"testing"

	"github.com/muesli/termenv"
)

// the text black hole is what it was before the pixel version: the same
// frames at fixed times, in several themes and without colour.
func TestBlackHoleASCIIUnchanged(t *testing.T) {
	var b strings.Builder
	for _, th := range []int{0, 4, 2} {
		for _, tt := range []float64{0, 7.3, 31.9} {
			b.WriteString("== theme " + themes[th].name + "\n")
			b.WriteString(blackHole(sidebarW-2, 9, tt, paletteOf(themes[th])).render(termenv.TrueColor))
			b.WriteString("\n")
		}
	}
	b.WriteString("== ascii\n")
	b.WriteString(blackHole(sidebarW-2, 9, 7.3, paletteOf(themes[0])).render(termenv.Ascii))
	b.WriteString("\n")
	want, err := os.ReadFile("testdata/blackhole.golden")
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != string(want) {
		t.Fatal("the text black hole changed")
	}
}
