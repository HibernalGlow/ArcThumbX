package theme

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Paint turns a colour token into a renderer colour.
//
// Inherit maps to lipgloss.NoColor{}, which emits no SGR at all: foreground
// falls back to the terminal's default text colour and a background is simply
// not drawn. That is how the Terminal Default preset works, and why no
// component ever has to ask whether the theme paints a ground.
func Paint(c Color) color.Color {
	if c == Inherit {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(string(c))
}

// Text couples a typographic role with the style compiled for it, so a
// component renders a string in one call and never has to know about case,
// letter-spacing or which colour token backs the role.
type Text struct {
	Spec  TypeSpec
	Style lipgloss.Style
}

// Render applies the role's case transform and then its style. A zero Text
// renders its input untouched, so an unset optional role cannot panic.
func (x Text) Render(s string) string {
	return x.Style.Render(x.Transform(s))
}

// RenderN renders every line of s with the role, so a multi-line block keeps one
// style owner instead of stitching strings at the call site.
func (x Text) RenderN(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = x.Render(l)
	}
	return strings.Join(lines, "\n")
}

// Transform applies case and letter-spacing without colour, for callers that
// need to measure before they paint.
func (x Text) Transform(s string) string { return applyType(x.Spec, s) }

// Extend lets a component add geometry to a role's style without redefining the
// role.
func (x Text) Extend(fn func(lipgloss.Style) lipgloss.Style) lipgloss.Style {
	return fn(x.Style)
}

func applyType(spec TypeSpec, s string) string {
	if spec.Uppercase {
		s = strings.ToUpper(s)
	}
	if spec.Spacing > 0 {
		s = letterspace(s, spec.Spacing)
	}
	return s
}

// letterspace pads between runes, and pads twice as hard around a space so word
// boundaries survive: a single space of gap is indistinguishable from the gap
// between letters, which turns an engraved legend into one unbroken run.
func letterspace(s string, n int) string {
	if n <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) < 2 {
		return s
	}
	pad := strings.Repeat(" ", n)
	word := strings.Repeat(" ", n*2+1)
	var b strings.Builder
	for i, r := range runes {
		if r == ' ' {
			b.WriteString(word)
			continue
		}
		if i > 0 && runes[i-1] != ' ' {
			b.WriteString(pad)
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " ")
}

// Components are the compiled per-component styles: every visual slot a
// component needs exists here, which is what keeps colour decisions out of
// rendering code.
type Components struct {
	// Containers. Dialog is the only component in the design system that is
	// allowed a full box, because a modal's job is to be unmistakably
	// separate.
	Root   lipgloss.Style
	Panel  lipgloss.Style
	Card   lipgloss.Style
	Dialog lipgloss.Style
	Scrim  lipgloss.Style

	// Header.
	Header     lipgloss.Style
	Brand      Text
	HeaderMeta Text

	// Navigation rail.
	Nav           lipgloss.Style
	NavHeader     Text
	NavItem       Text
	NavItemActive Text
	NavItemHover  lipgloss.Style
	NavBadge      lipgloss.Style

	// Tabs.
	TabBar    lipgloss.Style
	Tab       Text
	TabActive Text

	// Sections and rows. A row is composed from its parts rather than wrapped in
	// a style of its own: the line's width is already exact, and padding here
	// would widen it past the terminal.
	Section     Text
	SectionRule lipgloss.Style
	Label       Text
	LabelActive Text
	Value       Text
	ValueActive Text
	ValueFlag   Text
	Description Text

	// Controls. A toggle and a select carry their state in the glyph and the
	// value ink, so they need an on/off and an arrows slot but not a container.
	ToggleOn      lipgloss.Style
	ToggleOff     lipgloss.Style
	Select        Text
	SelectArrows  lipgloss.Style
	SliderTrack   lipgloss.Style
	SliderFill    lipgloss.Style
	Button        lipgloss.Style
	ButtonPrimary lipgloss.Style
	ButtonHover   lipgloss.Style
	Input         lipgloss.Style
	InputActive   lipgloss.Style

	// Lists. The list itself is a stack of rows, so only the row styles are
	// compiled; the cursor row is a different ink, not a different box.
	ListItem       Text
	ListItemActive Text

	// Status.
	StatusOn    Text
	StatusOff   Text
	StatusWarn  Text
	StatusError Text
	StatusInfo  Text
	Meter       lipgloss.Style
	Swatch      lipgloss.Style

	// Help bar.
	HelpBar    lipgloss.Style
	Inspector  lipgloss.Style
	HelpKey    Text
	HelpAction Text
	HelpDim    Text

	// Shared typographic roles for long-form pages. The rules and the grid are
	// here because the shell paints them; the rest of the roles a preset can
	// declare live in the typography table instead.
	Title   Text
	Heading Text
	Body    Text
	Caption Text
	Muted   Text
	Grid    lipgloss.Style
	Focus   lipgloss.Style
}

