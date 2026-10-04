package components

import (
	"strings"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// Form is the focus, scroll and hit-testing engine behind every settings page.
//
// A page builds sections of rows and hands them to a Form; the Form decides
// which row owns focus, keeps it on screen when the page is taller than the
// pane, and registers one hit target per row plus one over each control's value
// cell. That is why adding a page never means writing navigation code.
type Form struct {
	// ID namespaces this form's hit targets.
	ID string

	// Heading is the page title, drawn above the sections.
	Heading string
	// Sub is the one-line metadata under the heading.
	Sub string

	Sections []Section

	// Focus is the index into the flattened rows; Scroll is the first visible
	// line of the rendered body.
	Focus  int
	Scroll int

	// rowsByLine and lineOwners are rebuilt every frame from the render, so a
	// click always resolves against the geometry the user is looking at.
	lineOwners []int
	rows       []*Row
	bodyTop    int
	bodyW      int
}

// Rows flattens the sections into one slice, in draw order. It is called by
// every read path, not just View: a page that rebuilds its sections and then
// takes a keystroke before the next frame would otherwise act on last frame's
// rows, which are a different slice.
func (f *Form) Rows() []*Row {
	f.rows = f.rows[:0]
	for i := range f.Sections {
		for j := range f.Sections[i].Rows {
			f.rows = append(f.rows, &f.Sections[i].Rows[j])
		}
	}
	return f.rows
}

// Total counts the rows a form can focus.
func (f *Form) Total() int { return len(f.Rows()) }

// Current is the focused row, or nil when the form is empty.
func (f *Form) Current() *Row {
	rows := f.Rows()
	if f.Focus < 0 || f.Focus >= len(rows) {
		return nil
	}
	return rows[f.Focus]
}

// FocusRow moves focus and reports whether anything changed.
func (f *Form) FocusRow(i int) bool {
	rows := f.Rows()
	if len(rows) == 0 {
		f.Focus = 0
		return false
	}
	if i < 0 {
		i = 0
	}
	if i >= len(rows) {
		i = len(rows) - 1
	}
	if f.Focus == i {
		return false
	}
	f.Focus = i
	return true
}

// Move steps focus by dir and wraps, which is what ↑/↓ and Tab/Shift+Tab do.
// Wrapping means a user can always reach every row from any row in at most
// len(rows)-1 presses.
func (f *Form) Move(dir int) bool {
	n := f.Total()
	if n == 0 || dir == 0 {
		return false
	}
	next := (f.Focus + dir) % n
	if next < 0 {
		next += n
	}
	return f.FocusRow(next)
}

// Commit applies the primary action to the focused row.
func (f *Form) Commit() bool {
	r := f.Current()
	if r == nil || !r.editable() {
		return false
	}
	r.Control.Commit()
	return true
}

// Step nudges a bounded control left or right.
func (f *Form) Step(dir int) bool {
	r := f.Current()
	if r == nil || !r.editable() {
		return false
	}
	r.Control.Step(dir)
	return true
}

// SetScroll moves the viewport, clamped to the body height known from the last
// frame.
func (f *Form) SetScroll(n int) bool {
	if n < 0 {
		n = 0
	}
	if n == f.Scroll {
		return false
	}
	f.Scroll = n
	return true
}

// ScrollBy adjusts the viewport by dir lines.
func (f *Form) ScrollBy(dir int) bool { return f.SetScroll(f.Scroll + dir) }

// View renders the form into c.Rect, one line per row, and registers the hit
// targets that make mouse navigation work.
func (f *Form) View(c Ctx) string {
	t := c.T
	w := max(c.Rect.W, 20)
	h := max(c.Rect.H, 1)

	// Rows must be flattened before anything reads f.rows: the owner indexes
	// below refer to it.
	f.Rows()

	var lines []string
	var owners []int
	if f.Heading != "" {
		lines = append(lines, t.Components.Heading.Render(layout.Truncate(f.Heading, w)))
		owners = append(owners, -1)
		if f.Sub != "" {
			// The subtitle is prose and gets no second line: wrapping here
			// would shift every row below it against the pane's geometry.
			lines = append(lines, t.Components.Caption.Render(layout.Truncate(f.Sub, w)))
			owners = append(owners, -1)
		}
		lines = append(lines, "")
		owners = append(owners, -1)
	}

	// Focus is drawn with its real styling, so the focused row cannot be
	// patched up after the fact; every other row draws unstyled.
	firstRow := 0
	for i := range f.Sections {
		f.renderSection(t, w, i, &lines, &owners)
		firstRow += len(f.Sections[i].Rows)
	}

	f.lineOwners = owners
	f.bodyTop = c.Rect.Y
	f.bodyW = w

	// Keep the focused row on screen. The owner list, not the row list, is the
	// window: a row occupies exactly the lines between its owner marks.
	f.reveal(&lines, &owners, h)

	total := len(lines)
	if total > h {
		if f.Scroll > total-h {
			f.Scroll = total - h
		}
		lines = lines[f.Scroll : f.Scroll+h]
		owners = owners[f.Scroll : f.Scroll+h]
	}

	// The scroll area goes down first so the rows registered above it win a
	// click, while the wheel still finds a target anywhere in the pane.
	c.Register(Target{Kind: HitScrollArea, ID: f.ID}, layout.R(c.Rect.X, c.Rect.Y, w, h))

	out := make([]string, 0, len(lines))
	for i, line := range lines {
		y := c.Rect.Y + i
		owner := -1
		if i < len(owners) {
			owner = owners[i]
		}
		if owner >= 0 && owner < f.Total() {
			row := f.rows[owner]
			focused := owner == f.Focus
			redraw, valueX := row.View(t, w, focused)
			line = redraw
			c.Register(Target{Kind: HitRow, Index: owner, ID: f.ID}, layout.R(c.Rect.X, y, w, 1))
			if row.editable() {
				c.Register(Target{Kind: HitControl, Index: owner, ID: f.ID},
					layout.R(c.Rect.X+valueX, y, max(w-valueX, 1), 1))
			}
		}
		out = append(out, line)
	}
	for len(out) < h {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func (f *Form) renderSection(t *theme.Theme, w, i int, lines *[]string, owners *[]int) {
	firstRow := 0
	for j := 0; j < i; j++ {
		firstRow += len(f.Sections[j].Rows)
	}
	f.Sections[i].render(t, w, firstRow, lines, owners)
}

// reveal nudges Scroll until the focused row's first line is inside the pane of
// paneH lines.
func (f *Form) reveal(lines *[]string, owners *[]int, paneH int) {
	if f.Focus >= len(f.rows) {
		f.Focus = max(len(f.rows)-1, 0)
	}
	for i, o := range *owners {
		if o != f.Focus {
			continue
		}
		switch {
		case i < f.Scroll:
			f.Scroll = i
		case i >= f.Scroll+paneH:
			f.Scroll = i - paneH + 1
		}
		if f.Scroll < 0 {
			f.Scroll = 0
		}
		return
	}
}

// Inspector is the explanation strip for the focused row.
func (f *Form) Inspector(t *theme.Theme) string {
	r := f.Current()
	if r == nil {
		return t.Components.Caption.Render("no settings on this screen")
	}
	return r.Inspector(t)
}

// Progress renders the "row n of m" position marker for the footer, which is
// how a keyboard user knows where they are without a visible scrollbar.
func (f *Form) Progress(t *theme.Theme) string {
	n := f.Total()
	if n == 0 {
		return ""
	}
	return t.Components.Caption.Render(
		itoa(f.Focus+1) + "/" + itoa(n))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
