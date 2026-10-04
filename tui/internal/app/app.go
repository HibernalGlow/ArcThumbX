// Package app is the Bubble Tea model: it owns state, translates keys and mouse
// events into page intents, and assembles the shell.
//
// Nothing here draws. Pages compose components, components read theme tokens,
// and this package only decides what happens next.
package app

import (
	"os"
	"runtime"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/keymap"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/pages"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/tuicfg"
)

// Options is how main hands the model its world. Everything is injectable so a
// test or --render can build a frame without touching a user's files.
type Options struct {
	Store     arcthumb.Store
	Prefs     tuicfg.Settings
	PrefsPath string
	Version   string
	// ReadOnly refuses writes even when the backend would accept them.
	ReadOnly bool
	Width    int
	Height   int
}

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogQuit
	dialogHelp
	dialogThemes
	dialogMessage
)

// Model is the application state.
type Model struct {
	env pages.Env

	t    *theme.Theme
	pg   []pages.Page
	page int

	store     arcthumb.Store
	prefs     tuicfg.Settings
	prefsPath string
	settings  arcthumb.Settings
	live      arcthumb.Settings
	ignored   arcthumb.Ignored
	readOnly  bool
	version   string

	width, height int
	ready         bool
	hits          *layout.HitMap

	dialog    *components.Dialog
	kind      dialogKind
	themeList *components.List

	notice     string
	noticeKind components.State
}

// SettingsView and PrefsView expose the working copies read-only, so a test can
// assert that a click or a key produced a real change rather than a repaint.
func (m *Model) SettingsView() arcthumb.Settings { return m.settings }

// PrefsView is the front-end preference in force.
func (m *Model) PrefsView() tuicfg.Settings { return m.prefs }

// FirstControl finds the value-cell hit target at the given row index, so a test
// can click the same cell a user would instead of hard-coding coordinates that
// the layout owns.
func (m *Model) FirstControl() (components.Target, layout.Rect, bool) {
	for _, e := range m.hits.All() {
		if t, ok := e.Data.(components.Target); ok && t.Kind == components.HitControl {
			return t, e.Rect, true
		}
	}
	return components.Target{}, layout.Rect{}, false
}

// FindTarget locates a registered target by kind and ID.
func (m *Model) FindTarget(kind components.HitKind, id string) (components.Target, layout.Rect, bool) {
	for _, e := range m.hits.All() {
		if t, ok := e.Data.(components.Target); ok && t.Kind == kind && t.ID == id {
			return t, e.Rect, true
		}
	}
	return components.Target{}, layout.Rect{}, false
}

// FindTargetN is FindTarget for the n-th registration of an ID. A list registers
// every row under the same ID, so aiming at a specific row — the one furthest
// from where a guessed rect would put it — needs this.
func (m *Model) FindTargetN(kind components.HitKind, id string, n int) (components.Target, layout.Rect, bool) {
	for _, e := range m.hits.All() {
		if t, ok := e.Data.(components.Target); ok && t.Kind == kind && t.ID == id && t.Index == n {
			return t, e.Rect, true
		}
	}
	return components.Target{}, layout.Rect{}, false
}

// ShowPage selects a page by ID, for --render and for tests. An unknown ID keeps
// the current page rather than falling back silently, so a typo shows up as the
// frame the caller asked not to get.
func (m *Model) ShowPage(id string) {
	for i, p := range m.pg {
		if p.ID() == id {
			m.page = i
			return
		}
	}
}

// PageIDs lists the screens in navigation order.
func (m *Model) PageIDs() []string {
	out := make([]string, 0, len(m.pg))
	for _, p := range m.pg {
		out = append(out, p.ID())
	}
	return out
}

// HitTargets reports how many interactive regions the last frame registered. A
// frame with none would still look correct, so this is the number a mouse test
// asserts on.
func (m *Model) HitTargets() int { return m.hits.Len() }

// DialogOpen reports whether a modal is currently taking the screen.
func (m *Model) DialogOpen() bool { return m.dialog != nil }

// CurrentPage is the visible page ID.
func (m *Model) CurrentPage() string { return m.pg[m.page].ID() }

