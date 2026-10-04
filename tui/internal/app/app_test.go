package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/app"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/render"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme/presets"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/tuicfg"
)

func newModel(t *testing.T, store arcthumb.Store, w, h int) *app.Model {
	t.Helper()
	presets.Load()
	m := app.New(app.Options{
		Store:   store,
		Prefs:   tuicfg.Default(),
		Version: "test",
		Width:   w,
		Height:  h,
	})
	// Every assertion that follows reads geometry the frame produced, so draw
	// first exactly as the program would.
	m.View()
	return m
}

// TestFrameHeightIsExactAtEverySize guards the one invariant a terminal UI must
// never break: a frame taller than the screen pushes the help bar off it.
func TestFrameHeightIsExactAtEverySize(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {110, 34}, {96, 24}, {84, 20}, {60, 14}, {34, 10}} {
		m := newModel(t, &arcthumb.MemoryStore{}, size[0], size[1])
		for _, id := range m.PageIDs() {
			m.ShowPage(id)
			v := m.View()
			lines := strings.Split(strings.TrimRight(render.Strip(v.Content), "\n"), "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d page %s: %d lines, want exactly %d",
					size[0], size[1], id, len(lines), size[1])
			}
			for i, l := range lines {
				if w := lineWidth(l); w > size[0] {
					t.Errorf("%dx%d page %s line %d is %d cells wide: %q",
						size[0], size[1], id, i, w, firstRunes(l, 60))
					break
				}
			}
		}
	}
}

// TestEveryPageRegistersMouseTargets is the load-bearing mouse test: a page that
// painted controls but registered none would look completely fine.
func TestEveryPageRegistersMouseTargets(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	for _, id := range m.PageIDs() {
		m.ShowPage(id)
		m.View()
		if m.HitTargets() == 0 {
			t.Errorf("page %s registered no hit targets", id)
		}
		if _, _, ok := m.FindTarget(components.HitNav, "nav"); !ok {
			t.Errorf("page %s: navigation rail is not clickable", id)
		}
		switch id {
		case "thumbnail", "general", "formats", "integration":
			if _, _, ok := m.FirstControl(); !ok {
				t.Errorf("page %s has no clickable control", id)
			}
		}
		if id == "formats" {
			if _, _, ok := m.FindTarget(components.HitTab, "formats:tabs"); !ok {
				t.Errorf("formats: tab strip is not clickable")
			}
		}
		if id == "about" {
			if _, _, ok := m.FindTarget(components.HitListItem, "about:keys"); !ok {
				t.Errorf("about: list items are not clickable")
			}
		}
	}
}

// TestClickOnToggleEditsRealState proves the mouse path reaches the
// configuration, not just the focus ring.
func TestClickOnToggleEditsRealState(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	m.ShowPage("thumbnail")
	m.View()

	before := m.SettingsView()
	target, rect, ok := m.FirstControl()
	if !ok {
		t.Fatal("no control found on thumbnail page")
	}
	if target.Kind != components.HitControl {
		t.Fatalf("target kind = %v, want a value cell", target.Kind)
	}
	m.Update(tea.MouseClickMsg{X: rect.X, Y: rect.Y, Button: tea.MouseLeft})
	after := m.SettingsView()

	if after == before {
		t.Fatal("clicking a control changed nothing")
	}
	if !m.Dirty() {
		t.Error("an edit must mark the session dirty")
	}

	// Keyboard parity on a lamp, not a selector: a selector already on its last
	// option has nowhere to go, so "nothing changed" there would be correct
	// behaviour and a passing test about nothing.
	m.ShowPage("formats")
	m.View()
	beforeKeys := m.SettingsView()
	if _, _, ok := m.FirstControl(); !ok {
		t.Fatal("no control on formats")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.SettingsView() == beforeKeys {
		t.Error("Enter did not edit the focused control; keyboard is second-class")
	}
}

// TestDragIsAFirstClassGesture covers the motion-with-button case.
func TestDragIsAFirstClassGesture(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	m.ShowPage("formats")
	m.View()

	_, rect, ok := m.FirstControl()
	if !ok {
		t.Fatal("no control on formats")
	}
	m.Update(tea.MouseMotionMsg{X: rect.X, Y: rect.Y, Button: tea.MouseLeft})
	if !m.Dirty() {
		t.Error("dragging across value cells did not edit; drag is not wired")
	}
}

// TestWheelScrollsThePane checks the wheel reaches the page under the pointer.
// A short screen is used on purpose: if the page already fits, scrolling has
// nothing to do and the test would pass for the wrong reason.
func TestWheelScrollsThePane(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 14)
	m.ShowPage("formats")
	m.View()
	if m.HitTargets() == 0 {
		t.Fatal("nothing registered to scroll")
	}
	_, rect, ok := m.FirstControl()
	if !ok {
		t.Fatal("no control on formats")
	}
	before := m.View().Content
	m.Update(tea.MouseWheelMsg{X: rect.X, Y: rect.Y, Button: tea.MouseWheelDown})
	if plain := render.Strip(m.View().Content); render.Strip(before) == plain {
		t.Error("wheel down did not change the frame; the list is not scrollable")
	}
}

