package mermaid

import (
	"errors"
	"strings"
)

type participant struct {
	id, label string
	w, life   int // box width and lifeline column
	x         int
}

type message struct {
	from, to int // participant indexes
	label    string
	dashed   bool
	arrow    bool
	self     bool
}

type noteSide uint8

const (
	noteOver noteSide = iota
	noteLeft
	noteRight
)

// seqNote is a note box next to, or spanning, participants.
type seqNote struct {
	from, to int
	side     noteSide
	text     string
}

type itemKind uint8

const (
	itemMessage itemKind = iota
	itemNote
	itemFrameStart
	itemFrameElse
	itemFrameEnd
)

// item is one row-producing thing in a sequence diagram.
type item struct {
	kind  itemKind
	msg   message
	note  seqNote
	text  string // fragment kind plus its text, or an `else` label
	depth int    // fragment nesting, 0 is outermost
}

// seqModel is a parsed sequence diagram.
type seqModel struct {
	parts    []participant
	items    []item
	maxDepth int
}

// renderSequence draws a sequence diagram: participant boxes on top, one
// lifeline each, messages as arrows, notes as boxes and loop/alt/opt blocks as
// titled frames.
func renderSequence(src string) (*Art, error) {
	m, err := parseSequence(src)
	if err != nil {
		return nil, err
	}
	return m.draw(), nil
}

func parseSequence(src string) (*seqModel, error) {
	m := &seqModel{}
	index := map[string]int{}
	find := func(id string) int {
		if i, ok := index[id]; ok {
			return i
		}
		index[id] = len(m.parts)
		m.parts = append(m.parts, participant{id: id, label: id})
		return len(m.parts) - 1
	}
	alias := func(id, label string) {
		i := find(id)
		if label != "" {
			m.parts[i].label = label
		}
	}
	depth := 0
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" || strings.HasPrefix(strings.ToLower(line), "sequencediagram") {
			continue
		}
		low := strings.ToLower(line)
		switch {
		case strings.HasPrefix(low, "participant "), strings.HasPrefix(low, "actor "):
			rest := line[strings.IndexByte(line, ' ')+1:]
			id, label, ok := strings.Cut(rest, " as ")
			if ok {
				alias(strings.TrimSpace(id), strings.TrimSpace(label))
			} else {
				alias(strings.TrimSpace(rest), "")
			}
		case strings.HasPrefix(low, "note "):
			if n, ok := parseNote(line, find); ok {
				m.items = append(m.items, item{kind: itemNote, note: n, depth: depth})
			}
		case strings.HasPrefix(low, "loop"), strings.HasPrefix(low, "alt"),
			strings.HasPrefix(low, "opt"), strings.HasPrefix(low, "par"),
			strings.HasPrefix(low, "critical"), strings.HasPrefix(low, "break"),
			strings.HasPrefix(low, "rect"):
			kind := strings.Fields(low)[0]
			m.items = append(m.items, item{
				kind:  itemFrameStart,
				text:  strings.TrimSpace(kind + " " + strings.TrimSpace(line[len(kind):])),
				depth: depth,
			})
			depth++
			m.maxDepth = max(m.maxDepth, depth)
		case strings.HasPrefix(low, "else"):
			m.items = append(m.items, item{
				kind:  itemFrameElse,
				text:  strings.TrimSpace(line[len("else"):]),
				depth: max(0, depth-1),
			})
		case low == "end":
			if depth > 0 {
				depth--
				m.items = append(m.items, item{kind: itemFrameEnd, depth: depth})
			}
		case strings.HasPrefix(low, "activate"), strings.HasPrefix(low, "deactivate"),
			strings.HasPrefix(low, "autonumber"):
		default:
			from, to, label, dashed, arrow, ok := splitMessage(line)
			if !ok {
				continue
			}
			a, b := find(from), find(to)
			m.items = append(m.items, item{kind: itemMessage, msg: message{
				from: a, to: b, label: label, dashed: dashed, arrow: arrow, self: a == b,
			}, depth: depth})
		}
	}
	if len(m.parts) == 0 {
		return nil, errors.New("no participants")
	}
	return m, nil
}

// parseNote reads `Note over A,B: text`, `Note over A: text` and the left/right
// forms. Notes also introduce participants that were never declared.
func parseNote(line string, find func(string) int) (seqNote, bool) {
	body, text, ok := strings.Cut(line, ":")
	if !ok {
		return seqNote{}, false
	}
	rest := strings.TrimSpace(body[strings.IndexByte(strings.ToLower(body), ' ')+1:])
	side := noteOver
	low := strings.ToLower(rest)
	switch {
	case strings.HasPrefix(low, "left of "):
		side, rest = noteLeft, rest[len("left of "):]
	case strings.HasPrefix(low, "right of "):
		side, rest = noteRight, rest[len("right of "):]
	case strings.HasPrefix(low, "over "):
		rest = rest[len("over "):]
	}
	names := strings.Split(rest, ",")
	from := find(strings.TrimSpace(names[0]))
	to := from
	if len(names) > 1 {
		to = find(strings.TrimSpace(names[len(names)-1]))
	}
	if to < from {
		from, to = to, from
	}
	return seqNote{from: from, to: to, side: side, text: strings.TrimSpace(text)}, true
}

