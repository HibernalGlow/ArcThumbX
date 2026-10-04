package components

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
)

// NavItem is one entry in the navigation rail.
type NavItem struct {
	ID string
	// Label is drawn uppercased by the rail.
	Label string
	// Alert marks unsaved work on this page without stealing focus.
	Alert bool
}

// Sidebar is the navigation rail. It is a coloured band with a marker column,
// not a boxed menu: the frame already has one rule under the header, and a
// second box around the list would flatten the hierarchy it is supposed to
// establish.
type Sidebar struct {
	ID     string
	Items  []NavItem
	Active int
	// Width is the rail's cell count; 0 picks a default.
	Width int
}

// DefaultNavWidth is the rail width: wide enough for the longest page name
// without wrapping, narrow enough to leave the measure readable.
const DefaultNavWidth = 18

// View draws the rail into c.Rect and registers every entry as a click target.
func (s *Sidebar) View(c Ctx) string {
	t := c.T
	w := s.Width
	if w <= 0 {
		w = DefaultNavWidth
	}
	if len(s.Items) == 0 {
		return ""
	}
	// The rail's own caption, kept short on purpose: a letterspaced
	// "NAVIGATION" is 21 cells and would be cut mid-word inside an 18-cell rail.
	lines := []string{
		t.Components.NavHeader.Render(layout.Fit("NAV", w)),
		"",
	}
	for i, item := range s.Items {
		label := layout.Truncate(item.Label, w-4)
		var line string
		switch {
		case i == s.Active:
			marker := t.Components.Focus.Render(t.Glyphs.Focus)
			text := t.Components.NavItemActive.Render(layout.Fit(strings.ToUpper(label), w-2))
			line = marker + " " + strings.TrimRight(text, " ")
			// A selection band only makes sense when a background exists to
			// paint; the Terminal Default preset has none, so the marker and
			// the weight carry it there instead.
			if !t.Colors.Selection.TerminalDefault() {
				line = t.Components.NavItemHover.Render(layout.Fit(
					t.Glyphs.Focus+" "+strings.ToUpper(label), w))
			}
		default:
			band := "  "
			if item.Alert {
				band = t.Components.NavBadge.Render(t.Glyphs.On) + " "
			}
			line = band + t.Components.NavItem.Render(layout.Fit(label, w-2))
		}
		lines = append(lines, layout.Fit(line, w))
		c.Register(Target{Kind: HitNav, Index: i, ID: s.ID},
			layout.R(c.Rect.X, c.Rect.Y+len(lines)-1, w, 1))
	}
	for len(lines) < c.Rect.H {
		lines = append(lines, "")
	}
	return strings.Join(lines[:max(c.Rect.H, 0)], "\n")
}

// Tabs is a clickable strip used when the rail is folded away, and by pages that
// have sub-views of their own (Formats splits images from containers).
type Tabs struct {
	ID     string
	Labels []string
	Active int
}

// View draws the strip, underlining the active label so the state survives a
// preset that cannot paint a selection background.
func (tb *Tabs) View(c Ctx) string {
	t := c.T
	if len(tb.Labels) == 0 {
		return ""
	}
	x := c.Rect.X
	parts := make([]string, 0, len(tb.Labels))
	for i, label := range tb.Labels {
		cell := strings.ToUpper(label)
		if i == tb.Active {
			parts = append(parts, t.Components.TabActive.Render(layout.Fit(cell, len(cell)+2)))
		} else {
			parts = append(parts, t.Components.Tab.Render(layout.Fit(cell, len(cell)+2)))
		}
		w := len([]rune(cell)) + 2
		c.Register(Target{Kind: HitTab, Index: i, ID: tb.ID}, layout.R(x, c.Rect.Y, w, 1))
		x += w
		if i != len(tb.Labels)-1 {
			sep := " " + t.Components.Grid.Render(t.Glyphs.Divider) + " "
			parts = append(parts, sep)
			x += 3
		}
	}
	return layout.Fit(strings.Join(parts, ""), max(c.Rect.W, 1))
}

// ListItem is one row of a List.
type ListItem struct {
	Label string
	Value string
	Badge *StatusBadge
	// Detail is the inspector text for this item.
	Detail string
	// Tag carries an opaque payload — a preset ID, a path — so selecting an
	// item never depends on its label text.
	Tag string
}

// List is a read-mostly table: a label column, a right-aligned value column and
// an optional state badge. It is what the Dashboard module board and the About
// keymap reference are built from, and every line is a click target.
type List struct {
	ID     string
	Title  string
	Items  []ListItem
	Cursor int
	Scroll int
	// ValueWidth reserves the right column; 0 auto-sizes to the widest value.
	ValueWidth int
}

// Move steps the cursor, clamped.
func (l *List) Move(dir int) bool {
	if len(l.Items) == 0 || dir == 0 {
		return false
	}
	n := l.cursor() + dir
	if n < 0 {
		n = 0
	}
	if n >= len(l.Items) {
		n = len(l.Items) - 1
	}
	if n == l.Cursor {
		return false
	}
	l.Cursor = n
	return true
}

// Set moves the cursor to an absolute index.
func (l *List) Set(i int) bool {
	if i < 0 || i >= len(l.Items) || i == l.Cursor {
		return false
	}
	l.Cursor = i
	return true
}

func (l *List) cursor() int {
	if l.Cursor >= len(l.Items) {
		return max(len(l.Items)-1, 0)
	}
	if l.Cursor < 0 {
		return 0
	}
	return l.Cursor
}

