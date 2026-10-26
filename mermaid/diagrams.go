package mermaid

import (
	"errors"
	"fmt"
	"strings"
)

// Render parses any supported mermaid diagram and returns its art. An error
// means the caller should show the fenced source instead.
func Render(src string) (*Art, error) {
	switch kind(src) {
	case "flowchart":
		return renderFlow(src)
	case "state":
		return renderState(src)
	case "class":
		return renderClass(src)
	case "er":
		return renderER(src)
	case "sequence":
		return renderSequence(src)
	}
	return nil, errors.New("unsupported diagram type")
}

// kind looks at the first meaningful line of the source.
func kind(src string) string {
	inFrontMatter := false
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case line == "---":
			inFrontMatter = !inFrontMatter
			continue
		case inFrontMatter:
			continue
		}
		low := strings.ToLower(line)
		switch {
		case strings.HasPrefix(low, "graph"), strings.HasPrefix(low, "flowchart"):
			return "flowchart"
		case strings.HasPrefix(low, "statediagram"):
			return "state"
		case strings.HasPrefix(low, "classdiagram"):
			return "class"
		case strings.HasPrefix(low, "erdiagram"):
			return "er"
		case strings.HasPrefix(low, "sequencediagram"):
			return "sequence"
		}
		return ""
	}
	return ""
}

// relation is one parsed link between two entities.
type relation struct {
	left, right string
	label       string
	toLeft      bool // arrowhead sits on the left-hand node
	toRight     bool
	dashed      bool
}

// splitRelation parses `A <|-- B : label`, `A ||--o{ B : label`, `A --> B`
// and the rest of mermaid's link zoo, without enumerating every combination of
// arrowheads and cardinality markers.
func splitRelation(line string) (relation, bool) {
	var rel relation
	if i := strings.LastIndex(line, ":"); i >= 0 {
		rel.label = strings.TrimSpace(line[i+1:])
		line = strings.TrimSpace(line[:i])
	}
	core := strings.Index(line, "--")
	dashed := false
	if core < 0 {
		core = strings.Index(line, "..")
		dashed = core >= 0
	}
	if core < 0 {
		return relation{}, false
	}
	// Absorb the decorations around the link core: <|, |, *, o, {, }.
	l := core
	for l > 0 && strings.ContainsRune("<|*o{}", rune(line[l-1])) {
		l--
	}
	r := core + 2
	for r < len(line) && strings.ContainsRune(">|*o{}", rune(line[r])) {
		r++
	}
	rel.left = strings.TrimSpace(line[:l])
	rel.right = strings.TrimSpace(line[r:])
	rel.dashed = dashed
	op := line[l:r]
	rel.toLeft = strings.Contains(op, "<") || strings.Contains(op, "|") && strings.HasPrefix(op, "|")
	rel.toRight = strings.Contains(op, ">")
	if rel.left == "" || rel.right == "" {
		return relation{}, false
	}
	return rel, true
}

// cleanEntity strips quoted multiplicities and trailing markers from an entity
// reference: `CUSTOMER "1"` becomes `CUSTOMER`.
func cleanEntity(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, `"`); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// entity renders a named box with optional attribute rows.
func entity(g *graph, name string, sh shape) *node {
	label := strings.Trim(name, `"`)
	id := strings.ToUpper(label)
	if n, ok := g.byID[id]; ok {
		return n
	}
	return g.node(id, label, sh)
}

func newGraph(dir string) *graph {
	return &graph{dir: dir, byID: map[string]*node{}, owner: map[*node]*subgraph{}}
}

// renderState draws a state diagram on the flowchart engine: states are rounded
// boxes and `[*]` becomes a start or end dot.
func renderState(src string) (*Art, error) {
	g := parseState(src)
	if len(g.nodes) == 0 {
		return nil, errors.New("no states")
	}
	g.layout()
	return g.draw(), nil
}