// TestThemeSwitchIsLiveAndPersistent: picking another preset must repaint at
// once and must survive a restart.
func TestThemeSwitchIsLiveAndPersistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings")
	presets.Load()

	m := app.New(app.Options{
		Store:     &arcthumb.MemoryStore{},
		Prefs:     tuicfg.Default(),
		PrefsPath: path,
		Width:     120,
		Height:    40,
	})
	m.View()
	startName := m.Theme().Name

	m.Update(tea.KeyPressMsg{Code: rune('t'), Text: "t"})
	if !m.DialogOpen() {
		t.Fatal("T did not open the preset picker")
	}
	m.View()
	// Enter applies the highlighted preset; the picker starts on the active one,
	// so step the cursor first to prove the choice is honoured.
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.DialogOpen() {
		t.Error("dialog stayed open after applying")
	}
	if m.Theme().Name == startName {
		t.Errorf("preset did not change live: still %q", m.Theme().Name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("preferences were not persisted: %v", err)
	}
	reloaded := tuicfg.Parse(string(data))
	if reloaded.ThemeID != m.PrefsView().ThemeID {
		t.Errorf("persisted theme = %q, in-memory = %q", reloaded.ThemeID, m.PrefsView().ThemeID)
	}
	if reloaded.ThemeID == tuicfg.Default().ThemeID {
		t.Error("the picker wrote the preset it started on, so the change was not applied")
	}
}

// TestSaveWritesTheConfigFileEndToEnd is the disk-level gate: after clicking
// Save, a separate reader of the same file must see the edit.
func TestSaveWritesTheConfigFileEndToEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings")
	store := arcthumb.NewFileStore(path)
	m := newModel(t, store, 120, 40)

	m.ShowPage("thumbnail")
	m.View()
	_, rect, ok := m.FirstControl()
	if !ok {
		t.Fatal("no control")
	}
	m.Update(tea.MouseClickMsg{X: rect.X, Y: rect.Y, Button: tea.MouseLeft})
	if !m.Dirty() {
		t.Fatal("edit did not register")
	}

	// The action strip only appears once there is something to save, so the
	// frame has to be redrawn before the button can be located.
	m.View()
	target, saveRect, ok := m.FindTarget(components.HitButton, "save")
	if !ok {
		t.Fatal("Save button is not clickable while edits are pending")
	}
	_ = target
	m.Update(tea.MouseClickMsg{X: saveRect.X, Y: saveRect.Y, Button: tea.MouseLeft})

	if m.Dirty() {
		t.Fatalf("still dirty after save: notice=%q", m.Notice())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("save produced no file: %v", err)
	}
	got, _ := arcthumb.Parse(string(data))
	if got != m.SettingsView() {
		t.Errorf("disk says %+v, memory says %+v", got, m.SettingsView())
	}
	if !strings.HasPrefix(string(data), "# ArcThumb") {
		t.Error("written file lost the header the Rust writer emits")
	}
}

// TestQuitGuardOnlyFiresWhenPending: quitting with nothing pending must leave
// immediately, and with edits pending must ask.
func TestQuitGuardOnlyFiresWhenPending(t *testing.T) {
	clean := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	if cmd := updateKey(clean, 'q'); cmd == nil {
		t.Error("q with no pending edits did not return a quit command")
	}
	if clean.DialogOpen() {
		t.Error("clean quit opened a dialog")
	}

	dirty := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	dirty.ShowPage("thumbnail")
	dirty.View()
	_, rect, ok := dirty.FirstControl()
	if !ok {
		t.Fatal("no control")
	}
	dirty.Update(tea.MouseClickMsg{X: rect.X, Y: rect.Y, Button: tea.MouseLeft})
	if cmd := updateKey(dirty, 'q'); cmd != nil {
		t.Error("q with pending edits quit without asking")
	}
	if !dirty.DialogOpen() {
		t.Error("q with pending edits did not open the confirmation")
	}
	// Esc backs out of the guard without losing the edit.
	dirty.View()
	updateKey(dirty, 0x1b) // escape
	if !dirty.Dirty() {
		t.Error("dismissing the quit guard discarded the pending edit")
	}
}

