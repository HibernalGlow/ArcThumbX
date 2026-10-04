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

// TestWheelUpReturnsThePaneToItsFirstFrame proves the wheel's second direction
// is wired to something that reaches the list and lands back on the top clamp.
//
// What it deliberately does not claim is the size of a notch. The formats list is
// one rule plus 11 rows, so at every height where it overflows at all the whole
// range fits inside a single step: measured at heights 10, 12, 14, 16 and 20, a
// second downward notch never changed the frame above 14. A "one notch up undoes
// one notch down" assertion would therefore pass for a one-line wheel as well as
// for a three-line one, which is a green test about nothing. The step itself is
// the page's ActionScrollUp/ActionScrollDown pair, shared with the keyboard, so
// neither input can drift from the other on its own.
func TestWheelUpReturnsThePaneToItsFirstFrame(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 14)
	m.ShowPage("formats")
	m.View()

	_, rect, ok := m.FirstControl()
	if !ok {
		t.Fatal("no control on formats")
	}
	frame := func() string { return render.Strip(m.View().Content) }
	roll := func(up bool) string {
		b := tea.MouseWheelDown
		if up {
			b = tea.MouseWheelUp
		}
		_, _ = m.Update(tea.MouseWheelMsg{X: rect.X, Y: rect.Y, Button: b})
		return frame()
	}

	top := frame()
	// Above the first row there is nothing, so an upward notch must be inert.
	if got := roll(true); got != top {
		t.Error("scrolling up from the top moved the pane; the clamp is missing")
	}

	down := roll(false)
	if down == top {
		t.Fatal("wheel down did nothing here, so the upward roll below would prove nothing")
	}
	if got := roll(true); got != top {
		var line int
		g, w := strings.Split(got, "\n"), strings.Split(top, "\n")
		for i := range g {
			if i >= len(w) || g[i] != w[i] {
				line = i
				break
			}
		}
		t.Errorf("rolling up did not return the pane to its first frame (first difference at line %d)", line)
	}
}

