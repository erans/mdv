package mermaid

import (
	"errors"
	"strings"
)

// mindNode is one entry of a mindmap tree.
type mindNode struct {
	text     string
	indent   int
	children []*mindNode
}

// renderMindmap draws a mindmap as an indented tree. mermaid curves branches
// around a centre; a tree says the same thing and reads in a terminal.
func renderMindmap(src string) (*Art, error) {
	var roots []*mindNode
	var stack []*mindNode // path from a root to the node most recently added
	nodes := 0
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.EqualFold(line, "mindmap") {
			continue
		}
		text := cleanMindText(line)
		if text == "" || strings.HasPrefix(text, "::") { // ::icon(...) and similar
			continue
		}
		indent := indentOf(raw)
		n := &mindNode{text: text, indent: indent}
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, n)
		}
		stack = append(stack, n)
		nodes++
	}
	if len(roots) == 0 {
		return nil, errors.New("mindmap needs at least one node")
	}

	c := newCanvas(chartWidth, nodes+1)
	y := 0
	for _, root := range roots {
		c.text(0, y, truncateRunes(root.text, chartWidth), Title)
		y++
		y = c.mindChildren(root, "", y)
	}
	return c.art(), nil
}

// indentOf counts leading spaces, with a tab worth four.
func indentOf(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// cleanMindText strips what mermaid allows around a node label: the shape
// wrappers (`id[text]`, `id(text)`, `((text))`, `id{{text}}`) and `::icon(...)`.
func cleanMindText(s string) string {
	if i := strings.Index(s, "::"); i >= 0 {
		s = s[:i]
	}
	s = strings.NewReplacer("<br/>", " ", "<br>", " ").Replace(s)
	s = strings.TrimSpace(s)
	for _, wrap := range [][2]string{{"((", "))"}, {"{{", "}}"}, {"[[", "]]"}, {"[", "]"}, {"(", ")"}, {">", "]"}} {
		open, close := wrap[0], wrap[1]
		i := strings.Index(s, open)
		if i < 0 || !strings.HasSuffix(s, close) || len(s) < i+len(open)+len(close) {
			continue
		}
		head := strings.TrimSpace(s[:i])
		if head != "" && strings.ContainsAny(head, " \t") {
			continue // a sentence that happens to hold a bracket
		}
		s = strings.TrimSpace(s[i+len(open) : len(s)-len(close)])
		break
	}
	return strings.TrimSpace(s)
}

// mindChildren emits one node's subtree, carrying the connector prefix so the
// vertical lines stay continuous under the last sibling's parents.
func (c *canvas) mindChildren(n *mindNode, prefix string, y int) int {
	for i, child := range n.children {
		last := i == len(n.children)-1
		branch, cont := "├─ ", "│  "
		if last {
			branch, cont = "└─ ", "   "
		}
		c.text(0, y, prefix+branch, Border)
		labelX := len([]rune(prefix + branch))
		c.text(labelX, y, truncateRunes(child.text, chartWidth-labelX), Text)
		y++
		y = c.mindChildren(child, prefix+cont, y)
	}
	return y
}
