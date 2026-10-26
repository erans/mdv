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
		"pie\n \"a\": 3":                                  "",
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
	g := parseSequence(t, "sequenceDiagram\n  A->>B: hi\n  C->>A: back\n")
	if len(g) != 3 || g[0] != "A" || g[1] != "B" || g[2] != "C" {
		t.Errorf("participants = %v, want [A B C]", g)
	}
}

func parseSequence(t *testing.T, src string) []string {
	t.Helper()
	var parts []string
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		from, to, _, _, _, ok := splitMessage(line)
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