// TestLongFormMeasureFaderCapsWhatItSaysItCaps is the wiring gate for the
// General page's slider. A control whose number is only ever echoed somewhere
// else is a decoration, so the measure has to show up three ways: in painted
// geometry, in the saved preferences, and inside both clamps.
func TestLongFormMeasureFaderCapsWhatItSaysItCaps(t *testing.T) {
	presets.Load()
	path := filepath.Join(t.TempDir(), "prefs")
	prefs := tuicfg.Default()
	prefs.ContentWidth = tuicfg.MinContentWidth
	m := app.New(app.Options{
		Store:     &arcthumb.MemoryStore{},
		Prefs:     prefs,
		PrefsPath: path,
		Width:     200,
		Height:    30,
	})

	// The rule drawn under the about page's metadata is the cap made visible.
	// Only lines inside the pane count: the shell's own divider spans the whole
	// frame and deliberately follows the terminal, not the measure.
	ruleWidth := func() int {
		glyph := []rune(m.Theme().Glyphs.Rule)[0]
		best := 0
		for _, l := range strings.Split(render.Strip(m.View().Content), "\n") {
			_, tail, found := strings.Cut(l, "│")
			if !found {
				continue
			}
			ink := []rune(strings.TrimSpace(tail))
			if len(ink) == 0 {
				continue
			}
			only := true
			for _, r := range ink {
				if r != glyph {
					only = false
					break
				}
			}
			if only && len(ink) > best {
				best = len(ink)
			}
		}
		return best
	}

	m.ShowPage("about")
	narrow := ruleWidth()
	if narrow != tuicfg.MinContentWidth {
		t.Fatalf("a %d-col measure painted a %d-col rule: the cap is not read by the layout",
			tuicfg.MinContentWidth, narrow)
	}

	m.ShowPage("general")
	m.View()
	located := false
	for i := 0; i < 24 && !located; i++ {
		_, rect, ok := m.FindTargetN(components.HitRow, "general", i)
		if !ok {
			break
		}
		m.Update(tea.MouseClickMsg{X: rect.X + 1, Y: rect.Y, Button: tea.MouseLeft})
		located = strings.Contains(m.InspectorText(), "Width cap for prose")
	}
	if !located {
		t.Fatal("no row on the general page is the long-form measure fader")
	}

	start := m.PrefsView().ContentWidth
	updateKey(m, tea.KeyRight)
	updateKey(m, tea.KeyRight)
	want := start + 4 // the fader moves two columns per step
	if got := m.PrefsView().ContentWidth; got != want {
		t.Errorf("two fader steps = %d cols, want %d", got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the fader did not persist: %v", err)
	}
	if got := tuicfg.Parse(string(data)).ContentWidth; got != want {
		t.Errorf("preferences file holds %d cols, want %d", got, want)
	}

	// Upper clamp: the value must stop at the bound, not overrun it.
	for range 80 {
		updateKey(m, tea.KeyRight)
	}
	if got := m.PrefsView().ContentWidth; got != tuicfg.MaxContentWidth {
		t.Errorf("after 80 steps up the measure = %d, want the %d bound", got, tuicfg.MaxContentWidth)
	}
	m.ShowPage("about")
	wide := ruleWidth()
	if wide <= narrow {
		t.Errorf("the rule stayed %d cols while the measure grew to %d", wide, tuicfg.MaxContentWidth)
	}
	if wide > tuicfg.MaxContentWidth {
		t.Errorf("the rule painted %d cols past the %d bound", wide, tuicfg.MaxContentWidth)
	}

	// Lower clamp, then the geometry must come back to exactly the narrow rule.
	// The keys go to whichever page is on screen, so the fader has to be shown
	// again before they mean anything to it.
	m.ShowPage("general")
	for range 80 {
		updateKey(m, tea.KeyLeft)
	}
	if got := m.PrefsView().ContentWidth; got != tuicfg.MinContentWidth {
		t.Errorf("after 80 steps down the measure = %d, want the %d bound", got, tuicfg.MinContentWidth)
	}
	m.ShowPage("about")
	if got := ruleWidth(); got != narrow {
		t.Errorf("the rule is %d cols after returning to the narrow measure, want %d", got, narrow)
	}
}

// TestMouseTrackingToggleHandsThePointerBackLive covers the switch that decides
// whether the terminal reports the pointer at all. It is the one setting whose
// whole promise is about the frame that follows it, so the gate is the mode the
// next View asks for — in the same session, with no restart to hide a stale value.
func TestMouseTrackingToggleHandsThePointerBackLive(t *testing.T) {
	presets.Load()
	path := filepath.Join(t.TempDir(), "prefs")
	p := tuicfg.Default()
	p.MouseEnabled = true
	m := app.New(app.Options{
		Store:     &arcthumb.MemoryStore{},
		Prefs:     p,
		PrefsPath: path,
		Width:     120,
		Height:    36,
	})

	if !m.View().AltScreen {
		t.Fatal("the front end does not ask for the alternate screen")
	}
	if got := m.View().MouseMode; got != tea.MouseModeAllMotion {
		// Without this the "off" assertion below could pass on a model that was
		// never tracking motion in the first place.
		t.Fatalf("mouse tracking on asks for mode %v, want all motion", got)
	}

	m.ShowPage("general")
	m.View()
	located := false
	for i := 0; i < 24 && !located; i++ {
		_, rect, ok := m.FindTargetN(components.HitRow, "general", i)
		if !ok {
			break
		}
		m.Update(tea.MouseClickMsg{X: rect.X + 1, Y: rect.Y, Button: tea.MouseLeft})
		located = strings.Contains(m.InspectorText(), "native text selection")
	}
	if !located {
		t.Fatal("no row on the general page is the mouse tracking switch")
	}

	updateKey(m, tea.KeyEnter)
	if m.PrefsView().MouseEnabled {
		t.Fatal("Enter did not flip the mouse tracking switch")
	}
	if got := m.View().MouseMode; got != tea.MouseModeNone {
		t.Errorf("after turning tracking off the frame still asks for mode %v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the switch did not persist: %v", err)
	}
	if tuicfg.Parse(string(data)).MouseEnabled {
		t.Error("the preferences file still has mouse tracking on")
	}

	updateKey(m, tea.KeyEnter)
	if got := m.View().MouseMode; got != tea.MouseModeAllMotion {
		t.Errorf("turning tracking back on asks for mode %v, want all motion", got)
	}
}

// TestSidebarSwitchHidesTheRail is the readback for the rail toggle: the setting
// is sold as "the rail sits beside the pane on a wide terminal", so turning it
// off in a wide terminal has to remove the rail and hand navigation to the tabs,
// not merely repaint the lamp.
func TestSidebarSwitchHidesTheRail(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 36)
	m.View()
	if _, _, ok := m.FindTarget(components.HitNav, "nav"); !ok {
		t.Fatal("the default state registers no rail, so the comparison below is vacuous")
	}
	if _, _, ok := m.FindTarget(components.HitTab, "nav"); ok {
		t.Fatal("a wide terminal with the rail on must not also show folded tabs")
	}

	m.ShowPage("general")
	m.View()
	located := false
	for i := 0; i < 24 && !located; i++ {
		_, rect, ok := m.FindTargetN(components.HitRow, "general", i)
		if !ok {
			break
		}
		m.Update(tea.MouseClickMsg{X: rect.X + 1, Y: rect.Y, Button: tea.MouseLeft})
		located = strings.Contains(m.InspectorText(), "folds into tabs")
	}
	if !located {
		t.Fatal("no row on the general page is the navigation rail switch")
	}

	updateKey(m, tea.KeyEnter)
	if m.PrefsView().ShowSidebar {
		t.Fatal("Enter did not flip the rail switch")
	}
	m.View()
	if _, _, ok := m.FindTarget(components.HitNav, "nav"); ok {
		t.Error("the rail is still clickable after the switch turned it off")
	}
	if _, _, ok := m.FindTarget(components.HitTab, "nav"); !ok {
		t.Error("folding the rail did not hand navigation to the tab strip")
	}

	updateKey(m, tea.KeyEnter)
	m.View()
	if _, _, ok := m.FindTarget(components.HitNav, "nav"); !ok {
		t.Error("the rail did not come back when the switch was turned on again")
	}
}

