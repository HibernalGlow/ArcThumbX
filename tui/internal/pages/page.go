// Package pages holds the screens of the ArcThumbX TUI.
//
// A page composes components and owns its own focus and scroll state. It never
// draws a rule, a border or a colour: those decisions live in the theme, and the
// plumbing that turns a key into a movement lives in components.Form.
package pages

import (
	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/tuicfg"
)

// Intent is a command a page asks the app to run. Pages never write files or
// open dialogs themselves, which is what keeps one save path and one quit guard
// instead of six copies.
type Intent int

const (
	// IntentSave writes the pending ArcThumbX settings through the store.
	IntentSave Intent = iota
	// IntentRevert discards them.
	IntentRevert
	// IntentThemes opens the preset picker.
	IntentThemes
	// IntentPersist saves the front-end preferences immediately.
	IntentPersist
)

// Facts are the environment disclosures a page may show. Every field is
// something the TUI genuinely observed; absent information stays absent rather
// than being filled in with a plausible number.
type Facts struct {
	StoreKind      string
	StoreLocation  string
	StoreReadable  bool
	StoreWritable  bool
	BackendError   string
	IgnoredKeys    []string
	MalformedLines []string

	PrefsLocation string

	PresetID   theme.PresetID
	PresetName string
	Profile    string
	Version    string
	OS         string
	Arch       string
	GoVersion  string

	TermCols int
	TermRows int

	MouseOn     bool
	Regenerable bool
}

// Env is the whole world a page is allowed to touch: the live configuration it
// may edit, the preferences it may change, the facts it may disclose, and the
// narrow set of commands it may ask for.
type Env struct {
	// Settings is the working copy. Edits land here and are dirty until saved.
	Settings *arcthumb.Settings
	// Prefs is the front-end's own configuration, applied immediately.
	Prefs *tuicfg.Settings
	// Live is the last persisted snapshot, so dirty can be derived rather than
	// tracked by hand.
	Live arcthumb.Settings

	Facts Facts
	App   Host
}

// Host is the slice of the application a page needs.
type Host interface {
	Theme() *theme.Theme
	Request(i Intent)
	Dirty() bool
}

// Page is one screen of the panel.
type Page interface {
	// ID is stable and persisted as the last-visited page.
	ID() string
	// Label is the navigation rail text.
	Label() string
	// Heading and Sub are the page's tier-1 lines.
	Heading() string
	Sub() string
	// Render draws the body and registers its hit targets.
	Render(c components.Ctx) string
	// Act routes a focus or edit intent; it reports whether it consumed one.
	Act(a Action) bool
	// Hit routes a resolved mouse target; press distinguishes a click from a
	// hover.
	Mouse(t components.Target, press bool) bool
	// Inspector is the third-tier readout for the current focus.
	Inspector(t *theme.Theme) string
	// Progress is the position marker, e.g. "3/9".
	Progress(t *theme.Theme) string
}

// Action is the subset of keymap intents a page handles. It is redeclared here
// so that pages does not depend on the transport layer.
type Action int

const (
	ActionFocusNext Action = iota
	ActionFocusPrev
	ActionStepBack
	ActionStepFwd
	ActionActivate
	ActionScrollUp
	ActionScrollDown
)

// formPage is the common case: a page that is a stack of sections of setting
// rows. Pages that need a different body (the board on Dashboard, the split on
// Formats, the reference on About) embed it and override Render.
type formPage struct {
	env   *Env
	id    string
	label string
	sub   string
	form  *components.Form
	build func(e *Env) []components.Section
}

func newFormPage(e *Env, id, label, heading, sub string, build func(*Env) []components.Section) *formPage {
	return &formPage{
		env:   e,
		id:    id,
		label: label,
		sub:   sub,
		form:  &components.Form{ID: id, Heading: heading, Sub: sub},
		build: build,
	}
}

func (p *formPage) ID() string      { return p.id }
func (p *formPage) Label() string   { return p.label }
func (p *formPage) Heading() string { return p.form.Heading }
func (p *formPage) Sub() string     { return p.sub }

// sync rebuilds the rows every frame. Because every control reads and writes
// through an accessor closure, rebuilding costs nothing and a page can never
// render a stale value.
func (p *formPage) sync() {
	p.form.Sections = p.build(p.env)
}

func (p *formPage) Render(c components.Ctx) string {
	p.sync()
	return p.form.View(c)
}

func (p *formPage) Act(a Action) bool {
	p.sync()
	switch a {
	case ActionFocusNext:
		return p.form.Move(1)
	case ActionFocusPrev:
		return p.form.Move(-1)
	case ActionActivate:
		return p.form.Commit()
	case ActionStepBack:
		return p.form.Step(-1)
	case ActionStepFwd:
		return p.form.Step(1)
	case ActionScrollUp:
		return p.form.ScrollBy(-3)
	case ActionScrollDown:
		return p.form.ScrollBy(3)
	}
	return false
}

func (p *formPage) Mouse(t components.Target, press bool) bool {
	return routeFormHit(p.form, t, press)
}

func (p *formPage) Inspector(t *theme.Theme) string {
	// The shell asks for the readout before it draws the body, so on the very
	// first frame the form has no rows yet and would report "no settings".
	p.sync()
	return p.form.Inspector(t)
}

func (p *formPage) Progress(t *theme.Theme) string { return p.form.Progress(t) }
