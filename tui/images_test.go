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
