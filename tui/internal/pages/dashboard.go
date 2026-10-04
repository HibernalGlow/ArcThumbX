package pages

import (
	"strconv"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
)

// Dashboard is the readout screen: what the panel is connected to, and what the
// effective configuration currently is. It has no controls at all, which is the
// point — a status screen you can accidentally edit from is not a status
// screen.
type Dashboard struct{ *formPage }

// NewDashboard builds the overview screen.
func NewDashboard(e *Env) Page {
	return &Dashboard{formPage: newFormPage(e, "dashboard", "Dashboard",
		"SYSTEM OVERVIEW", "session state and effective ArcThumbX configuration",
		dashboardSections)}
}

func dashboardSections(e *Env) []components.Section {
	f := e.Facts
	s := e.Settings
	imgTable := len(arcthumb.SupportedImageExts())
	arcTable := len(arcthumb.SupportedArchiveExts())

	store := components.Disclosure("Settings store", f.StoreLocation,
		"Where ArcThumbX keeps the configuration this panel edits.", flagOf(f.StoreReadable))
	reach := components.Row{
		Label:  "Backend",
		Value:  f.StoreKind,
		Status: badgeForStore(f),
		Help:   storeHelp(f),
		Flag:   flagOf(true),
	}
	term := components.Disclosure("Terminal",
		strconv.Itoa(f.TermCols)+"x"+strconv.Itoa(f.TermRows)+" · "+f.Profile,
		"Cell grid and the colour depth the terminal reported.", components.SourceDerived)
	preset := components.Disclosure("Preset", f.PresetName,
		"Press T to switch. The choice is remembered between runs.", components.SourceLocal)

	session := components.Section{Title: "Session", Note: "arcthumb-tui", Rows: []components.Row{
		store, reach, term, preset,
		components.Disclosure("Mouse", mouseState(f),
			"When off, the terminal keeps its own text selection and drag-to-copy.", components.SourceLocal),
	}}

	config := components.Section{Title: "Effective configuration", Note: f.StoreKind, Rows: []components.Row{
		components.Disclosure("Page order", s.SortOrder.Label(),
			"How images inside a container are ordered before one is picked.", liveFlag(e)),
		components.Disclosure("Cover selection", s.CoverMode.Label(),
			"How cover-named images are treated when choosing that image.", liveFlag(e)),
		components.Disclosure("Image formats",
			strconv.Itoa(arcthumb.EnabledCount(s.EnabledImageExts, imgTable))+"/"+strconv.Itoa(imgTable)+" enabled",
			"Decodable extensions eligible as a cover source. See Formats.", liveFlag(e)),
		components.Disclosure("Container formats",
			strconv.Itoa(arcthumb.EnabledCount(s.EnabledArchiveExts, arcTable))+"/"+strconv.Itoa(arcTable)+" enabled",
			"Extensions that may be thumbnailed at all.", liveFlag(e)),
		components.Disclosure("Overlay border", YesNo(s.OverlayBorder),
			"The format-coloured frame baked into archive thumbnails.", liveFlag(e)),
		components.Disclosure("Format chip", YesNo(s.OverlayLabel),
			"The corner label baked into archive thumbnails.", liveFlag(e)),
		components.Disclosure("Diagnostics log", YesNo(s.LogEnabled),
			"Writes extension diagnostics to the temporary directory.", liveFlag(e)),
	}}

	return []components.Section{session, config}
}

func liveFlag(e *Env) components.Source {
	if e.Facts.StoreReadable {
		return components.SourceLive
	}
	return components.SourceUnavailable
}

func flagOf(ok bool) components.Source {
	if ok {
		return components.SourceLive
	}
	return components.SourceUnavailable
}

func mouseState(f Facts) string {
	if f.MouseOn {
		return "Tracking"
	}
	return "Off"
}

func badgeForStore(f Facts) *components.StatusBadge {
	switch {
	case f.StoreKind == "memory":
		return components.NewBadge(components.StateInfo, "offline")
	case !f.StoreReadable:
		return components.NewBadge(components.StateError, "unreadable")
	case f.BackendError != "":
		return components.NewBadge(components.StateWarn, "read-only")
	default:
		return components.OK("connected")
	}
}

func storeHelp(f Facts) string {
	if f.BackendError != "" {
		return "Backend reported: " + f.BackendError
	}
	return "A write path exists for this backend, behind a confirmation."
}
