package pages

import (
	"strconv"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/keymap"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// About is the reference screen: identity, the palette that identity is built
// from, and the binding table. It is the one page whose body is a List rather
// than a stack of settings, which is why it implements Page directly instead of
// riding on formPage.
type About struct {
	env   *Env
	list  *components.List
	lines []string
}

// NewAbout builds the reference screen.
func NewAbout(e *Env) Page {
	return &About{
		env:  e,
		list: &components.List{ID: "about:keys", Title: "Bindings", ValueWidth: 16},
	}
}

func (a *About) ID() string      { return "about" }
func (a *About) Label() string   { return "About" }
func (a *About) Heading() string { return "ARC/THUMBX" }
func (a *About) Sub() string     { return "terminal front end · phase one" }

// Render stacks the identity block, the palette preview and the scrollable
// binding list inside the pane it was given.
func (a *About) Render(c components.Ctx) string {
	t := c.T
	w := max(c.Rect.W, 20)

	identity := []string{
		t.Components.Title.Render("ARC"+t.Glyphs.Link+"THUMB") + " " +
			t.Components.NavBadge.Render("X"),
		t.Components.Caption.Render(a.env.Facts.Version + " · " +
			a.env.Facts.OS + "/" + a.env.Facts.Arch + " · " + a.env.Facts.GoVersion),
		t.Components.Caption.Render("Preset " + a.env.Facts.PresetName +
			" · measure " + strconv.Itoa(a.env.Prefs.ContentWidth) + " cols"),
		t.Components.Caption.Render(components.SwatchRow(t,
			[]string{"background", "surface", "surfaceElevated", "primary", "accent", "secondary", "success", "warning", "error", "border"})),
		t.Rule(w, t.Colors.BorderMuted),
	}

	// Every identity line is clipped to the pane: they are metadata, and a
	// wrapped one would push the bindings list out of the pane's geometry.
	for i := range identity {
		identity[i] = layout.Truncate(identity[i], w)
	}

	// The list gets whatever height is left, so a small terminal still shows the
	// identity block rather than scrolling it away.
	room := max(c.Rect.H-len(identity), 1)
	a.list.Items = keymap.Reference()
	bodyCtx := c
	bodyCtx.Rect = layout.R(c.Rect.X, c.Rect.Y+len(identity), w, room)

	out := append([]string{}, identity...)
	out = append(out, layout.Lines(a.list.View(bodyCtx))...)
	if len(out) > c.Rect.H {
		out = out[:c.Rect.H]
	}
	for len(out) < c.Rect.H {
		out = append(out, "")
	}
	return layout.JoinLines(out)
}

func (a *About) Act(act Action) bool {
	switch act {
	case ActionFocusNext:
		return a.list.Move(1)
	case ActionFocusPrev:
		return a.list.Move(-1)
	case ActionActivate:
		return false
	case ActionStepBack:
		return false
	case ActionStepFwd:
		return false
	case ActionScrollUp:
		return a.list.Scroll > 0 && func() bool { a.list.Scroll--; return true }()
	case ActionScrollDown:
		return a.list.Move(1)
	}
	return false
}

func (a *About) Mouse(t components.Target, press bool) bool {
	if t.ID != a.list.ID || t.Kind != components.HitListItem {
		return false
	}
	changed := a.list.Set(t.Index)
	if press && !changed {
		return true
	}
	return changed
}

func (a *About) Inspector(t *theme.Theme) string {
	cur := a.list.Current()
	if cur == nil {
		return t.Components.Caption.Render("about")
	}
	text := cur.Label + " — " + cur.Detail
	return t.Components.Description.Render(text)
}

func (a *About) Progress(t *theme.Theme) string {
	n := len(a.list.Items)
	if n == 0 {
		return ""
	}
	return t.Components.Caption.Render(strconv.Itoa(a.list.Cursor+1) + "/" + strconv.Itoa(n))
}
