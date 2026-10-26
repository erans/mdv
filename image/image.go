// Package image paints a local picture where a terminal can show one.
//
// Kitty graphics, iTerm2's OSC 1337 and sixel are the protocols on offer; half
// blocks are the fallback that works in every truecolor terminal.
package image

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	// decoders
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	xdraw "golang.org/x/image/draw"
)

// Protocol is how an image reaches the terminal.
type Protocol string

const (
	Auto      Protocol = "auto"
	Kitty     Protocol = "kitty"
	ITerm2    Protocol = "iterm2"
	Sixel     Protocol = "sixel"
	Halfblock Protocol = "halfblock"
	Off       Protocol = "off"
)

// Parse turns a -images flag value into a protocol.
func Parse(s string) (Protocol, error) {
	switch Protocol(strings.ToLower(s)) {
	case Auto, Kitty, ITerm2, Sixel, Halfblock, Off:
		return Protocol(strings.ToLower(s)), nil
	}
	return "", fmt.Errorf("unknown image mode %q (auto, kitty, iterm2, sixel, halfblock, off)", s)
}

// Detect guesses the protocol from the environment. tmux and screen swallow
// graphics, so they get the fallback.
func Detect() Protocol {
	env := func(k string) string { return strings.ToLower(os.Getenv(k)) }
	term := env("TERM")
	program := env("TERM_PROGRAM")
	if env("TMUX") != "" || strings.HasPrefix(term, "tmux") || strings.HasPrefix(term, "screen") {
		return Halfblock
	}
	switch {
	case env("KITTY_WINDOW_ID") != "", program == "kitty", program == "ghostty",
		strings.Contains(term, "ghostty"), env("GHOSTTY_RESOURCES_DIR") != "",
		env("WEZTERM_PANE") != "", program == "wezterm",
		program == "warpterminal", env("WARP_SESSION_ID") != "":
		return Kitty
	case env("ITERM_SESSION_ID") != "", program == "iterm.app":
		return ITerm2
	case strings.Contains(term, "sixel"), env("XTERM_SIXEL") != "", env("MLTERM") != "":
		return Sixel
	}
	return Halfblock
}

// Block is a rendered image: the text to write, already carrying the newlines
// that reserve its Rows rows on screen.
type Block struct {
	Text string
	Rows int
}

// Cell pixel size. The terminal is asked at startup; 10x20 is the assumption
// when it does not answer. Only the ratio affects how an image is laid out.
var (
	cellW, cellH = 10, 20
)

// SetCellSize records the terminal's cell geometry, ignoring values that are
// plainly wrong.
func SetCellSize(w, h int) {
	if w >= 3 && w <= 80 && h >= 6 && h <= 160 {
		cellW, cellH = w, h
	}
}

// CellSize is the geometry in use, in pixels.
func CellSize() (w, h int) { return cellW, cellH }

// Render paints the image at path into cols columns. maxRows caps the height;
// zero means only the width limits it.
func Render(path string, proto Protocol, cols, maxRows int) (Block, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Block{}, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Block{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if cols < 4 {
		cols = 4
	}
	bounds := img.Bounds()
	rows := rowsFor(bounds.Dx(), bounds.Dy(), cols)
	if maxRows > 0 && rows > maxRows {
		rows = maxRows
		cols = colsFor(bounds.Dx(), bounds.Dy(), rows)
	}
	switch proto {
	case Off:
		return Block{}, fmt.Errorf("images off")
	case Kitty:
		return Block{Text: encodeKitty(pngBase64(img), cols, rows) + reserve(rows), Rows: rows}, nil
	case ITerm2:
		return Block{Text: encodeITerm2(pngBase64(img), cols, rows) + reserve(rows), Rows: rows}, nil
	case Sixel:
		return Block{Text: encodeSixel(resize(img, cols*cellW, rows*cellH)) + reserve(rows), Rows: rows}, nil
	default:
		lines := halfblocks(resize(img, cols, rows*2))
		return Block{Text: strings.Join(lines, "\n") + "\n", Rows: rows}, nil
	}
}

// reserve is the newlines that make the terminal's own image occupy its rows:
// the protocols paint over the blank lines that follow the escape.
func reserve(rows int) string {
	return strings.Repeat("\n", max(0, rows-1))
}

func rowsFor(w, h, cols int) int {
	rows := float64(cols) * float64(cellW) / float64(cellH) * float64(h) / float64(w)
	return max(1, int(rows+0.5))
}

func colsFor(w, h, rows int) int {
	cols := float64(rows) * float64(cellH) / float64(cellW) * float64(w) / float64(h)
	return max(1, int(cols+0.5))
}

func pngBase64(img image.Image) string {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func resize(img image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, max(1, w), max(1, h)))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}

