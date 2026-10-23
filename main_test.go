package main

import (
	"os"
	"strings"
	"testing"

	"charm.land/glamour/v2"
)

func render(t *testing.T, src string, p Palette, width int) string {
	t.Helper()
	out, err := glamour.NewTermRenderer(
		glamour.WithStyles(p.StyleConfig()),
		glamour.WithWordWrap(width),
		glamour.WithTableWrap(true),
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := out.Render(src)
	if err != nil {
		t.Fatal(err)
	}
	return trimTrailing(rendered)
}

func TestPaletteFromOmarchyColors(t *testing.T) {
	p, err := loadOmarchyColors("testdata/tokyo-night.toml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Light {
		t.Error("tokyo-night has mode = dark")
	}
	for name, got := range map[string]string{
		"Bg": "#1a1b26", "Fg": "#a9b1d6", "Accent": "#7aa2f7",
		"Muted": "#414868", "Selection": "#292e42", "Blue": "#7aa2f7",
	} {
		if got != map[string]string{
			"Bg": p.Bg, "Fg": p.Fg, "Accent": p.Accent,
			"Muted": p.Muted, "Selection": p.Selection, "Blue": p.Blue,
		}[name] {
			t.Errorf("%s = %q, want %q", name, map[string]string{
				"Bg": p.Bg, "Fg": p.Fg, "Accent": p.Accent,
				"Muted": p.Muted, "Selection": p.Selection, "Blue": p.Blue,
			}[name], got)
		}
	}
	// A role the theme omits falls back to a neighbouring one.
	if p.Green == "" || p.Orange == "" {
		t.Error("missing roles should fall back, not stay empty")
	}
}

// Markdown binds to palette roles, so the theme's accent and selection must
// reach the terminal as truecolor escapes.
func TestStyleCarriesPaletteColors(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.md")
	if err != nil {
		t.Fatal(err)
	}
	p := Palette{Light: false, Bg: "#000000", Fg: "#eeeeee", Accent: "#7aa2f7",
		Muted: "#414868", Selection: "#292e42", Red: "#f00", Yellow: "#ff0",
		Orange: "#eb927b", Green: "#9ece6a", Cyan: "#449dab", Blue: "#7aa2f7", Magenta: "#ad8ee6"}
	out := render(t, string(src), p, 64)

	if !strings.Contains(out, "38;2;122;162;247") { // #7aa2f7: accent on the heading
		t.Error("heading did not use the palette accent")
	}
	if !strings.Contains(out, "48;2;41;46;66") { // #292e42: selection behind inline code
		t.Error("inline code did not use the palette selection")
	}
}

func TestTableRendersAsBoxDrawing(t *testing.T) {
	out := render(t, "| a | b |\n| --- | --- |\n| 1 | 2 |\n", builtinPalette(), 40)
	for _, want := range []string{"┼", "│", "a", "1"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "| --- |") {
		t.Error("table fell back to raw markdown pipes")
	}
}

func TestNoTrailingPadding(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.md")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(render(t, string(src), builtinPalette(), 64), "\n") {
		if strings.HasSuffix(strings.TrimRight(line, "\r"), " ") {
			t.Errorf("line %d ends with padding: %q", i+1, line)
		}
		if strings.Contains(line, "\x1b[") && strings.HasSuffix(line, "m") && !strings.HasSuffix(line, "\x1b[m") {
			t.Errorf("line %d ends inside a style: %q", i+1, line)
		}
	}
}

func TestParseOSCColors(t *testing.T) {
	// BEL and ST terminators, 2- and 4-digit channels, and an OSC 4 reply.
	reply := "\x1b]11;rgb:1a1a/1b1b/2626\a" +
		"\x1b]10;rgb:aaaa/bbbb/cccc\x1b\\" +
		"\x1b]4;1;rgb:ff/77/8e\x07" +
		"\x1b]4;4;rgb:7a/a2/f7\a"
	got := parseOSCColors(reply)
	want := map[int]string{11: "#1a1b26", 10: "#aabbcc", 101: "#ff778e", 104: "#7aa2f7"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("code %d = %q, want %q", k, got[k], v)
		}
	}
}

func TestTrimLineKeepsVisibleText(t *testing.T) {
	cases := map[string]string{
		"\x1b[38;2;1;2;3mheading\x1b[m   ":    "\x1b[38;2;1;2;3mheading",
		"\x1b[38;2;1;2;3m │ a │ \x1b[m   ":     "\x1b[38;2;1;2;3m │ a │",
		"\x1b[38;2;1;2;3m\x1b[m \x1b[m \x1b[m ": "",
	}
	for in, want := range cases {
		if got := trimLine(in); got != want {
			t.Errorf("trimLine(%q) = %q, want %q", in, got, want)
		}
	}
}
