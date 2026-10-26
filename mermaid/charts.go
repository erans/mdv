package mermaid

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// chartWidth is how wide a chart draws itself; the caller falls back to the
// fenced source when the pane is narrower.
const chartWidth = 72

// RenderPie draws a pie chart as a horizontal bar chart: there is no honest way
// to draw a real pie with box-drawing characters, and the shares read better as
// bars anyway.
func renderPie(src string) (*Art, error) {
	title := ""
	var labels []string
	var values []float64
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		low := strings.ToLower(line)
		switch {
		case line == "" || low == "pie":
			continue
		case strings.HasPrefix(low, "pie "):
			if rest, ok := strings.CutPrefix(line, "pie "); ok {
				if t, ok := strings.CutPrefix(strings.TrimSpace(rest), "title "); ok {
					title = strings.TrimSpace(t)
				}
			}
			continue
		case strings.HasPrefix(low, "title "):
			title = strings.TrimSpace(line[len("title "):])
			continue
		}
		label, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || v <= 0 {
			continue
		}
		labels = append(labels, strings.TrimSpace(strings.Trim(strings.TrimSpace(label), `"`)))
		values = append(values, v)
	}
	if len(values) == 0 {
		return nil, errors.New("pie needs at least one value")
	}

	total := 0.0
	widest := 0
	for i, v := range values {
		total += v
		widest = max(widest, len([]rune(labels[i])))
	}
	barW := max(10, chartWidth-widest-16)

	c := newCanvas(chartWidth, len(values)+3)
	y := 0
	if title != "" {
		c.text(0, y, title, Title)
		y += 2
	}
	for i, v := range values {
		share := v / total
		c.text(0, y, labels[i], Text)
		bar := max(1, int(share*float64(barW)+0.5))
		c.bar(widest+2, y, bar, Series1+Class(i%6))
		c.text(widest+2+bar, y, " ", Plain)
		pct := fmt.Sprintf("%5.1f%%", share*100)
		c.text(widest+4+barW, y, pct, EdgeLabel)
		c.text(widest+6+barW+len(pct), y, fmt.Sprintf("%g", v), EdgeLabel)
		y++
	}
	return c.art(), nil
}

// bar paints a filled run in the accent colour; the pie renderer keeps the hue
// for the label so a theme with one accent still reads.
func (c *canvas) bar(x, y, width int, cl Class) {
	for i := 0; i < width; i++ {
		c.put(x+i, y, '█', cl)
	}
}

// --- gantt ----------------------------------------------------------------

// renderGantt draws a gantt chart as a bar per task over a day axis. Dates are
// day numbers, so the arithmetic stays integers.
//
// shortcut: `dateFormat`/`axisFormat` are ignored and dates must be YYYY-MM-DD;
// `h` durations count as a day. Upgrade when a real chart needs an hour axis.
func renderGantt(src string) (*Art, error) {
	g, err := parseGantt(src)
	if err != nil {
		return nil, err
	}
	return g.draw(), nil
}

// ganttChart is a parsed gantt diagram.
type ganttChart struct {
	title    string
	sections []string
	tasks    []*ganttTask
	byID     map[string]*ganttTask
	lo, hi   int
}

// ganttTask is one row of the chart.
type ganttTask struct {
	name      string
	id        string
	start     int // day number
	end       int
	after     string
	days      int
	hasStart  bool
	hasEnd    bool
	done      bool
	crit      bool
	milestone bool
	section   int
}

func parseGantt(src string) (*ganttChart, error) {
	g := &ganttChart{}
	title := ""
	var tasks []*ganttTask
	var sections []string
	section := -1
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		low := strings.ToLower(line)
		switch {
		case line == "" || low == "gantt":
			continue
		case strings.HasPrefix(low, "title "):
			title = strings.TrimSpace(line[len("title "):])
			continue
		case strings.HasPrefix(low, "dateformat"), strings.HasPrefix(low, "axisformat"),
			strings.HasPrefix(low, "excludes"), strings.HasPrefix(low, "includes"),
			strings.HasPrefix(low, "todaymarker"), strings.HasPrefix(low, "weekday"):
			continue
		case strings.HasPrefix(low, "section "):
			sections = append(sections, strings.TrimSpace(line[len("section "):]))
			section++
			continue
		}
		name, tags, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		t := &ganttTask{name: strings.TrimSpace(name), section: section}
		for _, tag := range strings.Split(tags, ",") {
			tag = strings.TrimSpace(tag)
			lowTag := strings.ToLower(tag)
			switch {
			case tag == "":
			case lowTag == "done", lowTag == "active":
				t.done = lowTag == "done"
			case lowTag == "crit":
				t.crit = true
			case lowTag == "milestone":
				t.milestone = true
			case strings.HasPrefix(lowTag, "after "):
				t.after = strings.TrimSpace(tag[len("after "):])
			case isDate(tag):
				if !t.hasStart {
					t.start, t.hasStart = dayNumber(tag), true
				} else {
					t.end, t.hasEnd = dayNumber(tag), true
				}
			case isDuration(tag):
				t.days = durationDays(tag)
			default:
				t.id = tag
			}
		}
		tasks = append(tasks, t)
	}
	if len(tasks) == 0 {
		return nil, errors.New("gantt needs at least one task")
	}
	g.title, g.sections, g.tasks = title, sections, tasks
	byID := map[string]*ganttTask{}
	for _, t := range tasks {
		if t.id != "" {
			byID[t.id] = t
		}
	}
	// A task with a date and a duration knows both ends already.
	for _, t := range tasks {
		if t.hasStart && !t.hasEnd && t.days > 0 {
			t.end, t.hasEnd = t.start+t.days, true
		}
	}
	// `after id` starts where that task ended; chains may need several passes.
	for pass := 0; pass < len(tasks); pass++ {
		changed := false
		for _, t := range tasks {
			if t.hasStart || t.after == "" {
				continue
			}
			dep, ok := byID[t.after]
			if !ok || !dep.hasStart || !dep.hasEnd {
				continue
			}
			t.start, t.hasStart = dep.end, true
			if t.days > 0 {
				t.end, t.hasEnd = t.start+t.days, true
			}
			changed = true
		}
		if !changed {
			break
		}
	}
	for _, t := range tasks {
		if !t.hasStart {
			t.start, t.hasStart = 0, true
		}
		if !t.hasEnd {
			t.end = t.start + 1
		}
		if t.end <= t.start {
			t.end = t.start + 1
		}
	}

	g.byID = byID
	lo, hi := tasks[0].start, tasks[0].end
	for _, t := range tasks {
		lo, hi = min(lo, t.start), max(hi, t.end)
	}
	g.byID = byID
	g.lo, g.hi = lo, hi
	return g, nil
}

