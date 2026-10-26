package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// cellAlign is a column's alignment, from the `:---:` row.
type cellAlign uint8

const (
	alignLeft cellAlign = iota
	alignCenter
	alignRight
)

// table is a parsed markdown table. glamour draws tables by handing them to
// lipgloss, which truncates a cell when the table lands exactly on the wrap
// width; doing the width arithmetic here keeps every cell readable and lets the
// borders take the theme's muted colour.
type table struct {
	header []string
	align  []cellAlign
	rows   [][]string
	raw    []string // source lines, for the fallback when the pane is too narrow
}

// parseTable reads a GFM table at the start of lines. The second line must be a
// delimiter row (`|---|:--:|`), which is what tells a table from a paragraph
// that happens to contain pipes.
func parseTable(lines []string) (*table, int) {
	if len(lines) < 2 {
		return nil, 0
	}
	header, ok := splitRow(lines[0])
	if !ok {
		return nil, 0
	}
	aligns, ok := splitRow(lines[1])
	if !ok || len(aligns) != len(header) {
		return nil, 0
	}
	t := &table{header: header}
	for _, a := range aligns {
		switch {
		case strings.HasPrefix(a, ":") && strings.HasSuffix(a, ":"):
			t.align = append(t.align, alignCenter)
		case strings.HasSuffix(a, ":"):
			t.align = append(t.align, alignRight)
		default:
			t.align = append(t.align, alignLeft)
		}
		if strings.Trim(a, ":- ") != "" {
			return nil, 0 // not a delimiter row
		}
	}
	n := 2
	for n < len(lines) {
		row, ok := splitRow(lines[n])
		if !ok || strings.TrimSpace(lines[n]) == "" {
			break
		}
		t.rows = append(t.rows, row)
		n++
	}
	t.raw = lines[:n]
	return t, n
}

// splitRow splits one table line into cells. `\|` is an escaped pipe, and the
// outer pipes are optional.
func splitRow(line string) ([]string, bool) {
	if !strings.Contains(line, "|") {
		return nil, false
	}
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '|' {
			cur.WriteByte('|')
			i++
			continue
		}
		if s[i] == '|' {
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	cells = append(cells, strings.TrimSpace(cur.String()))
	return cells, true
}

const maxUnbrokenWord = 30

// render lays the table out in at most width columns. It returns nil when the
// pane cannot fit a one-column table, so the caller can show the source.
func (t *table) render(pal Palette, width int) []string {
	cols := len(t.header)
	overhead := 3*cols + 1 // "│ " + " │ " per column + " │"
	available := width - overhead
	if available < 2*cols { // narrower than that is not a table any more
		return nil
	}

	styled := make([][]string, len(t.rows)+1)
	styled[0] = t.styledRow(t.header, pal)
	for i, row := range t.rows {
		styled[i+1] = t.styledRow(row, pal)
	}

	natural := make([]int, cols)
	minWord := make([]int, cols)
	for _, row := range styled {
		for c := 0; c < cols && c < len(row); c++ {
			natural[c] = max(natural[c], ansi.StringWidth(row[c]))
			minWord[c] = max(minWord[c], longestWord(row[c]))
		}
	}
	for c := range minWord {
		minWord[c] = min(max(minWord[c], 1), maxUnbrokenWord)
	}
	widths := fitColumns(natural, minWord, available)

	lines := []string{border(widths, '┌', '┬', '┐', pal)}
	lines = append(lines, t.rowLines(styled[0], widths, true, pal)...)
	lines = append(lines, border(widths, '├', '┼', '┤', pal))
	for i, row := range styled[1:] {
		lines = append(lines, t.rowLines(row, widths, false, pal)...)
		if i < len(styled)-2 {
			lines = append(lines, border(widths, '├', '┼', '┤', pal))
		}
	}
	lines = append(lines, border(widths, '└', '┴', '┘', pal))
	return lines
}

// fitColumns returns a width per column: natural when everything fits, squeezed
// proportionally between the longest word and the natural width when it does not.
func fitColumns(natural, minWord []int, available int) []int {
	cols := len(natural)
	widths := make([]int, cols)
	totalNatural, totalMin := 0, 0
	for i := range natural {
		widths[i] = max(natural[i], minWord[i])
		totalNatural += widths[i]
		totalMin += minWord[i]
	}
	if totalNatural <= available {
		// Slide the spare width into the columns that want it most.
		spare := available - totalNatural
		for i := 0; spare > 0; i = (i + 1) % cols {
			widths[i]++
			spare--
		}
		return widths
	}
	if totalMin >= available {
		// Even the words do not fit: give every column an equal share.
		share, extra := available/cols, available%cols
		for i := range widths {
			widths[i] = share
			if i < extra {
				widths[i]++
			}
		}
		return widths
	}
	// Shrink towards the minimum word widths, in proportion to what each column
	// would lose, then hand out the rounding leftovers.
	extra := available - totalMin
	potential := totalNatural - totalMin
	allocated := 0
	for i := range widths {
		room := max(0, natural[i]-minWord[i])
		widths[i] = minWord[i]
		if potential > 0 {
			grow := room * extra / potential
			widths[i] += grow
			allocated += grow
		}
	}
	for i := 0; allocated < extra; i = (i + 1) % cols {
		if widths[i] < natural[i] {
			widths[i]++
			allocated++
			continue
		}
		giveUp := true
		for j := range widths {
			if widths[j] < natural[j] {
				giveUp = false
			}
		}
		if giveUp {
			break
		}
	}
	return widths
}

// wrapCell breaks a cell to the column width: on spaces first, and mid-word
// only when a single word cannot fit.
func wrapCell(text string, width int) []string {
	parts := strings.Split(ansi.Wordwrap(text, width, " ,.;:-/|"), "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if ansi.StringWidth(part) > width {
			out = append(out, strings.Split(ansi.Hardwrap(part, width, true), "\n")...)
			continue
		}
		out = append(out, part)
	}
	return out
}

