// Package mermaid turns a mermaid flowchart into box-drawing art.
//
// Supported: graph/flowchart with LR, RL, TD and TB layout, rectangle,
// rounded and decision nodes, solid/dashed/thick edges, edge labels, chains
// (A --> B --> C) and fan-out (A --> B & C).
//
// shortcut: subgraphs are dropped to their members; there is no framing box or
// title yet. Upgrade when READMEs start using them.
package mermaid

import (
	"errors"
	"strings"
)

// Class is what a cell means, so the caller can color it.
type Class uint8

const (
	Plain Class = iota
	Border
	Text
	Edge
	EdgeLabel
	Title
)

// Cell is one drawn character.
type Cell struct {
	Ch rune
	Cl Class
}

// Art is a rendered diagram: rows of cells, already trimmed.
type Art struct {
	Cells [][]Cell
	Width int
}

type shape uint8

const (
	rectShape shape = iota
	roundShape
	decisionShape
)

type node struct {
	id, label string
	shape     shape
	w, h      int
	x, y      int
	rank      int
}

type edge struct {
	from, to *node
	label    string
	dashed   bool
	arrow    bool
}

type graph struct {
	dir   string
	nodes []*node
	edges []*edge
	byID  map[string]*node

	gapRows    [][]int // vertical: free rows inside each rank gap
	gapCols    [][]int // horizontal: free columns inside each rank gap
	bypass     map[*edge]int
	bypassSide int
	declared   bool // saw a `graph`/`flowchart` header
}

// Render parses and draws a flowchart. An error means the caller should show
// the fenced source instead.
func Render(src string) (*Art, error) {
	g := parse(src)
	if !g.declared {
		return nil, errors.New("not a flowchart")
	}
	if len(g.nodes) == 0 {
		return nil, errors.New("no flowchart nodes")
	}
	g.layout()
	return g.draw(), nil
}

func parse(src string) *graph {
	g := &graph{dir: "TD", byID: map[string]*node{}}
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "graph ") || strings.HasPrefix(low, "flowchart ") {
			if f := strings.Fields(low); len(f) > 1 {
				g.dir = strings.ToUpper(f[1])
			}
			g.declared = true
			continue
		}
		if isDirective(low) {
			continue
		}
		g.chain(line)
	}
	return g
}

func isDirective(low string) bool {
	for _, p := range []string{"subgraph", "end", "style ", "classdef", "class ", "linkstyle", "click", "direction"} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

func stripComment(s string) string {
	if i := strings.Index(s, "%%"); i >= 0 {
		return s[:i]
	}
	return s
}

func (g *graph) node(id, label string, sh shape) *node {
	if label == "" {
		label = id
	}
	if n, ok := g.byID[id]; ok {
		if n.label == n.id && label != n.id {
			n.label = label
			n.shape = sh
		}
		return n
	}
	n := &node{id: id, label: label, shape: sh}
	g.byID[id] = n
	g.nodes = append(g.nodes, n)
	return n
}

// chain parses one statement: nodes, then arrows to nodes, with `&` collecting
// a set of endpoints on either side of an arrow.
func (g *graph) chain(s string) {
	p := &scan{s: s}
	first, ok := p.node(g)
	if !ok {
		return
	}
	sources := []*node{first}
	p.skip()
	for p.eat('&') {
		if n, ok := p.node(g); ok {
			sources = append(sources, n)
		}
	}
	for {
		a, ok := p.arrow()
		if !ok {
			return
		}
		var targets []*node
		for {
			n, ok := p.node(g)
			if !ok {
				return
			}
			targets = append(targets, n)
			p.skip()
			if !p.eat('&') {
				break
			}
		}
		for _, from := range sources {
			for _, to := range targets {
				g.edges = append(g.edges, &edge{from: from, to: to, label: a.label, dashed: a.dashed, arrow: a.arrow})
			}
		}
		if !p.more() {
			return
		}
		sources = targets
	}
}

type scan struct {
	s string
	i int
}

func (p *scan) more() bool { return p.i < len(p.s) }
func (p *scan) skip() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t' || p.s[p.i] == ';') {
		p.i++
	}
}
func (p *scan) eat(c byte) bool {
	if p.i < len(p.s) && p.s[p.i] == c {
		p.i++
		return true
	}
	return false
}

func isIDChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

