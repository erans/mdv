package mermaid

import (
	"strings"
	"testing"
)

func TestPieParsesTitleAndShares(t *testing.T) {
	art := render(t, `pie title Where the code goes
  "Layout" : 42
  "Parsing" : 28
  "Themes" : 18
  "Viewer" : 12
`)
	out := plain(art)
	for _, want := range []string{"Where the code goes", "Layout", "Parsing", "42.0%", "12.0%"} {
		if !strings.Contains(out, want) {
			t.Errorf("pie output missing %q:\n%s", want, out)
		}
	}
	// The biggest slice gets the longest bar.
	bars := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		for _, label := range []string{"Layout", "Parsing", "Themes", "Viewer"} {
			if strings.HasPrefix(line, label) {
				bars[label] = strings.Count(line, "█")
			}
		}
	}
	if bars["Layout"] <= bars["Parsing"] || bars["Parsing"] <= bars["Viewer"] {
		t.Errorf("bar lengths do not follow the values: %v", bars)
	}
}

func TestPieRejectsGarbage(t *testing.T) {
	if _, err := Render("pie\n  nothing here\n"); err == nil {
		t.Error("a pie without values should error so the source is shown")
	}
}

func TestGanttResolvesDependencies(t *testing.T) {
	m, err := parseGantt(`gantt
  dateFormat YYYY-MM-DD
  section Build
    Design  :d1, 2024-01-01, 10d
    Code    :c1, after d1, 5d
    Ship    :milestone, m1, after c1, 0d
`)
	if err != nil {
		t.Fatal(err)
	}
	design, code, ship := m.byID["d1"], m.byID["c1"], m.byID["m1"]
	if design == nil || code == nil || ship == nil {
		t.Fatalf("tasks not indexed: %+v", m.byID)
	}
	if !code.hasStart || code.start != design.end {
		t.Errorf("Code starts at %d, want Design's end %d (hasStart=%v)", code.start, design.end, code.hasStart)
	}
	if design.end-design.start != 10 {
		t.Errorf("Design spans %d days, want 10", design.end-design.start)
	}
	if !ship.hasStart || ship.start != code.end {
		t.Errorf("milestone starts at %d, want %d", ship.start, code.end)
	}
}

func TestGanttRendersSectionsAndBars(t *testing.T) {
	out := plain(render(t, `gantt
  title Roadmap
  section Foundations
    Renderer :done, r1, 2024-01-01, 30d
    Tables   :crit, t1, after r1, 21d
  section Ship
    v1.0     :milestone, m1, after t1, 0d
`))
	for _, want := range []string{"Roadmap", "Foundations", "Ship", "Renderer", "Tables", "v1.0", "◆", "2024-01-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("gantt output missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "▒") {
		t.Error("a done task should draw with its own fill")
	}
}

func TestGanttWithoutTasksErrors(t *testing.T) {
	if _, err := Render("gantt\n  title nothing\n"); err == nil {
		t.Error("a gantt without tasks should error")
	}
}

func TestDateRoundTrip(t *testing.T) {
	for _, date := range []string{"2024-01-01", "1999-12-31", "2024-02-29"} {
		y, m, d, err := parseDate(date)
		if err != nil {
			t.Fatalf("%s: %v", date, err)
		}
		got := axisLabel(daysFromCivil(y, m, d))
		if got != date {
			t.Errorf("%s round-tripped to %s", date, got)
		}
	}
}
