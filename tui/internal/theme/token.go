// Package theme holds the ArcThumbX TUI design tokens and compiles a
// declarative preset into ready-to-use styles.
//
// Components never hard-code a colour, border, glyph or spacing value: they
// read a compiled *Theme. Swapping presets therefore touches no component.
// Adding a preset is one file of pure data (see internal/theme/presets); the
// registry is a map lookup, not a chain of conditionals.
package theme

// PresetID is the key a preset is registered, selected and persisted under.
type PresetID string

// Scheme is what a preset assumes about the terminal's own background. It
// lets the app pick a legible fallback and decide whether painting a
// background is safe.
type Scheme int

const (
	// SchemeTerminal emits no background at all and inherits whatever the
	// user's terminal draws. Highest compatibility: assumes nothing about
	// the palette.
	SchemeTerminal Scheme = iota
	// SchemeDark paints a dark ground and expects light ink.
	SchemeDark
	// SchemeLight paints a light ground and expects dark ink.
	SchemeLight
)

func (s Scheme) String() string {
	switch s {
	case SchemeTerminal:
		return "terminal"
	case SchemeDark:
		return "dark"
	case SchemeLight:
		return "light"
	default:
		return "unknown"
	}
}

// Color is a token colour in #rgb or #rrggbb form; Inherit means "emit no SGR
// for this slot". Tokens stay strings rather than colour values so a preset
// never imports a renderer, and so the terminal layer remains free to
// downgrade truecolour to 256/16 ANSI for low-colour terminals.
type Color string

// Inherit leaves a colour slot to the terminal default. The Terminal Default
// preset fills every background slot with it.
const Inherit Color = ""

// Hex normalises a bare or '#' prefixed hex triple into token form.
func Hex(s string) Color {
	if s == "" {
		return Inherit
	}
	if s[0] != '#' {
		return Color("#" + s)
	}
	return Color(s)
}

// TerminalDefault reports whether the slot inherits from the terminal.
func (c Color) TerminalDefault() bool { return c == Inherit }

// Colors is the colour token set. The first seventeen are the shared
// vocabulary every component speaks; the rest exist because a terminal panel
// needs a dialog ground, a selection ink and a grid line that is fainter than
// a border.
type Colors struct {
	Background      Color
	Surface         Color
	SurfaceElevated Color
	SurfaceSunken   Color

	Foreground      Color
	ForegroundMuted Color

	Primary   Color
	Secondary Color
	Accent    Color

	Success Color
	Warning Color
	Error   Color
	Info    Color

	Border      Color
	BorderMuted Color
	Grid        Color

	Selection     Color
	SelectionText Color
	Focus         Color
	Disabled      Color

	// Overlay is the ground under a modal dialog. It must separate from
	// Surface without relying on a border.
	Overlay Color
}

// TypeSpec describes a typographic role. Terminals have no font sizes, so the
// hierarchy is carried by weight, case, letter-spacing and faintness.
type TypeSpec struct {
	Foreground Color
	Background Color

	Bold          bool
	Italic        bool
	Faint         bool
	Underline     bool
	Strikethrough bool

	// Uppercase and Spacing carry the section/instrument feel: a spaced
	// uppercase label reads as an engraved panel legend without any box.
	Uppercase bool
	Spacing   int
}

// Typeography is the type token set: nine roles, three information tiers.
type Typeography struct {
	Title    TypeSpec // tier 1 — product/brand line
	Heading  TypeSpec // tier 1 — page title
	Section  TypeSpec // tier 2 — section legend
	Body     TypeSpec // tier 2 — prose
	BodyStr  TypeSpec // tier 2 — emphasised prose
	Caption  TypeSpec // tier 3 — metadata
	Help     TypeSpec // tier 3 — key hints
	Value    TypeSpec // tier 2 — the current value of a setting
	Label    TypeSpec // tier 2 — the name of a setting
	Emphasis TypeSpec // accent inline (selected item, focus text)
}

// Spacing is the spacing scale in terminal cells.
type Spacing struct {
	XS int
	SM int
	MD int
	LG int
	XL int

	// Indent is one nesting level inside a section.
	Indent int
	// Gutter separates the label and value columns of a setting row.
	Gutter int
	// PadX is horizontal padding inside a panel or dialog.
	PadX int
}

// Edge is a border token: the glyphs to draw plus their colours. An all-empty
// Edge draws nothing, which is how "none" is expressed without a conditional.
type Edge struct {
	Top, Right, Bottom, Left                   string
	TopLeft, TopRight, BottomLeft, BottomRight string

	Foreground Color
	Background Color
}

// Borders is the border-weight scale. ArcThumbX draws rules, not boxes: most
// structure comes from Subtle and Normal, while Strong and Accent are reserved
// for the few elements that must read as elevated.
type Borders struct {
	None   Edge
	Subtle Edge
	Normal Edge
	Strong Edge
	Accent Edge
}

// Theme is a compiled design token set. Build fills Components from the
// declared tokens; presets only ever declare tokens.
type Theme struct {
	ID     PresetID
	Name   string
	Blurb  string
	Scheme Scheme

	Colors  Colors
	Type    Typeography
	Space   Spacing
	Borders Borders
	Glyphs  Glyphs

	Components Components
}

// BaseSpacing is the shared rhythm; presets that want a roomier or tighter
// panel override the fields they care about.
func BaseSpacing() Spacing {
	return Spacing{XS: 0, SM: 1, MD: 2, LG: 3, XL: 4, Indent: 2, Gutter: 3, PadX: 1}
}

// NoEdge draws nothing.
func NoEdge() Edge { return Edge{} }

// HairlineEdge draws a single-cell rule set, used by most dark presets.
func HairlineEdge(fg Color) Edge {
	return Edge{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "─", TopRight: "─", BottomLeft: "─", BottomRight: "─",
		Foreground: fg,
	}
}

// LightEdge draws ASCII frame characters, for terminals and presets that must
// not rely on Unicode box glyphs.
func LightEdge(fg Color) Edge {
	return Edge{
		Top: "-", Bottom: "-", Left: "|", Right: "|",
		TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
		Foreground: fg,
	}
}
