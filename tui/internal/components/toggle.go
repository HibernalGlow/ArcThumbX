package components

import (
	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// maxDisplay is the reserved width of the value column. Any longer and a row's
// label would be squeezed out of a narrow terminal, so values truncate here.
const maxDisplay = 28

// Toggle is a boolean control drawn as a state lamp, not a checkbox: a lamp
// reads at a glance across a column, and it keeps the panel free of the
// bracket noise that defines a bad terminal UI.
type Toggle struct {
	// Get reads the bound value and Set writes it back, so the state lives in
	// exactly one place.
	Get func() bool
	Set func(bool)

	// OnLabel and OffLabel replace the default ENABLED / DISABLED.
	OnLabel  string
	OffLabel string
}

// NewToggle binds a lamp to an accessor pair.
func NewToggle(get func() bool, set func(bool)) *Toggle {
	return &Toggle{Get: get, Set: set}
}

func (*Toggle) Kind() string { return "toggle" }

func (t *Toggle) labels() (string, string) {
	on, off := t.OnLabel, t.OffLabel
	if on == "" {
		on = "Enabled"
	}
	if off == "" {
		off = "Disabled"
	}
	return on, off
}

// Display renders "● Enabled" or "○ Disabled".
func (t *Toggle) Display(th *theme.Theme) string {
	on, off := t.labels()
	if t.Get() {
		return th.Components.ToggleOn.Render(th.Glyphs.On) + " " +
			th.Components.ValueFlag.Render(on)
	}
	return th.Components.ToggleOff.Render(th.Glyphs.Off) + " " +
		th.Components.Muted.Render(off)
}

func (t *Toggle) Width(*theme.Theme) int { return maxDisplay }

// Commit flips the bound value.
func (t *Toggle) Commit() { t.Set(!t.Get()) }

// Step forces the lamp on or off, which is what ← and → mean for a boolean on
// an instrument panel.
func (t *Toggle) Step(dir int) {
	if dir > 0 {
		t.Set(true)
	} else {
		t.Set(false)
	}
}

// Static is a value cell that is never editable: the read-only disclosure row
// the Integration page needs for facts the TUI can read but not write.
type Static struct {
	Value string
	// Help is drawn in the inspector when the row is focused.
	Help string
}

func (Static) Kind() string { return "static" }
func (s Static) Display(t *theme.Theme) string {
	if s.Value == "" {
		return t.Components.Muted.Render("—")
	}
	return t.Components.ValueFlag.Render(layout.Truncate(s.Value, maxDisplay))
}
func (Static) Width(*theme.Theme) int { return maxDisplay }
func (Static) Commit()                {}
func (Static) Step(int)               {}
