// Package render produces a frame without a terminal.
//
// It exists for two reasons: the binary's --render flag, and tests that need to
// assert on layout (line counts, column alignment, how many box glyphs a screen
// contains) instead of trusting that a component looked right in somebody's
// terminal.
package render

import (
	"regexp"
	"strings"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/app"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme/presets"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/tuicfg"
)

// Snapshot is one rendered frame, in both the styled and the plain form.
type Snapshot struct {
	ANSI  string
	Plain string
	Lines []string
	Width int
	// BoxGlyphs counts the frame characters of a traditional box UI, so a test
	// can prove the panel is not one.
	BoxGlyphs int
	// RuleGlyphs counts hairline rules, the structure ArcThumbX uses instead.
	RuleGlyphs int
	// SGRBlocks counts colour runs, the proxy for "not a wall of colour".
	SGRBlocks int
	// HitTargets counts the interactive regions registered by this frame, which
	// is how a test proves the mouse was wired rather than merely intended.
	HitTargets int
}

// Options picks the frame to render.
type Options struct {
	Preset  theme.PresetID
	Page    string
	Dialog  string
	Width   int
	Height  int
	Store   arcthumb.Store
	Prefs   tuicfg.Settings
	Version string
}

// Frame builds a model, drives it to the requested page and renders one frame.
func Frame(o Options) Snapshot {
	presets.Load()

	prefs := o.Prefs.Normalize()
	if o.Preset != "" {
		prefs.ThemeID = string(o.Preset)
	}
	store := o.Store
	if store == nil {
		store = &arcthumb.MemoryStore{}
	}
	m := app.New(app.Options{
		Store:   store,
		Prefs:   prefs,
		Version: o.Version,
		Width:   or(o.Width, 110),
		Height:  or(o.Height, 34),
	})
	if o.Page != "" {
		m.ShowPage(o.Page)
	}
	if o.Dialog != "" {
		m.OpenDialog(o.Dialog)
	}
	v := m.View()
	ansi := v.Content
	plain := Strip(ansi)

	s := Snapshot{
		ANSI:      ansi,
		Plain:     plain,
		Lines:     strings.Split(strings.TrimRight(plain, "\n"), "\n"),
		Width:     or(o.Width, 110),
		SGRBlocks: strings.Count(ansi, "\x1b["),
	}
	s.BoxGlyphs, s.RuleGlyphs = Measure(plain)
	s.HitTargets = m.HitTargets()
	return s
}

// Measure counts frame characters against hairlines in plain text. The split is
// the honest test of "few boxes, strong hierarchy": a settings UI drawn the
// ncurses way is mostly frame glyphs, ArcThumbX is mostly rules and whitespace.
func Measure(plain string) (boxes, rules int) {
	for _, r := range plain {
		switch r {
		case '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '╔', '╗', '╚', '╝', '╠', '╣', '│', '|':
			boxes++
		case '─', '━', '-':
			rules++
		}
	}
	return boxes, rules
}

// Strip removes ANSI escape sequences so a layout assertion reads plain text.
// It scans bytes rather than matching a pattern list, because a partial or
// unknown sequence must not survive into the assertion and silently shift every
// column.
func Strip(s string) string {
	return escapeSeq.ReplaceAllString(s, "")
}

// escapeSeq covers CSI (ESC [ parameters through a final byte in @-~), OSC
// (ESC ] … BEL or ST) and the two-character escapes. Matching with an explicit
// grammar beats the byte walk this function used to do: the walk mistook a
// parameter for a terminator on real frames and left "38;2;30;37;48m" printed on
// the line, which shifted every column the layout tests measure.
var escapeSeq = regexp.MustCompile(`\x1b\[[0-9;? !>]*[a-zA-Z@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\\\-_]`)

// Pages lists the page IDs a snapshot can request, so --render's usage text and
// the real page set cannot diverge.
func Pages() []string {
	presets.Load()
	m := app.New(app.Options{
		Store:  &arcthumb.MemoryStore{},
		Prefs:  tuicfg.Default(),
		Width:  60,
		Height: 20,
	})
	return m.PageIDs()
}

func or(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}
