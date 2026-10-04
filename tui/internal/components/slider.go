package components

import (
	"strconv"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// Slider is a bounded integer control drawn as an instrument bar, the terminal
// equivalent of a panel fader. It is used for values that have a real range and
// where seeing the position matters more than typing a number.
type Slider struct {
	Get func() int
	Set func(int)

	Min, Max int
	// Increment is the cell count per key press; 0 means 1. The method that
	// moves the fader is Step, so this field cannot share the name.
	Increment int
	// Unit is appended to the readout, e.g. "cols".
	Unit string
	// Track is how many cells the bar itself occupies.
	Track int
}

// NewSlider binds a fader to an accessor pair over an inclusive range.
func NewSlider(get func() int, set func(int), min, max int) *Slider {
	return &Slider{Get: get, Set: set, Min: min, Max: max, Increment: 1, Track: 10}
}

func (*Slider) Kind() string { return "slider" }

func (s *Slider) step() int {
	if s.Increment <= 0 {
		return 1
	}
	return s.Increment
}

func (s *Slider) clamp(v int) int {
	if v < s.Min {
		return s.Min
	}
	if v > s.Max {
		return s.Max
	}
	return v
}

// Value is the clamped current value.
func (s *Slider) Value() int { return s.clamp(s.Get()) }

// Display renders ▓▓▓▓░░░░░░ 96 cols.
func (s *Slider) Display(t *theme.Theme) string {
	v := s.Value()
	track := s.Track
	if track <= 0 {
		track = 10
	}
	filled := 0
	if s.Max > s.Min {
		filled = (v - s.Min) * track / (s.Max - s.Min)
	}
	if filled > track {
		filled = track
	}
	if s.Min == s.Max {
		filled = track
	}
	bar := t.Components.SliderFill.Render(repeat(t.Glyphs.SliderFull, filled)) +
		t.Components.SliderTrack.Render(repeat(t.Glyphs.SliderEmpty, track-filled))
	readout := strconv.Itoa(v)
	if s.Unit != "" {
		readout += " " + s.Unit
	}
	return bar + " " + t.Components.Value.Render(readout)
}

func (s *Slider) Width(t *theme.Theme) int {
	track := s.Track
	if track <= 0 {
		track = 10
	}
	return track + 1 + len(strconv.Itoa(s.Max)) + len(s.Unit) + 2
}

// Commit has no meaning for a fader, so it does nothing rather than guessing.
func (s *Slider) Commit() {}

// Step moves the fader by one increment per cell of direction.
func (s *Slider) Step(dir int) {
	if dir == 0 {
		return
	}
	s.Set(s.clamp(s.Value() + dir*s.step()))
}

func repeat(glyph string, n int) string {
	out := make([]rune, 0, n*len([]rune(glyph)))
	for i := 0; i < n; i++ {
		out = append(out, []rune(glyph)...)
	}
	return string(out)
}
