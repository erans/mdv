package mermaid

import (
	"errors"
	"strconv"
	"strings"
)

// journeyStep is one task of a user journey.
type journeyStep struct {
	name   string
	score  int
	actors string
}

type journeySection struct {
	name  string
	steps []journeyStep
}

// renderJourney draws a user journey: a section per phase, a five-cell score bar
// per step and the actors beside it.
func renderJourney(src string) (*Art, error) {
	title := ""
	var sections []journeySection
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		low := strings.ToLower(line)
		switch {
		case line == "" || low == "journey":
			continue
		case strings.HasPrefix(low, "title "):
			title = strings.TrimSpace(line[len("title "):])
			continue
		case strings.HasPrefix(low, "section "):
			sections = append(sections, journeySection{name: strings.TrimSpace(line[len("section "):])})
			continue
		}
		name, rest, ok := strings.Cut(line, ":")
		if !ok || len(sections) == 0 {
			continue
		}
		scoreText, actors, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		score, err := strconv.Atoi(strings.TrimSpace(scoreText))
		if err != nil {
			continue
		}
		sections[len(sections)-1].steps = append(sections[len(sections)-1].steps, journeyStep{
			name:   strings.TrimSpace(name),
			score:  min(max(score, 1), 5),
			actors: strings.TrimSpace(actors),
		})
	}
	steps := 0
	labelW := 0
	for _, s := range sections {
		steps += len(s.steps)
		for _, st := range s.steps {
			labelW = max(labelW, len([]rune(st.name)))
		}
	}
	if steps == 0 {
		return nil, errors.New("journey needs at least one step")
	}
	labelW = min(labelW, chartWidth-20)

	c := newCanvas(chartWidth, steps+2*len(sections)+4)
	y := 0
	if title != "" {
		c.text(0, y, title, Title)
		y += 2
	}
	for _, s := range sections {
		c.text(0, y, s.name, Title)
		y++
		for _, st := range s.steps {
			c.text(2, y, truncateRunes(st.name, labelW), Text)
			x := 2 + labelW + 2
			for i := 0; i < 5; i++ {
				if i < st.score {
					c.put(x+i, y, '█', Edge)
				} else {
					c.put(x+i, y, '░', Border)
				}
			}
			if st.actors != "" {
				c.text(x+7, y, st.actors, EdgeLabel)
			}
			y++
		}
	}
	return c.art(), nil
}