// TestFewBoxesWithPositiveControl is the visual-direction gate. The budget is
// not arbitrary: it is set against the count a box-drawn equivalent of the same
// screen would produce, which the control computes.
func TestFewBoxesWithPositiveControl(t *testing.T) {
	frame := render.Frame(render.Options{Page: "thumbnail", Width: 120, Height: 40})

	boxes, rules := frame.BoxGlyphs, frame.RuleGlyphs
	if boxes > 12 {
		t.Errorf("box frame glyphs = %d, want the panel to draw structure without boxes", boxes)
	}
	if rules < 20 {
		t.Errorf("hairline rules = %d, want the section legends to carry the hierarchy", rules)
	}

	// Positive control: the same counter must go loud on the style this project
	// explicitly rejects.
	ncurses := ""
	for i := 0; i < 20; i++ {
		ncurses += "├────────────────────┤\n"
	}
	cBoxes, cRules := render.Measure(ncurses)
	if cBoxes < 40 {
		t.Fatalf("control failed: Measure saw only %d box glyphs in a box-heavy fixture", cBoxes)
	}
	if cRules < 300 {
		t.Fatalf("control failed: Measure missed the fixture's rules (%d)", cRules)
	}
	if boxes >= cBoxes {
		t.Errorf("panel has %d box glyphs, at least the ncurses control's %d", boxes, cBoxes)
	}
}

// TestNoCheckboxWall: the brief bans a screen full of [ ], so brackets used as
// checkboxes must not appear in a rendered page.
func TestNoCheckboxWall(t *testing.T) {
	for _, id := range []string{"dashboard", "general", "thumbnail", "formats", "integration", "about"} {
		frame := render.Frame(render.Options{Page: id, Width: 120, Height: 40})
		if n := strings.Count(frame.Plain, "[x]") + strings.Count(frame.Plain, "[ ]"); n > 0 {
			t.Errorf("page %s renders %d checkbox brackets", id, n)
		}
	}
}

// TestAllPresetsRenderEveryPage is the cross-product gate: a preset must not
// break layout for any page, which is what token-level theming promises.
func TestAllPresetsRenderEveryPage(t *testing.T) {
	for _, entry := range presetsList() {
		for _, id := range []string{"dashboard", "general", "thumbnail", "formats", "integration", "about"} {
			frame := render.Frame(render.Options{
				Preset: entry, Page: id, Width: 120, Height: 40,
			})
			if len(frame.Lines) != 40 {
				t.Errorf("preset %s page %s: %d lines, want 40", entry, id, len(frame.Lines))
			}
			if frame.HitTargets == 0 {
				t.Errorf("preset %s page %s registered no targets", entry, id)
			}
			if !strings.Contains(frame.Plain, "ARC") {
				t.Errorf("preset %s page %s lost the nameplate", entry, id)
			}
		}
	}
}

// TestGroundIsPaintedBothWays is the regression gate for the class of defect
// where a preset declares a CRT canvas in its tokens and nothing ever paints it:
// the panel then silently shows whatever background the user's terminal happens
// to have. Terminal Default must do the opposite and paint nothing.
func TestGroundIsPaintedBothWays(t *testing.T) {
	dark := render.Frame(render.Options{
		Preset: "retro_future", Page: "thumbnail", Width: 96, Height: 24,
		Store: &arcthumb.MemoryStore{}, Prefs: tuicfg.Default(),
	})
	for _, want := range []string{"48;2;16;19;24", "48;2;21;26;33"} {
		if !strings.Contains(dark.ANSI, want) {
			t.Errorf("retro_future never painted ground %s: the canvas is a token, not a rumour", want)
		}
	}
	neutral := render.Frame(render.Options{
		Preset: "terminal_default", Page: "thumbnail", Width: 96, Height: 24,
		Store: &arcthumb.MemoryStore{}, Prefs: tuicfg.Default(),
	})
	if strings.Contains(neutral.ANSI, "48;2;") {
		t.Error("terminal_default painted a truecolour ground, breaking its fallback promise")
	}
}

