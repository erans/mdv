// mdv renders markdown for a terminal: tables, mermaid diagrams as box art,
// and images where the terminal can show them, colored by the desktop's theme.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"mdv/mermaid"
	"regexp"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"golang.org/x/term"
)

var sgr = regexp.MustCompile("\x1b\\[[0-9;]*m")

func main() {
	width := flag.Int("w", 0, "wrap width in columns (default: terminal width)")
	themePath := flag.String("theme", "", "colors.toml to use instead of the desktop theme")
	showPalette := flag.Bool("palette", false, "print the resolved palette and exit")
	plain := flag.Bool("plain", false, "print once instead of opening the viewer")
	frame := flag.Bool("frame", false, "print one viewer frame and exit (for screenshots)")
	height := flag.Int("h", 24, "frame height in rows, with -frame")
	flag.Parse()

	pal := resolvePalette(*themePath)
	if *showPalette {
		fmt.Printf("%+v\n", pal)
		return
	}

	src, err := readInput(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
	if *width == 0 {
		*width = 80
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			*width = w
		}
	}

	if *frame {
		fmt.Print(newViewer(title(flag.Args()), src, pal, *width, *height).View().Content)
		return
	}
	if *plain || !term.IsTerminal(int(os.Stdout.Fd())) {
		out, err := renderMarkdown(src, pal, *width)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mdv:", err)
			os.Exit(1)
		}
		fmt.Print(out)
		return
	}

	w, h := *width, 24
	if tw, th, err := term.GetSize(int(os.Stdout.Fd())); err == nil && tw > 0 && th > 0 {
		w, h = tw, th
	}
	if _, err := tea.NewProgram(newViewer(title(flag.Args()), src, pal, w, h)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
}

// renderMarkdown splits mermaid fences out of the document, draws them, and
// lets glamour render everything around them.
func renderMarkdown(src string, pal Palette, width int) (string, error) {
	var b strings.Builder
	for _, seg := range splitFences(src) {
		if seg.mermaid {
			art, err := mermaid.Render(seg.text)
			if err == nil && art.Width <= width {
				b.WriteString(styleArt(art, pal))
				b.WriteString("\n")
				continue
			}
			seg = segment{text: "```mermaid\n" + strings.TrimRight(seg.text, "\n") + "\n```\n"}
		}
		out, err := renderFragment(seg.text, pal, width)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String(), nil
}

type segment struct {
	text    string
	mermaid bool
}

// splitFences pulls top-level mermaid fences out of the document.
//
// shortcut: fences nested inside a list item or blockquote are left to glamour
// (they render as code). Upgrade if that ever shows up in a real README.
func splitFences(src string) []segment {
	var out []segment
	var text strings.Builder
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if lang, ok := fenceLang(line); ok && lang == "mermaid" {
			var body strings.Builder
			for i++; i < len(lines); i++ {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					break
				}
				body.WriteString(lines[i] + "\n")
			}
			if text.Len() > 0 {
				out = append(out, segment{text: text.String()})
				text.Reset()
			}
			out = append(out, segment{text: body.String(), mermaid: true})
			continue
		}
		text.WriteString(line + "\n")
	}
	if text.Len() > 0 {
		out = append(out, segment{text: text.String()})
	}
	return out
}

func fenceLang(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "```") {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(t[3:])), true
}

// styleArt colors a diagram by what each cell means.
func styleArt(art *mermaid.Art, pal Palette) string {
	var b strings.Builder
	for _, row := range art.Cells {
		for start := 0; start < len(row); {
			cl := row[start].Cl
			end := start
			for end < len(row) && row[end].Cl == cl {
				end++
			}
			var run strings.Builder
			for _, c := range row[start:end] {
				run.WriteRune(c.Ch)
			}
			b.WriteString(paintClass(pal, cl, run.String()))
			start = end
		}
		b.WriteString("\n")
	}
	return b.String()
}

func paintClass(pal Palette, cl mermaid.Class, s string) string {
	switch cl {
	case mermaid.Border:
		return paint(pal.Muted, s)
	case mermaid.Text:
		return paint(pal.Fg, s)
	case mermaid.Edge:
		return paint(pal.Accent, s)
	case mermaid.EdgeLabel:
		return paint(blend(pal.Fg, pal.Muted, 0.5), s)
	case mermaid.Title:
		return paint(pal.Accent, s)
	}
	return s
}

func renderFragment(src string, pal Palette, width int) (string, error) {
	out, err := glamour.NewTermRenderer(
		glamour.WithStyles(pal.StyleConfig()),
		glamour.WithWordWrap(width),
		glamour.WithTableWrap(true),
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
		return "", err
	}
	rendered, err := out.Render(src)
	if err != nil {
		return "", err
	}
	return trimTrailing(rendered), nil
}

func title(args []string) string {
	if len(args) == 0 {
		return "stdin"
	}
	return filepath.Base(args[0])
}

// trimTrailing drops the padding glamour writes to reach the wrap width, so the
// viewer never emits lines of invisible colored spaces.
func trimTrailing(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = trimLine(line)
	}
	return strings.Join(lines, "\n")
}

// trimLine cuts a line right after its last visible character, skipping over
// SGR sequences and trailing blanks.
func trimLine(line string) string {
	cut := 0
	for i := 0; i < len(line); {
		if loc := sgr.FindStringIndex(line[i:]); loc != nil && loc[0] == 0 {
			i += loc[1]
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		i += size
		if r != ' ' && r != '\t' {
			cut = i
		}
	}
	return line[:cut]
}

func readInput(args []string) (string, error) {
	if len(args) == 0 {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	b, err := os.ReadFile(args[0])
	return string(b), err
}
