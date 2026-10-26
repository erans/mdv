package mermaid

import (
	"strings"
	"testing"
)

func TestKindDispatch(t *testing.T) {
	cases := map[string]string{
		"graph LR\n A-->B":                                "flowchart",
		"flowchart TD\n A-->B":                            "flowchart",
		"sequenceDiagram\n A->>B: hi":                     "sequence",
		"stateDiagram-v2\n [*] --> A":                     "state",
		"classDiagram\n class A":                          "class",
		"erDiagram\n A ||--o{ B : x":                      "er",
		"---\ntitle: front matter\n---\ngraph TD\n A-->B": "flowchart",
		"pie\n \"a\": 3":                                  "pie",
		"gantt\n  title x":                                "gantt",
		"journey\n  title x":                              "journey",
		"mindmap\n  root":                                 "mindmap",
		"timeline\n  title x":                             "",
	}
	for src, want := range cases {
		if got := kind(src); got != want {
			t.Errorf("kind(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestSequenceDiagram(t *testing.T) {
	out := plain(render(t, `sequenceDiagram
  participant A as Browser
  participant B as Server
  A->>B: GET /readme
  B-->>A: 200 markdown
  B->>B: render
`))
	for _, want := range []string{"Browser", "Server", "GET /readme", "▶", "◀", "╌"} {
		if !strings.Contains(out, want) {
			t.Errorf("sequence output missing %q:\n%s", want, out)
		}
	}
	// The solid request and the dashed response must differ.
	if !strings.Contains(out, "───▶") && !strings.Contains(out, "──▶") {
		t.Errorf("no solid arrow:\n%s", out)
	}
}

func TestSequenceParticipantsFollowFirstUse(t *testing.T) {
	g := participantsOf("sequenceDiagram\n  A->>B: hi\n  C->>A: back\n")
	if len(g) != 3 || g[0] != "A" || g[1] != "B" || g[2] != "C" {
		t.Errorf("participants = %v, want [A B C]", g)
	}
}

func participantsOf(src string) []string {
	var parts []string
	for _, raw := range strings.Split(src, "\n") {
		from, to, _, _, _, ok := splitMessage(strings.TrimSpace(raw))
		if !ok {
			continue
		}
		for _, name := range []string{from, to} {
			seen := false
			for _, have := range parts {
				seen = seen || have == name
			}
			if !seen {
				parts = append(parts, name)
			}
		}
	}
	return parts
}

func TestStateDiagramRanksWithoutRunningAway(t *testing.T) {
	g := parseState(`stateDiagram-v2
  [*] --> Idle
  Idle --> Parsing
  Parsing --> Drawing
  Drawing --> Idle
  Failed --> [*]
`)
	g.layout()
	ranks := map[string]int{}
	for _, n := range g.nodes {
		ranks[n.label] = n.rank
	}
	if ranks["Idle"] >= ranks["Parsing"] {
		t.Errorf("Idle (%d) should rank above Parsing (%d)", ranks["Idle"], ranks["Parsing"])
	}
	if maxRank := ranks["Parsing"]; maxRank > 4 {
		t.Errorf("cycle pushed ranks to %d; ranking did not stabilise", maxRank)
	}
	out := plain(g.draw())
	for _, want := range []string{"●", "◉", "Idle", "Parsing"} {
		if !strings.Contains(out, want) {
			t.Errorf("state output missing %q:\n%s", want, out)
		}
	}
}

func TestClassDiagramBodiesAndInheritance(t *testing.T) {
	g := parseClass(`classDiagram
  class Renderer {
    +String name
    +render(src)
  }
  class Flow
  Renderer <|-- Flow
`)
	renderer := g.byID["RENDERER"]
	if renderer == nil {
		t.Fatal("Renderer not parsed")
	}
	if len(renderer.body) != 2 || renderer.body[1] != "+render(src)" {
		t.Errorf("Renderer body = %v", renderer.body)
	}
	if len(g.edges) != 1 {
		t.Fatalf("%d edges", len(g.edges))
	}
	// The arrowhead points at the parent, so the edge runs child -> parent.
	if e := g.edges[0]; e.to != renderer {
		t.Errorf("edge points at %v, want Renderer", e.to.id)
	}
	out := plain(render2(t, g))
	if !strings.Contains(out, "+render(src)") || !strings.Contains(out, "├") {
		t.Errorf("no compartment in the class box:\n%s", out)
	}
}

func render2(t *testing.T, g *graph) *Art {
	t.Helper()
	g.layout()
	return g.draw()
}

func TestERDiagram(t *testing.T) {
	g := parseER(`erDiagram
  CUSTOMER ||--o{ ORDER : places
  ORDER ||--|{ LINE_ITEM : contains
  CUSTOMER {
    string id PK
    string email
  }
`)
	if len(g.nodes) != 3 {
		t.Fatalf("%d entities, want 3", len(g.nodes))
	}
	// `ORDER ||--|{ LINE_ITEM` used to be split on a bare `--`.
	for _, n := range g.nodes {
		if strings.ContainsAny(n.label, "|{}<>") {
			t.Errorf("entity label %q kept cardinality markers", n.label)
		}
	}
	customer := g.byID["CUSTOMER"]
	if len(customer.body) != 2 {
		t.Errorf("CUSTOMER body = %v", customer.body)
	}
	if len(g.edges) != 2 || g.edges[0].label != "places" {
		t.Errorf("edges = %+v", g.edges)
	}
}

func TestSubgraphStillWorksForFlowcharts(t *testing.T) {
	out := plain(render(t, "graph LR\n  subgraph ci[CI]\n    A[a] --> B[b]\n  end\n  B --> C[c]\n"))
	if !strings.Contains(out, "╭─ CI ─") {
		t.Errorf("subgraph frame lost:\n%s", out)
	}
}

func TestSequenceFragmentsAndNotes(t *testing.T) {
	m, err := parseSequence(`sequenceDiagram
  A->>B: request
  loop every second
    B->>B: poll
  end
  alt ok
    B-->>A: body
  else failed
    B-->>A: error
  end
  Note over A,B: shared
  Note right of B: private
`)
	if err != nil {
		t.Fatal(err)
	}
	var starts, elses, ends, notes int
	for _, it := range m.items {
		switch it.kind {
		case itemFrameStart:
			starts++
		case itemFrameElse:
			elses++
		case itemFrameEnd:
			ends++
		case itemNote:
			notes++
			if it.note.side == noteRight && it.note.text != "private" {
				t.Errorf("right note = %+v", it.note)
			}
		}
	}
	if starts != 2 || elses != 1 || ends != 2 || notes != 2 {
		t.Fatalf("starts=%d elses=%d ends=%d notes=%d, want 2/1/2/2", starts, elses, ends, notes)
	}
	if m.maxDepth != 1 {
		t.Errorf("maxDepth = %d, want 1", m.maxDepth)
	}

	out := plain(render(t, `sequenceDiagram
  A->>B: request
  loop every second
    B->>B: poll
  end
  alt ok
    B-->>A: body
  else failed
    B-->>A: error
  end
  Note over A,B: shared
`))
	for _, want := range []string{"┌─ loop every second ", "┌─ alt ok ", "├─ failed ", "└", "shared"} {
		if !strings.Contains(out, want) {
			t.Errorf("sequence output missing %q:\n%s", want, out)
		}
	}
}

func TestSequenceNestedFragments(t *testing.T) {
	out := plain(render(t, `sequenceDiagram
  A->>B: go
  loop outer
    alt inner
      B-->>A: yes
    end
  end
`))
	if !strings.Contains(out, "┌─ loop outer ") || !strings.Contains(out, "┌─ alt inner ") {
		t.Errorf("nested frames missing:\n%s", out)
	}
	// The inner frame sits one gutter inside the outer one.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "alt inner") && !strings.HasPrefix(line, "│ ┌─ alt inner") {
			t.Errorf("inner frame not indented: %q", line)
		}
	}
}

func TestSequenceUnclosedFrameIsIgnored(t *testing.T) {
	out := plain(render(t, "sequenceDiagram\n  A->>B: hi\n  loop never closed\n    A->>B: again\n"))
	if strings.Contains(out, "└") && strings.Contains(out, "loop never closed ─") {
		t.Errorf("an unclosed loop should not draw a closed frame:\n%s", out)
	}
}