// Build compiles a token set into a ready-to-render theme. It mutates and
// returns t so a preset can be written as one literal, and it fills any token
// group a preset left zero so a partial preset still renders.
func Build(t *Theme) *Theme {
	if t.Space == (Spacing{}) {
		t.Space = BaseSpacing()
	}
	if t.Glyphs.Rule == "" {
		t.Glyphs = UnicodeGlyphs()
	}
	if t.Borders.Normal.Top == "" && t.Borders.Subtle.Top == "" {
		t.Borders = Borders{
			None:   NoEdge(),
			Subtle: HairlineEdge(t.Colors.BorderMuted),
			Normal: HairlineEdge(t.Colors.Border),
			Strong: HairlineEdge(t.Colors.ForegroundMuted),
			Accent: HairlineEdge(t.Colors.Accent),
		}
	}

	col := &t.Colors
	typ := &t.Type
	c := &t.Components

	c.Root = band(col.Background, col.Foreground)
	c.Panel = box(t, t.Borders.Subtle, col.Surface)
	c.Card = box(t, t.Borders.Normal, col.SurfaceElevated)
	// The modal's frame and the backdrop it floats on. The dialog keeps one line
	// of padding above its title because the shell counts that row when it maps a
	// click on an answer back to a cell.
	c.Dialog = box(t, t.Borders.Accent, t.dialogGround()).Padding(t.Space.SM, t.Space.MD)
	c.Scrim = lipgloss.NewStyle().Background(Paint(col.Overlay))

	c.Header = band(col.Surface, col.Foreground)
	c.Brand = text(typ.Title, col.Foreground)
	c.HeaderMeta = text(typ.Caption, col.ForegroundMuted)

	c.Nav = band(col.Surface, col.Foreground)
	c.NavHeader = text(typ.Section, col.ForegroundMuted)
	c.NavItem = text(typ.Body, col.Foreground)
	c.NavItemActive = text(merge(typ.BodyStr, TypeSpec{Foreground: col.SelectionText}), col.SelectionText)
	c.NavItemHover = band(col.Selection, col.SelectionText)
	c.NavBadge = ink(col.Accent)

	c.TabBar = band(col.Surface, col.Foreground)
	c.Tab = text(typ.Label, col.ForegroundMuted)
	c.TabActive = text(merge(typ.Label, TypeSpec{Bold: true, Underline: true}), col.Accent)

	c.Section = text(typ.Section, col.Accent)
	c.SectionRule = ink(col.BorderMuted)
	c.Label = text(typ.Label, col.Foreground)
	c.LabelActive = text(merge(typ.Label, TypeSpec{Bold: true}), col.SelectionText)
	c.Value = text(typ.Value, col.Primary)
	c.ValueActive = text(merge(typ.Value, TypeSpec{Bold: true}), col.SelectionText)
	c.ValueFlag = text(typ.Value, col.ForegroundMuted)
	c.Description = text(typ.Caption, col.ForegroundMuted)

	c.ToggleOn = ink(col.Success)
	c.ToggleOff = ink(col.Disabled)
	// The selector's value starts out identical to a plain value; it is a
	// separate slot so a preset can make the one control that changes meaning on
	// click read differently.
	// The option a click will change is tier-1 information, so it carries the
	// emphasis role rather than the plain value weight.
	c.Select = text(merge(typ.Emphasis, TypeSpec{Foreground: col.Primary}), col.Primary)
	c.SelectArrows = ink(col.ForegroundMuted)
	c.SliderTrack = ink(col.Disabled)
	c.SliderFill = ink(col.Primary)
	// Buttons are painted grounds, not boxes. They share a line with other
	// controls, and a four-sided border on a one-cell-tall string does not
	// render as a button — it renders as rules through the label. The dialog is
	// the only component in this system that gets a frame.
	c.Button = band(col.SurfaceElevated, col.Foreground)
	c.ButtonPrimary = band(col.Primary, col.Background)
	c.ButtonHover = band(col.Selection, col.SelectionText)
	c.Input = box(t, t.Borders.Subtle, col.SurfaceSunken)
	c.InputActive = box(t, t.Borders.Strong, col.SurfaceSunken)

	c.ListItem = text(typ.Body, col.Foreground)
	c.ListItemActive = text(merge(typ.BodyStr, TypeSpec{Foreground: col.SelectionText}), col.SelectionText)

	c.StatusOn = text(typ.Value, col.Success)
	c.StatusOff = text(typ.Value, col.Disabled)
	c.StatusWarn = text(typ.Value, col.Warning)
	c.StatusError = text(typ.Value, col.Error)
	c.StatusInfo = text(typ.Value, col.Info)
	c.Meter = ink(col.Primary)
	c.Swatch = lipgloss.NewStyle()

	c.HelpBar = band(col.Surface, col.ForegroundMuted)
	c.Inspector = band(col.Surface, col.ForegroundMuted)
	c.HelpKey = text(typ.Help, col.Accent)
	c.HelpAction = text(typ.Help, col.ForegroundMuted)
	c.HelpDim = text(typ.Help, col.Disabled)

	c.Title = text(typ.Title, col.Foreground)
	c.Heading = text(typ.Heading, col.Foreground)
	c.Body = text(typ.Body, col.Foreground)
	c.Caption = text(typ.Caption, col.ForegroundMuted)
	c.Muted = text(typ.Caption, col.Disabled)
	c.Grid = ink(col.Grid)
	c.Focus = ink(col.Focus)

	t.Components = *c
	return t
}