// Notice is the transient inspector message, exposed so a test can prove a save
// or a refusal was reported rather than swallowed.
func (m *Model) Notice() string { return m.notice }

// InspectorText is the readout for whatever currently holds focus. Focus has to
// be observable somewhere other than an internal counter for a hover to be
// provably equivalent to an arrow key, and this line is that readback.
func (m *Model) InspectorText() string { return m.current().Inspector(m.t) }

// New builds the model and performs the first read from the store. A failure to
// read is shown on the Dashboard rather than treated as fatal: a corrupt settings
// file must not prevent someone from opening the panel and fixing it.
func New(o Options) *Model {
	prefs := o.Prefs.Normalize()
	m := &Model{
		t:         theme.MustMake(theme.PresetID(prefs.ThemeID)),
		prefs:     prefs,
		prefsPath: o.PrefsPath,
		store:     o.Store,
		readOnly:  o.ReadOnly,
		version:   o.Version,
		width:     max(o.Width, 20),
		height:    max(o.Height, 10),
		hits:      layout.NewHitMap(),
	}
	if m.store == nil {
		m.store = &arcthumb.MemoryStore{}
	}
	s, ig, err := m.store.Load()
	if err != nil {
		m.notice = "could not read settings: " + err.Error()
		m.noticeKind = components.StateError
		s = arcthumb.Default()
	}
	m.settings = s.Clamp()
	m.live = s.Clamp()
	m.ignored = ig

	m.env = pages.Env{
		Settings: &m.settings,
		Prefs:    &m.prefs,
		Live:     m.live,
		App:      m,
	}
	m.pg = pages.All(&m.env)
	m.page = pages.IndexOf(m.pg, prefs.LastPage)
	return m
}

// Theme exposes the compiled preset, which is all a page is allowed to know about
// the renderer.
func (m *Model) Theme() *theme.Theme { return m.t }

// Dirty reports unsaved ArcThumbX edits by comparing the working copy against the
// last persisted snapshot, so the flag cannot drift from reality.
func (m *Model) Dirty() bool { return m.settings != m.live }

// Init sends the first size report. Bubble Tea delivers a real WindowSizeMsg as
// soon as the terminal answers, so nothing else is needed here.
func (m *Model) Init() tea.Cmd { return nil }

// Update is the whole event surface.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(msg.Width, 20)
		m.height = max(msg.Height, 8)
		m.ready = true
		return m, nil

	case tea.KeyPressMsg:
		return m, m.key(keymap.Default().Resolve(msg))

	case tea.MouseClickMsg:
		m.mouse(msg.X, msg.Y, true)
		return m, nil

	case tea.MouseMotionMsg:
		// Motion with a button held is a drag: the pointer is sweeping the panel,
		// so whatever it crosses is acted on. Motion without a button is a hover
		// and only moves focus, which is what makes the mouse a real second
		// input rather than a slower way to click.
		m.mouse(msg.X, msg.Y, msg.Button != tea.MouseNone)
		return m, nil

	case tea.MouseReleaseMsg:
		return m, nil

	case tea.MouseWheelMsg:
		dir := 1
		if msg.Button == tea.MouseWheelUp {
			dir = -3
		}
		if _, _, ok := m.hits.Pick(msg.X, msg.Y); ok {
			m.act(pageAction(dir))
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) key(a keymap.Action) tea.Cmd {
	if m.dialog != nil {
		return m.dialogKey(a)
	}
	switch a {
	case keymap.ActionQuit:
		return m.askQuit()
	case keymap.ActionHelp:
		m.openHelp()
		return nil
	case keymap.ActionThemes:
		m.openThemes()
		return nil
	case keymap.ActionPageNext:
		m.setPage(m.page + 1)
		return nil
	case keymap.ActionPagePrev:
		m.setPage(m.page - 1)
		return nil
	case keymap.ActionClose:
		m.notice = ""
		return nil
	case keymap.ActionSave:
		m.save()
		return nil
	case keymap.ActionRevert:
		m.revert()
		return nil
	default:
		m.act(pageActionForKey(a))
		return nil
	}
}

