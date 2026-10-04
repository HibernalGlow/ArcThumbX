package pages

import (
	"strings"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
)

// Integration reports how the panel and the shell extension are wired together,
// and carries the one live switch that belongs to diagnostics rather than to
// image selection.
//
// Anything this build cannot perform is disclosed as a read-only row. It is not
// rendered as a control: a button that silently does nothing is worse than a row
// that says so.
type Integration struct{ *formPage }

// NewIntegration builds the shell-integration screen.
func NewIntegration(e *Env) Page {
	return &Integration{formPage: newFormPage(e, "integration", "Integration",
		"SHELL INTEGRATION", "how the extension is reached, and what this panel can do about it",
		integrationSections)}
}

func integrationSections(e *Env) []components.Section {
	f := e.Facts

	wiring := components.Section{Title: "Wiring", Note: f.StoreKind, Rows: []components.Row{
		components.Disclosure("Backend", f.StoreKind,
			"The adapter this build was compiled for. Both backends share these key names.",
			components.SourceLive),
		components.Disclosure("Settings location", f.StoreLocation,
			"Read on every thumbnail request, so a save takes effect without restarting the shell.",
			flagOf(f.StoreReadable)),
		components.Disclosure("Claimed identity", partnerIdentity(f),
			"The sandboxed extension, or the registry subkey, that reads this configuration.",
			components.SourceLive),
		components.Row{
			Label:  "Writable",
			Value:  YesNo(f.StoreWritable),
			Status: writeBadge(f),
			Help:   writeHelp(f),
			Flag:   components.SourceDerived,
		},
	}}

	diagnostic := components.Section{Title: "Diagnostics", Rows: []components.Row{
		components.Bind("Verbose log", toggler(
			func() bool { return e.Settings.LogEnabled },
			func(v bool) { e.Settings.LogEnabled = v }),
			"Diagnostic messages from the extension. Leave it off until thumbnails are missing.",
			liveFlag(e)),
		components.Disclosure("Cache refresh", cacheState(f),
			"Drops the host's cached thumbnails so it re-asks the extension.",
			flagOf(f.Regenerable)),
	}}

	lint := components.Section{Title: "File lint", Note: "read pass", Rows: []components.Row{
		components.Disclosure("Unrecognised keys", joinOr(f.IgnoredKeys, "none"),
			"Forward compatibility works by ignoring these, so a newer file still loads here.",
			components.SourceDerived),
		components.Disclosure("Malformed lines", joinOr(f.MalformedLines, "none"),
			"Lines with no equals sign. Each was skipped; the rest of the file still applied.",
			components.SourceDerived),
	}}

	return []components.Section{wiring, diagnostic, lint}
}

func partnerIdentity(f Facts) string {
	switch f.StoreKind {
	case "registry":
		return `HKCU\Software\ArcThumb`
	case "memory":
		return "not connected — offline fixture"
	default:
		return arcthumb.ExtensionBundleID
	}
}

func cacheState(f Facts) string {
	if f.Regenerable {
		return "Available from the GUI"
	}
	return "Not wired here"
}

func writeBadge(f Facts) *components.StatusBadge {
	switch {
	case !f.StoreWritable:
		return components.NewBadge(components.StateOff, "read-only")
	case f.StoreKind == "memory":
		return components.NewBadge(components.StateInfo, "in memory")
	default:
		return components.OK("ready")
	}
}

func writeHelp(f Facts) string {
	switch {
	case f.BackendError != "":
		return f.BackendError
	case !f.StoreWritable:
		return "This backend cannot be written from the TUI, so edits stay local to the session."
	default:
		return "Writes go to the same file the extension reads, atomically, behind a confirmation."
	}
}

func joinOr(items []string, empty string) string {
	if len(items) == 0 {
		return empty
	}
	if len(items) > 4 {
		return strings.Join(items[:4], ", ") + ", +"
	}
	return strings.Join(items, ", ")
}
