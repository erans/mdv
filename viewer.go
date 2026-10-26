package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// link is a clickable span on one rendered line, in visible columns.
type link struct {
	start, end int
	url        string
}

type viewer struct {
	title    string
	src      string
	pal      Palette
	opts     renderOpts
	width    int
	height   int
	contentW int      // width left for the document
	lines    []string // rendered ANSI, as displayed
	links    [][]link // links[line][n]
	off      int      // first visible line
	hOff     int      // first visible column, for lines wider than the pane
	drag     bool     // scrollbar drag in progress

	toc     []tocEntry // headings and where they landed
	tocOpen bool
	tocSel  int
	tocOff  int

	query    string // accepted search, empty when off
	typing   bool   // the / prompt is open
	matches  []int  // lines containing the query
	matchIdx int
}

func newViewer(title, src string, pal Palette, opts renderOpts, width, height int) *viewer {
	opts.allowWide = true // the viewer scrolls wide diagrams instead of showing source
	v := &viewer{title: title, src: src, pal: pal, opts: opts}
	v.resize(width, height)
	return v
}

func (v *viewer) Init() tea.Cmd { return nil }

func (v *viewer) resize(width, height int) {
	if width < 20 {
		width = 20
	}
	if height < 4 {
		height = 4
	}
	v.width, v.height = width, height
	v.contentW = v.contentWidth()
	out, err := renderMarkdown(v.src, v.pal, v.opts, v.contentW)
	if err != nil {
		v.lines = []string{"mdv: " + err.Error()}
		v.links = nil
		return
	}
	v.lines = strings.Split(out, "\n")
	v.links = indexLinks(v.lines)
	v.buildTOC()
	v.runSearch()
	v.clamp()
}

// page is the number of content lines on screen (the footer takes one).
func (v *viewer) page() int { return v.height - 1 }

func (v *viewer) maxOff() int {
	if m := len(v.lines) - v.page(); m > 0 {
		return m
	}
	return 0
}

func (v *viewer) clamp() {
	if v.off > v.maxOff() {
		v.off = v.maxOff()
	}
	if v.off < 0 {
		v.off = 0
	}
	v.hOff = min(max(0, v.hOff), v.maxHOff())
}

// maxHOff is how far the widest line can be scrolled sideways.
func (v *viewer) maxHOff() int {
	widest := 0
	for _, line := range v.lines {
		widest = max(widest, ansi.StringWidth(line))
	}
	return max(0, widest-v.contentW)
}

// scrollX moves the sideways offset; text stays put because only lines wider
// than the pane are cut.
func (v *viewer) scrollX(delta int) {
	v.hOff = min(max(0, v.hOff+delta), v.maxHOff())
}

func (v *viewer) scroll(delta int) {
	v.off += delta
	v.clamp()
}

// jumpTo maps a screen row to a scroll position, for scrollbar clicks and drags.
func (v *viewer) jumpTo(row int) {
	if v.maxOff() == 0 {
		return
	}
	frac := float64(row) / float64(v.page()-1)
	v.off = int(frac*float64(v.maxOff()) + 0.5)
	v.clamp()
}

func (v *viewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.resize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		if v.typing {
			return v, v.searchKey(msg)
		}
		if v.tocOpen {
			if cmd, handled := v.tocKey(msg); handled {
				return v, cmd
			}
		}
		switch msg.String() {
		case "/":
			v.typing, v.query = true, ""
			v.runSearch()
		case "n":
			v.searchNext(1)
		case "N":
			v.searchNext(-1)
		case "t":
			v.toggleTOC()
		case "q", "esc", "ctrl+c":
			if v.query != "" || v.tocOpen {
				v.query, v.tocOpen = "", false
				v.resize(v.width, v.height)
				return v, nil
			}
			return v, tea.Quit
		case "j", "down":
			v.scroll(1)
		case "k", "up":
			v.scroll(-1)
		case "d", "pgdown", " ":
			v.scroll(v.page() / 2)
		case "u", "pgup":
			v.scroll(-v.page() / 2)
		case "H", "shift+right":
			v.scrollX(8)
		case "L", "shift+left":
			v.scrollX(-8)
		case "0":
			v.hOff = 0
		case "g", "home":
			v.off = 0
		case "G", "end":
			v.off = v.maxOff()
		}
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			v.scroll(-3)
		case tea.MouseWheelDown:
			v.scroll(3)
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			break
		}
		switch {
		case v.tocOpen && msg.X > v.contentW: // table of contents
			if i := v.tocOff + msg.Y; i >= 0 && i < len(v.toc) && msg.Y < v.page() {
				v.tocSel = i
				v.off = v.toc[i].line
				v.clamp()
			}
		case msg.X >= v.contentW: // scrollbar
			v.drag = true
			v.jumpTo(msg.Y)
		case msg.Y < v.page():
			if url := v.linkAt(msg.Y, msg.X); url != "" {
				return v, openURL(url)
			}
		}
	case tea.MouseMotionMsg:
		if v.drag && msg.X >= v.contentW {
			v.jumpTo(msg.Y)
		}
	case tea.MouseReleaseMsg:
		v.drag = false
	}
	return v, nil
}