func pageActionForKey(a keymap.Action) pages.Action {
	switch a {
	case keymap.ActionFocusNext:
		return pages.ActionFocusNext
	case keymap.ActionFocusPrev:
		return pages.ActionFocusPrev
	case keymap.ActionStepBack:
		return pages.ActionStepBack
	case keymap.ActionStepFwd:
		return pages.ActionStepFwd
	case keymap.ActionActivate:
		return pages.ActionActivate
	case keymap.ActionScrollUp:
		return pages.ActionScrollUp
	case keymap.ActionScrollDown:
		return pages.ActionScrollDown
	default:
		return pages.ActionActivate
	}
}

func pageAction(dir int) pages.Action {
	if dir < 0 {
		return pages.ActionScrollUp
	}
	return pages.ActionScrollDown
}

func (m *Model) dialogKey(a keymap.Action) tea.Cmd {
	// The theme picker is a list, so the arrows move a cursor; every other
	// dialog is a set of answers, so the same keys cycle the highlight.
	list := m.kind == dialogThemes && m.themeList != nil
	switch a {
	case keymap.ActionClose:
		m.closeDialog()
		return nil
	case keymap.ActionFocusNext:
		if list {
			m.themeList.Move(1)
		} else {
			m.dialog.Move(1)
		}
		return nil
	case keymap.ActionFocusPrev:
		if list {
			m.themeList.Move(-1)
		} else {
			m.dialog.Move(-1)
		}
		return nil
	case keymap.ActionPageNext, keymap.ActionStepFwd:
		if !list {
			m.dialog.Move(1)
		}
		return nil
	case keymap.ActionPagePrev, keymap.ActionStepBack:
		if !list {
			m.dialog.Move(-1)
		}
		return nil
	case keymap.ActionActivate:
		return m.commitDialog()
	}
	return nil
}

func (m *Model) commitDialog() tea.Cmd {
	switch m.kind {
	case dialogThemes:
		if m.themeList != nil {
			if item := m.themeList.Current(); item != nil {
				m.applyPreset(item.Tag)
			}
		}
		m.closeDialog()
		return nil
	case dialogQuit:
		i, ok := m.dialog.Activate()
		id := ""
		if ok && i < len(m.dialog.Buttons) {
			id = m.dialog.Buttons[i].ID
		}
		m.closeDialog()
		switch id {
		case "save":
			m.save()
			// Only leave when the working copy actually matches disk again;
			// a refused write must not be reported as a saved exit.
			if !m.Dirty() {
				return m.persistAndQuit()
			}
			return nil
		case "discard":
			return m.persistAndQuit()
		default:
			return nil
		}
	default:
		m.closeDialog()
		return nil
	}
}

func (m *Model) closeDialog() {
	m.dialog = nil
	m.kind = dialogNone
	m.themeList = nil
}

// mouse resolves a cell against the hit map left by the previous frame and
// dispatches it. Registration order is paint order, so a dialog's buttons win
// over anything drawn beneath them.
func (m *Model) mouse(x, y int, press bool) {
	id, data, ok := m.hits.Pick(x, y)
	if !ok {
		return
	}
	_ = id
	target, valid := data.(components.Target)
	if !valid {
		return
	}
	switch target.Kind {
	case components.HitButton:
		if press {
			m.command(target.ID)
		}
	case components.HitNav:
		if press {
			m.setPage(target.Index)
		}
	case components.HitTab:
		m.current().Mouse(target, press)
	case components.HitListItem:
		// Inside the preset picker a row is the option, so clicking it moves the
		// cursor the way ↑↓ do; Apply then commits whatever is under it.
		if press && m.themeList != nil && target.ID == m.themeList.ID {
			m.themeList.Set(target.Index)
		}
	default:
		if m.dialog != nil {
			return
		}
		m.current().Mouse(target, press)
	}
}

// command is the shared handler for every button and help-bar target, so a
// click and its key equivalent cannot drift apart.
func (m *Model) command(id string) {
	// A modal's answers are buttons too, but what they mean is "highlight this
	// answer, then press Enter". Routing them through the dialog's own commit is
	// the only way a click and the keyboard can't drift apart: as plain IDs they
	// would need a case per dialog, and Apply/Save/Discard would silently no-op.
	if m.dialog != nil {
		if i := m.dialog.IndexOf(id); i >= 0 {
			m.dialog.Selected = i
			m.commitDialog()
			return
		}
	}
	switch id {
	case "quit":
		m.askQuit()
	case "help", "help:edit":
		if id == "help:edit" {
			m.act(pages.ActionActivate)
			return
		}
		m.openHelp()
	case "help:page":
		m.setPage(m.page + 1)
	case "theme":
		m.openThemes()
	case "save":
		m.save()
	case "revert":
		m.revert()
	case "close":
		m.closeDialog()
	}
}

