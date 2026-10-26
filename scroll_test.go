package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erans/mdv/image"
)

const wideDoc = "# Wide\n\n```mermaid\ngraph LR\n  A[first step in the pipeline] --> B{is this decision wide?}\n  B -->|yes it is| C[a rather long action name]\n  C --> E[and the final destination node]\n```\n\nafter\n"

// The viewer keeps a diagram that is wider than the pane and scrolls it; plain
// output has no scrollbar, so it shows the source instead of a wrapped mess.
func TestWideDiagramKeptForTheViewerOnly(t *testing.T) {
	plainOut, err := renderMarkdown(wideDoc, builtinPalette(), renderOpts{images: image.Off}, 40)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ansi.Strip(plainOut), "A[first step in the pipeline]") {
		t.Errorf("plain output should fall back to the source:\n%s", ansi.Strip(plainOut))
	}
	if strings.Contains(ansi.Strip(plainOut), "▶") {
		t.Errorf("plain output should not draw the wide diagram:\n%s", ansi.Strip(plainOut))
	}

	viewerOut, err := renderMarkdown(wideDoc, builtinPalette(), renderOpts{images: image.Off, allowWide: true}, 40)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ansi.Strip(viewerOut), "A[first step in the pipeline]") {
		t.Errorf("the viewer should keep the diagram, not the source:\n%s", ansi.Strip(viewerOut))
	}
	if !strings.Contains(ansi.Strip(viewerOut), "▶") {
		t.Errorf("no diagram arrows in the viewer output:\n%s", ansi.Strip(viewerOut))
	}
}

func TestHorizontalScrollClampsAndShifts(t *testing.T) {
	v := newViewer("wide.md", wideDoc, builtinPalette(), renderOpts{images: image.Off}, 40, 10)
	if v.maxHOff() == 0 {
		t.Fatalf("expected a diagram wider than the 40-column pane, lines: %d columns", widest(v.lines))
	}
	before := ansi.Strip(v.View().Content)

	v.scrollX(8)
	if v.hOff != 8 {
		t.Errorf("hOff = %d after scrolling 8, want 8", v.hOff)
	}
	if after := ansi.Strip(v.View().Content); after == before {
		t.Error("the view did not change when scrolling sideways")
	}

	v.scrollX(10_000)
	if v.hOff != v.maxHOff() {
		t.Errorf("hOff = %d, want the clamp at %d", v.hOff, v.maxHOff())
	}
	v.scrollX(-10_000)
	if v.hOff != 0 {
		t.Errorf("hOff = %d, want 0", v.hOff)
	}
}

// Text that fits the pane must not shift when a diagram scrolls.
func TestNarrowLinesIgnoreHorizontalScroll(t *testing.T) {
	v := newViewer("wide.md", wideDoc, builtinPalette(), renderOpts{images: image.Off}, 40, 10)
	row := -1
	for i, line := range v.lines {
		if ansi.StringWidth(line) <= v.contentW && strings.TrimSpace(ansi.Strip(line)) != "" {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatal("no narrow line to test")
	}
	v.hOff = 0
	first := v.clip(v.lines[row])
	v.hOff = 20
	again := v.clip(v.lines[row])
	if first != again {
		t.Errorf("a narrow line was clipped: %q vs %q", ansi.Strip(first), ansi.Strip(again))
	}
}

func TestWideLineLinksFollowTheScroll(t *testing.T) {
	// A clickable span at columns 60..64 on a line twice the pane width.
	pad := strings.Repeat("x", 60)
	line := pad + "\x1b]8;;https://example.com\x07docs\x1b]8;;\x07 tail"
	v := &viewer{
		contentW: 40,
		height:   10,
		lines:    []string{line},
		links:    indexLinks([]string{line}),
		hOff:     30,
	}
	if url := v.linkAt(0, 30); url != "https://example.com" {
		t.Errorf("click at the shifted column = %q, want the link", url)
	}
	v.hOff = 0
	if url := v.linkAt(0, 30); url != "" {
		t.Errorf("click at column 30 without scrolling should miss, got %q", url)
	}
}

func widest(lines []string) int {
	w := 0
	for _, line := range lines {
		w = max(w, ansi.StringWidth(line))
	}
	return w
}
