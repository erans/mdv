package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erans/mdv/image"
)

func TestParseTable(t *testing.T) {
	src := []string{
		"| Left | Mid | Right |",
		"| :--- | :-: | ----: |",
		"| a | `b` | 1 |",
		"| c \\| d | e | 2 |",
		"",
		"after",
	}
	tbl, used := parseTable(src)
	if tbl == nil {
		t.Fatal("table not parsed")
	}
	if used != 4 {
		t.Errorf("consumed %d lines, want 4", used)
	}
	wantAlign := []cellAlign{alignLeft, alignCenter, alignRight}
	for i, want := range wantAlign {
		if tbl.align[i] != want {
			t.Errorf("column %d align = %v, want %v", i, tbl.align[i], want)
		}
	}
	if len(tbl.rows) != 2 {
		t.Fatalf("%d rows", len(tbl.rows))
	}
	if tbl.rows[1][0] != "c | d" {
		t.Errorf("escaped pipe = %q", tbl.rows[1][0])
	}
}

func TestParseTableRejectsParagraphs(t *testing.T) {
	for _, src := range [][]string{
		{"a | b", "not a delimiter"},
		{"| a | b |", "| --- |"},
		{"plain text", "-----"},
		{"a | b"},
	} {
		if tbl, _ := parseTable(src); tbl != nil {
			t.Errorf("parsed %q as a table", src)
		}
	}
}

// The bug this renderer exists for: a table whose natural width lands exactly
// on the wrap width must keep every character.
func TestTableFitsTheExactBoundaryWidth(t *testing.T) {
	src := []string{
		"| Flag | Type | Default | Notes |",
		"| --- | --- | --- | --- |",
		"| -palette | bool | false | print the resolved palette and exit |",
	}
	tbl, _ := parseTable(src)
	if tbl == nil {
		t.Fatal("table not parsed")
	}
	// Natural width: 9 + 4 + 13 + 34 cells + 3*4+1 borders.
	width := 9 + 4 + 13 + 34 + 13
	lines := tbl.render(builtinPalette(), width)
	if lines == nil {
		t.Fatal("render refused a width that fits")
	}
	joined := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "print the resolved palette and exit") {
		t.Errorf("cell text was clipped:\n%s", joined)
	}
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line is %d wide, over the %d budget: %q", w, width, ansi.Strip(line))
		}
	}
}

func TestTableWrapsInsteadOfClipping(t *testing.T) {
	src := []string{
		"| a | b |",
		"| --- | --- |",
		"| one two three four | five |",
	}
	tbl, _ := parseTable(src)
	lines := tbl.render(builtinPalette(), 34)
	if lines == nil {
		t.Fatal("render refused")
	}
	joined := ansi.Strip(strings.Join(lines, "\n"))
	for _, word := range []string{"one", "two", "three", "four"} {
		if !strings.Contains(joined, word) {
			t.Errorf("word %q lost in wrapping:\n%s", word, joined)
		}
	}
}

func TestTableFallsBackWhenTooNarrow(t *testing.T) {
	src := []string{
		"| a | b | c |",
		"| --- | --- | --- |",
		"| 1 | 2 | 3 |",
	}
	tbl, _ := parseTable(src)
	if lines := tbl.render(builtinPalette(), 8); lines != nil {
		t.Errorf("a 8-column pane cannot hold three columns, got:\n%s", ansi.Strip(strings.Join(lines, "\n")))
	}
}

func TestTableCellStyling(t *testing.T) {
	tbl, _ := parseTable([]string{
		"| code | link |",
		"| --- | --- |",
		"| `x := 1` | [docs](https://example.com) |",
	})
	pal := builtinPalette()
	pal.Orange = "#102030" // a colour no default would produce
	pal.Blue = "#405060"
	lines := tbl.render(pal, 60)
	raw := strings.Join(lines, "\n")
	if !strings.Contains(raw, "38;2;16;32;48") {
		t.Error("inline code in a cell lost its colour")
	}
	if !strings.Contains(raw, "\x1b]8;;https://example.com\x07") {
		t.Error("a link in a cell should stay clickable")
	}
	if strings.Contains(ansi.Strip(raw), "`") || strings.Contains(ansi.Strip(raw), "](") {
		t.Error("markdown syntax leaked into the rendered cell")
	}
}

func TestAlignmentPadsTheRightSide(t *testing.T) {
	tbl, _ := parseTable([]string{
		"| l | c | r |",
		"| :- | :-: | -: |",
		"| a | b | 1 |",
	})
	lines := tbl.render(builtinPalette(), 40)
	row := ansi.Strip(lines[len(lines)-2]) // last row before the bottom border
	if !strings.Contains(row, "│ a ") {
		t.Errorf("left column not left aligned: %q", row)
	}
	if !strings.Contains(row, " 1 │") {
		t.Errorf("right column not right aligned: %q", row)
	}
}

func TestMarkdownTablesUseOurRenderer(t *testing.T) {
	out, err := renderMarkdown("| a | b |\n| --- | --- |\n| 1 | 2 |\n", builtinPalette(),
		renderOpts{baseDir: ".", images: image.Off}, 40)
	if err != nil {
		t.Fatal(err)
	}
	plainOut := ansi.Strip(out)
	if !strings.Contains(plainOut, "┌") || !strings.Contains(plainOut, "┬") {
		t.Errorf("expected our box borders:\n%s", plainOut)
	}
}