// TestFoldAtNarrowWidth: below the breakpoint the rail must become tabs rather
// than squeeze the measure.
func TestFoldAtNarrowWidth(t *testing.T) {
	wide := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	if _, _, ok := wide.FindTarget(components.HitNav, "nav"); !ok {
		t.Error("wide frame has no clickable rail")
	}
	narrow := newModel(t, &arcthumb.MemoryStore{}, 60, 30)
	if _, _, ok := narrow.FindTarget(components.HitNav, "nav"); ok {
		t.Error("narrow frame kept the rail instead of folding it into tabs")
	}
	if _, _, ok := narrow.FindTarget(components.HitTab, "nav"); !ok {
		t.Error("narrow frame folded the rail but drew no tabs")
	}
}

// TestDialogButtonsAreWhereTheyArePainted is the overlay gate. A modal draws
// itself centred and then re-registers its answer buttons at computed offsets;
// if those offsets were wrong the click would still "work" against the hit map
// while the user aims at a different row. So the assertion is that the painted
// row and the registered row are the same row.
func TestDialogButtonsAreWhereTheyArePainted(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 96, 26)
	if !m.OpenDialog("themes") {
		t.Fatal("themes dialog did not open")
	}
	v := m.View()
	lines := strings.Split(strings.TrimRight(render.Strip(v.Content), "\n"), "\n")
	if len(lines) != 26 {
		t.Fatalf("overlay frame is %d lines, want the full 26", len(lines))
	}
	target, rect, ok := m.FindTarget(components.HitButton, "apply")
	if !ok {
		t.Fatal("the themes dialog registered no Apply target")
	}
	if rect.Y >= len(lines) {
		t.Fatalf("Apply registered on row %d, past the frame", rect.Y)
	}
	if !strings.Contains(lines[rect.Y], "APPLY") {
		t.Errorf("row %d reads %q: the hit target is not on the painted button", rect.Y, lines[rect.Y])
	}
	if target.Kind != components.HitButton {
		t.Fatalf("target kind = %v", target.Kind)
	}

	// The page underneath must be unreachable: no stale control may sit under a
	// modal, or a click answers the dialog and edits the setting at once.
	if _, _, ok := m.FirstControl(); ok {
		t.Error("a page control is still hittable while the dialog is open")
	}

	// Clicking the painted answer applies what the cursor is on. The cursor has
	// to be moved first: it opens on the active preset, so "nothing changed"
	// would be correct behaviour there and a passing test about nothing.
	entries := theme.Registered()
	if len(entries) < 2 {
		t.Fatalf("only %d presets registered", len(entries))
	}
	if before := m.PrefsView().ThemeID; before != string(entries[0].ID) {
		t.Fatalf("picker opens on %q, expected the active preset %q to be first",
			before, entries[0].ID)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.View()
	m.Update(tea.MouseClickMsg{X: rect.X + 1, Y: rect.Y, Button: tea.MouseLeft})
	if m.DialogOpen() {
		t.Error("clicking Apply left the dialog open")
	}
	if got := m.PrefsView().ThemeID; got != string(entries[1].ID) {
		t.Errorf("Apply committed %q, want the highlighted %q", got, entries[1].ID)
	}
}

// TestDialogBodyIsWhereItIsPainted covers the modal's other layer. The preset
// list is drawn by a body closure that cannot know where the centred box landed,
// so if the dialog hands it a guessed rect every row's hit target sits several
// cells above the paint: the user aims at "Light", the model reads "Dark", and
// nothing on screen says so. Lower rows drift the most, so that is where the aim
// is checked, and the click is only believed because Apply then lands the same
// preset in the preferences.
func TestDialogBodyIsWhereItIsPainted(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 96, 30)
	if !m.OpenDialog("themes") {
		t.Fatal("themes dialog did not open")
	}
	lines := strings.Split(strings.TrimRight(render.Strip(m.View().Content), "\n"), "\n")
	entries := theme.Registered()
	if len(entries) < 3 {
		t.Fatalf("only %d presets registered", len(entries))
	}
	chosen := entries[2]

	target, rect, ok := m.FindTargetN(components.HitListItem, "themes", 2)
	if !ok {
		t.Fatal("the picker registered no third row")
	}
	if target.Index != 2 {
		t.Fatalf("target index = %d", target.Index)
	}
	row := "<past the frame>"
	if rect.Y < len(lines) {
		row = lines[rect.Y]
	}
	if !strings.Contains(row, chosen.Name) {
		t.Fatalf("registered row %d reads %q, want the painted %q", rect.Y, row, chosen.Name)
	}

	m.Update(tea.MouseClickMsg{X: rect.X + 2, Y: rect.Y, Button: tea.MouseLeft})
	m.View()
	_, applyRect, ok := m.FindTarget(components.HitButton, "apply")
	if !ok {
		t.Fatal("the themes dialog registered no Apply target")
	}
	m.Update(tea.MouseClickMsg{X: applyRect.X + 1, Y: applyRect.Y, Button: tea.MouseLeft})
	if m.DialogOpen() {
		t.Fatal("clicking Apply left the dialog open")
	}
	if got := m.PrefsView().ThemeID; got != string(chosen.ID) {
		t.Errorf("click-through applied %q, want %q", got, chosen.ID)
	}
}

