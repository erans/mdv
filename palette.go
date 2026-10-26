package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Palette is the whole color vocabulary of the viewer. Everything else renders
// against these names, never against hex.
type Palette struct {
	Name      string
	Light     bool
	Bg        string // terminal/page background, used for contrast checks only
	Fg        string
	Accent    string
	Muted     string
	Selection string
	Red       string
	Yellow    string
	Orange    string
	Green     string
	Cyan      string
	Blue      string
	Magenta   string
}

// resolvePalette picks the first source that answers: an explicit file, the
// Omarchy theme the desktop is actually wearing, the terminal's own colors,
// then a built-in. The theme file wins over the terminal on purpose: over SSH
// the local terminal is only a window onto someone else's desktop.
func resolvePalette(explicit string) Palette {
	if explicit != "" {
		if p, err := loadOmarchyColors(explicit); err == nil {
			return p
		}
	}
	for _, path := range omarchyColorPaths() {
		if p, err := loadOmarchyColors(path); err == nil {
			p.Name = "omarchy:" + themeName(path)
			return p
		}
	}
	if p, ok := loadTerminal(); ok {
		return p
	}
	return builtinPalette()
}

// omarchyColorPaths is the active theme's colors.toml, newest layout first.
// omarchy-theme-set stages the theme's whole directory under the state dir and
// synthesizes colors.toml from alacritty.toml for old themes, so this file is
// always there; the config path is Omarchy 2's location.
func omarchyColorPaths() []string {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, _ := os.UserHomeDir()
		state = filepath.Join(home, ".local", "state")
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, _ := os.UserHomeDir()
		cfg = filepath.Join(home, ".config")
	}
	return []string{
		filepath.Join(state, "omarchy", "current", "theme", "colors.toml"),
		filepath.Join(cfg, "omarchy", "current", "theme", "colors.toml"),
	}
}

func themeName(colorPath string) string {
	b, err := os.ReadFile(filepath.Join(filepath.Dir(colorPath), "..", "theme.name"))
	if err != nil {
		return "unnamed"
	}
	return strings.TrimSpace(string(b))
}

// loadOmarchyColors reads an Omarchy colors.toml.
//
// shortcut: flat `key = "#rrggbb"` files only (that is the whole format), no
// TOML tables or multi-line strings; upgrade if Omarchy nests values.
func loadOmarchyColors(path string) (Palette, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Palette{}, err
	}
	kv := map[string]string{}
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		kv[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	p := Palette{
		Light:     kv["mode"] == "light",
		Bg:        kv["background"],
		Fg:        kv["foreground"],
		Accent:    kv["accent"],
		Muted:     kv["muted"],
		Selection: kv["selection"],
		Red:       kv["red"],
		Yellow:    kv["yellow"],
		Orange:    kv["orange"],
		Green:     kv["green"],
		Cyan:      kv["cyan"],
		Blue:      kv["blue"],
		Magenta:   kv["magenta"],
	}
	if p.Bg == "" || p.Fg == "" {
		return Palette{}, os.ErrNotExist
	}
	// A theme that omits a role gets the nearest one that is present.
	fallback := func(dst *string, srcs ...string) {
		for _, s := range srcs {
			if *dst == "" && s != "" {
				*dst = s
				return
			}
		}
	}
	fallback(&p.Muted, p.Fg)
	fallback(&p.Accent, p.Blue, p.Magenta, p.Fg)
	fallback(&p.Selection, p.Muted, p.Bg)
	for _, c := range []*string{&p.Red, &p.Yellow, &p.Orange, &p.Green, &p.Cyan, &p.Blue, &p.Magenta} {
		fallback(c, p.Accent, p.Fg)
	}
	return p, nil
}

func builtinPalette() Palette {
	return Palette{
		Name:      "builtin:dark",
		Bg:        "#18181e",
		Fg:        "#e4e4e7",
		Accent:    "#c4a7e7",
		Muted:     "#7b7b8f",
		Selection: "#2a2a3c",
		Red:       "#eb6f92", Yellow: "#f6c177", Orange: "#ebbcba",
		Green: "#95b1ac", Cyan: "#9ccfd8", Blue: "#31748f", Magenta: "#c4a7e7",
	}
}

// loadTerminal asks the terminal for its background, foreground and the six
// ANSI hues, then keeps whatever it answered. One write, one read: 16 separate
// OSC 4 round trips are not worth the latency.
func loadTerminal() (Palette, bool) {
	ans, ok := queryTerminal([]int{10, 11, 4})
	if !ok {
		return Palette{}, false
	}
	p := builtinPalette()
	p.Name = "terminal"
	p.Bg, p.Fg = ans[11], ans[10]
	for i, dst := range []*string{&p.Red, &p.Green, &p.Yellow, &p.Blue, &p.Magenta, &p.Cyan} {
		if c := ans[100+i]; c != "" {
			*dst = c
		}
	}
	if p.Bg != "" && p.Fg != "" {
		p.Light = luminance(p.Bg) > 0.5
	}
	return p, true
}

// luminance is a 0..1 estimate from an #rrggbb string; enough to pick a theme.
func luminance(hex string) float64 {
	r, g, b, ok := parseHex(hex)
	if !ok {
		return 0
	}
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255
}

func parseHex(hex string) (r, g, b int, ok bool) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v >> 16 & 0xff), int(v >> 8 & 0xff), int(v & 0xff), true
}

// blend mixes a toward b by t (0..1) and returns #rrggbb.
func blend(a, b string, t float64) string {
	ar, ag, ab, ok1 := parseHex(a)
	br, bg, bb, ok2 := parseHex(b)
	if !ok1 || !ok2 {
		return a
	}
	mix := func(x, y int) int { return int(float64(x)*(1-t) + float64(y)*t) }
	return "#" + hex2(mix(ar, br)) + hex2(mix(ag, bg)) + hex2(mix(ab, bb))
}

// paint wraps s in a truecolor foreground escape.
func paint(hex, s string) string {
	r, g, b, ok := parseHex(hex)
	if !ok {
		return s
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[m", r, g, b, s)
}

// paintOn draws text with an explicit background and foreground, for highlights
// and selected rows: reverse video over themed colours is unreadable.
func paintOn(bgHex, fgHex, s string) string {
	open := paintOpen(bgHex, fgHex)
	if open == "" {
		return s
	}
	return open + s + "\x1b[39;49m"
}

// paintOpen is the opening half of paintOn, for text that must stay styled.
func paintOpen(bgHex, fgHex string) string {
	br, bg, bb, ok1 := parseHex(bgHex)
	fr, fg, fb, ok2 := parseHex(fgHex)
	if !ok1 || !ok2 {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm", fr, fg, fb, br, bg, bb)
}

func hex2(v int) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[(v>>4)&0xf], digits[v&0xf]})
}
