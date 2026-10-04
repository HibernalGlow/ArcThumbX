package components

import (
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// Option is one choice in a Select.
type Option struct {
	// Label is the short form drawn in the value cell.
	Label string
	// Detail explains the choice in the inspector line.
	Detail string
}

// Select cycles a bounded enum, drawn as ‹ VALUE › rather than as a dropdown.
// A terminal has no room for a popup list of three cover modes, and cycling
// shows the current value while you change it.
type Select struct {
	Get     func() int
	Set     func(int)
	Options []Option

	// Wrap makes ← from the first choice land on the last.
	Wrap bool
}

// NewSelect binds a cycling selector to an index accessor pair.
func NewSelect(get func() int, set func(int), opts []Option) *Select {
	return &Select{Get: get, Set: set, Options: opts}
}

func (*Select) Kind() string { return "select" }

func (s *Select) clamp(i int) int {
	n := len(s.Options)
	if n == 0 {
		return 0
	}
	if i < 0 {
		if s.Wrap {
			return n - 1
		}
		return 0
	}
	if i >= n {
		if s.Wrap {
			return 0
		}
		return n - 1
	}
	return i
}

// Current is the selected option, or a zero Option when the list is empty.
func (s *Select) Current() Option {
	if len(s.Options) == 0 {
		return Option{}
	}
	return s.Options[s.clamp(s.Get())]
}

// Display shows arrows only while focused, so a screen of selectors does not
// turn into a wall of chevrons.
func (s *Select) Display(t *theme.Theme) string {
	label := s.Current().Label
	if label == "" {
		label = "—"
	}
	return t.Components.Value.Render(label)
}

// DisplayFocused adds the cycle affordance.
func (s *Select) DisplayFocused(t *theme.Theme) string {
	return t.Components.SelectArrows.Render(t.Glyphs.Prev) + " " +
		s.Display(t) + " " +
		t.Components.SelectArrows.Render(t.Glyphs.Next)
}

func (s *Select) Width(*theme.Theme) int {
	w := 0
	for _, o := range s.Options {
		if n := len([]rune(o.Label)); n+4 > w {
			w = n + 4
		}
	}
	if w > maxDisplay {
		return maxDisplay
	}
	if w == 0 {
		return 8
	}
	return w
}

// Commit advances one option, which is what Enter means for a cycle control.
func (s *Select) Commit() { s.Set(s.clamp(s.Get() + 1)) }

// Step moves to the previous or next option.
func (s *Select) Step(dir int) { s.Set(s.clamp(s.Get() + dir)) }
