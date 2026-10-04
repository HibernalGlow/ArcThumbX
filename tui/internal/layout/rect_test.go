package layout_test

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
)

func TestHitMapLastRegisteredWins(t *testing.T) {
	h := layout.NewHitMap()
	// Paint order: the page first, then the dialog that sits on top of it.
	h.Add("page", layout.R(0, 0, 40, 10), "page")
	h.Add("dialog", layout.R(5, 2, 20, 6), "dialog")

	if id, data, ok := h.Pick(10, 4); !ok || id != "dialog" || data != "dialog" {
		t.Errorf("overlapping pick resolved to %v/%v, want the dialog on top", id, data)
	}
	if id, _, ok := h.Pick(1, 1); !ok || id != "page" {
		t.Errorf("edge pick resolved to %q, want the page underneath", id)
	}
	if _, _, ok := h.Pick(100, 100); ok {
		t.Error("outside the frame must not hit anything")
	}
}

func TestEmptyRectNeverHits(t *testing.T) {
	h := layout.NewHitMap()
	h.Add("zero", layout.R(3, 3, 0, 0), nil)
	h.Add("negative", layout.R(-1, -1, 5, -2), nil)
	if h.Len() != 0 {
		t.Fatalf("degenerate rects registered %d targets", h.Len())
	}
	if _, _, ok := h.Pick(0, 0); ok {
		t.Error("empty hit map returned a target")
	}
}

func TestResetKeepsCapacity(t *testing.T) {
	h := layout.NewHitMap()
	h.Add("a", layout.R(0, 0, 2, 2), nil)
	h.Reset()
	if h.Len() != 0 {
		t.Error("Reset left targets behind; the frame would double-hit")
	}
	h.Add("b", layout.R(0, 0, 2, 2), nil)
	if id, _, _ := h.Pick(0, 0); id != "b" {
		t.Errorf("after Reset the map resolves %q", id)
	}
}

func TestFitPadsAndTruncates(t *testing.T) {
	if got := layout.Fit("ab", 5); got != "ab   " {
		t.Errorf("Fit pad = %q", got)
	}
	if got := layout.FitRight("ab", 5); got != "   ab" {
		t.Errorf("FitRight = %q", got)
	}
	if got := layout.Truncate("abcdef", 4); got != "abc…" {
		t.Errorf("Truncate = %q, want abc…", got)
	}
	// A wide-rune run must never exceed the cell budget, or a CJK label would
	// push the value column off the line.
	if w := lipgloss.Width(layout.Truncate("一二三", 4)); w > 4 {
		t.Errorf("wide-rune truncation overshot to %d cells", w)
	}
}

func TestFitIsStyleAgnostic(t *testing.T) {
	// The same cell count must come out whether or not the text carries SGR,
	// otherwise a themed value would break the column.
	plain := layout.Fit("value", 12)
	styled := layout.Fit("\x1b[38;2;240;195;70mvalue\x1b[0m", 12)
	if lipgloss.Width(plain) != 12 || lipgloss.Width(styled) != 12 {
		t.Errorf("widths = %d and %d, want 12 (escape bytes leaked into the measure)",
			lipgloss.Width(plain), lipgloss.Width(styled))
	}
}

func TestClampAndPad(t *testing.T) {
	if got := layout.ClampTo("a\nb\nc", 2); got != "a\nb" {
		t.Errorf("ClampTo = %q", got)
	}
	if got := layout.PadLeft("a\n\nb", 2); got != "  a\n\n  b" {
		t.Errorf("PadLeft = %q", got)
	}
	if got := layout.ColumnWidth(200, 96); got != 96 {
		t.Errorf("ColumnWidth ignored the measure: %d", got)
	}
	if got := layout.ColumnWidth(10, 96); got != 20 {
		t.Errorf("ColumnWidth let a pane shrink below readable: %d", got)
	}
}

func TestRectRows(t *testing.T) {
	rows := layout.R(2, 5, 10, 3).Rows(5)
	if len(rows) != 3 {
		t.Fatalf("Rows produced %d, want the rect height", len(rows))
	}
	if rows[2].Y != 7 || rows[2].X != 2 {
		t.Errorf("third row = %v", rows[2])
	}
}
