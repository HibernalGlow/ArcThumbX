package components

import (
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// StatusBadge is a compact state indicator: a lamp plus a word, coloured from
// the semantic tokens rather than from a literal.
type StatusBadge struct {
	State State
	Label string
}

// State names the semantic status slots a badge can carry.
type State int

const (
	// StateOK is a confirmed, healthy condition.
	StateOK State = iota
	// StateOff is a confirmed inactive condition — different from unknown.
	StateOff
	// StateWarn is degraded but working.
	StateWarn
	// StateError is broken or refused.
	StateError
	// StateInfo carries no judgement, only a fact.
	StateInfo
	// StateUnknown means the backend could not be asked.
	StateUnknown
)

func (s State) String() string {
	switch s {
	case StateOK:
		return "ok"
	case StateOff:
		return "off"
	case StateWarn:
		return "warn"
	case StateError:
		return "error"
	case StateInfo:
		return "info"
	default:
		return "unknown"
	}
}

// NewBadge builds a badge.
func NewBadge(state State, label string) *StatusBadge {
	return &StatusBadge{State: state, Label: label}
}

// OK and Missing are the two badges used most: a thing that works and a thing
// that was not found.
func OK(label string) *StatusBadge { return NewBadge(StateOK, label) }
func Missing(label string) *StatusBadge {
	return NewBadge(StateUnknown, label)
}

// View draws the badge.
func (b *StatusBadge) View(t *theme.Theme) string {
	if b == nil {
		return ""
	}
	style := t.Components.StatusInfo
	lamp := t.Glyphs.Off
	switch b.State {
	case StateOK:
		style, lamp = t.Components.StatusOn, t.Glyphs.On
	case StateOff:
		style, lamp = t.Components.StatusOff, t.Glyphs.Off
	case StateWarn:
		style, lamp = t.Components.StatusWarn, t.Glyphs.On
	case StateError:
		style, lamp = t.Components.StatusError, t.Glyphs.Cross
	case StateInfo:
		style, lamp = t.Components.StatusInfo, t.Glyphs.Bullet
	case StateUnknown:
		// Unknown never borrows the "off" lamp: an unasked question is not a
		// negative answer.
		style, lamp = t.Components.Muted, t.Glyphs.Bullet
	}
	return style.Render(lamp) + " " + style.Render(b.Label)
}

// Meter is the small signal ramp used for load-style indicators, drawn from the
// theme's signal glyph ramp so it degrades to ASCII where it has to.
type Meter struct {
	Value, Max int
	Steps      int
	Label      string
}

// NewMeter builds a signal meter.
func NewMeter(value, max, steps int, label string) *Meter {
	if steps <= 0 {
		steps = 4
	}
	return &Meter{Value: value, Max: max, Steps: steps, Label: label}
}

// View draws ▁▂▄▇ plus a caption.
func (m *Meter) View(t *theme.Theme) string {
	ramp := t.Glyphs.Signal
	if len(ramp) == 0 {
		ramp = []string{"|"}
	}
	filled := 0
	if m.Max > 0 {
		filled = m.Value * m.Steps / m.Max
	}
	if filled < 0 {
		filled = 0
	}
	if filled > m.Steps {
		filled = m.Steps
	}
	out := ""
	for i := 0; i < m.Steps; i++ {
		r := ramp[0]
		if i < filled {
			r = ramp[len(ramp)-1]
		} else if filled > 0 && i == filled {
			// The leading cell of an unfilled ramp shows the fraction.
			r = ramp[min(filled, len(ramp)-1)]
		}
		out += r
	}
	s := t.Components.Meter.Render(out)
	if m.Label != "" {
		s += " " + t.Components.Caption.Render(m.Label)
	}
	return s
}

// SwatchRow draws the palette of a theme, which is what the About page uses to
// let a user see a preset before switching to it.
func SwatchRow(t *theme.Theme, names []string) string {
	tokens := t.Colors
	lookup := map[string]theme.Color{
		"background":      tokens.Background,
		"surface":         tokens.Surface,
		"surfaceElevated": tokens.SurfaceElevated,
		"foreground":      tokens.Foreground,
		"muted":           tokens.ForegroundMuted,
		"primary":         tokens.Primary,
		"secondary":       tokens.Secondary,
		"accent":          tokens.Accent,
		"success":         tokens.Success,
		"warning":         tokens.Warning,
		"error":           tokens.Error,
		"info":            tokens.Info,
		"border":          tokens.Border,
		"focus":           tokens.Focus,
	}
	out := ""
	for _, n := range names {
		c, ok := lookup[n]
		if !ok {
			continue
		}
		block := "  "
		if c == theme.Inherit {
			out += t.Components.Muted.Render(n+":default") + "  "
			continue
		}
		out += t.Components.Swatch.Background(theme.Paint(c)).Render(block) + " "
	}
	return out
}

// RegisterScrollArea marks a pane as accepting the wheel. The app checks the
// hit map for this target when a wheel event arrives anywhere in the pane.
func RegisterScrollArea(c Ctx, id string) {
	c.Register(Target{Kind: HitScrollArea, ID: id}, c.Rect)
}
