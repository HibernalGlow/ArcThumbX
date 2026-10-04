package theme_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme/presets"
)

func TestAll(t *testing.T) { presets.Load() }

// requiredColors is the token list the objective names. A preset that leaves one
// unset would render an invisible element, so this is checked by reflection
// rather than by hand-maintained field lists.
var requiredColors = []string{
	"Background", "Surface", "SurfaceElevated", "Foreground", "ForegroundMuted",
	"Primary", "Secondary", "Accent", "Success", "Warning", "Error", "Info",
	"Border", "BorderMuted", "Selection", "Focus", "Disabled",
}

func TestEveryPresetDeclaresEveryRequiredToken(t *testing.T) {
	presets.Load()
	entries := theme.Registered()
	if len(entries) < 5 {
		t.Fatalf("registered presets = %d, want at least 5", len(entries))
	}

	names := map[theme.PresetID]bool{}
	for _, e := range entries {
		names[e.ID] = true
	}
	for _, want := range []theme.PresetID{
		"terminal_default", "light", "dark", "high_contrast", theme.DefaultPreset,
	} {
		if !names[want] {
			t.Errorf("preset %s is not registered", want)
		}
	}

	for _, e := range entries {
		th, ok := theme.Make(e.ID)
		if !ok {
			t.Fatalf("%s: Make failed", e.ID)
		}
		if th.Name == "" || th.Blurb == "" {
			t.Errorf("%s: missing name or blurb", e.ID)
		}
		missing := th.MissingColorTokens()
		if th.Scheme == theme.SchemeTerminal {
			// For this preset an unset token is the feature, not a gap: it is
			// how the terminal's own colours show through. So the gate runs the
			// other way — the structural slots must be inherited, and only the
			// semantic states are allowed to name a colour.
			for _, field := range []string{"Background", "Surface", "SurfaceElevated",
				"Foreground", "Border", "Selection", "Focus"} {
				if !contains(missing, field) {
					t.Errorf("%s painted %s, which breaks the compatibility fallback", e.ID, field)
				}
			}
			for _, field := range []string{"Success", "Warning", "Error"} {
				if contains(missing, field) {
					t.Errorf("%s left %s inherited: a state lamp that matches its label conveys nothing", e.ID, field)
				}
			}
		} else {
			for _, field := range missing {
				if contains(requiredColors, field) {
					t.Errorf("%s: required colour token %s is unset", e.ID, field)
				}
			}
		}
		if th.Glyphs.Rule == "" || th.Glyphs.On == "" || th.Glyphs.Focus == "" {
			t.Errorf("%s: glyph set incomplete", e.ID)
		}
		if len(th.Glyphs.Signal) == 0 {
			t.Errorf("%s: no signal ramp", e.ID)
		}
		for _, edge := range []struct {
			name string
			edge theme.Edge
		}{
			{"None", th.Borders.None}, {"Subtle", th.Borders.Subtle},
			{"Normal", th.Borders.Normal}, {"Strong", th.Borders.Strong},
			{"Accent", th.Borders.Accent},
		} {
			if edge.name != "None" && edge.edge.Top == "" {
				t.Errorf("%s: border token %s is empty", e.ID, edge.name)
			}
		}
		if th.Space.SM == 0 || th.Space.Gutter == 0 {
			t.Errorf("%s: spacing scale incomplete", e.ID)
		}
	}
}

// TestTerminalDefaultEmitsNoBackground is the compatibility promise: the preset
// must not paint a ground at all, or it stops being the fallback.
func TestTerminalDefaultEmitsNoBackground(t *testing.T) {
	presets.Load()
	th, ok := theme.Make("terminal_default")
	if !ok {
		t.Fatal("terminal_default missing")
	}
	if th.Scheme != theme.SchemeTerminal {
		t.Errorf("scheme = %v, want terminal", th.Scheme)
	}
	rendered := th.Components.Inspector.Render("sample")
	if strings.Contains(rendered, "48;") {
		t.Errorf("terminal default painted a background: %q", stripReset(rendered))
	}
	for _, s := range []struct {
		name  string
		style lipgloss.Style
	}{
		{"Root", th.Components.Root},
		{"Header", th.Components.Header},
		{"Nav", th.Components.Nav},
		{"TabBar", th.Components.TabBar},
		{"HelpBar", th.Components.HelpBar},
		{"Dialog", th.Components.Dialog},
		{"ButtonPrimary", th.Components.ButtonPrimary},
		{"NavItemHover", th.Components.NavItemHover},
	} {
		out := s.style.Render("x")
		if strings.Contains(out, "48;2:") || strings.Contains(out, "48;5:") {
			t.Errorf("%s emitted a truecolour/256 background: %q", s.name, stripReset(out))
		}
	}
}

