package pages

import (
	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/tuicfg"
)

// General edits the front-end's own preferences. These apply immediately and are
// persisted on the spot: a look-and-feel knob that requires a save step is a
// knob that will look broken, because the user cannot tell a pending edit from
// an ignored one.
type General struct{ *formPage }

// NewGeneral builds the preferences screen.
func NewGeneral(e *Env) Page {
	return &General{formPage: newFormPage(e, "general", "General",
		"FRONT-END PREFERENCES", "how this panel looks and behaves", generalSections)}
}

func generalSections(e *Env) []components.Section {
	p := e.Prefs

	appearance := components.Section{Title: "Appearance", Note: "tui", Rows: []components.Row{
		components.Bind("Theme preset", presetSelect(e),
			"Repaints every token in the panel: colours, glyphs, weights, rules. Applied at once.",
			components.SourceLocal),
		components.Bind("Navigation rail", toggler(
			func() bool { return p.ShowSidebar },
			func(v bool) { p.ShowSidebar = v; e.App.Request(IntentPersist) }),
			"On a wide terminal the rail sits beside the pane; below the breakpoint it folds into tabs by itself.",
			components.SourceLocal),
		components.Bind("Long-form measure", fader(e),
			"Width cap for prose and metadata, so a 200-column terminal does not produce a 200-column paragraph.",
			components.SourceLocal),
	}}

	behaviour := components.Section{Title: "Behaviour", Note: "tui", Rows: []components.Row{
		components.Bind("Mouse tracking", toggler(
			func() bool { return p.MouseEnabled },
			func(v bool) { p.MouseEnabled = v; e.App.Request(IntentPersist) }),
			"Off hands the pointer back to the terminal, which restores native text selection and copy.",
			components.SourceLocal),
		components.Bind("Confirm before quit", toggler(
			func() bool { return p.ConfirmOnQuit },
			func(v bool) { p.ConfirmOnQuit = v; e.App.Request(IntentPersist) }),
			"Asks only when ArcThumbX edits are pending.",
			components.SourceLocal),
	}}

	// These two live in the ArcThumbX settings file, not the TUI's own: they are
	// the config panel's language and theme rocker. Editing them here writes the
	// same keys the GUI reads, so the two front ends cannot drift apart, and
	// "never chosen" stays a real third state rather than being collapsed into
	// whichever value the OS would have picked.
	shared := components.Section{Title: "Config panel", Note: "shared file", Rows: []components.Row{
		components.Bind("Panel theme", guiThemeSelect(e),
			"The desktop panel's own dark/light rocker, stored with the thumbnail settings. Not chosen leaves the decision to the panel's default.",
			liveFlag(e)),
		components.Bind("Panel language", guiLocaleSelect(e),
			"Keycaps and labels in the desktop panel. Not chosen lets the OS locale decide.",
			liveFlag(e)),
	}}

	record := components.Section{Title: "Storage", Rows: []components.Row{
		components.Disclosure("Preferences file", e.Facts.PrefsLocation,
			"ArcThumbX settings and TUI preferences are separate files. This is the front-end's own.",
			components.SourceLocal),
		components.Disclosure("ArcThumbX settings", e.Facts.StoreLocation,
			"The file the shell extension reads.", components.SourceLive),
		components.Disclosure("Preset identifier", string(e.Facts.PresetID),
			"The key written to the preferences file.", components.SourceDerived),
	}}

	return []components.Section{appearance, behaviour, shared, record}
}

// guiThemeSelect cycles the GUI's theme rocker stored in the shared file. Index
// 0 is "not chosen", which writes no key at all.
func guiThemeSelect(e *Env) components.Control {
	tags := []string{"", "dark", "light"}
	labels := []string{"Not chosen", "Dark", "Light"}
	details := []string{
		"No key is written, so the panel keeps its own default.",
		"Writes theme = dark.",
		"Writes theme = light.",
	}
	return tagSelect(e, tags, labels, details,
		func() string { return e.Settings.Theme },
		func(v string) { e.Settings.Theme = v })
}

// guiLocaleSelect cycles the GUI's language keycaps, same three-state rule.
func guiLocaleSelect(e *Env) components.Control {
	tags := []string{"", "en", "ja", "zh"}
	labels := []string{"Not chosen", "English", "日本語", "中文"}
	details := []string{
		"No key is written, so the OS locale decides.",
		"Writes language = en.",
		"Writes language = ja.",
		"Writes language = zh.",
	}
	return tagSelect(e, tags, labels, details,
		func() string { return e.Settings.Language },
		func(v string) { e.Settings.Language = v })
}

// tagSelect is a cycling selector over stored tag values, reused by both
// shared-file controls so neither can drift from the other's index rules.
func tagSelect(e *Env, tags, labels, details []string, get func() string, set func(string)) components.Control {
	opts := make([]components.Option, 0, len(tags))
	for i, label := range labels {
		opts = append(opts, components.Option{Label: label, Detail: details[i]})
	}
	return components.NewSelect(
		func() int {
			for i, tag := range tags {
				if tag == get() {
					return i
				}
			}
			return 0
		},
		func(i int) {
			if i < 0 || i >= len(tags) {
				return
			}
			set(tags[i])
		},
		opts,
	)
}

// toggler binds a lamp to a boolean accessor pair.
func toggler(get func() bool, set func(bool)) components.Control {
	return components.NewToggle(get, set)
}

func fader(e *Env) components.Control {
	s := components.NewSlider(
		func() int { return e.Prefs.ContentWidth },
		func(v int) { e.Prefs.ContentWidth = v; e.App.Request(IntentPersist) },
		tuicfg.MinContentWidth, tuicfg.MaxContentWidth)
	s.Unit = "cols"
	s.Increment = 2
	return s
}

// presetSelect maps the registered presets onto a cycling selector. The index is
// derived from the stored ID on read, so a preset added later cannot desync the
// two representations of the same value.
func presetSelect(e *Env) components.Control {
	entries := theme.Registered()
	opts := make([]components.Option, 0, len(entries))
	for _, en := range entries {
		opts = append(opts, components.Option{Label: en.Name, Detail: en.Blurb})
	}
	return components.NewSelect(
		func() int {
			for i, en := range entries {
				if string(en.ID) == e.Prefs.ThemeID {
					return i
				}
			}
			return 0
		},
		func(i int) {
			if i < 0 || i >= len(entries) {
				return
			}
			e.Prefs.ThemeID = string(entries[i].ID)
			e.App.Request(IntentPersist)
		},
		opts,
	)
}