// Rule returns a hairline n cells wide in the given colour token.
func (t *Theme) Rule(n int, edge Color) string {
	if n < 0 {
		n = 0
	}
	s := strings.Repeat(t.Glyphs.Rule, n)
	return lipgloss.NewStyle().Foreground(Paint(edge)).Render(s)
}

// FrameRule is the shell's full-width divider. A component asks for a rule
// instead of picking a colour for it, which is the difference between a theme
// and a palette hardcoded in the renderer.
func (t *Theme) FrameRule(n int) string {
	return t.Rule(n, t.Colors.Border)
}

// GridColumn renders a vertical divider n cells tall in the grid colour. It is
// the only pane separator the shell draws, and it is deliberately fainter than
// a border token.
func (t *Theme) GridColumn(n int) string {
	if n < 1 {
		n = 1
	}
	line := lipgloss.NewStyle().Foreground(Paint(t.Colors.Grid)).Render(t.Glyphs.Grid)
	return strings.Repeat(line+"\n", n)
}

// PaintBackground exposes the token conversion to components that build their
// own styles, such as the modal box.
func (t *Theme) PaintBackground(c Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(Paint(c))
}

func (t *Theme) overlayGround() Color {
	if t.Colors.Overlay != Inherit {
		return t.Colors.Overlay
	}
	return t.Colors.SurfaceElevated
}

// dialogGround is the modal's interior. It is the surface token rather than the
// overlay one because the overlay is what the scrim is made of: painting the box
// the same colour as its own backdrop leaves only a line between them. A
// terminal-default preset inherits instead, so nothing is emitted at all.
func (t *Theme) dialogGround() Color {
	if t.Colors.Overlay.TerminalDefault() {
		return Inherit
	}
	return t.Colors.Surface
}

// Border converts an edge token into the renderer's border shape. An empty edge
// yields an all-blank border, which the renderer draws as nothing — that is how
// "none" is expressed without a conditional at any call site.
func (e Edge) Border() lipgloss.Border {
	return lipgloss.Border{
		Top: e.Top, Bottom: e.Bottom, Left: e.Left, Right: e.Right,
		TopLeft: e.TopLeft, TopRight: e.TopRight,
		BottomLeft: e.BottomLeft, BottomRight: e.BottomRight,
	}
}

// band is a full-bleed strip: colour, no border. Most of the panel's structure
// is made this way rather than with boxes.
func band(bg, fg Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Paint(fg)).Background(Paint(bg))
}

// ink styles foreground only, for rules and glyph runs.
func ink(fg Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Paint(fg))
}

// box is the one place a border is drawn.
func box(t *Theme, edge Edge, bg Color) lipgloss.Style {
	s := lipgloss.NewStyle().
		Background(Paint(bg)).
		BorderForeground(Paint(edge.Foreground)).
		BorderBackground(Paint(edge.Background))
	if edge.Top == "" && edge.Bottom == "" && edge.Left == "" && edge.Right == "" {
		return s.Padding(t.Space.SM, t.Space.PadX)
	}
	return s.Border(edge.Border(), true).Padding(t.Space.XS, t.Space.PadX)
}

// text compiles a typographic role. A zero foreground on the spec means the
// role inherits the caller's fallback token, which is what lets a preset declare
// only the roles it cares about.
func text(spec TypeSpec, fallback Color) Text {
	fg := spec.Foreground
	if fg == Inherit {
		fg = fallback
	}
	s := lipgloss.NewStyle().
		Foreground(Paint(fg)).
		Background(Paint(spec.Background)).
		Bold(spec.Bold).
		Italic(spec.Italic).
		Faint(spec.Faint).
		Underline(spec.Underline).
		Strikethrough(spec.Strikethrough)
	return Text{Spec: spec, Style: s}
}

// merge overlays a variant onto a base role so variants (active, bold body) stay
// declarative instead of becoming new tokens.
func merge(base TypeSpec, over TypeSpec) TypeSpec {
	if over.Foreground != Inherit {
		base.Foreground = over.Foreground
	}
	if over.Background != Inherit {
		base.Background = over.Background
	}
	base.Bold = base.Bold || over.Bold
	base.Italic = base.Italic || over.Italic
	base.Faint = base.Faint || over.Faint
	base.Underline = base.Underline || over.Underline
	base.Strikethrough = base.Strikethrough || over.Strikethrough
	base.Uppercase = base.Uppercase || over.Uppercase
	if over.Spacing > base.Spacing {
		base.Spacing = over.Spacing
	}
	return base
}
