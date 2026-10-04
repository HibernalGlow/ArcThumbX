package presets

import "github.com/HibernalGlow/ArcThumbX/tui/internal/theme"

// HighContrast is for terminals that only have the sixteen ANSI slots, for
// projected demos where a hairline disappears, and for users who need the
// hierarchy to survive being read in greyscale.
//
// It therefore changes three things at once: colours collapse to the four
// semantic ANSI indices that every palette defines, glyphs drop to ASCII, and
// structure is carried by bold, underline and reverse video instead of by hue.
func HighContrast() *theme.Theme {
	// ANSI-16 indices, not hex: the terminal's own palette decides the actual
	// hue, so this preset cannot clash with it. 11 is a bright yellow in every
	// scheme, 10 a bright green, 12 a bright red, 14 a bright cyan.
	c := theme.Colors{
		Background:      theme.Hex("0"),
		Surface:         theme.Hex("0"),
		SurfaceElevated: theme.Hex("0"),
		SurfaceSunken:   theme.Hex("0"),

		Foreground:      theme.Hex("15"),
		ForegroundMuted: theme.Hex("7"),

		Primary:   theme.Hex("14"),
		Secondary: theme.Hex("12"),
		Accent:    theme.Hex("11"),

		Success: theme.Hex("10"),
		Warning: theme.Hex("11"),
		Error:   theme.Hex("12"),
		Info:    theme.Hex("14"),

		Border:      theme.Hex("7"),
		BorderMuted: theme.Hex("8"),
		Grid:        theme.Hex("8"),

		Selection:     theme.Hex("15"),
		SelectionText: theme.Hex("0"),
		Focus:         theme.Hex("15"),
		Disabled:      theme.Hex("8"),

		Overlay: theme.Hex("0"),
	}

	return theme.Build(&theme.Theme{
		ID:     "high_contrast",
		Name:   "High Contrast",
		Blurb:  "ANSI-only, ASCII glyphs, weight instead of hue",
		Scheme: theme.SchemeDark,
		Colors: c,
		Type: theme.Typeography{
			Title:    theme.TypeSpec{Foreground: c.Foreground, Bold: true, Uppercase: true},
			Heading:  theme.TypeSpec{Foreground: c.Foreground, Bold: true, Uppercase: true},
			Section:  theme.TypeSpec{Foreground: c.Accent, Bold: true, Uppercase: true, Spacing: 1},
			Body:     theme.TypeSpec{Foreground: c.Foreground},
			BodyStr:  theme.TypeSpec{Foreground: c.Foreground, Bold: true, Underline: true},
			Caption:  theme.TypeSpec{Foreground: c.ForegroundMuted},
			Help:     theme.TypeSpec{Foreground: c.ForegroundMuted, Uppercase: true},
			Value:    theme.TypeSpec{Foreground: c.Primary, Bold: true},
			Label:    theme.TypeSpec{Foreground: c.Foreground},
			Emphasis: theme.TypeSpec{Foreground: c.Foreground, Bold: true, Underline: true},
		},
		// Roomier spacing: reverse video on a tight line reads as a smear.
		Space: theme.Spacing{XS: 0, SM: 1, MD: 2, LG: 3, XL: 4, Indent: 2, Gutter: 4, PadX: 1},
		Borders: theme.Borders{
			None:   theme.NoEdge(),
			Subtle: theme.LightEdge(c.BorderMuted),
			Normal: theme.LightEdge(c.Border),
			Strong: theme.LightEdge(c.Foreground),
			Accent: theme.LightEdge(c.Accent),
		},
		Glyphs: theme.ASCIIGlyphs(),
	})
}
