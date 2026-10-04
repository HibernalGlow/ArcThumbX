package presets

import "github.com/HibernalGlow/ArcThumbX/tui/internal/theme"

// TerminalDefault draws no ground at all.
//
// Every structural colour slot is Inherit, which compiles to no SGR: the
// terminal's own default text and background show through, so the layout looks
// native on a green-phosphor vt100 emulator, a solarised prompt or a white
// xterm. Hierarchy survives on weight, case and underline alone; only the four
// semantic states borrow an ANSI index, because a status lamp that cannot be
// distinguished from its label is not conveying anything.
//
// This is the preset to reach for when a theme looks wrong somewhere: if it
// still reads here, the layout is at fault rather than the palette.
func TerminalDefault() *theme.Theme {
	c := theme.Colors{
		Background:      theme.Inherit,
		Surface:         theme.Inherit,
		SurfaceElevated: theme.Inherit,
		SurfaceSunken:   theme.Inherit,

		Foreground:      theme.Inherit,
		ForegroundMuted: theme.Inherit,

		Primary:   theme.Inherit,
		Secondary: theme.Inherit,
		Accent:    theme.Inherit,

		Success: theme.Hex("10"),
		Warning: theme.Hex("11"),
		Error:   theme.Hex("12"),
		Info:    theme.Inherit,

		Border:      theme.Inherit,
		BorderMuted: theme.Inherit,
		Grid:        theme.Inherit,

		Selection:     theme.Inherit,
		SelectionText: theme.Inherit,
		Focus:         theme.Inherit,
		Disabled:      theme.Inherit,

		Overlay: theme.Inherit,
	}

	return theme.Build(&theme.Theme{
		ID:     "terminal_default",
		Name:   "Terminal Default",
		Blurb:  "Inherits the terminal's own colours; assumes nothing",
		Scheme: theme.SchemeTerminal,
		Colors: c,
		Type: theme.Typeography{
			Title:    theme.TypeSpec{Bold: true, Uppercase: true},
			Heading:  theme.TypeSpec{Bold: true, Underline: true},
			Section:  theme.TypeSpec{Bold: true, Uppercase: true},
			Body:     theme.TypeSpec{},
			BodyStr:  theme.TypeSpec{Bold: true},
			Caption:  theme.TypeSpec{Faint: true},
			Help:     theme.TypeSpec{Faint: true},
			Value:    theme.TypeSpec{Bold: true},
			Label:    theme.TypeSpec{},
			Emphasis: theme.TypeSpec{Bold: true, Underline: true},
		},
		// No backgrounds means selection has to be visible, so the focused row
		// is marked by reverse video through the border/focus token instead.
		Space: theme.BaseSpacing(),
		Borders: theme.Borders{
			None:   theme.NoEdge(),
			Subtle: theme.LightEdge(theme.Inherit),
			Normal: theme.LightEdge(theme.Inherit),
			Strong: theme.LightEdge(theme.Inherit),
			Accent: theme.LightEdge(theme.Inherit),
		},
		Glyphs: theme.UnicodeGlyphs(),
	})
}
