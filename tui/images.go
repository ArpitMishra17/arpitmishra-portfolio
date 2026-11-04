package main

// Still images for the TUI, as PNG bytes: company logos for the experience
// section (a monogram tile when a company has none). Everything here is a pure function, so the bytes can
// be built once and reused.

import (
	"bytes"
	"cmp"
	"embed"
	"image"
	"image/color"
	"image/png"
	"math"
	"slices"
	"strings"
	"sync"
)

//go:embed assets/logos/*.png
var logoFS embed.FS

// logoSlug is the lowercase letters and digits of a company's first word,
// which names its logo file: "Secuodsoft Technologies" -> "secuodsoft".
func logoSlug(company string) string {
	fields := strings.Fields(company)
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	for _, r := range strings.ToLower(fields[0]) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// companyLogoPNG returns the embedded logo for a company, cut to the tile's
// rounded corners, or nil if it has none.
func companyLogoPNG(company string) []byte {
	slug := logoSlug(company)
	if slug == "" {
		return nil
	}
	b, err := logoFS.ReadFile("assets/logos/" + slug + ".png")
	if err != nil {
		return nil
	}
	return roundTile(b)
}

// roundTile cuts the corners of a PNG with the tile's rounded square, so a logo
// has the same outline as a monogram. Pixels outside the square are transparent.
func roundTile(src []byte) []byte {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			_, cover := tileEdge(x, y, b.Dx(), b.Dy())
			c.A = uint8(math.Round(float64(c.A) * cover))
			out.SetNRGBA(x, y, c)
		}
	}
	return palettePNG(out)
}

// experienceImagePNG is the logo for a company, or its monogram tile.
func experienceImagePNG(company string) []byte {
	if b := companyLogoPNG(company); b != nil {
		return b
	}
	return monogramPNG(company)
}

// --- monograms ---

var (
	monoTile   = hexRGB("#2b3036")
	monoEdge   = hexRGB("#48505a")
	monoLetter = hexRGB("#c9d1d9")
)

// monoRune is the company's first character, upper-cased, if the pixel font
// has it; anything else is '?'.
func monoRune(company string) rune {
	r := []rune(strings.ToUpper(strings.TrimSpace(company)))
	if len(r) > 0 {
		if _, ok := pixelFont[r[0]]; ok {
			return r[0]
		}
	}
	return '?'
}

// tileRadius is the corner radius of a tile on a 128 px square. Logos and
// monograms share it, so every tile has one outline.
const tileRadius = 22.0

// tileEdge measures pixel (x, y) of a w x h tile with rounded corners. sd is the
// signed distance from the pixel's centre to the edge, negative inside, and
// cover is how much of the pixel lies inside.
func tileEdge(x, y, w, h int) (sd, cover float64) {
	hw, hh := float64(w)/2, float64(h)/2
	qx := math.Abs(float64(x)+0.5-hw) - (hw - tileRadius)
	qy := math.Abs(float64(y)+0.5-hh) - (hh - tileRadius)
	sd = math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - tileRadius
	return sd, clamp01(0.5 - sd)
}

// monogramPNG draws the tile for a company without a logo: a dark rounded
// square with a light outline, and the company's first letter in the pixel
// font, scaled up and centred. Corners are transparent.
func monogramPNG(company string) []byte {
	const (
		n     = 128
		scale = 10
	)
	gw, gh := 5*scale, 7*scale
	gx0, gy0 := (n-gw)/2, (n-gh)/2
	glyph := pixelFont[monoRune(company)]
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			sd, cover := tileEdge(x, y, n, n)
			if cover == 0 {
				continue
			}
			col := monoTile.mix(monoEdge, clamp01(1+sd)) // a 1px outline inside the edge
			if gx, gy := x-gx0, y-gy0; gx >= 0 && gy >= 0 && gx < gw && gy < gh && glyph[gy/scale][gx/scale] == '#' {
				col = monoLetter
			}
			img.SetNRGBA(x, y, nrgba(col, cover))
		}
	}
	return palettePNG(img)
}

