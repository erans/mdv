package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestOSC8At(t *testing.T) {
	intro := "\x1b]8;id=7;https://example.com\x07text"
	if url, n, ok := osc8At(intro); !ok || url != "https://example.com" || intro[n:] != "text" {
		t.Errorf("introducer: url=%q n=%d ok=%v", url, n, ok)
	}
	term := "\x1b]8;;\x1b\\after"
	if url, n, ok := osc8At(term); !ok || url != "" || term[n:] != "after" {
		t.Errorf("ST terminator: url=%q n=%d ok=%v", url, n, ok)
	}
	if _, _, ok := osc8At("\x1b[31m"); ok {
		t.Error("SGR must not parse as a hyperlink")
	}
}

func TestIndexLinksFindsRenderedLink(t *testing.T) {
	src := "see [the docs](https://example.com/docs) now\n"
	out, err := renderMarkdown(src, builtinPalette(), 60)
	if err != nil {
		t.Fatal(err)
	}
	links := indexLinks(strings.Split(out, "\n"))
	var spans []link
	for _, line := range links {
		spans = append(spans, line...)
	}
	if len(spans) == 0 {
		t.Fatal("no link recovered from the rendered line")
	}
	// `see [the docs](url) now`: the link text at column 4, the printed URL at 13.
	if spans[0].start != 4 || spans[0].end != 12 {
		t.Errorf("link text span = %d..%d, want 4..12", spans[0].start, spans[0].end)
	}
	if !strings.Contains(spans[0].url, "example.com") {
		t.Errorf("link url = %q", spans[0].url)
	}
}

func TestViewFillsExactlyTheScreen(t *testing.T) {
	src := strings.Repeat("line\n\n", 200)
	v := newViewer("t.md", src, builtinPalette(), 40, 10)
	rows := strings.Split(strings.TrimSuffix(v.View().Content, "\r\n"), "\r\n")
	if len(rows) != 10 {
		t.Fatalf("rendered %d rows, want 10", len(rows))
	}
	for i, row := range rows {
		if w := ansi.StringWidth(row); w != 40 {
			t.Errorf("row %d is %d columns, want 40", i, w)
		}
	}
}

func TestThumbTracksProgress(t *testing.T) {
	v := newViewer("t.md", strings.Repeat("x\n", 400), builtinPalette(), 40, 11)
	page := v.page()
	top0, _ := v.thumb(page)
	v.off = v.maxOff()
	topEnd, length := v.thumb(page)
	if top0 != 0 {
		t.Errorf("thumb starts at %d at the top", top0)
	}
	if topEnd+length != page {
		t.Errorf("thumb ends at %d of %d at the bottom", topEnd+length, page)
	}

	v.jumpTo(page - 1)
	if v.off != v.maxOff() {
		t.Errorf("clicking the last row scrolled to %d, want %d", v.off, v.maxOff())
	}
	v.jumpTo(0)
	if v.off != 0 {
		t.Errorf("clicking the first row scrolled to %d, want 0", v.off)
	}
}

func TestLinkAtUsesVisibleColumns(t *testing.T) {
	v := &viewer{
		lines:    []string{"plain", "a link", "plain"},
		links:    [][]link{{}, {{2, 6, "https://x"}}, {}},
		height:   5,
		contentW: 40,
	}
	if url := v.linkAt(1, 3); url != "https://x" {
		t.Errorf("click inside the link = %q", url)
	}
	if url := v.linkAt(1, 7); url != "" {
		t.Errorf("click past the link = %q", url)
	}
	if url := v.linkAt(0, 3); url != "" {
		t.Errorf("click on plain text = %q", url)
	}
}