// encodeKitty transmits a PNG and shows it in cols x rows cells. Long payloads
// are split into 4 KiB chunks, as the protocol requires.
func encodeKitty(data string, cols, rows int) string {
	const chunk = 4096
	params := fmt.Sprintf("a=T,f=100,q=2,c=%d,r=%d", cols, rows)
	var b strings.Builder
	if len(data) <= chunk {
		b.WriteString("\x1b_G" + params + ";" + data + "\x1b\\")
		return b.String()
	}
	for off, first := 0, true; off < len(data); first = false {
		end := min(off+chunk, len(data))
		part := data[off:end]
		switch {
		case first:
			b.WriteString("\x1b_G" + params + ",m=1;" + part + "\x1b\\")
		case end == len(data):
			b.WriteString("\x1b_Gm=0;" + part + "\x1b\\")
		default:
			b.WriteString("\x1b_Gm=1;" + part + "\x1b\\")
		}
		off = end
	}
	return b.String()
}

func encodeITerm2(data string, cols, rows int) string {
	w, h := cols*cellW, rows*cellH
	return fmt.Sprintf("\x1b]1337;File=inline=1;size=%d;width=%dpx;height=%dpx;preserveAspectRatio=1:%s\x07",
		len(data)*3/4, w, h, data)
}

// encodeSixel writes sixel data with a fixed 216-colour cube plus 24 greys.
// Six rows of pixels share one sixel character, and `-` starts the next band.
func encodeSixel(img *image.RGBA) string {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var b strings.Builder
	b.WriteString("\x1bP0;1;0q")
	fmt.Fprintf(&b, "\"1;1;%d;%d", w, h)
	for band := 0; band < h; band += 6 {
		// Which palette entries appear in this band?
		present := make([]bool, 256)
		codes := make([][]int, w)
		for x := 0; x < w; x++ {
			codes[x] = make([]int, 6)
			for k := 0; k < 6; k++ {
				y := band + k
				idx := 0
				if y < h {
					idx = paletteIndex(img, x, y)
					present[idx] = true
				}
				codes[x][k] = idx
			}
		}
		for idx := 0; idx < 256; idx++ {
			if !present[idx] {
				continue
			}
			fmt.Fprintf(&b, "#%d", idx)
			runChar, runLen := byte(0), 0
			flush := func() {
				if runLen == 0 {
					return
				}
				if runLen > 3 {
					fmt.Fprintf(&b, "!%d%c", runLen, runChar)
				} else {
					for i := 0; i < runLen; i++ {
						b.WriteByte(runChar)
					}
				}
				runLen = 0
			}
			for x := 0; x < w; x++ {
				mask := 0
				for k := 0; k < 6; k++ {
					if codes[x][k] == idx {
						mask |= 1 << k
					}
				}
				ch := byte(63 + mask)
				if ch == runChar {
					runLen++
					continue
				}
				flush()
				runChar, runLen = ch, 1
			}
			flush()
			b.WriteByte('$')
		}
		b.WriteString("-")
	}
	b.WriteString("\x1b\\")
	return b.String()
}

// paletteIndex maps a pixel to the 6x6x6 cube (0-215) or the grey ramp (216-239).
func paletteIndex(img *image.RGBA, x, y int) int {
	i := img.PixOffset(x, y)
	r, g, b := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
	if abs(r-g) < 12 && abs(g-b) < 12 && abs(r-b) < 12 {
		grey := (r + g + b) / 3 * 23 / 255
		return 216 + grey
	}
	return 36*r/256*6 + 6*g/256 + b/256
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// halfblocks paints two pixels per cell with `▀`: the top pixel in the
// foreground, the bottom one in the background. It survives scrolling because
// it is ordinary text.
func halfblocks(img *image.RGBA) []string {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	lines := make([]string, 0, h/2)
	for y := 0; y+1 < h; y += 2 {
		var b strings.Builder
		for x := 0; x < w; x++ {
			top := img.Pix[img.PixOffset(x, y):]
			bottom := img.Pix[img.PixOffset(x, y+1):]
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀",
				top[0], top[1], top[2], bottom[0], bottom[1], bottom[2])
		}
		b.WriteString("\x1b[m")
		lines = append(lines, b.String())
	}
	return lines
}
