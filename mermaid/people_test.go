package mermaid

import (
	"strings"
	"testing"
)

func TestJourneyRendersScoresAndActors(t *testing.T) {
	art := render(t, `journey
  title Opening a README
  section Find the file
    Clone the repo: 3: Reader
    cd into it: 5: Reader
  section Read it
    Run mdv: 1: Reader, Cat
`)
	out := plain(art)
	for _, want := range []string{"Opening a README", "Find the file", "Clone the repo", "Reader, Cat"} {
		if !strings.Contains(out, want) {
			t.Errorf("journey output missing %q:\n%s", want, out)
		}
	}
	// A score of 3 is three filled cells and two empty ones; 5 is all filled.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Clone the repo") {
			if n := strings.Count(line, "█"); n != 3 {
				t.Errorf("score 3 bar has %d filled cells: %q", n, line)
			}
			if n := strings.Count(line, "░"); n != 2 {
				t.Errorf("score 3 bar has %d empty cells: %q", n, line)
			}
		}
		if strings.Contains(line, "cd into it") && strings.Count(line, "█") != 5 {
			t.Errorf("score 5 bar is not full: %q", line)
		}
	}
}

func TestJourneyClampsScoresAndNeedsSteps(t *testing.T) {
	out := plain(render(t, "journey\n  section S\n    low: 0: Me\n    high: 99: Me\n"))
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "low"):
			if strings.Count(line, "█") != 1 {
				t.Errorf("score 0 should clamp to 1: %q", line)
			}
		case strings.Contains(line, "high"):
			if strings.Count(line, "█") != 5 {
				t.Errorf("score 99 should clamp to 5: %q", line)
			}
		}
	}
	if _, err := Render("journey\n  title empty\n"); err == nil {
		t.Error("a journey without steps should error")
	}
}

func TestMindmapTree(t *testing.T) {
	out := plain(render(t, `mindmap
  root((mdv))
    Renderer
      Markdown
        Tables
      Diagrams
    Viewer
      Search
`))
	for _, want := range []string{"mdv", "├─ Renderer", "└─ Viewer", "│  ├─ Markdown", "│  │  └─ Tables", "│  └─ Diagrams"} {
		if !strings.Contains(out, want) {
			t.Errorf("mindmap output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "root") || strings.Contains(out, "((") {
		t.Errorf("node shape wrappers leaked into the output:\n%s", out)
	}
}

func TestCleanMindText(t *testing.T) {
	cases := map[string]string{
		"root((mdv))":                 "mdv",
		"id[Square node]":             "Square node",
		"id(Rounded node)":            "Rounded node",
		"id{{Hexagon}}":               "Hexagon",
		"plain words":                 "plain words",
		"node: ::icon(fa fa-book)":    "node:",
		"needs a<br/>line break":      "needs a line break",
		"a sentence (with a bracket)": "a sentence (with a bracket)",
	}
	for in, want := range cases {
		if got := cleanMindText(in); got != want {
			t.Errorf("cleanMindText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIndentOf(t *testing.T) {
	if got := indentOf("\t  x"); got != 6 {
		t.Errorf("tab+2 spaces = %d, want 6", got)
	}
	if got := indentOf("x"); got != 0 {
		t.Errorf("no indent = %d", got)
	}
}

func TestMindmapNeedsNodes(t *testing.T) {
	if _, err := Render("mindmap\n"); err == nil {
		t.Error("an empty mindmap should error")
	}
}