func (m *Model) current() pages.Page { return m.pg[m.page] }

func (m *Model) setPage(i int) {
	n := len(m.pg)
	if n == 0 {
		return
	}
	i %= n
	if i < 0 {
		i += n
	}
	m.page = i
	m.prefs.LastPage = m.pg[i].ID()
	m.notice = ""
}

func (m *Model) act(a pages.Action) {
	if m.dialog != nil {
		return
	}
	m.current().Act(a)
}

func (m *Model) askQuit() tea.Cmd {
	if !m.Dirty() || !m.prefs.ConfirmOnQuit {
		return m.persistAndQuit()
	}
	m.kind = dialogQuit
	m.dialog = &components.Dialog{
		ID:    "quit",
		Title: "UNSAVED CHANGES",
		Lines: []string{
			"ArcThumbX settings were edited but not written.",
			"Writing them changes what the shell extension reads next request.",
		},
		Buttons: []components.Button{
			{ID: "save", Label: "Save", Primary: m.canWrite(), Enabled: m.canWrite()},
			{ID: "discard", Label: "Quit anyway", Enabled: true},
			{ID: "close", Label: "Stay", Enabled: true},
		},
		Selected: 1,
	}
	return nil
}

func (m *Model) canWrite() bool {
	return !m.readOnly && m.storeWriteOK()
}

func (m *Model) storeWriteOK() bool {
	// The memory fixture is only writable when a test opted in, and a registry
	// backend refuses by definition. Probing with a nil call keeps the check
	// honest without side effects.
	_, ok := m.store.(*arcthumb.FileStore)
	if ok {
		return true
	}
	mem, isMem := m.store.(*arcthumb.MemoryStore)
	return isMem && mem.Writable
}

func (m *Model) save() {
	if !m.canWrite() {
		m.notice = "this backend cannot be written from the TUI"
		m.noticeKind = components.StateWarn
		return
	}
	err := m.store.Save(m.settings)
	if err != nil {
		m.notice = "write failed: " + err.Error()
		m.noticeKind = components.StateError
		return
	}
	m.live = m.settings
	m.env.Live = m.live
	m.notice = "settings written to " + m.store.Location()
	m.noticeKind = components.StateOK
}

func (m *Model) revert() {
	if !m.Dirty() {
		m.notice = "nothing to revert"
		m.noticeKind = components.StateInfo
		return
	}
	m.settings = m.live
	m.notice = "edits discarded"
	m.noticeKind = components.StateInfo
}

// applyPreset recompiles the theme and persists the preference at once: a visual
// change that needed a save step would be indistinguishable from one that was
// ignored.
func (m *Model) applyPreset(id string) {
	if id == "" {
		return
	}
	m.prefs.ThemeID = id
	m.env.Prefs = &m.prefs
	m.t = theme.MustMake(theme.PresetID(id))
	m.Request(pages.IntentPersist)
}

// Request is the page-to-app command channel.
func (m *Model) Request(i pages.Intent) {
	switch i {
	case pages.IntentSave:
		m.save()
	case pages.IntentRevert:
		m.revert()
	case pages.IntentThemes:
		m.openThemes()
	case pages.IntentPersist:
		m.t = theme.MustMake(theme.PresetID(m.prefs.ThemeID))
		if m.prefsPath != "" {
			// SaveAt, never Save: the app must write back to the file it loaded,
			// including one chosen by --settings.
			if err := tuicfg.SaveAt(m.prefsPath, m.prefs); err != nil {
				m.notice = "preferences not saved: " + err.Error()
				m.noticeKind = components.StateError
			}
		}
	}
}

