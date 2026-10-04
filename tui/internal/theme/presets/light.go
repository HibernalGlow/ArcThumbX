package presets

import "github.com/HibernalGlow/ArcThumbX/tui/internal/theme"

// Light inverts the ground for bright rooms, projected demos and `script`
// output that ends up on paper. Accents are darkened relative to the dark
// presets because a saturated hue on white reads as a highlighter mark.
func Light() *theme.Theme {
	c := theme.Colors{
		Background:      theme.Hex("F4F6F8"),
		Surface:         theme.Hex("ECEFF3"),
		SurfaceElevated: theme.Hex("FFFFFF"),
		SurfaceSunken:   theme.Hex("E3E7ED"),

		Foreground:      theme.Hex("1A1E24"),
		ForegroundMuted: theme.Hex("5B6472"),

		Primary:   theme.Hex("1D6A8C"),
		Secondary: theme.Hex("3A7CA5"),
		Accent:    theme.Hex("8A6410"),

		Success: theme.Hex("2C7A4B"),
		Warning: theme.Hex("8A6410"),
		Error:   theme.Hex("A5322E"),
		Info:    theme.Hex("2A5D8A"),

		Border:      theme.Hex("C6CDD8"),
		BorderMuted: theme.Hex("DCE1E8"),
		Grid:        theme.Hex("D3D9E2"),

		Selection:     theme.Hex("1A1E24"),
		SelectionText: theme.Hex("F4F6F8"),
		Focus:         theme.Hex("8A6410"),
		Disabled:      theme.Hex("9BA3AE"),

		Overlay: theme.Hex("FFFFFF"),
	}

	return theme.Build(&theme.Theme{
		ID:     "light",
		Name:   "Light",
		Blurb:  "Light ground for bright rooms and printed output",
		Scheme: theme.SchemeLight,
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
