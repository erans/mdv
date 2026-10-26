package main

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// tocEntry is one heading, and the rendered line it turned into.
type tocEntry struct {
	level int
	text  string
	line  int
}

// tocWidth is the pane's width: a third of the screen, kept sane.
func tocWidth(width int) int {
	return min(max(20, width/3), 36)
}

// contentWidth is what is left for the document: the scrollbar and, when open,
// the table of contents get their own columns.
func (v *viewer) contentWidth() int {
	w := v.width - 1
	if v.tocOpen {
		w -= tocWidth(v.width) + 1
	}
	return max(20, w)
}

// buildTOC reads the headings out of the markdown source and finds the rendered
// line each one became. glamour draws headings without their `#` markers, so the
// text is enough to match on.
func (v *viewer) buildTOC() {
	v.toc = v.toc[:0]
	heads := headings(v.src)
	next := 0
	for _, h := range heads {
		idx := -1
		for i := next; i < len(v.lines); i++ {
			if strings.TrimSpace(ansi.Strip(v.lines[i])) == h.text {
				idx = i
				break
			}
		}
		if idx >= 0 {
			next = idx + 1
		} else {
			idx = min(next, max(0, len(v.lines)-1))
		}
		v.toc = append(v.toc, tocEntry{level: h.level, text: h.text, line: idx})
	}
	if v.tocSel >= len(v.toc) {
		v.tocSel = max(0, len(v.toc)-1)
	}
}

type heading struct {
	level int
	text  string
}

// headings lists the ATX headings of a document, ignoring fenced code.
func headings(src string) []heading {
	var out []heading
	inFence := false
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(line, "#") {
			continue
		}
		level := 0
		for level < len(line) && line[level] == '#' {
			level++
		}
		if level > 6 || level == len(line) || line[level] != ' ' {
			continue
		}
		text := stripInline(strings.TrimSpace(line[level+1:]))
		if text == "" {
			continue
		}
		out = append(out, heading{level: level, text: text})
	}
	return out
}

// stripInline removes the markdown that does not survive into the rendered
// text: code backticks, emphasis, strikethrough and link syntax.
func stripInline(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "`"):
			if j := strings.Index(s[i+1:], "`"); j >= 0 {
				b.WriteString(s[i+1 : i+1+j])
				i += j + 2
				continue
			}
		case strings.HasPrefix(s[i:], "**"), strings.HasPrefix(s[i:], "__"),
			strings.HasPrefix(s[i:], "~~"):
			i += 2
			continue
		case s[i] == '*' || s[i] == '_':
			i++
			continue
		case s[i] == '[':
			if text, _, n, ok := inlineLink(s[i:]); ok {
				b.WriteString(stripInline(text))
				i += n
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return strings.TrimSpace(b.String())
}

// --- search ---------------------------------------------------------------

// runSearch recomputes the matches of the current query.
func (v *viewer) runSearch() {
	v.matches = v.matches[:0]
	q := strings.ToLower(v.query)
	if q == "" {
		return
	}
	for i, line := range v.lines {
		if strings.Contains(strings.ToLower(ansi.Strip(line)), q) {
			v.matches = append(v.matches, i)
		}
	}
}

// searchJump moves to the match at index i (wrapping), so it lands just under
// the top of the screen.
func (v *viewer) searchJump(i int) {
	if len(v.matches) == 0 {
		return
	}
	v.matchIdx = ((i % len(v.matches)) + len(v.matches)) % len(v.matches)
	v.off = max(0, v.matches[v.matchIdx]-1)
	v.clamp()
}

// searchNext steps through the matches by delta, wrapping at either end.
func (v *viewer) searchNext(delta int) {
	if len(v.matches) == 0 {
		return
	}
	v.searchJump(v.matchIdx + delta)
}

// highlight wraps every occurrence of the query in reverse video. The line is
// walked once, copying escape sequences through untouched and opening or closing
// the highlight at the visible byte offsets where the query matches.
func highlight(line, query string, pal Palette) string {
	if query == "" {
		return line
	}
	plain := ansi.Strip(line)
	q := strings.ToLower(query)
	lower := strings.ToLower(plain)
	if !strings.Contains(lower, q) {
		return line
	}
	starts := map[int]bool{}
	ends := map[int]bool{}
	for from := 0; from+len(q) <= len(plain); {
		k := strings.Index(lower[from:], q)
		if k < 0 {
			break
		}
		a, b := from+k, from+k+len(q)
		starts[a], ends[b] = true, true
		from = b
	}

	var out strings.Builder
	for i, visible := 0, 0; i < len(line); {
		if loc := sgr.FindStringIndex(line[i:]); loc != nil && loc[0] == 0 {
			out.WriteString(line[i : i+loc[1]])
			i += loc[1]
			continue
		}
		if _, n, ok := osc8At(line[i:]); ok {
			out.WriteString(line[i : i+n])
			i += n
			continue
		}
		_, size := utf8.DecodeRuneInString(line[i:])
		if starts[visible] {
			out.WriteString(paintOpen(pal.Accent, pal.Bg))
		}
		out.WriteString(line[i : i+size])
		visible += size
		if ends[visible] {
			out.WriteString("\x1b[39;49m")
		}
		i += size
	}
	return out.String()
}
