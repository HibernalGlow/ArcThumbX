package pages

import (
	"strconv"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// Formats is the extension bitmask, presented as lamps rather than as a number.
// The core stores one bit per extension in a fixed, append-only order, so this
// page must never reorder its table — a shuffled list would silently reinterpret
// every saved mask.
type Formats struct {
	env      *Env
	tab      int
	form     *components.Form
	sections func(e *Env, tab int) []components.Section
}

// NewFormats builds the split images/containers screen.
func NewFormats(e *Env) Page {
	return &Formats{
		env: e,
		form: &components.Form{
			ID:      "formats",
			Heading: "FORMAT ELIGIBILITY",
			Sub:     "which extensions may be decoded or opened at all",
		},
		sections: formatSections,
	}
}

func (f *Formats) ID() string      { return "formats" }
func (f *Formats) Label() string   { return "Formats" }
func (f *Formats) Heading() string { return f.form.Heading }
func (f *Formats) Sub() string     { return f.form.Sub }

// TabLabels are the two halves of the mask.
func (f *Formats) TabLabels() []string {
	return []string{"Images", "Containers"}
}

// Render draws the tab strip in the pane's first line and the row list beneath.
func (f *Formats) Render(c components.Ctx) string {
	tabs := &components.Tabs{
		ID:     "formats:tabs",
		Labels: f.TabLabels(),
		Active: f.tab,
	}
	strip := tabs.View(c)

	f.form.Sections = f.sections(f.env, f.tab)
	f.form.Heading = ""
	f.form.Sub = ""
	body := c
	body.Rect = layout.R(c.Rect.X, c.Rect.Y+1, max(c.Rect.W, 20), max(c.Rect.H-1, 1))

	out := append([]string{strip}, layout.Lines(f.form.View(body))...)
	if len(out) > c.Rect.H {
		out = out[:c.Rect.H]
	}
	for len(out) < c.Rect.H {
		out = append(out, "")
	}
	return layout.JoinLines(out)
}

func (f *Formats) Act(a Action) bool {
	f.form.Sections = f.sections(f.env, f.tab)
	switch a {
	case ActionFocusNext:
		return f.form.Move(1)
	case ActionFocusPrev:
		return f.form.Move(-1)
	case ActionActivate:
		return f.form.Commit()
	case ActionStepBack:
		return f.form.Step(-1)
	case ActionStepFwd:
		return f.form.Step(1)
	case ActionScrollUp:
		return f.form.ScrollBy(-3)
	case ActionScrollDown:
		return f.form.ScrollBy(3)
	}
	return false
}

// Mouse routes a tab click to the strip and everything else to the form.
func (f *Formats) Mouse(t components.Target, press bool) bool {
	if t.ID == "formats:tabs" && t.Kind == components.HitTab {
		if !press || t.Index == f.tab {
			return t.ID != ""
		}
		f.tab = t.Index
		f.form.Focus = 0
		f.form.Scroll = 0
		return true
	}
	return routeFormHit(f.form, t, press)
}

func (f *Formats) Inspector(t *theme.Theme) string {
	if cur := f.form.Current(); cur != nil {
		n := len(arcthumb.SupportedImageExts())
		if f.tab == 1 {
			n = len(arcthumb.SupportedArchiveExts())
		}
		return cur.Inspector(t) + "  " + t.Components.Muted.Render(
			f.TabLabels()[f.tab]+" · bit order is append-only · "+strconv.Itoa(n)+" entries")
	}
	return t.Components.Caption.Render("formats")
}

func (f *Formats) Progress(t *theme.Theme) string { return f.form.Progress(t) }

// routeFormHit is the shared click routing for a form: a row focuses, a value
// cell focuses and edits in the same gesture.
func routeFormHit(form *components.Form, t components.Target, press bool) bool {
	if t.ID != form.ID {
		return false
	}
	switch t.Kind {
	case components.HitScrollArea:
		return false
	case components.HitRow:
		return form.FocusRow(t.Index)
	case components.HitControl:
		changed := form.FocusRow(t.Index)
		if press {
			if r := form.Current(); r != nil && r.Control != nil {
				r.Control.Commit()
			}
			changed = true
		}
		return changed
	}
	return false
}

func formatSections(e *Env, tab int) []components.Section {
	if tab == 1 {
		return containerSections(e)
	}
	return imageSections(e)
}

func imageSections(e *Env) []components.Section {
	table := arcthumb.SupportedImageExts()
	rows := make([]components.Row, 0, len(table))
	for i, ext := range table {
		rows = append(rows, components.Row{
			Label: ext,
			Help:  "Eligible as a cover source inside a container.",
			Flag:  liveFlag(e),
			Control: components.NewToggle(
				func() bool { return arcthumb.ExtEnabled(e.Settings.EnabledImageExts, i) },
				func(v bool) { e.Settings.EnabledImageExts = arcthumb.ExtSet(e.Settings.EnabledImageExts, i, v) },
			),
		})
	}
	return []components.Section{{
		Title: "Decodable image extensions",
		Note:  "bit 0…" + strconv.Itoa(len(table)-1),
		Rows:  rows,
	}}
}

func containerSections(e *Env) []components.Section {
	table := arcthumb.SupportedArchiveExts()
	rows := make([]components.Row, 0, len(table))
	for i, ext := range table {
		rows = append(rows, components.Row{
			Label: "." + ext,
			Help:  "Container that may be opened for a cover image.",
			Flag:  liveFlag(e),
			Control: components.NewToggle(
				func() bool { return arcthumb.ExtEnabled(e.Settings.EnabledArchiveExts, i) },
				func(v bool) { e.Settings.EnabledArchiveExts = arcthumb.ExtSet(e.Settings.EnabledArchiveExts, i, v) },
			),
		})
	}
	return []components.Section{{
		Title: "Container extensions",
		Note:  "comic, book, tar",
		Rows:  rows,
	}}
}