// pixelFont is a 5x7 bitmap font for A-Z, 0-9 and '?'; '#' is ink.
var pixelFont = map[rune][7]string{
	'A': {".###.", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'B': {"####.", "#...#", "#...#", "####.", "#...#", "#...#", "####."},
	'C': {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
	'D': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'E': {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	'F': {"#####", "#....", "#....", "####.", "#....", "#....", "#...."},
	'G': {".###.", "#...#", "#....", "#.###", "#...#", "#...#", ".####"},
	'H': {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'I': {".###.", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'J': {"..###", "...#.", "...#.", "...#.", "...#.", "#..#.", ".##.."},
	'K': {"#...#", "#..#.", "#.#..", "##...", "#.#..", "#..#.", "#...#"},
	'L': {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'M': {"#...#", "##.##", "#.#.#", "#.#.#", "#...#", "#...#", "#...#"},
	'N': {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
	'O': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'P': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'Q': {".###.", "#...#", "#...#", "#...#", "#.#.#", "#..#.", ".##.#"},
	'R': {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	'S': {".####", "#....", "#....", ".###.", "....#", "....#", "####."},
	'T': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U': {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'V': {"#...#", "#...#", "#...#", "#...#", "#...#", ".#.#.", "..#.."},
	'W': {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "#...#"},
	'X': {"#...#", "#...#", ".#.#.", "..#..", ".#.#.", "#...#", "#...#"},
	'Y': {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#..", "..#.."},
	'Z': {"#####", "....#", "...#.", "..#..", ".#...", "#....", "#####"},
	'0': {".###.", "#...#", "#..##", "#.#.#", "##..#", "#...#", ".###."},
	'1': {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'2': {".###.", "#...#", "....#", "...#.", "..#..", ".#...", "#####"},
	'3': {"#####", "...#.", "..#..", "...#.", "....#", "#...#", ".###."},
	'4': {"...#.", "..##.", ".#.#.", "#..#.", "#####", "...#.", "...#."},
	'5': {"#####", "#....", "####.", "....#", "....#", "#...#", ".###."},
	'6': {"..##.", ".#...", "#....", "####.", "#...#", "#...#", ".###."},
	'7': {"#####", "....#", "...#.", "..#..", ".#...", ".#...", ".#..."},
	'8': {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###."},
	'9': {".###.", "#...#", "#...#", ".####", "....#", "...#.", ".##.."},
	'?': {".###.", "#...#", "....#", "...#.", "..#..", ".....", "..#.."},
}

// --- encoding ---

func byteOf(v float64) uint8 { return uint8(clampf(math.Round(v), 0, 255)) }

func nrgba(c rgb, a float64) color.NRGBA {
	if a <= 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{byteOf(c.r), byteOf(c.g), byteOf(c.b), byteOf(a * 255)}
}

// palettePNG encodes img as an indexed PNG when it has at most 256 distinct
// colours, which keeps sprites small; otherwise it encodes RGBA. The palette runs
// from transparent to opaque, and dark to light, so neighbouring shades get
// neighbouring indices and the rows compress better.
func palettePNG(img *image.NRGBA) []byte {
	b := img.Bounds()
	seen := map[color.NRGBA]bool{}
	var colours []color.NRGBA
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			if c.A == 0 {
				c = color.NRGBA{} // all transparent pixels share one entry
			}
			if !seen[c] {
				if len(colours) == 256 {
					return encodePNG(img)
				}
				seen[c] = true
				colours = append(colours, c)
			}
		}
	}
	slices.SortFunc(colours, func(a, b color.NRGBA) int {
		if d := cmp.Compare(a.A, b.A); d != 0 {
			return d
		}
		return cmp.Compare(brightness(a), brightness(b))
	})
	pal := make(color.Palette, len(colours))
	index := make(map[color.NRGBA]uint8, len(colours))
	for i, c := range colours {
		pal[i] = c
		index[c] = uint8(i)
	}
	out := image.NewPaletted(b, pal)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			if c.A == 0 {
				c = color.NRGBA{}
			}
			out.SetColorIndex(x, y, index[c])
		}
	}
	return encodePNG(out)
}

// brightness is the Rec. 709 luma of a colour, from 0 to 255.
func brightness(c color.NRGBA) float64 {
	return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// --- kitty ids and uploads ---

// expLogoCols and expLogoRows size an experience logo in cells.
const expLogoCols, expLogoRows = 4, 2

// expImageID is the kitty image id of experience i: 1 to 8 for the eight roles.
func expImageID(i int) int { return 1 + i }

// imageUploads is every image the TUI can show, as kitty upload escapes. It is
// built once per process; a session that speaks the protocol is sent it once.
var imageUploads = sync.OnceValue(func() []byte {
	var out []byte
	for i, e := range experiences {
		out = append(out, kittyUpload(uint8(expImageID(i)), experienceImagePNG(e.company), expLogoCols, expLogoRows)...)
	}
	return out
})