func (m *Model) persistAndQuit() tea.Cmd {
	if m.prefsPath != "" {
		_ = tuicfg.SaveAt(m.prefsPath, m.prefs)
	}
	return tea.Quit
}

// OpenDialog shows one of the app's modals on demand. It exists so --render and
// the tests can look at an overlay without replaying the key path that opens
// it, and it returns false for an unknown name rather than guessing.
func (m *Model) OpenDialog(id string) bool {
	switch id {
	case "themes":
		m.openThemes()
		return true
	case "help":
		m.openHelp()
		return true
	case "quit":
		m.askQuit()
		return true
	default:
		return false
	}
}

// openHelp shows the binding reference, built from the same table the help bar
// and the About page use so the three can never disagree.
func (m *Model) openHelp() {
	m.kind = dialogHelp
	items := keymap.Reference()
	lines := make([]string, 0, len(items))
	for _, it := range items {
		lines = append(lines, m.t.Components.ListItem.Render(
			layout.Fit(it.Label, 18))+"  "+
			m.t.Components.ValueFlag.Render(layout.Fit(it.Value, 14))+"  "+
			m.t.Components.Caption.Render(it.Detail))
	}
	m.dialog = &components.Dialog{
		ID:    "help",
		Title: "KEYS AND MOUSE",
		Lines: lines,
		Buttons: []components.Button{
			{ID: "close", Label: "Close", Primary: true, Enabled: true},
		},
	}
}

// openThemes lists every registered preset with its blurb, marking the active
// one. Adding a preset file makes it appear here without touching this code.
func (m *Model) openThemes() {
	entries := theme.Registered()
	items := make([]components.ListItem, 0, len(entries))
	cursor := 0
	for i, e := range entries {
		mark := ""
		if string(e.ID) == m.prefs.ThemeID {
			cursor = i
			mark = "active"
		}
		items = append(items, components.ListItem{
			Label:  e.Name,
			Value:  mark,
			Detail: e.Blurb,
			Tag:    string(e.ID),
		})
	}
	m.kind = dialogThemes
	m.themeList = &components.List{
		ID:         "themes",
		Title:      "PRESETS",
		Items:      items,
		Cursor:     cursor,
		ValueWidth: 8,
	}
	m.dialog = &components.Dialog{
		ID:         "themes",
		Title:      "THEME PRESET",
		Body:       m.themeBody,
		BodyHeight: len(items) + 1,
		Buttons: []components.Button{
			{ID: "apply", Label: "Apply", Primary: true, Enabled: true},
			{ID: "close", Label: "Cancel", Enabled: true},
		},
	}
}

func (m *Model) themeBody(c components.Ctx) string {
	return m.themeList.View(c)
}

// View assembles the frame. It rebuilds the hit map from scratch every frame so
// a click can only ever resolve against the geometry currently on screen.
func (m *Model) View() tea.View {
	m.hits.Reset()
	m.refreshFacts()

	page := m.current()
	shell := components.Shell{
		Header: components.Header{
			Brand:  "ARC" + m.t.Glyphs.Link + "THUMB",
			Suffix: "X",
			Mode:   "CONFIG",
			Meta: []string{
				m.t.Components.HeaderMeta.Render(m.t.Glyphs.Terminal) + " " + m.t.Name,
				strconv.Itoa(m.width) + "x" + strconv.Itoa(m.height),
				m.store.Kind(),
				m.version,
			},
		},
		ShowNav: m.prefs.ShowSidebar,
		Nav: &components.Sidebar{
			ID:     "nav",
			Items:  m.navItems(),
			Active: m.page,
		},
		Folded: &components.Tabs{
			ID:     "nav",
			Labels: m.navLabels(),
			Active: m.page,
		},
		Inspector: components.Inspector{
			Text:    m.inspectorText(page),
			Right:   page.Progress(m.t),
			Badges:  m.badges(),
			Buttons: m.actionBar(),
		},
		Help: components.HelpBar{ID: "help", Items: keymap.GlobalHelp()},
		Content: func(c components.Ctx) string {
			return page.Render(c)
		},
		Overlay: m.dialog,
	}

	content := shell.View(components.Ctx{
		T:    m.t,
		Rect: layout.R(0, 0, m.width, m.height),
		Hits: m.hits,
	})

	v := tea.NewView(content)
	v.AltScreen = true
	if m.prefs.MouseEnabled {
		// All motion, not just cell motion: hover has to be a real event or the
		// pointer is second-class the moment the user stops dragging.
		v.MouseMode = tea.MouseModeAllMotion
	} else {
		v.MouseMode = tea.MouseModeNone
	}
	return v
}