// TestLastPageSurvivesARestart checks the only claim the front end makes about
// where the user was: quitting from a screen must put them back on that screen,
// which means the value has to travel through the preferences file rather than
// live only in the model that is about to be thrown away.
func TestLastPageSurvivesARestart(t *testing.T) {
	presets.Load()
	path := filepath.Join(t.TempDir(), "prefs")
	first := app.New(app.Options{
		Store:     &arcthumb.MemoryStore{},
		Prefs:     tuicfg.Default(),
		PrefsPath: path,
		Width:     120,
		Height:    36,
	})
	if got := first.CurrentPage(); got != "dashboard" {
		t.Fatalf("a fresh front end starts on %q, want dashboard", got)
	}

	// Navigate the way a user does: ShowPage is the --render back door and
	// deliberately skips the bookkeeping, so only a rail click proves the front
	// end records where it left the user.
	railIndex := -1
	for i, id := range first.PageIDs() {
		if id == "formats" {
			railIndex = i
		}
	}
	if railIndex < 0 {
		t.Fatal("formats is not in the navigation order")
	}
	first.View()
	target, rect, ok := first.FindTargetN(components.HitNav, "nav", railIndex)
	if !ok {
		t.Fatal("the rail has no row for formats")
	}
	if target.Index != railIndex {
		t.Fatalf("rail row %d carries index %d", railIndex, target.Index)
	}
	first.Update(tea.MouseClickMsg{X: rect.X + 1, Y: rect.Y, Button: tea.MouseLeft})
	if got := first.CurrentPage(); got != "formats" {
		t.Fatalf("clicking the rail landed on %q, want formats", got)
	}
	// Quitting with nothing pending writes the preferences on the spot.
	if _, cmd := first.Update(tea.KeyPressMsg{Code: rune('q'), Text: "q"}); cmd == nil {
		t.Fatal("q did not ask the program to quit")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("quitting wrote no preferences: %v", err)
	}
	if got := tuicfg.Parse(string(data)).LastPage; got != "formats" {
		t.Fatalf("preferences remember %q as the last page, want formats", got)
	}

	second := app.New(app.Options{
		Store:     &arcthumb.MemoryStore{},
		Prefs:     tuicfg.Parse(string(data)),
		PrefsPath: path,
		Width:     120,
		Height:    36,
	})
	if got := second.CurrentPage(); got != "formats" {
		t.Errorf("a restart from the same file lands on %q, want formats", got)
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

// TestEveryBriefKeyHasAnObservableEffect is §6's keyboard list as a gate. The
// keys are named one by one in the brief, so each one is pressed against a fresh
// session and the readback is the state it claims to drive — not "the action was
// non-empty", which would pass for a binding that goes nowhere.
func TestEveryBriefKeyHasAnObservableEffect(t *testing.T) {
	cases := []struct {
		name  string
		page  string
		setup func(t *testing.T, m *app.Model)
		key   func(m *app.Model) tea.Cmd
		fresh func(m *app.Model) string
		want  string
	}{
		{
			name: "Down moves focus", page: "thumbnail",
			fresh: func(m *app.Model) string { return m.InspectorText() },
			key:   func(m *app.Model) tea.Cmd { return updateKey(m, tea.KeyDown) },
		},
		{
			name: "Up moves focus back", page: "thumbnail",
			fresh: func(m *app.Model) string { return m.InspectorText() },
			key:   func(m *app.Model) tea.Cmd { return updateKey(m, tea.KeyUp) },
		},
		{
			name: "Right steps the focused selector", page: "general",
			fresh: func(m *app.Model) string { return m.PrefsView().ThemeID },
			key:   func(m *app.Model) tea.Cmd { return updateKey(m, tea.KeyRight) },
		},
		{
			name: "Left steps it back", page: "general",
			setup: func(_ *testing.T, m *app.Model) { updateKey(m, tea.KeyRight) },
			fresh: func(m *app.Model) string { return m.PrefsView().ThemeID },
			key:   func(m *app.Model) tea.Cmd { return updateKey(m, tea.KeyLeft) },
		},
		{
			name: "Tab changes page", page: "dashboard",
			fresh: func(m *app.Model) string { return m.CurrentPage() },
			key:   func(m *app.Model) tea.Cmd { return updateKey(m, tea.KeyTab) },
		},
		{
			name: "Shift+Tab changes page the other way", page: "dashboard",
			fresh: func(m *app.Model) string { return m.CurrentPage() },
			key: func(m *app.Model) tea.Cmd {
				m.View()
				_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
				return cmd
			},
		},
		{
			name: "Enter edits the focused control", page: "thumbnail",
			fresh: func(m *app.Model) string { return arcthumb.Marshal(m.SettingsView()) },
			key:   func(m *app.Model) tea.Cmd { return updateKey(m, tea.KeyEnter) },
		},
		{
			name: "Space edits it the same way", page: "thumbnail",
			fresh: func(m *app.Model) string { return arcthumb.Marshal(m.SettingsView()) },
			key:   func(m *app.Model) tea.Cmd { return updateKey(m, ' ') },
		},
		{
			name: "Esc dismisses a dialog", page: "general",
			setup: func(_ *testing.T, m *app.Model) { m.OpenDialog("help"); m.View() },
			fresh: func(m *app.Model) string {
				if m.DialogOpen() {
					return "open"
				}
				return "closed"
			},
			want: "open",
			key:  func(m *app.Model) tea.Cmd { return updateKey(m, tea.KeyEsc) },
		},
		{
			name: "q asks before leaving with unsaved edits", page: "thumbnail",
			setup: func(t *testing.T, m *app.Model) {
				_, rect, ok := m.FirstControl()
				if !ok {
					t.Fatal("no control to edit")
				}
				m.View()
				m.Update(tea.MouseClickMsg{X: rect.X, Y: rect.Y, Button: tea.MouseLeft})
				// Editing a front-end preference is not an unsaved change, so the
				// premise has to be checked or q is correctly silent and the case
				// would pass while proving nothing.
				if !m.Dirty() {
					t.Fatal("precondition: the click left the session clean")
				}
			},
			fresh: func(m *app.Model) string {
				if m.DialogOpen() {
					return "open"
				}
				return "closed"
			},
			want: "closed",
			key:  func(m *app.Model) tea.Cmd { return updateKey(m, 'q') },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
			m.ShowPage(tc.page)
			m.View()
			if tc.setup != nil {
				tc.setup(t, m)
				m.View()
			}
			before := tc.fresh(m)
			// The readback has to start where the case claims it does, or the
			// "it changed" assertion below could be satisfied by anything.
			if tc.want != "" && before != tc.want {
				t.Fatalf("precondition: readback was %q, want %q", before, tc.want)
			}
			tc.key(m)
			m.View()
			if after := tc.fresh(m); after == before {
				t.Errorf("%s: state stayed %q, so the key is bound but goes nowhere", tc.name, before)
			}
		})
	}
}

// TestEnterAndSpaceAreEquivalent pins the primary-interaction pair from §11: the
// same focused control must land in the same state whichever of the two is used.
func TestEnterAndSpaceAreEquivalent(t *testing.T) {
	press := func(k rune) string {
		m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
		m.ShowPage("thumbnail")
		m.View()
		if _, _, ok := m.FirstControl(); !ok {
			t.Fatal("no control")
		}
		m.Update(tea.KeyPressMsg{Code: k, Text: string(k)})
		m.View()
		return arcthumb.Marshal(m.SettingsView())
	}
	if a, b := press(tea.KeyEnter), press(' '); a != b {
		t.Errorf("Enter left %q, Space left %q: two primary keys, two behaviours", a, b)
	}
}

// TestClickOnTabAndRailDoWhatTheyLookLike covers the last two §6 clickables that
// only had "a target was registered" evidence. A hit map can be perfectly
// populated and still address the wrong row, so the assertion is the state the
// gesture claims to cause: the tab strip switches which half of the extension
// mask is on screen, and a rail row switches the page.
func TestClickOnTabAndRailDoWhatTheyLookLike(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	m.ShowPage("formats")
	m.View()

	before := m.InspectorText()
	if !strings.Contains(before, "Images") {
		t.Fatalf("formats opens on the image half, readback was %q", before)
	}
	target, rect, ok := m.FindTargetN(components.HitTab, "formats:tabs", 1)
	if !ok {
		t.Fatal("the tab strip registered no clickable second tab")
	}
	if target.Index != 1 {
		t.Fatalf("second tab target index = %d", target.Index)
	}
	m.Update(tea.MouseClickMsg{X: rect.X + 1, Y: rect.Y, Button: tea.MouseLeft})
	m.View()
	after := m.InspectorText()
	if after == before {
		t.Fatal("clicking the Containers tab changed nothing")
	}
	if !strings.Contains(after, "Containers") {
		t.Errorf("readback after the tab click is %q", after)
	}

	// Positive control: the tab already under the cursor must not "change" when
	// clicked, or the pair of assertions above could be satisfied by any click.
	m.Update(tea.MouseClickMsg{X: rect.X + 1, Y: rect.Y, Button: tea.MouseLeft})
	m.View()
	if again := m.InspectorText(); again != after {
		t.Errorf("re-clicking the active tab moved it from %q to %q", after, again)
	}

	navTarget, navRect, ok := m.FindTarget(components.HitNav, "nav")
	if !ok {
		t.Fatal("the rail registered no clickable row")
	}
	if navTarget.Index != 0 {
		t.Fatalf("first rail row index = %d, want the page it is painted for", navTarget.Index)
	}
	m.Update(tea.MouseClickMsg{X: navRect.X + 1, Y: navRect.Y, Button: tea.MouseLeft})
	m.View()
	if got := m.CurrentPage(); got != "dashboard" {
		t.Errorf("clicking the first rail row landed on %q, want dashboard", got)
	}
}

// TestPanelLanguageCyclesInPanelKeycapOrder is the readback for the shared-file
// locale selector: what the user reads, what gets written, and the order the
// Rust panel's own keycaps use must be one fact, not three hand-made copies.
func TestPanelLanguageCyclesInPanelKeycapOrder(t *testing.T) {
	m := newModel(t, &arcthumb.MemoryStore{}, 120, 40)
	m.ShowPage("general")

	located := false
	for i := 0; i < 24 && !located; i++ {
		m.View()
		_, rect, ok := m.FindTargetN(components.HitRow, "general", i)
		if !ok {
			break
		}
		m.Update(tea.MouseClickMsg{X: rect.X + 1, Y: rect.Y, Button: tea.MouseLeft})
		located = strings.Contains(m.InspectorText(), "Keycaps and labels")
	}
	if !located {
		t.Fatal("no row on the general page offers the panel's language keycaps")
	}
	if got := m.SettingsView().Language; got != "" {
		t.Fatalf("a fresh store starts with language=%q, want unset", got)
	}
	if m.Dirty() {
		t.Fatal("focusing rows made the model dirty; a click must not edit")
	}

	// 中文 leads because the panel puts its own first keycap there, and the tag
	// written is the one Rust's Locale::from_tag resolves.
	for _, want := range []struct{ tag, label string }{{"zh", "中文"}, {"en", "English"}, {"ja", "日本語"}} {
		updateKey(m, tea.KeyRight)
		if got := m.SettingsView().Language; got != want.tag {
			t.Fatalf("keycap after unset = %q, want %q", got, want.tag)
		}
		if frame := render.Strip(m.View().Content); !strings.Contains(frame, want.label) {
			t.Errorf("the row never repainted to %q for tag %q", want.label, want.tag)
		}
	}

	updateKey(m, tea.KeyLeft)
	if got := m.SettingsView().Language; got != "en" {
		t.Errorf("stepping back landed on %q, want en", got)
	}
	if out := arcthumb.Marshal(m.SettingsView()); !strings.Contains(out, "language = en") {
		t.Errorf("the persisted form lost the key:\n%s", out)
	}

	// The unset keycap must write nothing at all, or the panel would read an
	// explicit choice the user never made.
	for i := 0; i < 4; i++ {
		updateKey(m, tea.KeyLeft)
	}
	if got := m.SettingsView().Language; got != "" {
		t.Fatalf("walking back to the first slot = %q, want the unset sentinel", got)
	}
	if out := arcthumb.Marshal(m.SettingsView()); strings.Contains(out, "language") {
		t.Errorf("the unset slot wrote a language key:\n%s", out)
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
