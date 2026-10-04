// Package presets holds the theme presets that satisfy the ArcThumbX design
// tokens declared in internal/theme.
//
// A preset is pure data: colours, type roles, spacing, border weights and
// glyphs. It never builds a style and never mentions a component, which is why
// adding one is a new file plus one line in Load. Third-party palettes
// (catppuccin, gruvbox, nord, dracula, tokyo night) belong here as presets and
// nowhere else — they are colour choices, not a second design system.
package presets

import (
	"sync"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// registered guards the table: Load is called from main, from --render and from
// every test, and Register rejects a duplicate ID by design. Without this the
// second call would panic the binary on a path a unit test cannot skip.
var registered sync.Once

// Load registers every preset in picker order. The first entries are the
// compatibility tiers, so a user whose terminal misrenders truecolour can reach
// Terminal Default in one keystroke.
func Load() {
	registered.Do(func() {
		theme.Register(theme.Entry{
			ID: theme.DefaultPreset, Name: "Retro Future A",
			Blurb: "ArcThumbX identity: CRT ground, phosphor ink, one amber accent",
			Make:  RetroFutureA,
		})
		theme.Register(theme.Entry{
			ID: "dark", Name: "Dark",
			Blurb: "Modern neutral dark, no period styling",
			Make:  Dark,
		})
		theme.Register(theme.Entry{
			ID: "light", Name: "Light",
			Blurb: "Light ground for bright rooms and printed output",
			Make:  Light,
		})
		theme.Register(theme.Entry{
			ID: "high_contrast", Name: "High Contrast",
			Blurb: "ANSI-only, ASCII glyphs, weight instead of hue",
			Make:  HighContrast,
		})
		theme.Register(theme.Entry{
			ID: "terminal_default", Name: "Terminal Default",
			Blurb: "Inherits the terminal's own colours; assumes nothing",
			Make:  TerminalDefault,
		})
	})
}