// Current is the focused item, or nil.
func (l *List) Current() *ListItem {
	if len(l.Items) == 0 {
		return nil
	}
	return &l.Items[l.cursor()]
}

// View draws the list body.
func (l *List) View(c Ctx) string {
	t := c.T
	w := max(c.Rect.W, 10)
	vw := l.ValueWidth
	if vw == 0 {
		for _, it := range l.Items {
			if n := lipgloss.Width(it.Value) + badgeWidth(it.Badge); n > vw {
				vw = n
			}
		}
		vw = min(max(vw, 6), w/2)
	}
	var lines []string
	if l.Title != "" {
		lines = append(lines, t.Components.Section.Render(
			t.Glyphs.Section+" "+layout.Truncate(strings.ToUpper(l.Title), w-2)))
	}
	start := l.Scroll
	if start > len(l.Items) {
		start = 0
	}
	room := c.Rect.H - len(lines)
	if start > max(len(l.Items)-room, 0) {
		start = max(len(l.Items)-room, 0)
	}
	l.Scroll = start
	for i := start; i < len(l.Items) && len(lines) < c.Rect.H; i++ {
		it := l.Items[i]
		marker := t.Components.Body.Render("  ")
		label := t.Components.ListItem
		if i == l.cursor() {
			marker = t.Components.Focus.Render(t.Glyphs.Focus) + " "
			label = t.Components.ListItemActive
		}
		value := it.Value
		if it.Badge != nil {
			value = layout.FitRight(value, vw-badgeWidth(it.Badge)) + " " + it.Badge.View(t)
		} else {
			value = layout.FitRight(value, vw)
		}
		labelW := max(w-3-vw, 6)
		line := marker + label.Render(layout.Truncate(it.Label, labelW))
		line += strings.Repeat(" ", max(w-lipgloss.Width(line)-lipgloss.Width(value), 1))
		line += value
		lines = append(lines, line)
		c.Register(Target{Kind: HitListItem, Index: i, ID: l.ID},
			layout.R(c.Rect.X, c.Rect.Y+len(lines)-1, w, 1))
	}
	for len(lines) < c.Rect.H {
		lines = append(lines, "")
	}
	return strings.Join(lines[:max(c.Rect.H, 0)], "\n")
}

func badgeWidth(b *StatusBadge) int {
	if b == nil {
		return 0
	}
	return len([]rune(b.Label)) + 2
}

// Button is a push button. It appears in the action strip when a page has
// something to commit, and inside a dialog as an answer.
type Button struct {
	ID      string
	Label   string
	Primary bool
	// Enabled false draws the button as inert and makes it a no-op target,
	// which is how a write path that the backend refuses is shown instead of
	// silently dropped.
	Enabled bool
}

// ButtonBar lays buttons out left to right and registers each one.
type ButtonBar struct {
	ID      string
	Buttons []Button
	Hover   int
}

// View draws the bar into one line.
func (b *ButtonBar) View(c Ctx) string {
	t := c.T
	x := c.Rect.X
	var parts []string
	for i, btn := range b.Buttons {
		label := strings.ToUpper(btn.Label)
		cell := " " + label + " "
		var drawn string
		switch {
		case !btn.Enabled:
			// A write path the backend refuses is shown greyed rather than
			// hidden, so the user can tell "not available" from "not needed".
			drawn = t.Components.Muted.Render(cell)
		case btn.Primary:
			drawn = t.Components.ButtonPrimary.Render(cell)
		default:
			drawn = t.Components.Button.Render(cell)
		}
		if i == b.Hover && btn.Enabled {
			drawn = t.Components.ButtonHover.Render(cell)
		}
		parts = append(parts, drawn)
		w := len([]rune(cell))
		id := btn.ID
		if id == "" {
			id = label
		}
		c.Register(Target{Kind: HitButton, Index: i, ID: id}, layout.R(x, c.Rect.Y, w, 1))
		x += w + 1
		if i != len(b.Buttons)-1 {
			parts = append(parts, " ")
			x++
		}
	}
	return layout.Fit(strings.Join(parts, ""), max(c.Rect.W, 1))
}

// Inspector is a one-line readout strip above the help bar: the explanation for
type Inspector struct {
	Text string
	// Right is the position marker, e.g. "3/9".
	Right   string
	Badges  string
	Buttons *ButtonBar
}

// View draws the strip.
func (in *Inspector) View(c Ctx) string {
	t := c.T
	w := max(c.Rect.W, 10)
	left := in.Text
	if left == "" {
		left = t.Components.Muted.Render("—")
	} else {
		left = t.Components.Description.Render(left)
	}
	if in.Badges != "" {
		left += "  " + in.Badges
	}

	var right string
	switch {
	case in.Buttons != nil && len(in.Buttons.Buttons) > 0:
		right = in.Buttons.View(Ctx{
			T:    t,
			Rect: c.At(w-buttonStripWidth(in.Buttons), 0),
			Hits: c.Hits,
		})
	case in.Right != "":
		right = t.Components.Caption.Render(in.Right)
	}
	if right == "" {
		return layout.Fit(layout.Truncate(left, w), w)
	}
	rightW := lipgloss.Width(right)
	// The readout yields to the actions: an explanation that does not fit is
	// truncated, but the buttons must stay clickable.
	left = layout.Truncate(left, max(w-rightW-1, 1))
	pad := max(w-lipgloss.Width(left)-rightW, 1)
	return left + strings.Repeat(" ", pad) + right
}

func buttonStripWidth(b *ButtonBar) int {
	n := 0
	for _, btn := range b.Buttons {
		n += len([]rune(btn.Label)) + 2 + 1
	}
	return n
}
