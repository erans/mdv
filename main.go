// mdv renders markdown for a terminal: tables, mermaid diagrams as box art,
// and images where the terminal can show them, colored by the desktop's theme.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"charm.land/glamour/v2"
	"golang.org/x/term"
)

var sgr = regexp.MustCompile("\x1b\\[[0-9;]*m")

func main() {
	width := flag.Int("w", 0, "wrap width in columns (default: terminal width)")
	themePath := flag.String("theme", "", "colors.toml to use instead of the desktop theme")
	showPalette := flag.Bool("palette", false, "print the resolved palette and exit")
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

	out, err := glamour.NewTermRenderer(
		glamour.WithStyles(pal.StyleConfig()),
		glamour.WithWordWrap(*width),
		glamour.WithTableWrap(true),
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
	rendered, err := out.Render(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
	fmt.Print(trimTrailing(rendered))
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