func (p *scan) node(g *graph) (*node, bool) {
	p.skip()
	start := p.i
	for p.i < len(p.s) && isIDChar(p.s[p.i]) {
		p.i++
	}
	if start == p.i {
		return nil, false
	}
	id := p.s[start:p.i]
	var label string
	sh := rectShape
	switch {
	case p.eat('['):
		label = p.balanced(']')
	case p.eat('{'):
		label, sh = p.balanced('}'), decisionShape
	case p.eat('('):
		switch {
		case p.eat('('):
			label = p.balancedPair(')', ')')
		case p.eat('['):
			label = p.balancedPair(']', ')')
		default:
			label = p.balanced(')')
		}
		sh = roundShape
	case p.eat('>'):
		label = p.balanced(']')
	}
	if i := strings.Index(label, ":::"); i >= 0 {
		label = label[:i]
	}
	label = strings.Trim(strings.TrimSpace(label), `"`)
	return g.node(id, label, sh), true
}

func (p *scan) balanced(closing byte) string {
	start := p.i
	for p.i < len(p.s) && p.s[p.i] != closing {
		p.i++
	}
	out := p.s[start:p.i]
	p.i = min(p.i+1, len(p.s))
	return out
}

func (p *scan) balancedPair(first, second byte) string {
	start := p.i
	for p.i+1 < len(p.s) && !(p.s[p.i] == first && p.s[p.i+1] == second) {
		p.i++
	}
	out := p.s[start:p.i]
	p.i = min(p.i+2, len(p.s))
	return out
}

type arrowSpec struct {
	label         string
	dashed, arrow bool
}

func (p *scan) arrow() (arrowSpec, bool) {
	p.skip()
	p.i = min(p.i, len(p.s))
	start := p.i
	for p.i < len(p.s) && strings.ContainsRune("-=.>", rune(p.s[p.i])) {
		p.i++
	}
	tok := p.s[start:p.i]
	if tok == "" || !strings.ContainsAny(tok, "-=") {
		p.i = start
		return arrowSpec{}, false
	}
	spec := arrowSpec{arrow: strings.Contains(tok, ">"), dashed: strings.Contains(tok, ".")}
	switch tok {
	case "--", "-.", "==": // `-- text -->` puts the label between two half arrows
		closer := map[string]string{"--": "-->", "-.": ".->", "==": "==>"}[tok]
		rest := p.s[p.i:]
		if i := strings.Index(rest, closer); i >= 0 {
			spec.label, spec.arrow = strings.TrimSpace(rest[:i]), true
			p.i += i + len(closer)
		} else if i := strings.Index(rest, tok); i >= 0 {
			spec.label = strings.TrimSpace(rest[:i])
			p.i += i + len(tok)
		} else {
			p.i = start
			return arrowSpec{}, false
		}
	}
	if p.eat('|') {
		spec.label = strings.TrimSpace(p.balanced('|'))
	}
	return spec, true
}

// --- layout ---------------------------------------------------------------

