package components_test

import (
	"strconv"
	"testing"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
)

func plainRows(n int) []components.Row {
	out := make([]components.Row, 0, n)
	for i := range n {
		out = append(out, components.Row{Label: "row " + strconv.Itoa(i)})
	}
	return out
}

func scrollForm() *components.Form {
	return &components.Form{ID: "f", Sections: []components.Section{{
		Title: "S",
		Rows:  plainRows(20),
	}}}
}

// TestScrollStepsCancelAndClampAtTheTop pins the arithmetic both inputs rest on:
// the wheel and the keyboard page step are the same gesture, so a step down and
// the matching step back must land exactly where they started, and the top must
// hold. SetScroll has a lower clamp and no upper one, which is why the ceiling is
// not asserted here — the pane owns it, and an app-level frame test is the only
// place that can see it.
func TestScrollStepsCancelAndClampAtTheTop(t *testing.T) {
	f := scrollForm()
	if f.Scroll != 0 {
		t.Fatalf("a fresh form starts at scroll %d", f.Scroll)
	}
	for range 4 {
		f.ScrollBy(3)
	}
	if f.Scroll != 12 {
		t.Fatalf("four steps of three = %d, want 12", f.Scroll)
	}
	for range 4 {
		f.ScrollBy(-3)
	}
	if f.Scroll != 0 {
		t.Errorf("the matching steps back did not cancel: scroll = %d", f.Scroll)
	}

	if changed := f.ScrollBy(-3); changed {
		t.Error("scrolling above the top reported a change; the clamp is missing")
	}
	if f.Scroll != 0 {
		t.Errorf("scroll = %d after an upward step from the top", f.Scroll)
	}
}

// TestScrollArithmeticSeesAnAsymmetricStep is the positive control: the check
// above must go red if the two directions stop being equal, otherwise it proves
// nothing about the wheel.
func TestScrollArithmeticSeesAnAsymmetricStep(t *testing.T) {
	f := scrollForm()
	f.ScrollBy(3)
	f.ScrollBy(-1)
	if f.Scroll == 0 {
		t.Fatal("positive control failed: three down and one up compared as equal")
	}
	f.ScrollBy(-2)
	if f.Scroll != 0 {
		t.Fatalf("positive control failed: the same offset could not be restored, scroll = %d", f.Scroll)
	}
}