// draw paints the chart: an axis row, a header per section and one bar per task.
func (g *ganttChart) draw() *Art {
	tasks, sections, title := g.tasks, g.sections, g.title
	lo, hi := g.lo, g.hi
	span := max(1, hi-lo)
	indent := 2
	labelW := 0
	for _, t := range tasks {
		labelW = max(labelW, len([]rune(t.name))+indent)
	}
	for _, s := range sections {
		labelW = max(labelW, len([]rune(s))+indent)
	}
	labelW = min(labelW, 24)
	barW := max(12, chartWidth-labelW-2)

	y := 0
	if title != "" {
		y = 2
	}
	c := newCanvas(chartWidth, y+len(tasks)+len(sections)+4)
	if title != "" {
		c.text(0, 0, title, Title)
	}
	// Axis: first and last day, plus the middle one when there is room.
	c.text(labelW, y, axisLabel(lo), EdgeLabel)
	end := axisLabel(hi)
	c.text(labelW+barW-len(end), y, end, EdgeLabel)
	if barW > 30 {
		mid := axisLabel(lo + span/2)
		c.text(labelW+barW/2-len(mid)/2, y, mid, EdgeLabel)
	}
	y++

	pos := func(day int) int {
		return labelW + int(float64(day-lo)/float64(span)*float64(barW-1))
	}
	lastSection := -2
	for _, t := range tasks {
		if t.section != lastSection && t.section >= 0 && t.section < len(sections) {
			c.text(0, y, sections[t.section], Title)
			lastSection = t.section
			y++
		}
		c.text(0, y, strings.Repeat(" ", indent)+truncateRunes(t.name, labelW-indent), Text)
		start, stop := pos(t.start), pos(t.end)
		c.taskbar(start, stop, y, t)
		y++
	}
	return c.art()
}

// taskbar draws one task: a milestone is a diamond, everything else a bar.
func (c *canvas) taskbar(start, stop, y int, t *ganttTask) {
	if t.milestone {
		c.put(min(start, c.w-1), y, '◆', Edge)
		return
	}
	ch := '█'
	cl := Edge
	switch {
	case t.crit:
		ch, cl = '▓', Alert
	case t.done:
		ch, cl = '▒', Border
	}
	for x := max(0, start); x <= min(stop, c.w-1); x++ {
		c.put(x, y, ch, cl)
	}
}

func axisLabel(day int) string {
	// Day numbers are days since 1970-01-01, so this is a plain civil date.
	y, m, d := civilFromDays(day)
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

func isDate(s string) bool {
	if len(s) < 8 {
		return false
	}
	_, _, _, err := parseDate(s)
	return err == nil
}

func isDuration(s string) bool {
	if len(s) < 2 {
		return false
	}
	unit := s[len(s)-1]
	if !strings.ContainsRune("dhw", rune(unit)) {
		return false
	}
	_, err := strconv.Atoi(s[:len(s)-1])
	return err == nil
}

func durationDays(s string) int {
	n, _ := strconv.Atoi(s[:len(s)-1])
	switch s[len(s)-1] {
	case 'w':
		return n * 7
	case 'h':
		return max(1, n/24)
	}
	return n
}

// dayNumber turns YYYY-MM-DD into days since the epoch.
func dayNumber(s string) int {
	y, m, d, err := parseDate(s)
	if err != nil {
		return 0
	}
	return daysFromCivil(y, m, d)
}

func parseDate(s string) (y, m, d int, err error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 3 {
		return 0, 0, 0, errors.New("not a date")
	}
	if y, err = strconv.Atoi(parts[0]); err != nil {
		return
	}
	if m, err = strconv.Atoi(parts[1]); err != nil {
		return
	}
	if d, err = strconv.Atoi(parts[2]); err != nil {
		return
	}
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return 0, 0, 0, errors.New("not a date")
	}
	return y, m, d, nil
}

// daysFromCivil and civilFromDays are Howard Hinnant's algorithms: calendar
// arithmetic without pulling in time zone machinery.
func daysFromCivil(y, m, d int) int {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func civilFromDays(days int) (y, m, d int) {
	z := days + 719468
	era := z / 146097
	if z < 0 {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	yy := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d = doy - (153*mp+2)/5 + 1
	m = mp + 3
	if mp >= 10 {
		m = mp - 9
	}
	if m <= 2 {
		yy++
	}
	return yy, m, d
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return string(r[:n-1]) + "…"
}