// layout ranks nodes by longest path, then reserves one free lane per edge in
// every gap that edge crosses, so no line ever runs through a box.
func (g *graph) layout() {
	for i := 0; i < len(g.nodes); i++ { // bounded: a cycle cannot rank forever
		changed := false
		for _, e := range g.edges {
			if e.to.rank < e.from.rank+1 {
				e.to.rank = e.from.rank + 1
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	for _, n := range g.nodes {
		n.w, n.h = len([]rune(n.label))+4, 3
	}
	if g.dir == "RL" || g.dir == "BT" {
		max := 0
		for _, n := range g.nodes {
			if n.rank > max {
				max = n.rank
			}
		}
		for _, n := range g.nodes {
			n.rank = max - n.rank
		}
	}
	if g.vertical() {
		g.placeVertical()
	} else {
		g.placeHorizontal()
	}
}

func (g *graph) vertical() bool { return g.dir != "LR" && g.dir != "RL" }

func (g *graph) ranks() [][]*node {
	max := 0
	for _, n := range g.nodes {
		if n.rank > max {
			max = n.rank
		}
	}
	out := make([][]*node, max+1)
	for _, n := range g.nodes {
		out[n.rank] = append(out[n.rank], n)
	}
	return out
}

// spanning counts, for each gap, how many edges cross it.
func (g *graph) spanning() []int {
	out := make([]int, len(g.ranks()))
	for _, e := range g.edges {
		for r := min(e.from.rank, e.to.rank); r < max(e.from.rank, e.to.rank); r++ {
			if r >= 0 && r < len(out) {
				out[r]++
			}
		}
	}
	return out
}

// lanes assigns every edge a distinct lane inside each gap it crosses.
func (g *graph) lanes() map[*edge]map[int]int {
	out := map[*edge]map[int]int{}
	used := make([]int, len(g.ranks()))
	for _, e := range g.edges {
		out[e] = map[int]int{}
		for r := min(e.from.rank, e.to.rank); r < max(e.from.rank, e.to.rank); r++ {
			if r < 0 || r >= len(used) {
				continue
			}
			out[e][r] = used[r]
			used[r]++
		}
	}
	return out
}

// placeVertical stacks ranks as rows and remembers the free rows of each gap.
func (g *graph) placeVertical() {
	ranks := g.ranks()
	span := g.spanning()
	widths := make([]int, len(ranks))
	widest := 0
	for i, rank := range ranks {
		for j, n := range rank {
			widths[i] += n.w
			if j < len(rank)-1 {
				widths[i] += 3
			}
		}
		widest = max(widest, widths[i])
	}
	g.gapRows = make([][]int, len(ranks))
	y := 0
	for i, rank := range ranks {
		x := (widest - widths[i]) / 2
		for _, n := range rank {
			n.x, n.y = x, y
			x += n.w + 3
		}
		y += 3
		gap := max(2, span[i]+1) // last row stays free for arrowheads
		g.gapRows[i] = make([]int, 0, gap)
		for k := 0; k < gap; k++ {
			g.gapRows[i] = append(g.gapRows[i], y+k)
		}
		y += gap
	}
	g.bypass = make(map[*edge]int)
	side := 0
	for _, e := range g.edges {
		if max(e.from.rank, e.to.rank)-min(e.from.rank, e.to.rank) > 1 {
			g.bypass[e] = side
			side++
		}
	}
	g.bypassSide = side
}

// placeHorizontal stacks ranks as columns and remembers the free columns of
// each gap; long edges travel below the diagram.
func (g *graph) placeHorizontal() {
	ranks := g.ranks()
	span := g.spanning()
	heights := make([]int, len(ranks))
	tallest := 0
	for i, rank := range ranks {
		for j, n := range rank {
			heights[i] += n.h
			if j < len(rank)-1 {
				heights[i]++
			}
		}
		tallest = max(tallest, heights[i])
	}
	g.gapCols = make([][]int, len(ranks))
	x := 0
	for i, rank := range ranks {
		y := (tallest - heights[i]) / 2
		width := 0
		for _, n := range rank {
			n.x, n.y = x, y
			y += n.h + 1
			width = max(width, n.w)
		}
		x += width
		gap := max(3, span[i]+2) // last column stays free for arrowheads
		g.gapCols[i] = make([]int, 0, gap)
		for k := 0; k < gap; k++ {
			g.gapCols[i] = append(g.gapCols[i], x+k)
		}
		x += gap
	}
	g.bypass = make(map[*edge]int)
	side := 0
	for _, e := range g.edges {
		if max(e.from.rank, e.to.rank)-min(e.from.rank, e.to.rank) > 1 {
			g.bypass[e] = side
			side++
		}
	}
	g.bypassSide = side
}

// --- drawing --------------------------------------------------------------

func (g *graph) draw() *Art {
	w, h := 1, 1
	maxRight, maxBottom := 0, 0
	for _, n := range g.nodes {
		w = max(w, n.x+n.w+2)
		h = max(h, n.y+n.h+2)
		maxRight = max(maxRight, n.x+n.w)
		maxBottom = max(maxBottom, n.y+n.h)
	}
	if g.bypassSide > 0 {
		if g.vertical() {
			w += 1 + g.bypassSide
		} else {
			h += 1 + g.bypassSide
		}
	}
	c := newCanvas(w, h)
	c.g = g
	c.bypassX = map[*edge]int{}
	c.bypassY = map[*edge]int{}
	for e, i := range g.bypass {
		if g.vertical() {
			c.bypassX[e] = maxRight + 1 + i
		} else {
			c.bypassY[e] = maxBottom + 1 + i
		}
	}
	for _, n := range g.nodes {
		c.box(n)
	}
	lanes := g.lanes()
	for _, e := range g.edges {
		if g.vertical() {
			c.edgeVertical(e, lanes[e])
		} else {
			c.edgeHorizontal(e, lanes[e])
		}
	}
	for _, e := range g.edges {
		c.arrow(e, g.vertical())
	}
	for _, e := range g.edges {
		c.edgeLabel(e, lanes[e], g.vertical())
	}
	return c.art()
}

func (c *canvas) box(n *node) {
	topL, topR, botL, botR := '┌', '┐', '└', '┘'
	switch n.shape {
	case roundShape:
		topL, topR, botL, botR = '╭', '╮', '╰', '╯'
	case decisionShape:
		topL, topR, botL, botR = '╱', '╲', '╲', '╱'
	}
	c.put(n.x, n.y, topL, Border)
	c.put(n.x+n.w-1, n.y, topR, Border)
	c.put(n.x, n.y+2, botL, Border)
	c.put(n.x+n.w-1, n.y+2, botR, Border)
	for x := n.x + 1; x < n.x+n.w-1; x++ {
		c.put(x, n.y, '─', Border)
		c.put(x, n.y+2, '─', Border)
	}
	c.put(n.x, n.y+1, '│', Border)
	c.put(n.x+n.w-1, n.y+1, '│', Border)
	c.text(n.x+2, n.y+1, n.label, Text)
}

// edgeVertical routes one edge between two ranks. Adjacent ranks take the
// direct staircase; a longer edge swings out through a bypass column so it
// never cuts through the boxes it skips over.
func (c *canvas) edgeVertical(e *edge, lanes map[int]int) {
	h, v := c.hline, c.vline
	if e.dashed {
		h, v = c.hlineDashed, c.vlineDashed
	}
	sx, sy := e.from.x+e.from.w/2, e.from.y+3
	tx, ty := e.to.x+e.to.w/2, e.to.y-1
	up := max(e.from.rank, e.to.rank) == e.from.rank
	lo, hi := min(e.from.rank, e.to.rank), max(e.from.rank, e.to.rank)
	if lo == hi {
		return
	}
	rows := func(r int) []int { return c.rowsAt(r) }
	first := rows(lo)[lanes[lo]]
	last := rows(hi - 1)[lanes[hi-1]]
	if up {
		first, last = last, first
	}
	if hi-lo == 1 {
		if abs(sx-tx) <= 1 { // centres off by one: draw straight, avoid a jog
			v(sy, ty, tx) // at the target's centre, where the arrow lands
			return
		}
		v(sy, first, sx)
		h(sx, tx, first)
		v(first, ty, tx)
		return
	}
	mx := c.bypassColumn(e)
	v(sy, first, sx)
	h(sx, mx, first)
	v(first, last, mx)
	h(tx, mx, last)
	v(last, ty, tx)
}

func (c *canvas) edgeHorizontal(e *edge, lanes map[int]int) {
	h, v := c.hline, c.vline
	if e.dashed {
		h, v = c.hlineDashed, c.vlineDashed
	}
	sx, sy := e.from.x+e.from.w, e.from.y+1
	tx, ty := e.to.x-1, e.to.y+1
	lo, hi := min(e.from.rank, e.to.rank), max(e.from.rank, e.to.rank)
	if lo == hi {
		return
	}
	cols := func(r int) []int { return c.colsAt(r) }
	first := cols(lo)[lanes[lo]]
	last := cols(hi - 1)[lanes[hi-1]]
	if hi-lo == 1 {
		if abs(sy-ty) <= 1 {
			h(sx, tx, ty)
			return
		}
		h(sx, first, sy)
		v(sy, ty, first)
		h(first, tx, ty)
		return
	}
	my := c.bypassRow(e)
	h(sx, first, sy)
	v(sy, my, first)
	h(first, last, my)
	v(my, ty, last)
	h(last, tx, ty)
}

func (c *canvas) arrow(e *edge, vertical bool) {
	if !e.arrow {
		return
	}
	if vertical {
		ch := '▼'
		if e.to.rank < e.from.rank {
			ch = '▲'
		}
		if e.to.rank == e.from.rank {
			return
		}
		c.put(e.to.x+e.to.w/2, e.to.y-1, ch, Edge)
		return
	}
	ch := '▶'
	if e.to.rank < e.from.rank {
		ch = '◀'
	}
	c.put(e.to.x-1, e.to.y+1, ch, Edge)
}

// edgeLabel writes the label into the first free spot next to its arrow, so a
// label never cuts a box or a line.
func (c *canvas) edgeLabel(e *edge, lanes map[int]int, vertical bool) {
	if e.label == "" {
		return
	}
	n := len([]rune(e.label))
	if vertical {
		tx, ty := e.to.x+e.to.w/2, e.to.y-1
		lo := min(e.from.rank, e.to.rank)
		row := c.rowsAt(lo)[lanes[lo]]
		for _, p := range [][2]int{
			{tx + 2, ty}, {tx + 2, ty - 1}, {tx - n - 1, ty}, {tx + 2, row},
			{tx + 2, ty - 2}, {tx + 2, row - 1}, {tx + 2, row + 1},
		} {
			if c.blank(p[0], p[1], n) {
				c.write(p[0], p[1], e.label)
				return
			}
		}
		return
	}
	_, sy := e.from.x, e.from.y+1
	tx, ty := e.to.x-1, e.to.y+1
	lo := min(e.from.rank, e.to.rank)
	col := c.colsAt(lo)[lanes[lo]]
	for _, p := range [][2]int{
		{col + 1, ty - 1}, {tx - n, ty - 1}, {col + 1, sy + 1}, {tx - n, ty + 1},
		{col + 1, ty}, {tx - n, ty}, {col + 1, ty - 2},
	} {
		if c.blank(p[0], p[1], n) {
			c.write(p[0], p[1], e.label)
			return
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
