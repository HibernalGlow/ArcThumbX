package presets

import "github.com/HibernalGlow/ArcThumbX/tui/internal/theme"

// RetroFutureA is ArcThumbX's visual identity.
//
// Every ground, ink and rule colour is lifted from the shipped brand artwork
// rather than invented: #101318 canvas, #151A21 card and #1B212A tile from
// assets/readme/hero.svg + formats.svg, #232A34 border, #20242C highlight tile
// from architecture.svg, and the #EDF1F6 → #5E6876 ink ramp from the same set.
// The two accents are the product's own format colours from src/overlay.rs:
// amber #F0C346 for archives, indigo #3A7CA5 for e-books.
//
// Only the phosphor teal is new, and it earns its place by being the one
// colour that reads as a screen rather than as a brand chip. Saturation is kept
// low everywhere else on purpose: this is an instrument panel, not a neon sign.
func RetroFutureA() *theme.Theme {
	c := theme.Colors{
		Background:      theme.Hex("101318"),
		Surface:         theme.Hex("151A21"),
		SurfaceElevated: theme.Hex("1B212A"),
		SurfaceSunken:   theme.Hex("0C0F13"),

		Foreground:      theme.Hex("EDF1F6"),
		ForegroundMuted: theme.Hex("96A1B0"),

		Primary:   theme.Hex("57C8BC"),
		Secondary: theme.Hex("3A7CA5"),
		Accent:    theme.Hex("F0C346"),

		Success: theme.Hex("7FBB8E"),
		Warning: theme.Hex("F0C346"),
		Error:   theme.Hex("D2686A"),
		Info:    theme.Hex("5E9DC6"),

		Border:      theme.Hex("232A34"),
		BorderMuted: theme.Hex("1B212A"),
		Grid:        theme.Hex("1E2530"),

		Selection:     theme.Hex("20242C"),
		SelectionText: theme.Hex("EDF1F6"),
		Focus:         theme.Hex("F0C346"),
		Disabled:      theme.Hex("5E6876"),

		Overlay: theme.Hex("0A0C10"),
	}

	return theme.Build(&theme.Theme{
		ID:     theme.DefaultPreset,
		Name:   "Retro Future A",
		Blurb:  "ArcThumbX identity: CRT ground, phosphor ink, one amber accent",
		Scheme: theme.SchemeDark,
		Colors: c,
		Type: theme.Typeography{
			// Spaced uppercase carries the engraved-legend hierarchy that a GUI
			// would get from font weight and size. Title is deliberately not
			// spaced: the brand is drawn as several coloured runs (the amber X
			// on the end), and gaps between runs would read as a word break.
			Title:    theme.TypeSpec{Foreground: c.Foreground, Bold: true, Uppercase: true},
			Heading:  theme.TypeSpec{Foreground: c.Foreground, Bold: true, Uppercase: true},
			Section:  theme.TypeSpec{Foreground: c.Accent, Bold: true, Uppercase: true, Spacing: 1},
			Body:     theme.TypeSpec{Foreground: c.Foreground},
			BodyStr:  theme.TypeSpec{Foreground: c.Foreground, Bold: true},
			Caption:  theme.TypeSpec{Foreground: c.ForegroundMuted, Faint: true},
			Help:     theme.TypeSpec{Foreground: c.ForegroundMuted},
			Value:    theme.TypeSpec{Foreground: c.Primary},
			Label:    theme.TypeSpec{Foreground: c.Foreground},
			Emphasis: theme.TypeSpec{Foreground: c.Primary, Bold: true},
		},
		Space: theme.Spacing{
			XS: 0, SM: 1, MD: 2, LG: 3, XL: 4,
			Indent: 2, Gutter: 3, PadX: 1,
		},
		Borders: theme.Borders{
			None:   theme.NoEdge(),
			Subtle: theme.HairlineEdge(c.BorderMuted),
			Normal: theme.HairlineEdge(c.Border),
			Strong: theme.HairlineEdge(c.ForegroundMuted),
			Accent: theme.HairlineEdge(c.Accent),
		},
		Glyphs: theme.UnicodeGlyphs(),
	})
}
