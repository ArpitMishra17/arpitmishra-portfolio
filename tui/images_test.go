package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func decodePNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}

// pixelAt is the pixel as straight (non-premultiplied) RGBA, whatever the PNG's colour type.
func pixelAt(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func TestPipeSpritesDecode(t *testing.T) {
	for p := pipePiece(0); p < pipePieces; p++ {
		for c := range pipeColors {
			img := decodePNG(t, pipeSpritePNG(p, c))
			if b := img.Bounds(); b.Dx() != spriteW || b.Dy() != spriteH {
				t.Fatalf("piece %d colour %d: bounds %v", p, c, b)
			}
			for _, pt := range [][2]int{{0, 0}, {spriteW - 1, 0}, {0, spriteH - 1}, {spriteW - 1, spriteH - 1}} {
				if p == pipeH && pixelAt(img, pt[0], pt[1]).A != 0 {
					t.Fatalf("pipeH corner %v is not transparent", pt)
				}
			}
			if p == pipeCap || p >= pipeEndN || (p >= pipeNE && p <= pipeSW) {
				if pixelAt(img, spriteW/2, spriteH/2).A != 255 {
					t.Fatalf("piece %d: joint at the centre is not opaque", p)
				}
			}
		}
	}
	if pipeSpritePNG(pipePieces, 0) != nil {
		t.Fatal("out-of-range piece should give nil")
	}
}

func TestPipeSpriteBudget(t *testing.T) {
	total := 0
	for p := pipePiece(0); p < pipePieces; p++ {
		for c := range pipeColors {
			total += len(pipeSpritePNG(p, c))
		}
	}
	if total > 55*1024 {
		t.Fatalf("sprites total %d bytes, want under 55 KB", total)
	}
}

// the joins: a horizontal tube's right edge matches a corner's right edge, and a
// vertical tube's bottom edge matches a corner's bottom edge, pixel for pixel.
func TestPipeSpriteJoints(t *testing.T) {
	for c := range pipeColors {
		h := decodePNG(t, pipeSpritePNG(pipeH, c))
		v := decodePNG(t, pipeSpritePNG(pipeV, c))
		se := decodePNG(t, pipeSpritePNG(pipeSE, c))
		sw := decodePNG(t, pipeSpritePNG(pipeSW, c))
		for y := 0; y < spriteH; y++ {
			if a, b := pixelAt(h, spriteW-1, y), pixelAt(se, spriteW-1, y); a != b {
				t.Fatalf("colour %d row %d: pipeH right edge %v, pipeSE %v", c, y, a, b)
			}
		}
		for x := 0; x < spriteW; x++ {
			if a, b := pixelAt(v, x, spriteH-1), pixelAt(sw, x, spriteH-1); a != b {
				t.Fatalf("colour %d col %d: pipeV bottom edge %v, pipeSW %v", c, x, a, b)
			}
		}
	}
}

func TestLogoSlug(t *testing.T) {
	cases := map[string]string{
		"Vyazen":                  "vyazen",
		"Secuodsoft Technologies": "secuodsoft",
		"  NALCO  ":               "nalco",
		"C++ Labs":                "c",
		"KJSSE (IIT)":             "kjsse",
		"":                        "",
		"   ":                     "",
	}
	for in, want := range cases {
		if got := logoSlug(in); got != want {
			t.Errorf("logoSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompanyLogo(t *testing.T) {
	b := companyLogoPNG("Vyazen")
	if b == nil {
		t.Fatal("no embedded logo for Vyazen")
	}
	if img := decodePNG(t, b); img.Bounds().Dx() != 128 || img.Bounds().Dy() != 128 {
		t.Fatalf("logo bounds %v, want 128x128", img.Bounds())
	}
	if companyLogoPNG("Nobody Inc") != nil {
		t.Fatal("unknown company should have no logo")
	}
}

func TestMonogram(t *testing.T) {
	a := monogramPNG("Alpha")
	img := decodePNG(t, a)
	if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 128 {
		t.Fatalf("monogram bounds %v, want 128x128", b)
	}
	if pixelAt(img, 0, 0).A != 0 {
		t.Fatal("monogram corner should be transparent")
	}
	if pixelAt(img, 64, 64).A != 255 {
		t.Fatal("monogram centre should be opaque")
	}
	if bytes.Equal(a, monogramPNG("Beta")) {
		t.Fatal("monograms for different letters should differ")
	}
	decodePNG(t, monogramPNG("ünïcode 9"))
	decodePNG(t, monogramPNG(""))
}

func TestExperienceImages(t *testing.T) {
	for _, e := range experiences {
		b := experienceImagePNG(e.company)
		if b == nil {
			t.Errorf("no image for %s", e.company)
			continue
		}
		decodePNG(t, b)
	}
}

// a logo and a monogram share one outline: the same pixels are opaque.
func TestLogoTileShape(t *testing.T) {
	logo := decodePNG(t, companyLogoPNG("Vyazen"))
	mono := decodePNG(t, monogramPNG("Vyazen"))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			if a, b := pixelAt(logo, x, y).A, pixelAt(mono, x, y).A; a != b {
				t.Fatalf("pixel (%d,%d): logo alpha %d, monogram alpha %d", x, y, a, b)
			}
		}
	}
	if pixelAt(logo, 0, 0).A != 0 {
		t.Fatal("logo corner should be transparent")
	}
}