// rowLines wraps one row into as many text lines as its tallest cell needs.
func (t *table) rowLines(row []string, widths []int, header bool, pal Palette) []string {
	cellLines := make([][]string, len(widths))
	height := 1
	for c := range widths {
		text := ""
		if c < len(row) {
			text = row[c]
		}
		cellLines[c] = wrapCell(text, widths[c])
		height = max(height, len(cellLines[c]))
	}
	out := make([]string, 0, height)
	for line := 0; line < height; line++ {
		var b strings.Builder
		b.WriteString(paint(pal.Muted, "│"))
		for c := range widths {
			text := ""
			if line < len(cellLines[c]) {
				text = cellLines[c][line]
			}
			if w := ansi.StringWidth(text); w > widths[c] {
				text = ansi.Truncate(text, widths[c], "…")
			}
			pad := max(0, widths[c]-ansi.StringWidth(text))
			left, right := 0, pad // left aligned unless the delimiter row says otherwise
			if t.align[c] == alignRight {
				left, right = pad, 0
			} else if t.align[c] == alignCenter {
				left, right = pad/2, pad-pad/2
			}
			b.WriteString(" ")
			b.WriteString(strings.Repeat(" ", left))
			if header {
				b.WriteString(bold(text))
			} else {
				b.WriteString(text)
			}
			b.WriteString(strings.Repeat(" ", right))
			b.WriteString(" ")
			b.WriteString(paint(pal.Muted, "│"))
		}
		out = append(out, b.String())
	}
	return out
}

func border(widths []int, left, mid, right rune, pal Palette) string {
	var b strings.Builder
	b.WriteRune(left)
	for i, w := range widths {
		if i > 0 {
			b.WriteRune(mid)
		}
		b.WriteString(strings.Repeat("─", w+2))
	}
	b.WriteRune(right)
	return paint(pal.Muted, b.String())
}

// styledRow turns a row's cells into ANSI text; short cells keep their inline
// code and links, so a table reads like the rest of the document.
func (t *table) styledRow(cells []string, pal Palette) []string {
	out := make([]string, len(t.header))
	for c := range out {
		if c < len(cells) {
			out[c] = inlineStyle(cells[c], pal)
		}
	}
	return out
}

// longestWord is the widest word of a styled cell: the narrowest a column may
// get before a word has to be broken.
func longestWord(styled string) int {
	plain := ansi.Strip(styled)
	longest := 0
	for _, word := range strings.Fields(plain) {
		longest = max(longest, ansi.StringWidth(word))
	}
	return longest
}

func bold(s string) string {
	if s == "" {
		return s
	}
	return "\x1b[1m" + s + "\x1b[22m"
}

// inlineStyle renders the inline markdown a table cell uses: code, bold, italic,
// strikethrough and links (as OSC 8, so a cell link is clickable too).
func inlineStyle(s string, pal Palette) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "`"):
			if j := strings.Index(s[i+1:], "`"); j >= 0 {
				b.WriteString(paint(pal.Orange, s[i+1:i+1+j]))
				i += j + 2
				continue
			}
		case strings.HasPrefix(s[i:], "**"), strings.HasPrefix(s[i:], "__"):
			mark := s[i : i+2]
			if j := strings.Index(s[i+2:], mark); j >= 0 {
				b.WriteString(bold(inlineStyle(s[i+2:i+2+j], pal)))
				i += j + 4
				continue
			}
		case strings.HasPrefix(s[i:], "~~"):
			if j := strings.Index(s[i+2:], "~~"); j >= 0 {
				b.WriteString("\x1b[9m" + inlineStyle(s[i+2:i+2+j], pal) + "\x1b[29m")
				i += j + 4
				continue
			}
		case strings.HasPrefix(s[i:], "["):
			if text, url, n, ok := inlineLink(s[i:]); ok {
				b.WriteString("\x1b]8;;" + url + "\x07")
				b.WriteString(paint(pal.Blue, inlineStyle(text, pal)))
				b.WriteString("\x1b]8;;\x07")
				i += n
				continue
			}
		}
		// Plain text, up to the next marker.
		j := i + 1
		for j < len(s) && !strings.ContainsRune("`*_[~", rune(s[j])) {
			j++
		}
		b.WriteString(s[i:j])
		i = j
	}
	return b.String()
}

// inlineLink parses `[text](url)` at the start of s.
func inlineLink(s string) (text, url string, n int, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return "", "", 0, false
	}
	close := strings.Index(s, "](")
	if close < 0 {
		return "", "", 0, false
	}
	end := strings.Index(s[close+2:], ")")
	if end < 0 {
		return "", "", 0, false
	}
	return s[1:close], s[close+2 : close+2+end], close + 3 + end, true
}
