package components

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// Section is a titled group of rows. It is the unit of visual hierarchy under a
// page: an engraved legend with a hairline trailing it, deliberately not a box.
type Section struct {
	Title string

	// Note is a short right-aligned remark on the legend line, e.g. "HKCU".
	Note string

	Rows []Row
}

// render appends the section's lines to out, recording which row index owns
// each line in owners (-1 marks chrome that is not a target). firstRow is the
// global index of this section's first row.
func (s *Section) render(t *theme.Theme, w, firstRow int, lines *[]string, owners *[]int) {
	if *lines != nil && len(*lines) > 0 {
		*lines = append(*lines, "")
		*owners = append(*owners, -1)
	}

	title := t.Glyphs.Section + " " + t.Components.Section.Transform(s.Title)
	note := ""
	if s.Note != "" {
		note = t.Components.Caption.Render(s.Note)
	}
	*lines = append(*lines, legend(t, w, title, note))
	*owners = append(*owners, -1)

	for i := range s.Rows {
		line, _ := s.Rows[i].View(t, w, false)
		*lines = append(*lines, line)
		*owners = append(*owners, firstRow+i)
	}
}

// legend draws a section legend: marker, spaced uppercase title, then a hairline
// filling the rest of the line, with an optional note hung at the right.
func legend(t *theme.Theme, w int, title, note string) string {
	titleW := len([]rune(title))
	noteW := 0
	if note != "" {
		// The note keeps its own cell of separation from the rule.
		noteW = lipgloss.Width(note) + 1
	}
	// The single space between the title and the rule has to be paid for out of
	// the same w: leaving it unaccounted made every legend one cell too wide.
	rest := w - titleW - 1 - noteW
	if rest < 2 {
		rest = 2
	}
	rule := t.Components.SectionRule.Render(strings.Repeat(t.Glyphs.Rule, rest))
	out := title + " " + rule
	if note != "" {
		out += " " + note
	}
	return layout.Truncate(out, w)
}