type noteRow struct {
	note seqNote
	y    int
}

type frameRow struct {
	label  string
	depth  int
	start  int
	end    int
	elses  []int
	labels []string
}

// draw lays the model out and paints it: messages first, lifelines over the
// gaps, then notes (opaque boxes) and finally the fragment frames, which merge
// with whatever they cross.
func (m *seqModel) draw() *Art {
	parts := m.parts
	gap := 6
	for _, it := range m.items {
		gap = max(gap, min(len([]rune(it.msg.label))+3, 26))
	}
	shift := 2 * m.maxDepth // room for one frame border per nesting level
	x := shift
	for i := range parts {
		parts[i].w = len([]rune(parts[i].label)) + 4
		parts[i].x = x
		parts[i].life = x + parts[i].w/2
		x += parts[i].w
		if i < len(parts)-1 {
			x += gap
		}
	}
	// Size the canvas to the widest thing drawn: a self-message label or a note
	// can reach past the last lifeline.
	right := x
	for _, it := range m.items {
		switch it.kind {
		case itemMessage:
			m := it.msg
			label := len([]rune(m.label))
			if m.self {
				right = max(right, parts[m.from].life+4+label)
			} else {
				lo, hi := parts[m.from].life, parts[m.to].life
				right = max(right, (lo+hi)/2+label/2+1)
			}
		case itemNote:
			n := it.note
			boxW := noteWidth(parts, n) + 4
			switch n.side {
			case noteRight:
				right = max(right, parts[n.to].life+2+boxW)
			case noteOver:
				lo, hi := parts[n.from].life, parts[n.to].life
				right = max(right, (lo+hi)/2+boxW/2+1)
			}
		}
	}
	width := right + 2*m.maxDepth + 2
	c := newCanvas(width, 4+len(m.items)*5+8)

	for i := range parts {
		p := &parts[i]
		c.put(p.x, 0, '┌', Border)
		c.put(p.x+p.w-1, 0, '┐', Border)
		c.put(p.x, 2, '└', Border)
		c.put(p.x+p.w-1, 2, '┘', Border)
		for xx := p.x + 1; xx < p.x+p.w-1; xx++ {
			c.put(xx, 0, '─', Border)
			c.put(xx, 2, '─', Border)
		}
		c.put(p.x, 1, '│', Border)
		c.put(p.x+p.w-1, 1, '│', Border)
		c.text(p.x+2, 1, p.label, Title)
		c.put(p.life, 2, '┬', Border)
	}

	var notes []noteRow
	var frames []*frameRow
	var open []*frameRow
	y := 4
	for _, it := range m.items {
		switch it.kind {
		case itemFrameStart:
			f := &frameRow{label: it.text, depth: it.depth, start: y}
			frames = append(frames, f)
			open = append(open, f)
			y++
			continue
		case itemFrameElse:
			if len(open) > 0 {
				f := open[len(open)-1]
				f.elses = append(f.elses, y)
				f.labels = append(f.labels, it.text)
				y++
			}
			continue
		case itemFrameEnd:
			if len(open) > 0 {
				open[len(open)-1].end = y
				open = open[:len(open)-1]
				y++
			}
			continue
		case itemNote:
			notes = append(notes, noteRow{note: it.note, y: y})
			y += 2 + len(wrapWords(it.note.text, noteWidth(parts, it.note)))
			continue
		}
		msg := it.msg
		a, b := parts[msg.from], parts[msg.to]
		if msg.self {
			if msg.label != "" {
				c.text(a.life+2, y, msg.label, EdgeLabel)
				y++
			}
			c.put(a.life, y, '├', Border)
			for xx := a.life + 1; xx <= a.life+2; xx++ {
				c.put(xx, y, '─', Edge)
			}
			c.put(a.life+3, y, '┐', Edge)
			c.put(a.life+3, y+1, '┘', Edge)
			for xx := a.life + 1; xx < a.life+3; xx++ {
				c.put(xx, y+1, '─', Edge)
			}
			c.put(a.life+1, y+1, '◀', Edge)
			y += 2
			continue
		}
		if msg.label != "" {
			lo, hi := min(a.life, b.life), max(a.life, b.life)
			start := max((lo+hi)/2-len([]rune(msg.label))/2, lo+1)
			c.text(start, y, msg.label, EdgeLabel)
			y++
		}
		ch := '─'
		if msg.dashed {
			ch = '╌'
		}
		for xx := min(a.life, b.life) + 1; xx < max(a.life, b.life); xx++ {
			c.line(xx, y, ch, Edge)
		}
		if msg.arrow {
			if b.life > a.life {
				c.put(b.life-1, y, '▶', Edge)
			} else {
				c.put(b.life+1, y, '◀', Edge)
			}
		}
		y++
	}

	for i := range parts {
		for yy := 3; yy <= y; yy++ {
			c.line(parts[i].life, yy, '│', Border)
		}
	}
	for _, n := range notes {
		c.noteBox(parts, n.note, n.y, width)
	}
	for _, f := range frames {
		c.frameBox(2*f.depth, f.start, width-1-2*f.depth, f, width)
	}
	return c.art()
}