func parseState(src string) *graph {
	g := newGraph("TD")
	starts, ends := 0, 0
	var pending *node // state whose body we are collecting (not used in v1)
	_ = pending
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" || strings.HasPrefix(strings.ToLower(line), "statediagram") {
			continue
		}
		low := strings.ToLower(line)
		if d, ok := strings.CutPrefix(low, "direction "); ok {
			g.dir = strings.ToUpper(strings.TrimSpace(d))
			continue
		}
		if isDirective(low) || strings.HasPrefix(low, "note") || strings.HasPrefix(low, "state ") && !strings.Contains(line, "-->") {
			if rest, ok := strings.CutPrefix(line, "state "); ok {
				// `state Red` or `state "waiting" as W`
				name, id := strings.TrimSpace(rest), strings.TrimSpace(rest)
				if left, right, found := strings.Cut(strings.ToLower(rest), " as "); found {
					name, id = strings.TrimSpace(left), strings.TrimSpace(rest[len(left)+4:])
					_ = right
				}
				entity(g, id, roundShape).label = strings.Trim(name, `"`)
			}
			continue
		}
		rel, ok := splitRelation(line)
		if !ok {
			continue
		}
		from := stateNode(g, rel.left, &starts, &ends)
		to := stateNode(g, rel.right, &starts, &ends)
		g.edges = append(g.edges, &edge{from: from, to: to, label: rel.label, arrow: true})
	}
	return g
}

// stateNode resolves one side of a transition, inventing a dot for `[*]`.
func stateNode(g *graph, ref string, starts, ends *int) *node {
	ref = cleanEntity(ref)
	if ref != "[*]" {
		return entity(g, ref, roundShape)
	}
	if len(g.edges) == 0 && len(g.nodes) == 0 {
		*starts++
		return g.node(fmt.Sprintf("__start%d", *starts), "●", roundShape)
	}
	*ends++
	return g.node(fmt.Sprintf("__end%d", *ends), "◉", roundShape)
}

// renderClass draws a UML class box per class, with its members inside.
func renderClass(src string) (*Art, error) {
	g := parseClass(src)
	if len(g.nodes) == 0 {
		return nil, errors.New("no classes")
	}
	g.layout()
	return g.draw(), nil
}

func parseClass(src string) *graph {
	g := newGraph("TD")
	cur := (*node)(nil)
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "classdiagram") {
			continue
		}
		if cur != nil {
			if line == "}" {
				cur = nil
				continue
			}
			cur.body = append(cur.body, line)
			continue
		}
		low := strings.ToLower(line)
		if rest, ok := strings.CutPrefix(line, "class "); ok {
			name, body, open := strings.Cut(rest, "{")
			n := entity(g, strings.TrimSpace(name), rectShape)
			if open {
				if fields := strings.TrimSpace(body); fields != "" {
					n.body = append(n.body, fields)
				} else {
					cur = n
				}
			}
			continue
		}
		if isDirective(low) || strings.HasPrefix(low, "note") {
			continue
		}
		rel, ok := splitRelation(line)
		if !ok {
			continue
		}
		from, to := entity(g, cleanEntity(rel.left), rectShape), entity(g, cleanEntity(rel.right), rectShape)
		e := &edge{from: from, to: to, label: rel.label, dashed: rel.dashed, arrow: rel.toLeft || rel.toRight}
		if rel.toLeft {
			e.from, e.to = to, from // the arrowhead points at the left-hand class
		}
		g.edges = append(g.edges, e)
	}
	return g
}

// renderER draws entities with their attributes and the relations between them.
func renderER(src string) (*Art, error) {
	g := parseER(src)
	if len(g.nodes) == 0 {
		return nil, errors.New("no entities")
	}
	g.layout()
	return g.draw(), nil
}

func parseER(src string) *graph {
	g := newGraph("TD")
	cur := (*node)(nil)
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "erdiagram") {
			continue
		}
		if cur != nil {
			if line == "}" {
				cur = nil
				continue
			}
			cur.body = append(cur.body, strings.Join(strings.Fields(line), " "))
			continue
		}
		if isDirective(strings.ToLower(line)) {
			continue
		}
		if i := strings.Index(line, "{"); i >= 0 && !strings.ContainsAny(line[:i], "-.") {
			name := cleanEntity(line[:i])
			cur = entity(g, name, rectShape)
			if fields := strings.TrimSpace(line[i+1:]); fields != "" {
				cur.body = append(cur.body, strings.Join(strings.Fields(fields), " "))
				if strings.HasSuffix(fields, "}") {
					cur = nil
				}
			}
			continue
		}
		rel, ok := splitRelation(line)
		if !ok {
			continue
		}
		from, to := entity(g, cleanEntity(rel.left), rectShape), entity(g, cleanEntity(rel.right), rectShape)
		g.edges = append(g.edges, &edge{from: from, to: to, label: rel.label, dashed: rel.dashed, arrow: true})
	}
	return g
}