// TestDialogEscapeRestoresThePage checks the other half of the modal contract.
func TestDialogEscapeRestoresThePage(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 96, 26)
	m.OpenDialog("help")
	m.View()
	if _, _, ok := m.FindTarget(components.HitButton, "close"); !ok {
		t.Fatal("help dialog has no hittable Close")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.DialogOpen() {
		t.Error("Esc did not dismiss the dialog")
	}
	m.View()
	if _, _, ok := m.FindTarget(components.HitNav, "nav"); !ok {
		t.Error("the rail did not become clickable again")
	}
}

// TestHoverMovesFocusWithoutEditing is the plain mouse-motion requirement:
// hovering must track the pointer, but it must not flip anything.
func TestHoverMovesFocusWithoutEditing(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	m.ShowPage("thumbnail")
	m.View()
	_, first, ok := m.FirstControl()
	if !ok {
		t.Fatal("no control")
	}
	// One row down: on this page each row explains something different, so the
	// readout is a sensitive readback of where focus actually went.
	before := m.SettingsView()
	beforeText := m.InspectorText()
	m.Update(tea.MouseMotionMsg{X: first.X, Y: first.Y + 1, Button: tea.MouseNone})
	m.View()
	if m.InspectorText() == beforeText {
		t.Errorf("hover did not move focus; readout stayed %q", beforeText)
	}
	if m.SettingsView() != before {
		t.Error("hover edited a setting; motion must only move focus")
	}
}

// TestHelpBarDoublesAsButtons proves the key hints are real controls for mouse
// users, not decoration: the theme hint carries an ID and clicking it works.
func TestHelpBarDoublesAsButtons(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	_, rect, ok := m.FindTarget(components.HitButton, "theme")
	if !ok {
		t.Fatal("the help bar registered no clickable theme item")
	}
	m.Update(tea.MouseClickMsg{X: rect.X, Y: rect.Y, Button: tea.MouseLeft})
	if !m.DialogOpen() {
		t.Error("clicking the THEME hint did not open the picker")
	}
}

func updateKey(m *app.Model, r rune) tea.Cmd {
	text := string(r)
	msg := tea.KeyPressMsg{Code: r, Text: text}
	m.View()
	_, cmd := m.Update(msg)
	return cmd
}

func plainOf(m *app.Model) string {
	m.View()
	return render.Strip(m.View().Content)
}

func presetsList() []theme.PresetID {
	return []theme.PresetID{
		"retro_future", "dark", "light", "high_contrast", "terminal_default",
	}
}

func lineWidth(s string) int {
	// Plain text only: callers pass a stripped line.
	n := 0
	for _, r := range s {
		switch {
		case r >= 0x1100 && r <= 0x115F,
			r >= 0x2E80 && r <= 0x303E,
			r >= 0x3041 && r <= 0x33FF,
			r >= 0x3400 && r <= 0x4DBF,
			r >= 0x4E00 && r <= 0x9FFF,
			r >= 0xA000 && r <= 0xA4CF,
			r >= 0xAC00 && r <= 0xD7A3,
			r >= 0xF900 && r <= 0xFAFF,
			r >= 0xFE30 && r <= 0xFE6F,
			r >= 0xFF00 && r <= 0xFF60,
			r >= 0xFFE0 && r <= 0xFFE6:
			n += 2
		default:
			n++
		}
	}
	return n
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