// clip cuts a line that is wider than the pane to the current sideways offset.
// Lines that fit are left alone, so body text never shifts.
func (v *viewer) clip(line string) string {
	if ansi.StringWidth(line) <= v.contentW {
		return line
	}
	return ansi.Cut(line, v.hOff, v.hOff+v.contentW)
}

func (v *viewer) linkAt(row, col int) string {
	line := v.off + row
	if line < 0 || line >= len(v.links) {
		return ""
	}
	if ansi.StringWidth(v.lines[line]) > v.contentW {
		col += v.hOff // the line is scrolled, so are its links
	}
	for _, l := range v.links[line] {
		if col >= l.start && col < l.end {
			return l.url
		}
	}
	return ""
}

func (v *viewer) View() tea.View {
	var b strings.Builder
	page := v.page()
	thumbTop, thumbLen := v.thumb(page)

	for i := 0; i < page; i++ {
		n := v.off + i
		line := ""
		if n < len(v.lines) {
			line = v.clip(v.lines[n])
			line = highlight(line, v.query, v.pal)
		}
		b.WriteString(line)
		b.WriteString("\x1b[m") // a trimmed line ends inside its style; stop it before padding
		b.WriteString(strings.Repeat(" ", max(0, v.contentW-ansi.StringWidth(line))))
		if v.tocOpen {
			b.WriteString(v.tocCell(i, page))
		}
		b.WriteString(v.scrollbarCell(i, page, thumbTop, thumbLen))
		b.WriteString("\r\n")
	}
	b.WriteString(v.footer())
	b.WriteString("\r\n")

	view := tea.NewView(b.String())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

// thumb returns the thumb's top row and length in a track of `page` rows.
func (v *viewer) thumb(page int) (top, length int) {
	if v.maxOff() == 0 || page <= 0 {
		return 0, 0
	}
	length = max(1, page*page/len(v.lines))
	top = (page - length) * v.off / v.maxOff()
	return top, length
}

func (v *viewer) scrollbarCell(row, page, thumbTop, thumbLen int) string {
	if thumbLen == 0 {
		return " "
	}
	if row >= thumbTop && row < thumbTop+thumbLen {
		return paint(v.pal.Accent, "█")
	}
	return paint(v.pal.Muted, "│")
}

func (v *viewer) footer() string {
	if v.typing {
		prompt := "/" + v.query
		status := "enter go  esc cancel"
		if v.query != "" {
			status = fmt.Sprintf("%s  %d match", plural(len(v.matches)), len(v.matches))
			if len(v.matches) == 0 {
				status = "no match  esc cancel"
			}
		}
		left := paint(v.pal.Accent, prompt) + paint(v.pal.Muted, "▏")
		right := paint(v.pal.Muted, status)
		gap := v.width - ansi.StringWidth(left) - ansi.StringWidth(right)
		return left + strings.Repeat(" ", max(1, gap)) + right
	}
	pct := 100
	if v.maxOff() > 0 {
		pct = v.off * 100 / v.maxOff()
	}
	hints := "q quit  / search  t toc"
	if v.maxHOff() > 0 {
		hints = fmt.Sprintf("q quit  / search  t toc  H/L ↔ %d/%d", v.hOff, v.maxHOff())
	}
	if v.query != "" {
		hints = fmt.Sprintf("%d/%d  n/N  esc clear", min(v.matchIdx+1, len(v.matches)), len(v.matches))
	}
	left := paint(v.pal.Muted, v.title)
	right := paint(v.pal.Muted, fmt.Sprintf("%d%%  %d/%d  %s", pct, min(v.off+v.page(), len(v.lines)), len(v.lines), hints))
	gap := v.width - ansi.StringWidth(left) - ansi.StringWidth(right)
	return left + strings.Repeat(" ", max(1, gap)) + right
}

func plural(n int) string {
	if n == 1 {
		return "match"
	}
	return "matches"
}

// searchKey handles the / prompt: text goes into the query, enter accepts and
// jumps, escape throws it away.
func (v *viewer) searchKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "ctrl+c":
		v.typing, v.query = false, ""
		v.runSearch()
		return nil
	case "enter":
		v.typing = false
		v.runSearch()
		v.searchJump(0)
		return nil
	case "backspace":
		if v.query != "" {
			r := []rune(v.query)
			v.query = string(r[:len(r)-1])
			v.runSearch()
		}
		return nil
	}
	if t := msg.String(); len([]rune(t)) == 1 {
		v.query += t
		v.runSearch()
		v.searchJump(0)
	}
	return nil
}

