package main

// Still images for the TUI, as PNG bytes: company logos for the experience
// section (a monogram tile when a company has none) and the sprites for the
// 3D pipes screensaver. Everything here is a pure function, so the bytes can
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

// --- pipe sprites ---

// pipePiece is one cell of a pipe run: a straight, a corner, or a ball joint.
type pipePiece uint8

const (
	pipeH      pipePiece = iota // left-right
	pipeV                       // up-down
	pipeNE                      // joins up and right (like └ )
	pipeNW                      // joins up and left (┘)
	pipeSE                      // joins down and right (┌)
	pipeSW                      // joins down and left (┐)
	pipeCap                     // a lone ball joint: a pipe that cannot move
	pipeEndN                    // a pipe's first or last cell: a ball on a stub up
	pipeEndE                    // ... a stub right
	pipeEndS                    // ... a stub down
	pipeEndW                    // ... a stub left
	pipePieces                  // count
)

// pipeColors are six saturated, XP-like pipe colours.
var pipeColors = []rgb{
	hexRGB("#d6453d"), // red
	hexRGB("#3ea85e"), // green
	hexRGB("#3b6fc4"), // blue
	hexRGB("#e2ae2c"), // yellow
	hexRGB("#2aa6b8"), // cyan
	hexRGB("#a24fb5"), // magenta
}

// sprite geometry, in pixels. A cell's centre is (12, 24).
const (
	spriteW, spriteH = 24, 48
	tubeHalf         = 9.0  // a tube is 18 px thick both ways
	jointR           = 11.0 // a corner's sphere, about 22 px across
	capR             = 11.0 // an end cap's sphere, the same size
)

// arm bits: the cell edges a piece reaches. Direction d is the bit 1<<d.
const (
	armN uint8 = 1 << iota
	armE
	armS
	armW
)

var pipeArms = [pipePieces]uint8{
	pipeH:    armE | armW,
	pipeV:    armN | armS,
	pipeNE:   armN | armE,
	pipeNW:   armN | armW,
	pipeSE:   armS | armE,
	pipeSW:   armS | armW,
	pipeEndN: armN,
	pipeEndE: armE,
	pipeEndS: armS,
	pipeEndW: armW,
}

// the light sits upper left and in front; the highlight is where the surface
// normal meets the half vector between light and eye. A faint bounce comes
// from the lower right, so a tube's shadowed side is not black.
var (
	lightDir  = unit3(-0.5, -0.55, 0.67)
	lightHalf = unit3(-0.5, -0.55, 1.67)
	fillDir   = unit3(0.5, 0.6, 0.6)
)

// how tight a highlight is: a tube's is a broad streak, a sphere's a spot.
const (
	tubeGloss = 12.0
	ballGloss = 24.0
)

func unit3(x, y, z float64) [3]float64 {
	l := math.Sqrt(x*x + y*y + z*z)
	return [3]float64{x / l, y / l, z / l}
}

