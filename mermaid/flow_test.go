package mermaid

import (
	"strings"
	"testing"
)

func render(t *testing.T, src string) *Art {
	t.Helper()
	art, err := Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return art
}

func plain(art *Art) string {
	var b strings.Builder
	for _, row := range art.Cells {
		for _, c := range row {
			b.WriteRune(c.Ch)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestParseShapesEdgesAndLabels(t *testing.T) {
	g := parse(`graph LR
  A[read the file] --> B{is it mermaid?}
  B -->|yes| C[draw it]
  B -.->|no| D(show source)
  C & D --> E[done]
`)
	if g.dir != "LR" {
		t.Errorf("dir = %q", g.dir)
	}
	if len(g.nodes) != 5 {
		t.Fatalf("%d nodes, want 5", len(g.nodes))
	}
	if got := g.byID["B"].shape; got != decisionShape {
		t.Errorf("B shape = %v, want decision", got)
	}
	if got := g.byID["D"].shape; got != roundShape {
		t.Errorf("D shape = %v, want rounded", got)
	}
	if len(g.edges) != 5 { // A->B, B->C, B->D, C->E, D->E (fan-out)
		t.Fatalf("%d edges, want 5", len(g.edges))
	}
	var dashed, labelled int
	for _, e := range g.edges {
		if e.dashed {
			dashed++
		}
		if e.label != "" {
			labelled++
		}
	}
	if dashed != 1 || labelled != 2 {
		t.Errorf("dashed=%d labelled=%d, want 1 and 2", dashed, labelled)
	}
}

func TestDashLabelForm(t *testing.T) {
	g := parse("graph TD\n  A -- retry -- B\n  B == hard ==> C\n")
	if len(g.edges) != 2 {
		t.Fatalf("%d edges", len(g.edges))
	}
	if g.edges[0].label != "retry" {
		t.Errorf("edge label = %q", g.edges[0].label)
	}
	if g.edges[1].label != "hard" || !g.edges[1].arrow {
		t.Errorf("thick edge = %+v", g.edges[1])
	}
}

// The invariant that matters visually: a routed line must never cut through a
// node's interior.
func TestNoLineRunsThroughABox(t *testing.T) {
	for _, src := range []string{
		"graph TD\n A[one] --> B{two}\n B -->|yes| C[three]\n B -->|no| D[four]\n C --> E[end]\n D --> E\n",
		"graph LR\n A[one] --> B{two}\n B -->|yes| C[three]\n B -->|no| D[four]\n C --> E[end]\n D --> E\n",
		"graph TD\n A[a] --> C[c]\n B[b] --> C\n A --> D[d]\n D --> C\n",
	} {
		g := parse(src)
		g.layout()
		art := g.draw()
		for _, n := range g.nodes {
			for y := n.y + 1; y < n.y+n.h-1; y++ {
				for x := n.x + 1; x < n.x+n.w-1; x++ {
					if y >= len(art.Cells) || x >= len(art.Cells[y]) {
						continue
					}
					if ch := art.Cells[y][x].Ch; isLine(ch) {
						t.Fatalf("%q: line %q inside box %q at %d,%d\n%s", src, ch, n.label, x, y, plain(art))
					}
				}
			}
		}
	}
}

func TestArtIsTrimmedAndBounded(t *testing.T) {
	art := render(t, "graph LR\n  A[short] --> B[a much longer label here]\n")
	if art.Width < 20 {
		t.Errorf("width %d looks too small", art.Width)
	}
	for i, row := range art.Cells {
		if len(row) > art.Width {
			t.Errorf("row %d is %d cells, wider than Art.Width %d", i, len(row), art.Width)
		}
		if len(row) > 0 && row[len(row)-1].Ch == ' ' {
			t.Errorf("row %d keeps trailing space", i)
		}
	}
}

func TestUnsupportedSourceIsAnError(t *testing.T) {
	for _, src := range []string{"timeline\n  title x\n", "quadrantChart\n  title x\n", "hello world\n"} {
		if _, err := Render(src); err == nil {
			t.Errorf("%q should report an error so the caller shows the source", src)
		}
	}
}

// A label wider than the space beside its arrow used to index the canvas with
// a negative column.
func TestLongEdgeLabelDoesNotPanic(t *testing.T) {
	art := render(t, "graph TD\n  A[a] -->|an absurdly long edge label for the room available| B[b]\n")
	if art.Width == 0 {
		t.Error("empty art")
	}
}

func TestSubgraphsNestAndClaimMembers(t *testing.T) {
	g := parse(`graph LR
  subgraph outer[Pipeline]
    subgraph one[Build]
      A[checkout] --> B[compile]
    end
    B --> C[unit]
  end
  C --> D[deploy]
`)
	if len(g.subgraphs) != 2 {
		t.Fatalf("%d subgraphs, want 2", len(g.subgraphs))
	}
	outer, one := g.subgraphs[0], g.subgraphs[1]
	if outer.title != "Pipeline" || one.title != "Build" {
		t.Errorf("titles = %q, %q", outer.title, one.title)
	}
	if one.parent != outer {
		t.Error("Build should be nested in Pipeline")
	}
	if len(outer.members) != 1 || outer.members[0].id != "C" {
		t.Errorf("outer members = %v", outer.members)
	}
	if len(one.members) != 2 {
		t.Errorf("Build members = %d, want 2", len(one.members))
	}
	// B is mentioned inside both; the first frame that claimed it keeps it, and
	// it still counts as inside the enclosing frame.
	if !g.inside(g.byID["B"], one) {
		t.Error("B belongs to Build")
	}
	if !g.inside(g.byID["B"], outer) {
		t.Error("Build is nested in Pipeline, so B is inside Pipeline too")
	}
	if g.inside(g.byID["D"], outer) {
		t.Error("D is outside every frame")
	}
}

func TestSubgraphFrameIsDrawnWithTitle(t *testing.T) {
	out := plain(render(t, "graph LR\n  subgraph ci[CI]\n    A[a] --> B[b]\n  end\n  B --> C[c]\n"))
	if !strings.Contains(out, "╭─ CI ─") {
		t.Errorf("no titled frame:\n%s", out)
	}
	if !strings.Contains(out, "╰") || !strings.Contains(out, "╯") {
		t.Errorf("frame is not closed:\n%s", out)
	}
}

// A node outside the frame must never end up enclosed by it.
func TestOutsiderIsPushedOutOfTheFrame(t *testing.T) {
	g := parse(`graph TD
  subgraph box[Box]
    A[a] --> B[b]
  end
  A --> C[outsider]
`)
	g.layout()
	for _, sg := range g.subgraphs {
		for _, n := range g.nodes {
			if g.inside(n, sg) {
				continue
			}
			inX := n.x+n.w-1 >= sg.x0 && n.x <= sg.x1
			inY := n.y+n.h-1 >= sg.y0 && n.y <= sg.y1
			if inX && inY {
				t.Errorf("node %q sits inside frame %q", n.id, sg.title)
			}
		}
	}
}

func TestEmptySubgraphDrawsNothing(t *testing.T) {
	out := plain(render(t, "graph LR\n  subgraph lonely\n  end\n  A[a] --> B[b]\n"))
	if strings.Contains(out, "╭") {
		t.Errorf("empty subgraph drew a frame:\n%s", out)
	}
}
