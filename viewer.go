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
	width    int
	height   int
	contentW int      // width - scrollbar
	lines    []string // rendered ANSI, as displayed
	links    [][]link // links[line][n]
	off      int      // first visible line
	drag     bool     // scrollbar drag in progress
}

func newViewer(title, src string, pal Palette, width, height int) *viewer {
	v := &viewer{title: title, src: src, pal: pal}
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
	v.contentW = width - 1 // last column is the scrollbar
	out, err := renderMarkdown(v.src, v.pal, v.contentW)
	if err != nil {
		v.lines = []string{"mdv: " + err.Error()}
		v.links = nil
		return
	}
	v.lines = strings.Split(out, "\n")
	v.links = indexLinks(v.lines)
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
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return v, tea.Quit
		case "j", "down":
			v.scroll(1)
		case "k", "up":
			v.scroll(-1)
		case "d", "pgdown", " ":
			v.scroll(v.page() / 2)
		case "u", "pgup":
			v.scroll(-v.page() / 2)
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

func (v *viewer) linkAt(row, col int) string {
	line := v.off + row
	if line < 0 || line >= len(v.links) {
		return ""
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
			line = v.lines[n]
		}
		b.WriteString(line)
		b.WriteString("\x1b[m") // a trimmed line ends inside its style; stop it before padding
		b.WriteString(strings.Repeat(" ", max(0, v.contentW-ansi.StringWidth(line))))
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
	pct := 100
	if v.maxOff() > 0 {
		pct = v.off * 100 / v.maxOff()
	}
	left := paint(v.pal.Muted, v.title)
	right := paint(v.pal.Muted, fmt.Sprintf("%d%%  %d/%d  q quit", pct, min(v.off+v.page(), len(v.lines)), len(v.lines)))
	gap := v.width - ansi.StringWidth(left) - ansi.StringWidth(right)
	return left + strings.Repeat(" ", max(1, gap)) + right
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
