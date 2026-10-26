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

// renderSequence draws a sequence diagram: participant boxes on top, one
// lifeline each, messages as arrows between them with their labels above.
func renderSequence(src string) (*Art, error) {
	var parts []participant
	var msgs []message
	index := map[string]int{}
	find := func(id string) int {
		if i, ok := index[id]; ok {
			return i
		}
		index[id] = len(parts)
		parts = append(parts, participant{id: id, label: id})
		return len(parts) - 1
	}
	alias := func(id, label string) {
		i := find(id)
		if label != "" {
			parts[i].label = label
		}
	}

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
			continue
		case strings.HasPrefix(low, "note"), strings.HasPrefix(low, "activate"),
			strings.HasPrefix(low, "deactivate"), strings.HasPrefix(low, "loop"),
			strings.HasPrefix(low, "alt"), strings.HasPrefix(low, "else"),
			strings.HasPrefix(low, "opt"), strings.HasPrefix(low, "par"),
			strings.HasPrefix(low, "end"), strings.HasPrefix(low, "critical"),
			strings.HasPrefix(low, "break"), strings.HasPrefix(low, "rect"),
			strings.HasPrefix(low, "autonumber"):
			continue
		}
		from, to, label, dashed, arrow, ok := splitMessage(line)
		if !ok {
			continue
		}
		msgs = append(msgs, message{from: find(from), to: find(to), label: label, dashed: dashed, arrow: arrow, self: from == to})
	}
	if len(parts) == 0 {
		return nil, errors.New("no participants")
	}

	// Column geometry: boxes on top, lifelines centred, gaps wide enough for
	// the longest label that has to sit between two neighbours.
	gap := 6
	for _, m := range msgs {
		gap = max(gap, min(len([]rune(m.label))+3, 26))
	}
	x := 0
	for i := range parts {
		parts[i].w = len([]rune(parts[i].label)) + 4
		parts[i].x = x
		parts[i].life = x + parts[i].w/2
		x += parts[i].w + gap
	}

	rows := 4 // header box (3) plus one spacer
	for _, m := range msgs {
		rows += 1 + boolRows(m.label)
		if m.self {
			rows++
		}
	}
	width := 0
	for i := range parts {
		width = max(width, parts[i].x+parts[i].w)
	}
	for _, m := range msgs {
		if l := len([]rune(m.label)); l > 0 {
			width = max(width, max(parts[m.from].life, parts[m.to].life)+l+2)
		}
	}
	c := newCanvas(width+2, rows+1)

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
		for y := 3; y <= rows; y++ {
			c.put(p.life, y, '│', Border)
		}
	}

	y := 4
	for _, m := range msgs {
		a, b := parts[m.from], parts[m.to]
		if m.self {
			if m.label != "" {
				c.text(a.life+2, y, m.label, EdgeLabel)
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
		if m.label != "" {
			lo, hi := min(a.life, b.life), max(a.life, b.life)
			start := (lo+hi)/2 - len([]rune(m.label))/2
			start = max(start, lo+1)
			c.text(start, y, m.label, EdgeLabel)
			y++
		}
		ch := '─'
		if m.dashed {
			ch = '╌'
		}
		for xx := min(a.life, b.life) + 1; xx < max(a.life, b.life); xx++ {
			c.line(xx, y, ch, Edge)
		}
		if m.arrow {
			if b.life > a.life {
				c.put(b.life-1, y, '▶', Edge)
			} else {
				c.put(b.life+1, y, '◀', Edge)
			}
		}
		y++
	}
	return c.art(), nil
}

func boolRows(label string) int {
	if label == "" {
		return 0
	}
	return 1
}

// splitMessage parses `A->>B: text`, `A-->>B: text`, `A-)B: text` and friends.
func splitMessage(line string) (from, to, label string, dashed, arrow, ok bool) {
	body, label := line, ""
	if i := strings.Index(line, ":"); i >= 0 {
		body, label = strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	for _, token := range []string{"-->>", "-->", "->>", "-)", "-x", "--x", "->", "--"} {
		if i := strings.Index(body, token); i > 0 {
			from = strings.TrimSpace(body[:i])
			to = strings.TrimSpace(body[i+len(token):])
			if from == "" || to == "" {
				return "", "", "", false, false, false
			}
			return from, to, label,
				strings.HasPrefix(token, "--") || strings.Contains(token, "x"),
				!strings.HasSuffix(token, ")"), true
		}
	}
	return "", "", "", false, false, false
}
