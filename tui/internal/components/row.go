package components

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// Row is one line of a settings panel: a label, a value cell, and the widget
// that owns the value. This is the atom the UI is built from — pages compose
// sections of rows, and only rows know how to lay a control out.
type Row struct {
	Label string

	// Help is shown in the inspector strip while the row is focused, which is
	// what keeps the panel dense without hiding explanation.
	Help string

	// Control is the editable widget. A row without one is a read-only
	// disclosure and is drawn in the muted value colour.
	Control Control

	// Value is the disclosure text for a read-only row.
	Value string

	// Status is an optional badge drawn after the value.
	Status *StatusBadge

	// Flag records where the value came from, so nothing simulated can be
	// mistaken for live ArcThumbX state.
	Flag Source
}

// Bind makes an editable row.
func Bind(label string, c Control, help string, flag Source) Row {
	return Row{Label: label, Control: c, Help: help, Flag: flag}
}

// Disclosure makes a read-only row for a fact the TUI can see but not change.
// An empty value draws a dash: the UI never substitutes a word the backend did
// not actually report.
func Disclosure(label, value, help string, flag Source) Row {
	return Row{Label: label, Value: value, Help: help, Flag: flag}
}

func (r *Row) editable() bool { return r.Control != nil }

// DisplayValue renders the value cell, letting a focused selector show its
// cycle affordance.
func (r *Row) DisplayValue(t *theme.Theme, focused bool) string {
	var out string
	switch {
	case r.Control != nil:
		if sel, ok := r.Control.(*Select); ok && focused {
			out = sel.DisplayFocused(t)
		} else {
			out = r.Control.Display(t)
		}
	default:
		out = Static{Value: r.Value}.Display(t)
	}
	if r.Status != nil {
		out += " " + r.Status.View(t)
	}
	return out
}

// View draws one line of the form: focus marker, source marker, label, then the
// value right-aligned in its own column so a page of values reads like a set of
// gauge faceplates rather than a text file.
//
// It also reports the cell where the value column starts, so the form can
// register a hit target exactly over the control instead of guessing.
func (r *Row) View(t *theme.Theme, w int, focused bool) (line string, valueX int) {
	labelStyle := t.Components.Label
	if focused {
		labelStyle = t.Components.LabelActive
	}

	marker := "  "
	if focused {
		marker = t.Components.Focus.Render(t.Glyphs.Focus) + " "
	}
	flagMark := ""
	if r.Flag != SourceLive {
		flagMark = t.Components.Muted.Render(r.Flag.marker(t)) + " "
	}

	value := r.DisplayValue(t, focused)
	valueW := lipgloss.Width(value)
	if cap := w * 2 / 3; cap > 0 && valueW > cap {
		value = layout.Truncate(value, cap)
		valueW = cap
	}
	gap := max(t.Space.Gutter, 1)
	prefixW := lipgloss.Width(marker) + lipgloss.Width(flagMark)
	labelW := max(w-prefixW-valueW-gap, 6)
	left := marker + flagMark + labelStyle.Render(layout.Truncate(r.Label, labelW))
	pad := max(w-lipgloss.Width(left)-valueW, gap)
	valueX = lipgloss.Width(left) + pad
	return left + strings.Repeat(" ", pad) + layout.Fit(value, valueW), valueX
}

// Inspector is the explanation line for this row: its help text, falling back
// to what the control reports, so the strip is never blank for no reason.
func (r *Row) Inspector(t *theme.Theme) string {
	text := r.Help
	if text == "" {
		switch c := r.Control.(type) {
		case *Select:
			text = c.Current().Detail
		case *Toggle:
			text = "Toggle with enter, space or click."
		case *Slider:
			text = "Adjust with the arrow keys or the mouse wheel."
		}
	}
	if text == "" {
		text = r.Flag.String() + " value"
	}
	return t.Components.Description.Render(layout.Truncate(text, 400))
}