// showTOC opens the contents pane if it is not open yet, which is what the
// -toc flag asks for on start.
func (v *viewer) showTOC() {
	if !v.tocOpen {
		v.toggleTOC()
	}
}

// toggleTOC opens or closes the table of contents; the document has to be
// re-rendered at the narrower width.
func (v *viewer) toggleTOC() {
	v.tocOpen = !v.tocOpen
	if v.tocOpen {
		if i := v.tocIndexAt(v.off); i >= 0 {
			v.tocSel = i
		}
	}
	v.resize(v.width, v.height)
}

// tocKey handles the TOC pane: j/k move the selection, enter jumps, esc closes.
func (v *viewer) tocKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down":
		v.tocSel = min(v.tocSel+1, len(v.toc)-1)
		v.scrollTOC()
		return nil, true
	case "k", "up":
		v.tocSel = max(v.tocSel-1, 0)
		v.scrollTOC()
		return nil, true
	case "enter":
		if v.tocSel < len(v.toc) {
			v.off = v.toc[v.tocSel].line
			v.clamp()
		}
		return nil, true
	case "t":
		v.toggleTOC()
		return nil, true
	}
	return nil, false
}

func (v *viewer) scrollTOC() {
	if v.tocSel < v.tocOff {
		v.tocOff = v.tocSel
	}
	if v.tocSel >= v.tocOff+v.page() {
		v.tocOff = v.tocSel - v.page() + 1
	}
}

// tocIndexAt is the heading a document line belongs to.
func (v *viewer) tocIndexAt(line int) int {
	idx := -1
	for i, e := range v.toc {
		if e.line <= line {
			idx = i
		}
	}
	return idx
}

// tocCell renders one row of the contents pane, including its separator column.
func (v *viewer) tocCell(row, page int) string {
	w := tocWidth(v.width)
	sep := paint(v.pal.Muted, "│")
	if row >= page {
		return sep
	}
	i := v.tocOff + row
	if len(v.toc) == 0 {
		if row == 0 {
			return sep + " " + paint(v.pal.Muted, ansi.Truncate("no headings", w-2, "…"))
		}
		return sep + strings.Repeat(" ", w)
	}
	if i < 0 || i >= len(v.toc) {
		return sep + strings.Repeat(" ", w)
	}
	e := v.toc[i]
	text := strings.Repeat("  ", max(0, e.level-1)) + e.text
	text = ansi.Truncate(text, w-2, "…")
	pad := strings.Repeat(" ", max(0, w-2-ansi.StringWidth(text)))
	switch {
	case i == v.tocSel:
		return sep + paintOn(v.pal.Selection, v.pal.Fg, " "+text+pad+" ")
	case i == v.tocIndexAt(v.off):
		return sep + " " + paint(v.pal.Accent, text) + pad + " "
	default:
		return sep + " " + paint(v.pal.Muted, text) + pad + " "
	}
}

// indexLinks recovers the clickable spans of each line from the OSC 8
// hyperlinks the renderer emitted.
func indexLinks(lines []string) [][]link {
	out := make([][]link, len(lines))
	for i, line := range lines {
		var current string
		var start, col int
		for j := 0; j < len(line); {
			if url, n, ok := osc8At(line[j:]); ok {
				if url == "" {
					if current != "" {
						out[i] = append(out[i], link{start, col, current})
						current = ""
					}
				} else if current == "" {
					current, start = url, col
				}
				j += n
				continue
			}
			if loc := sgr.FindStringIndex(line[j:]); loc != nil && loc[0] == 0 {
				j += loc[1]
				continue
			}
			r, size := utf8.DecodeRuneInString(line[j:])
			col += ansi.StringWidth(string(r))
			j += size
		}
		if current != "" {
			out[i] = append(out[i], link{start, col, current})
		}
	}
	return out
}

// osc8At parses an OSC 8 hyperlink introducer or terminator at the start of s.
func osc8At(s string) (url string, n int, ok bool) {
	if !strings.HasPrefix(s, "\x1b]8;") {
		return "", 0, false
	}
	rest := s[4:]
	bel := strings.IndexByte(rest, '\a')
	st := strings.Index(rest, "\x1b\\")
	var payload string
	switch {
	case bel >= 0 && (st < 0 || bel < st):
		payload, n = rest[:bel], bel+1
	case st >= 0:
		payload, n = rest[:st], st+2
	default:
		return "", 0, false
	}
	_, url, found := strings.Cut(payload, ";")
	if !found {
		return "", 0, false
	}
	return url, 4 + n, true
}

// openURL hands the URL to the desktop, detached from the viewer.
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		opener := "xdg-open"
		if runtime.GOOS == "darwin" {
			opener = "open"
		}
		cmd := exec.Command(opener, url)
		_ = cmd.Start()
		go func() { _ = cmd.Wait() }() // reap, but never block the UI
		return nil
	}
}
