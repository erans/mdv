package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/erans/mdv/image"
)

const tocDoc = `# Title

intro text

## First section

One two three four five six seven eight nine ten eleven twelve thirteen
fourteen fifteen sixteen seventeen eighteen nineteen twenty.

### Nested bit

More words here so the document is taller than the pane and every heading can
be scrolled to without clamping.

## Second section

Body three wraps a few lines as well, giving the last heading somewhere to go
when the table of contents jumps to it. Adding still more prose here keeps the
document comfortably taller than the pane, so a jump to any heading lands on the
line it names instead of being clamped at the bottom.

And a final paragraph so the scroll range reaches past the last heading.
`

func key(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func special(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func send(v *viewer, msgs ...tea.KeyPressMsg) {
	for _, m := range msgs {
		v.Update(m)
	}
}

func plainView(v *viewer) string {
	return ansi.Strip(v.View().Content)
}

func TestHeadingsSkipCodeAndStripMarkup(t *testing.T) {
	got := headings("# One\n\n```sh\n# not a heading\n```\n\n## `code` and **bold** [link](http://x)\n\n####### seven\n")
	if len(got) != 2 {
		t.Fatalf("headings = %+v, want 2", got)
	}
	if got[0].text != "One" || got[0].level != 1 {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].text != "code and bold link" || got[1].level != 2 {
		t.Errorf("second = %+v", got[1])
	}
}

func TestTOCOpensAndJumps(t *testing.T) {
	v := newViewer("doc.md", tocDoc, builtinPalette(), renderOpts{images: image.Off}, 60, 10)
	if len(v.toc) != 4 {
		t.Fatalf("toc = %+v, want 4 entries", v.toc)
	}
	before := v.contentW
	send(v, key('t'))
	if !v.tocOpen {
		t.Fatal("t did not open the toc")
	}
	if v.contentW >= before {
		t.Errorf("content width %d should shrink with the pane open (was %d)", v.contentW, before)
	}
	if got := plainView(v); !strings.Contains(got, "First section") {
		t.Errorf("toc pane missing a heading:\n%s", got)
	}
	// Make sure the selection starts on a real heading, then walk to the last one.
	v.tocSel = 0
	send(v, key('j'), key('j'), key('j'))
	if v.tocSel != 3 {
		t.Fatalf("tocSel = %d, want 3", v.tocSel)
	}
	if want := v.toc[3].line; v.maxOff() < want {
		t.Fatalf("document too short to test the jump: maxOff=%d, heading at %d", v.maxOff(), want)
	}
	send(v, special(tea.KeyEnter))
	if v.off != v.toc[3].line {
		t.Errorf("enter scrolled to %d, want the heading line %d", v.off, v.toc[3].line)
	}
	// Esc closes the pane instead of quitting.
	send(v, special(tea.KeyEscape))
	if v.tocOpen {
		t.Error("esc did not close the toc")
	}
}

func TestSearchTypesJumpsAndCycles(t *testing.T) {
	v := newViewer("doc.md", tocDoc, builtinPalette(), renderOpts{images: image.Off}, 60, 12)
	send(v, key('/'))
	if !v.typing {
		t.Fatal("/ did not open the prompt")
	}
	for _, r := range "heading" {
		send(v, key(r))
	}
	if v.query != "heading" {
		t.Fatalf("query = %q", v.query)
	}
	if len(v.matches) < 2 {
		t.Fatalf("matches = %v, want at least 2", v.matches)
	}
	send(v, special(tea.KeyEnter))
	if v.typing {
		t.Error("enter should close the prompt")
	}
	if v.off != max(0, v.matches[0]-1) {
		t.Errorf("enter scrolled to %d, want the first match area %d", v.off, v.matches[0])
	}
	if want := max(0, v.matches[0]-1); v.off != min(want, v.maxOff()) {
		t.Errorf("enter scrolled to %d, want %d", v.off, min(want, v.maxOff()))
	}
	first := v.matchIdx
	send(v, key('n'))
	if v.matchIdx == first {
		t.Errorf("n did not advance (still %d)", v.matchIdx)
	}
	send(v, key('N'))
	if v.matchIdx != first {
		t.Errorf("N did not go back to %d (got %d)", first, v.matchIdx)
	}
	send(v, special(tea.KeyEscape))
	if v.query != "" || len(v.matches) != 0 {
		t.Errorf("esc left query %q with %d matches", v.query, len(v.matches))
	}
}

func TestSearchIsCaseInsensitiveAndHighlights(t *testing.T) {
	v := newViewer("doc.md", tocDoc, builtinPalette(), renderOpts{images: image.Off}, 60, 20)
	v.typing = true
	send(v, key('f'), key('i'), key('r'), key('s'), key('t'))
	if len(v.matches) == 0 {
		t.Fatalf("no match for %q", v.query)
	}
	view := v.View().Content
	if !strings.Contains(view, "48;2;") {
		t.Error("matches are not highlighted in the view")
	}
	// The highlight must not eat the styling the renderer put around the match.
	if !strings.Contains(view, "\x1b[38;") {
		t.Error("highlighting dropped the surrounding colours")
	}
}

func TestHighlightKeepsStylesIntact(t *testing.T) {
	line := "\x1b[38;2;1;2;3mplain \x1b[1mbold\x1b[22m and more\x1b[m"
	out := highlight(line, "and more", builtinPalette())
	if !strings.Contains(out, "48;2;") || !strings.Contains(out, "\x1b[39;49m") {
		t.Fatalf("no highlight span in %q", out)
	}
	if ansi.Strip(out) != ansi.Strip(line) {
		t.Errorf("highlight changed the text: %q", ansi.Strip(out))
	}
	if !strings.Contains(out, "\x1b[1mbold\x1b[22m") {
		t.Errorf("bold span was disturbed: %q", out)
	}
}

func TestSearchPromptShowsCounts(t *testing.T) {
	v := newViewer("doc.md", tocDoc, builtinPalette(), renderOpts{images: image.Off}, 70, 10)
	send(v, key('/'), key('s'), key('e'), key('c'))
	footer := ansi.Strip(strings.Split(v.View().Content, "\r\n")[v.page()])
	if !strings.Contains(footer, "/sec") || !strings.Contains(footer, "match") {
		t.Errorf("footer = %q", footer)
	}
	v.typing = false
	v.query = "zzz"
	v.runSearch()
	footer = ansi.Strip(strings.Split(v.View().Content, "\r\n")[v.page()])
	if !strings.Contains(footer, "0/0") {
		t.Errorf("footer without matches = %q", footer)
	}
}

// -toc has to work for the interactive viewer too, not only for -frame.
func TestShowTOCOpensThePane(t *testing.T) {
	v := newViewer("doc.md", tocDoc, builtinPalette(), renderOpts{images: image.Off}, 60, 10)
	wide := v.contentW
	v.showTOC()
	if !v.tocOpen {
		t.Fatal("-toc did not open the pane")
	}
	if v.contentW >= wide {
		t.Errorf("content width %d did not shrink from %d", v.contentW, wide)
	}
	if v.tocSel != 0 {
		t.Errorf("selection starts at %d, want the first heading", v.tocSel)
	}
	if got := plainView(v); !strings.Contains(got, "Second section") {
		t.Errorf("pane is there but has no headings:\n%s", got)
	}
	v.showTOC() // idempotent: asking twice must not close it
	if !v.tocOpen {
		t.Error("calling showTOC again closed the pane")
	}
}

func TestTOCWithoutHeadingsSaysSo(t *testing.T) {
	v := newViewer("doc.md", "just a paragraph, no headings at all\n", builtinPalette(),
		renderOpts{images: image.Off}, 60, 10)
	v.showTOC()
	if len(v.toc) != 0 {
		t.Fatalf("toc = %+v, want none", v.toc)
	}
	if got := plainView(v); !strings.Contains(got, "no headings") {
		t.Errorf("empty pane should say so:\n%s", got)
	}
	// Nothing to select, and jumping must not panic.
	send(v, special(tea.KeyEnter))
	send(v, key('j'), key('j'))
}