func dot3(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// litPipe shades a surface like glossy enamel: diffuse from the upper left, so
// the far edges fall dark, a bounce from the lower right lifts that dark edge,
// and a white highlight sits just off centre, up and left.
func litPipe(c rgb, n [3]float64, gloss float64) rgb {
	d := clamp01(dot3(n, lightDir))
	fill := clamp01(dot3(n, fillDir))
	s := math.Pow(clamp01(dot3(n, lightHalf)), gloss)
	v := 0.16 + 0.8*d + 0.14*fill
	return rgb{c.r * v, c.g * v, c.b * v}.mix(rgb{255, 255, 255}, 0.7*s)
}

// tubeAt reports whether a tube covers pixel offset (dx, dy) from the cell
// centre, and the surface normal there. A tube shades by its cross coordinate
// only, so neighbouring sprites meet without a seam. Where two arms overlap in
// a corner, the arm nearer its own axis wins, which makes a clean mitre.
func tubeAt(arms uint8, dx, dy float64) (bool, [3]float64) {
	inV, inH := math.Abs(dx) < tubeHalf, math.Abs(dy) < tubeHalf
	vert := inV && (arms&armN != 0 && dy < 0 || arms&armS != 0 && dy > 0)
	horz := inH && (arms&armE != 0 && dx > 0 || arms&armW != 0 && dx < 0)
	if vert && horz {
		vert = math.Abs(dy) >= math.Abs(dx)
		horz = !vert
	}
	switch {
	case vert:
		c := dx / tubeHalf
		return true, [3]float64{c, 0, math.Sqrt(1 - c*c)}
	case horz:
		c := dy / tubeHalf
		return true, [3]float64{0, c, math.Sqrt(1 - c*c)}
	}
	return false, [3]float64{}
}

// pipePixel is the colour and coverage of the sprite at pixel centre (px, py).
// ball is the radius of the joint sphere at the centre, 0 for none.
func pipePixel(arms uint8, ball, px, py float64, c rgb) (rgb, float64) {
	dx, dy := px-spriteW/2, py-spriteH/2
	tube, n := tubeAt(arms, dx, dy)
	if ball > 0 {
		if cov := clamp01(ball - math.Hypot(dx, dy) + 0.5); cov > 0 && (cov >= 1 || !tube) {
			nx, ny := dx/ball, dy/ball
			return litPipe(c, [3]float64{nx, ny, math.Sqrt(math.Max(0, 1-nx*nx-ny*ny))}, ballGloss), cov
		}
	}
	if tube {
		return litPipe(c, n, tubeGloss), 1
	}
	return rgb{}, 0
}

// pipeSpritePNG renders one piece in one colour as a 24x48 RGBA PNG with a
// transparent background. Tubes reach the cell edges they connect to, so
// neighbouring sprites join seamlessly. Corners and caps add a shaded sphere.
func pipeSpritePNG(piece pipePiece, color int) []byte {
	if piece >= pipePieces {
		return nil
	}
	n := len(pipeColors)
	c := pipeColors[((color%n)+n)%n]
	ball := 0.0
	switch {
	case piece == pipeCap || piece >= pipeEndN:
		ball = capR
	case piece >= pipeNE && piece <= pipeSW:
		ball = jointR
	}
	img := image.NewNRGBA(image.Rect(0, 0, spriteW, spriteH))
	for y := 0; y < spriteH; y++ {
		for x := 0; x < spriteW; x++ {
			col, a := pipePixel(pipeArms[piece], ball, float64(x)+0.5, float64(y)+0.5, c)
			img.SetNRGBA(x, y, nrgba(stepped(col), a))
		}
	}
	return palettePNG(img)
}

// --- encoding ---

func byteOf(v float64) uint8 { return uint8(clampf(math.Round(v), 0, 255)) }

// stepped rounds each channel to a multiple of 8. Steps of 4 would be smoother,
// but take the sprites past 65 KB; at 8 the banding is hard to see in a terminal.
func stepped(c rgb) rgb {
	q := func(v float64) float64 { return math.Min(math.Round(v/8)*8, 255) }
	return rgb{q(c.r), q(c.g), q(c.b)}
}

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

// pipeSpriteID is the kitty image id of the pipe sprite for colour c and piece
// p. The ids start after the experience ids, so they never overlap.
func pipeSpriteID(c int, p pipePiece) int { return 32 + c*int(pipePieces) + int(p) }

// imageUploads is every image the TUI can show, as kitty upload escapes, one
// image apiece. It is built once per process; a session that speaks the
// protocol is sent it once, an image a write.
var imageUploads = sync.OnceValue(func() [][]byte {
	var out [][]byte
	for i, e := range experiences {
		out = append(out, kittyUpload(uint8(expImageID(i)), experienceImagePNG(e.company), expLogoCols, expLogoRows))
	}
	for c := range pipeColors {
		for p := pipePiece(0); p < pipePieces; p++ {
			out = append(out, kittyUpload(uint8(pipeSpriteID(c, p)), pipeSpritePNG(p, c), 1, 1))
		}
	}
	return out
})
