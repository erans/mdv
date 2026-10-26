package mermaid

// canvas is a grid of cells with the junction merging a diagram needs: a
// horizontal line crossing a vertical one becomes a cross instead of one of
// them winning.
type canvas struct {
	cells [][]Cell
	w, h  int
	g     *graph

	bypassX map[*edge]int
	bypassY map[*edge]int
}

// rowsAt and colsAt are the free lanes of one rank gap.
func (c *canvas) rowsAt(rank int) []int {
	if rank < 0 || rank >= len(c.g.gapRows) {
		return []int{0}
	}
	return c.g.gapRows[rank]
}

func (c *canvas) colsAt(rank int) []int {
	if rank < 0 || rank >= len(c.g.gapCols) {
		return []int{0}
	}
	return c.g.gapCols[rank]
}

// bypassColumn and bypassRow are the outside tracks long edges travel on, just
// past the widest and lowest node.
func (c *canvas) bypassColumn(e *edge) int { return c.bypassX[e] }
func (c *canvas) bypassRow(e *edge) int    { return c.bypassY[e] }

func newCanvas(w, h int) *canvas {
	c := &canvas{w: w, h: h, cells: make([][]Cell, h)}
	for y := range c.cells {
		c.cells[y] = make([]Cell, w)
		for x := range c.cells[y] {
			c.cells[y][x] = Cell{Ch: ' ', Cl: Plain}
		}
	}
	return c
}

func (c *canvas) put(x, y int, ch rune, cl Class) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	c.cells[y][x] = Cell{Ch: ch, Cl: cl}
}

func (c *canvas) text(x, y int, s string, cl Class) {
	for _, r := range s {
		c.put(x, y, r, cl)
		x++
	}
}

// putBlank writes text into the first run of empty cells at or right of x, so
// a label never cuts through a line or a box.
func (c *canvas) putBlank(x, y int, s string) {
	for shift := 0; shift < 14; shift++ {
		start := x + shift
		if c.blank(start, y, len([]rune(s))) {
			for i, r := range []rune(s) {
				c.cells[y][start+i] = Cell{Ch: r, Cl: EdgeLabel}
			}
			return
		}
	}
}

func (c *canvas) write(x, y int, s string) {
	for i, r := range []rune(s) {
		c.cells[y][x+i] = Cell{Ch: r, Cl: EdgeLabel}
	}
}

func (c *canvas) blank(x, y, n int) bool {
	for i := 0; i < n; i++ {
		if x+i < 0 || x+i >= c.w || y < 0 || y >= c.h {
			return false
		}
		if ch := c.cells[y][x+i]; ch.Ch != ' ' && ch.Cl != EdgeLabel {
			return false
		}
	}
	return true
}

func (c *canvas) hline(x0, x1, y int) {
	for x := min(x0, x1); x <= max(x0, x1); x++ {
		c.line(x, y, '─', Edge)
	}
}

func (c *canvas) hlineDashed(x0, x1, y int) {
	for x := min(x0, x1); x <= max(x0, x1); x++ {
		if (x-min(x0, x1))%2 == 0 {
			c.line(x, y, '╌', Edge)
		}
	}
}

func (c *canvas) vlineDashed(y0, y1, x int) {
	for y := min(y0, y1); y <= max(y0, y1); y++ {
		if (y-min(y0, y1))%2 == 0 {
			c.line(x, y, '╎', Edge)
		}
	}
}

func (c *canvas) vline(y0, y1, x int) {
	for y := min(y0, y1); y <= max(y0, y1); y++ {
		c.line(x, y, '│', Edge)
	}
}

// line draws a line character, merging with whatever line was already there.
func (c *canvas) line(x, y int, ch rune, cl Class) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	old := c.cells[y][x]
	if old.Ch == ' ' || old.Ch == ch {
		c.cells[y][x] = Cell{Ch: ch, Cl: cl}
		return
	}
	switch merged := junction(old.Ch, ch); merged {
	case 0:
		// Existing content wins over a line that would erase it.
		if !isLine(old.Ch) {
			return
		}
		c.cells[y][x] = Cell{Ch: ch, Cl: cl}
	default:
		c.cells[y][x] = Cell{Ch: merged, Cl: cl}
	}
}

func isLine(r rune) bool {
	switch r {
	case '─', '│', '┼', '┬', '┴', '├', '┤', '╌', '╎', '▶', '◀', '▼', '▲':
		return true
	}
	return false
}

// junction returns the glyph for two crossing line characters, or 0 when the
// pair has no combined form.
func junction(a, b rune) rune {
	isH := func(r rune) bool { return r == '─' || r == '╌' || r == '▶' || r == '◀' }
	isV := func(r rune) bool { return r == '│' || r == '╎' || r == '▼' || r == '▲' }
	switch {
	case isH(a) && isH(b):
		return pick(a, b, '─', '╌')
	case isV(a) && isV(b):
		return pick(a, b, '│', '╎')
	case (isH(a) && isV(b)) || (isV(a) && isH(b)):
		return '┼'
	}
	return 0
}

// pick keeps the heavier of two line weights.
func pick(a, b, solid, dashed rune) rune {
	if a == solid || b == solid {
		return solid
	}
	return dashed
}

func (c *canvas) art() *Art {
	rows := make([][]Cell, 0, c.h)
	width := 0
	for y := 0; y < c.h; y++ {
		end := c.w
		for end > 0 && c.cells[y][end-1].Ch == ' ' {
			end--
		}
		if end == 0 {
			continue
		}
		rows = append(rows, c.cells[y][:end])
		width = max(width, end)
	}
	return &Art{Cells: rows, Width: width}
}