// noteWidth is the inside width of a note box: wide enough for its text, and at
// least as wide as the participants it covers.
func noteWidth(parts []participant, n seqNote) int {
	span := parts[n.to].life - parts[n.from].life
	return max(len([]rune(n.text)), span-2, 6)
}

// noteBox draws an opaque note, so it hides the lifelines it covers.
func (c *canvas) noteBox(parts []participant, n seqNote, y, width int) {
	inside := noteWidth(parts, n)
	boxW := inside + 4
	lo, hi := parts[n.from].life, parts[n.to].life
	var x0 int
	switch n.side {
	case noteLeft:
		x0 = lo - 2 - boxW
	case noteRight:
		x0 = hi + 2
	default:
		x0 = (lo+hi)/2 - boxW/2
	}
	x0 = max(1, min(x0, width-boxW-1))
	x1 := x0 + boxW - 1
	lines := wrapWords(n.text, inside)
	y1 := y + 1 + len(lines)

	for yy := y; yy <= y1; yy++ { // opaque: hide the lifelines underneath
		for x := x0; x <= x1; x++ {
			c.put(x, yy, ' ', Plain)
		}
	}

	c.put(x0, y, '┌', Border)
	c.put(x1, y, '┐', Border)
	c.put(x0, y1, '└', Border)
	c.put(x1, y1, '┘', Border)
	for x := x0 + 1; x < x1; x++ {
		c.put(x, y, '─', Border)
		c.put(x, y1, '─', Border)
	}
	for yy := y + 1; yy < y1; yy++ {
		c.put(x0, yy, '│', Border)
		c.put(x1, yy, '│', Border)
	}
	for i, line := range lines {
		c.text(x0+2, y+1+i, line, Text)
	}
}

// frameBox draws a loop/alt/opt frame: a titled top border, a divider per
// `else`, and a bottom border.
func (c *canvas) frameBox(x0, y0, x1 int, f *frameRow, width int) {
	y1 := f.end
	if y1 <= y0 {
		return
	}
	for y := y0; y <= y1; y++ {
		if y != y0 && y != y1 && !containsInt(f.elses, y) {
			c.line(x0, y, '│', Border)
			c.line(x1, y, '│', Border)
		}
	}
	c.cornerLine(x0, x1, y0, '┌', '┐')
	c.cornerLine(x0, x1, y1, '└', '┘')
	if f.label != "" {
		c.text(x0+2, y0, " "+f.label+" ", Title)
	}
	for i, y := range f.elses {
		c.cornerLine(x0, x1, y, '├', '┤')
		if i < len(f.labels) && f.labels[i] != "" {
			c.text(x0+2, y, " "+f.labels[i]+" ", Title)
		}
	}
}

// cornerLine draws a horizontal border with its two corner glyphs, merging with
// the arrows it crosses.
func (c *canvas) cornerLine(x0, x1, y int, left, right rune) {
	c.line(x0, y, left, Border)
	c.line(x1, y, right, Border)
	for x := x0 + 1; x < x1; x++ {
		c.line(x, y, '─', Border)
	}
}

func containsInt(list []int, v int) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// wrapWords breaks text into lines of at most width runes, on spaces first.
func wrapWords(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	var cur string
	for _, word := range strings.Fields(text) {
		switch {
		case cur == "":
			cur = word
		case len([]rune(cur))+1+len([]rune(word)) <= width:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// splitMessage parses `A->>B: text`, `A-->>B: text`, `A-)B: text` and friends.
func splitMessage(line string) (from, to, label string, dashed, arrow, ok bool) {
	body, label := line, ""
	if i := strings.Index(line, ":"); i >= 0 {
		body, label = strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	for _, token := range []string{"-->>", "-->", "->>", "-)", "-x", "--x", "->", "--"} {
		i := strings.Index(body, token)
		if i <= 0 {
			continue
		}
		from = strings.TrimSpace(body[:i])
		to = strings.TrimSpace(body[i+len(token):])
		if from == "" || to == "" {
			return "", "", "", false, false, false
		}
		return from, to, label,
			strings.HasPrefix(token, "--") || strings.Contains(token, "x"),
			!strings.HasSuffix(token, ")"), true
	}
	return "", "", "", false, false, false
}