// TestTerminalDefaultDetectsNoColourIsThePositiveControl: the same assertion must
// go red on a preset that does paint, otherwise a passing check proves nothing.
func TestTerminalDefaultDetectsNoColourIsThePositiveControl(t *testing.T) {
	presets.Load()
	th, ok := theme.Make(theme.DefaultPreset)
	if !ok {
		t.Fatal("retro_future missing")
	}
	if !strings.Contains(th.Components.Inspector.Render("sample"), "48;") {
		t.Fatal("positive control failed: Retro Future painted no background, so the gate above is blind")
	}
}

// TestNewPresetNeedsNoBranching proves the extensibility requirement: a preset
// added at runtime appears in the registry and resolves, with no conditional in
// the theme or app code.
func TestNewPresetNeedsNoBranching(t *testing.T) {
	presets.Load()
	before := len(theme.Registered())

	const id theme.PresetID = "test_only_preset"
	theme.Register(theme.Entry{
		ID: id, Name: "Test Only", Blurb: "registered by a test",
		Make: func() *theme.Theme {
			c := theme.Colors{Foreground: theme.Hex("#000000"), Accent: theme.Hex("#FF0000")}
			return theme.Build(&theme.Theme{ID: id, Name: "Test Only", Colors: c,
				Type: theme.Typeography{
					Title:   theme.TypeSpec{Foreground: c.Foreground},
					Section: theme.TypeSpec{Foreground: c.Accent, Bold: true},
				},
				Glyphs: theme.UnicodeGlyphs()})
		},
	})

	if got := len(theme.Registered()); got != before+1 {
		t.Fatalf("registry size = %d, want %d", got, before+1)
	}
	th, ok := theme.Make(id)
	if !ok {
		t.Fatal("runtime preset did not resolve")
	}
	if th.Glyphs.Rule == "" || th.Borders.Normal.Top == "" {
		t.Error("Build did not fill the token groups the preset left zero")
	}
	// Build must be able to supply defaults for anything a preset omits, so a
	// component never reads a zero style.
	if !th.Components.Section.Style.GetBold() {
		t.Error("declared Bold did not survive compilation")
	}
	if th.Name != "Test Only" {
		t.Errorf("Name = %q", th.Name)
	}
}

func TestPaintMapsInheritToNoColor(t *testing.T) {
	if _, ok := theme.Paint(theme.Inherit).(lipgloss.NoColor); !ok {
		t.Error("Inherit must compile to NoColor so the terminal default survives")
	}
	if _, ok := theme.Paint(theme.Hex("101318")).(lipgloss.NoColor); ok {
		t.Error("a hex token must not compile to NoColor")
	}
}

func TestRetroFutureUsesBrandGround(t *testing.T) {
	presets.Load()
	th, _ := theme.Make(theme.DefaultPreset)
	// These exact values are the shipped brand tokens; if somebody re-derives the
	// palette from a mood board instead of the artwork, this goes red.
	for name, want := range map[string]theme.Color{
		"Background":      theme.Hex("101318"),
		"Surface":         theme.Hex("151A21"),
		"SurfaceElevated": theme.Hex("1B212A"),
		"Border":          theme.Hex("232A34"),
		"Foreground":      theme.Hex("EDF1F6"),
		"Accent":          theme.Hex("F0C346"),
		"Secondary":       theme.Hex("3A7CA5"),
	} {
		got := th.ColorToken(name)
		if got != want {
			t.Errorf("Retro Future %s = %q, want brand token %q", name, got, want)
		}
	}
}

func TestLetterspacingAndCase(t *testing.T) {
	presets.Load()
	th, _ := theme.Make(theme.DefaultPreset)
	got := th.Components.Section.Transform("formats")
	if !strings.Contains(got, "F O R M A T S") {
		t.Errorf("section role did not apply case+spacing: %q", got)
	}
	if strings.HasPrefix(got, " ") || strings.HasSuffix(got, " ") {
		t.Errorf("letterspacing changed the column edge: %q", got)
	}
}

func stripReset(s string) string {
	return strings.ReplaceAll(s, "\x1b[0m", "!")
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