func (m *Model) navItems() []components.NavItem {
	out := make([]components.NavItem, 0, len(m.pg))
	for _, p := range m.pg {
		out = append(out, components.NavItem{
			ID:    p.ID(),
			Label: p.Label(),
			Alert: m.Dirty() && sharesSettings(p.ID()),
		})
	}
	return out
}

func (m *Model) navLabels() []string {
	out := make([]string, 0, len(m.pg))
	for _, p := range m.pg {
		out = append(out, p.Label())
	}
	return out
}

// sharesSettings marks the pages whose rows write the ArcThumbX file, so the rail's
// unsaved dot points at where the edits actually are.
func sharesSettings(id string) bool {
	switch id {
	case "thumbnail", "formats", "integration":
		return true
	default:
		return false
	}
}

func (m *Model) inspectorText(p pages.Page) string {
	if m.notice != "" {
		return m.notice
	}
	return p.Inspector(m.t)
}

func (m *Model) badges() string {
	out := ""
	if m.Dirty() {
		out += m.t.Components.StatusWarn.Render(m.t.Glyphs.On + " unsaved")
	}
	if !m.ignored.Empty() && !m.Dirty() {
		out += m.t.Components.StatusWarn.Render(m.t.Glyphs.Bullet + " " +
			strconv.Itoa(len(m.ignored.UnknownKeys)+len(m.ignored.Malformed)) + " lines ignored")
	}
	return out
}

func (m *Model) actionBar() *components.ButtonBar {
	if !m.Dirty() {
		return nil
	}
	return &components.ButtonBar{ID: "actions", Buttons: []components.Button{
		{ID: "save", Label: "Save", Primary: true, Enabled: m.canWrite()},
		{ID: "revert", Label: "Revert", Enabled: true},
	}}
}

// refreshFacts rebuilds the disclosures every frame: the terminal size changes,
// and after a save the dirty state and backend error do too.
func (m *Model) refreshFacts() {
	f := pages.Facts{
		StoreKind:      m.store.Kind(),
		StoreLocation:  m.store.Location(),
		StoreReadable:  true,
		StoreWritable:  m.canWrite(),
		IgnoredKeys:    m.ignored.UnknownKeys,
		MalformedLines: m.ignored.Malformed,
		PrefsLocation:  m.prefsPath,
		PresetID:       theme.PresetID(m.prefs.ThemeID),
		PresetName:     m.t.Name,
		Profile:        colorDepth(),
		Version:        m.version,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		GoVersion:      goVersion(),
		TermCols:       m.width,
		TermRows:       m.height,
		MouseOn:        m.prefs.MouseEnabled,
	}
	if _, isReg := m.store.(*arcthumb.RegistryStore); isReg {
		f.StoreReadable = false
		f.BackendError = "registry backend is not implemented in this build"
	}
	if _, isMem := m.store.(*arcthumb.MemoryStore); isMem {
		f.BackendError = "offline fixture: nothing is read from disk"
	}
	// Only a macOS file store can ask the host to drop its thumbnail cache, and
	// the TUI does not offer an action it cannot perform.
	if _, isFile := m.store.(*arcthumb.FileStore); isFile {
		_, f.Regenerable = m.store.(arcthumb.Regenerator)
		f.Regenerable = f.Regenerable && runtime.GOOS == "darwin"
	}
	m.env.Facts = f
	m.env.Live = m.live
}

// colorDepth reports what the terminal claims rather than what the renderer
// picked internally, because that is the number a user can compare against their
// own setup.
func colorDepth() string {
	switch v := os.Getenv("COLORTERM"); v {
	case "truecolor", "24bit":
		return "truecolor"
	case "":
		if t := os.Getenv("TERM"); t != "" {
			return t
		}
		return "unknown"
	default:
		return v
	}
}

func goVersion() string { return runtime.Version() }
