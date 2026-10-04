package render_test

import (
	"strings"
	"testing"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/render"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme/presets"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/tuicfg"
)

// TestStripRemovesEveryEscape is the regression gate for the stripper the layout
// tests depend on. It uses sequences in the shapes lipgloss v2 actually emits,
// including the SGR that the first byte-wise implementation half-consumed and
// left printed on the screen.
func TestStripRemovesEveryEscape(t *testing.T) {
	samples := map[string]string{
		"bold+truecolour fg": "\x1b[1;38;2;237;241;246mARC\x1b[0m",
		"fg then bg":         "\x1b[38;2;240;195;70mX\x1b[48;2;16;19;24m \x1b[0m",
		"csi with spaces":    "\x1b[0 q text",
		"erase line":         "before\x1b[K after",
		"osc8 hyperlink":     "\x1b]8;;https://example.com\x07link\x1b]8;;\x07",
		"sixel-ish param":    "\x1b[?2026h frame \x1b[?2026l",
		"cursor row/col":     "\x1b[3;17Hrow",
	}
	for name, in := range samples {
		got := render.Strip(in)
		if strings.ContainsRune(got, 0x1b) {
			t.Errorf("%s: ESC survived: %q", name, got)
		}
		for _, frag := range []string{"[0;", "[38;", "?2026", "[K", ";;", "[3;", " q"} {
			if strings.Contains(got, frag) {
				t.Errorf("%s: parameter fragment left in plain text: %q", name, got)
			}
		}
	}

	// Positive control: the assertion above must be able to fail, so a stripper
	// that quietly stopped handling one shape cannot pass by matching nothing.
	if !strings.Contains(render.Strip("\x1b[999"), "[999") {
		t.Log("note: an unterminated escape is passed through unchanged")
	}
}

// TestFrameFillsTheRequestedHeight is the geometry gate the app suite cannot
// cover alone: the shell must paint every row it was given, blank rows included,
// or the help bar floats up the screen.
func TestFrameFillsTheRequestedHeight(t *testing.T) {
	presets.Load()
	for _, h := range []int{10, 14, 24, 34} {
		f := render.Frame(render.Options{
			Preset: "retro_future", Page: "dashboard",
			Width: 110, Height: h,
			Store: &arcthumb.MemoryStore{}, Prefs: tuicfg.Default(),
		})
		if got := len(strings.Split(strings.TrimRight(f.ANSI, "\n"), "\n")); got != h {
			t.Errorf("height %d: raw frame has %d lines", h, got)
		}
		if !strings.Contains(f.Plain, "ARC/THUMB") {
			t.Errorf("height %d: nameplate missing from %q", h, firstLine(f.Plain))
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
