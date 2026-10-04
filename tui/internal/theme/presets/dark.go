package presets

import "github.com/HibernalGlow/ArcThumbX/tui/internal/theme"

// Dark is a neutral modern dark theme: the same token skeleton as Retro Future
// A with the period styling removed, which makes it the control that proves the
// token layer is doing its job — no component changes between the two.
func Dark() *theme.Theme {
	c := theme.Colors{
		Background:      theme.Hex("14161A"),
		Surface:         theme.Hex("1B1E24"),
		SurfaceElevated: theme.Hex("22262E"),
		SurfaceSunken:   theme.Hex("101215"),

		Foreground:      theme.Hex("E6E8EC"),
		ForegroundMuted: theme.Hex("9AA2AE"),

		Primary:   theme.Hex("6BAEDC"),
		Secondary: theme.Hex("8E9BA8"),
		Accent:    theme.Hex("6BAEDC"),

		Success: theme.Hex("7FC29A"),
		Warning: theme.Hex("E0B04A"),
		Error:   theme.Hex("E0736F"),
		Info:    theme.Hex("6BAEDC"),

		Border:      theme.Hex("2A2F38"),
		BorderMuted: theme.Hex("20242B"),
		Grid:        theme.Hex("242931"),

		Selection:     theme.Hex("262C36"),
		SelectionText: theme.Hex("F2F4F7"),
		Focus:         theme.Hex("6BAEDC"),
		Disabled:      theme.Hex("5A6169"),

		Overlay: theme.Hex("0E1013"),
	}

	return theme.Build(&theme.Theme{
		ID:     "dark",
		Name:   "Dark",
		Blurb:  "Modern neutral dark, no period styling",
		Scheme: theme.SchemeDark,
		Colors: c,
		Type: theme.Typeography{
			Title:    theme.TypeSpec{Foreground: c.Foreground, Bold: true},
			Heading:  theme.TypeSpec{Foreground: c.Foreground, Bold: true},
			Section:  theme.TypeSpec{Foreground: c.Accent, Bold: true, Uppercase: true},
			Body:     theme.TypeSpec{Foreground: c.Foreground},
			BodyStr:  theme.TypeSpec{Foreground: c.Foreground, Bold: true},
			Caption:  theme.TypeSpec{Foreground: c.ForegroundMuted},
			Help:     theme.TypeSpec{Foreground: c.ForegroundMuted},
			Value:    theme.TypeSpec{Foreground: c.Primary},
			Label:    theme.TypeSpec{Foreground: c.Foreground},
			Emphasis: theme.TypeSpec{Foreground: c.Primary, Bold: true},
		},
		Space:   theme.BaseSpacing(),
		Borders: hairlineBorders(c),
		Glyphs:  theme.UnicodeGlyphs(),
	})
}

// hairlineBorders is the shared five-step border scale used by the colour
// presets. Strong stays a hairline: weight in a terminal comes from colour and
// background, not from a thicker glyph.
func hairlineBorders(c theme.Colors) theme.Borders {
	return theme.Borders{
		None:   theme.NoEdge(),
		Subtle: theme.HairlineEdge(c.BorderMuted),
		Normal: theme.HairlineEdge(c.Border),
		Strong: theme.HairlineEdge(c.ForegroundMuted),
		Accent: theme.HairlineEdge(c.Accent),
	}
}
