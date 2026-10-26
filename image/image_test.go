package image

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(1)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = seed*1664525 + 1013904223 // noise, so PNG cannot compress it away
			img.Set(x, y, color.RGBA{uint8(seed >> 24), uint8(seed >> 16), uint8(seed >> 8), 0xff})
		}
	}
	path := filepath.Join(t.TempDir(), "px.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParse(t *testing.T) {
	for _, want := range []string{"auto", "kitty", "iterm2", "sixel", "halfblock", "off"} {
		if got, err := Parse(want); err != nil || string(got) != want {
			t.Errorf("Parse(%q) = %q, %v", want, got, err)
		}
	}
	if _, err := Parse("gif"); err == nil {
		t.Error("unknown mode should error")
	}
}

func TestDetect(t *testing.T) {
	cases := map[string]Protocol{
		"KITTY_WINDOW_ID":       "kitty",
		"GHOSTTY_RESOURCES_DIR": "kitty",
		"ITERM_SESSION_ID":      "iterm2",
		"TMUX":                  "halfblock",
		"XTERM_SIXEL":           "sixel",
	}
	for key, want := range cases {
		t.Run(key, func(t *testing.T) {
			// A clean environment, then one hint.
			for _, k := range []string{"KITTY_WINDOW_ID", "TERM_PROGRAM", "TERM", "ITERM_SESSION_ID", "GHOSTTY_RESOURCES_DIR", "WEZTERM_PANE", "TMUX", "XTERM_SIXEL", "MLTERM"} {
				t.Setenv(k, "")
			}
			t.Setenv(key, "1")
			if got := Detect(); got != want {
				t.Errorf("Detect() = %q, want %q", got, want)
			}
		})
	}
}

func TestRenderProtocols(t *testing.T) {
	path := writePNG(t, 120, 60)
	cases := map[Protocol]string{
		Kitty:     "\x1b_Ga=T,f=100",
		ITerm2:    "\x1b]1337;File=inline=1",
		Sixel:     "\x1bP0;1;0q",
		Halfblock: "\x1b[38;2;",
	}
	for proto, prefix := range cases {
		t.Run(string(proto), func(t *testing.T) {
			block, err := Render(path, proto, 40, 0)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(block.Text, prefix) {
				t.Errorf("%s output starts with %q, want %q", proto, first(block.Text, 24), prefix)
			}
			if block.Rows < 1 {
				t.Errorf("rows = %d", block.Rows)
			}
			if proto == Halfblock {
				if got := strings.Count(block.Text, "\n"); got != block.Rows {
					t.Errorf("halfblock has %d lines, want %d", got, block.Rows)
				}
			} else if got := strings.Count(block.Text, "\n"); got != block.Rows-1 {
				t.Errorf("%s reserves %d rows with %d newlines, want %d", proto, block.Rows, got, block.Rows-1)
			}
		})
	}
}

func TestKittyPayloadIsChunked(t *testing.T) {
	path := writePNG(t, 300, 300) // big enough to exceed one 4 KiB chunk
	block, err := Render(path, Kitty, 80, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block.Text, ",m=1;") || !strings.Contains(block.Text, "\x1b_Gm=0;") {
		t.Error("a large payload should be sent as more, then final, chunks")
	}
}

func TestSixelStructure(t *testing.T) {
	path := writePNG(t, 40, 30)
	block, err := Render(path, Sixel, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	body := block.Text
	if !strings.HasPrefix(body, "\x1bP0;1;0q\"1;1;") {
		t.Errorf("bad sixel header: %q", first(body, 20))
	}
	if !strings.HasSuffix(strings.TrimRight(body, "\n"), "\x1b\\") {
		t.Error("sixel data should end with ST")
	}
	if strings.Count(body, "-") < 2 {
		t.Error("sixel should have several bands")
	}
	if !strings.Contains(body, "#") {
		t.Error("no palette selector")
	}
}

func TestAspectRatioDrivesRows(t *testing.T) {
	// A 2:1 image in 40 columns is 40*10/20/2 = 10 rows.
	if got := rowsFor(200, 100, 40); got != 10 {
		t.Errorf("rowsFor(200x100, 40) = %d, want 10", got)
	}
	// A tall image is capped by maxRows, and the width shrinks with it.
	path := writePNG(t, 60, 300)
	block, err := Render(path, Kitty, 40, 5)
	if err != nil {
		t.Fatal(err)
	}
	if block.Rows != 5 {
		t.Errorf("rows = %d, want the 5 cap", block.Rows)
	}
	if !strings.Contains(block.Text, "c=2,r=5") {
		t.Errorf("width should shrink with the cap: %q", first(block.Text, 40))
	}
}

func TestMissingFile(t *testing.T) {
	if _, err := Render("/no/such/file.png", Kitty, 20, 0); err == nil {
		t.Error("a missing file should error so the caller can fall back")
	}
}

func first(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}
