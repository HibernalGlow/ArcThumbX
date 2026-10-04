// Package components is the ArcThumbX TUI widget layer: the only place that
// knows how a cell is painted. Business pages compose these and never draw.
//
// Nothing here imports Bubble Tea. Controls report a Display string and accept
// intents (Commit/Step), and the app translates key and mouse events into those
// intents. That keeps every widget testable as a pure function of state, and it
// keeps the transport out of the design system.
package components

import (
	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// Ctx is what a component needs to draw one frame: the compiled theme, where
// absolutely on screen it is, and the hit map to register its interactive areas
// in.
type Ctx struct {
	T *theme.Theme
	// Rect is the component's absolute allocation in terminal cells.
	Rect layout.Rect
	// Hits collects interactive areas during this frame. May be nil, which
	// means the caller only wants pixels (tests, --render).
	Hits *layout.HitMap
	// Width is the measure long text should wrap to; 0 falls back to Rect.W.
	Width int
}

// Theme is a shorthand accessor, since almost every line in this package reads
// a token.
func (c Ctx) Theme() *theme.Theme { return c.T }

// X and Y place a sub-rect inside this component's allocation.
func (c Ctx) At(x, y int) layout.Rect {
	return layout.Rect{X: c.Rect.X + x, Y: c.Rect.Y + y, W: c.Rect.W - x, H: c.Rect.H - y}
}

// Shrink returns a copy with n cells taken off the left.
func (c Ctx) ShrinkLeft(n int) Ctx {
	c.Rect.X += n
	c.Rect.W -= n
	return c
}

// ShrinkTop returns a copy whose first n lines are consumed.
func (c Ctx) ShrinkTop(n int) Ctx {
	c.Rect.Y += n
	c.Rect.H -= n
	return c
}

// WithHits swaps the hit map, used when a component must draw into an
// overlay's own map slot.
func (c Ctx) WithHits(h *layout.HitMap) Ctx { c.Hits = h; return c }

// HitKind says what a registered region is, so the app can dispatch a click
// without string-matching IDs.
type HitKind int

const (
	// HitRow is the whole line of a setting row: clicking it takes focus.
	HitRow HitKind = iota
	// HitControl is the value cell of a row: clicking it activates the
	// widget, which is what makes a toggle a one-click affair.
	HitControl
	// HitNav is a sidebar entry.
	HitNav
	// HitTab is a tab label.
	HitTab
	// HitButton is a push button, including a dialog action.
	HitButton
	// HitListItem is a row in a List.
	HitListItem
	// HitScrollArea covers a pane that accepts the wheel.
	HitScrollArea
)

func (k HitKind) String() string {
	switch k {
	case HitRow:
		return "row"
	case HitControl:
		return "control"
	case HitNav:
		return "nav"
	case HitTab:
		return "tab"
	case HitButton:
		return "button"
	case HitListItem:
		return "listitem"
	case HitScrollArea:
		return "scrollarea"
	default:
		return "unknown"
	}
}

// Target is the payload stored with a hit rect.
type Target struct {
	Kind HitKind
	// Index is the row, tab, item or button index.
	Index int
	// ID disambiguates within a kind, e.g. which list was clicked.
	ID string
}

// Register records a target for a region. A nil hit map means pixels-only, so
// every caller is safe without a nil check.
func (c Ctx) Register(t Target, r layout.Rect) {
	if c.Hits == nil || r.Empty() {
		return
	}
	c.Hits.Add(layout.HitID(t.Kind.String()+":"+t.ID), r, t)
}

// Control is an editable widget inside a setting row.
//
// A control owns no state: it reads and writes through accessors supplied by
// the page, which is why there is exactly one copy of every value and why the
// dirty flag can be derived from the state itself rather than tracked by hand.
type Control interface {
	// Kind names the widget for the inspector line and for tests.
	Kind() string
	// Display is the value cell, already styled from tokens.
	Display(t *theme.Theme) string
	// Width is how many cells Display can occupy at most, so the row can
	// reserve its value column before drawing anything.
	Width(t *theme.Theme) int
	// Commit applies the primary action: Enter, Space or a click.
	Commit()
	// Step moves a discrete value; dir is -1 or +1. Controls with no range
	// ignore it.
	Step(dir int)
}

// sourceGlyphs labels where a value came from, so a number that is not live
// state can never be mistaken for one that is.
type Source int

const (
	// SourceLive is read from ArcThumbX configuration.
	SourceLive Source = iota
	// SourceLocal is a TUI preference, stored in the front-end's own file.
	SourceLocal
	// SourceDerived is computed from live values, e.g. a count of enabled
	// extensions.
	SourceDerived
	// SourceUnavailable is a fact this build cannot obtain. It draws a dash,
	// never a made-up number.
	SourceUnavailable
)

func (s Source) String() string {
	switch s {
	case SourceLive:
		return "live"
	case SourceLocal:
		return "pref"
	case SourceDerived:
		return "calc"
	default:
		return "n/a"
	}
}

func (s Source) marker(t *theme.Theme) string {
	g := t.Glyphs
	switch s {
	case SourceLive:
		return g.On
	case SourceLocal:
		return g.Check
	case SourceDerived:
		return g.Bullet
	default:
		return g.Off
	}
}
