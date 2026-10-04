package pages

import (
	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
)

// Thumbnail edits how the extension chooses and dresses the picture it hands
// back. Every row here writes straight into the shared ArcThumbX settings, so the
// dirty marker and the save path are the real ones.
type Thumbnail struct{ *formPage }

// NewThumbnail builds the thumbnail-picking screen.
func NewThumbnail(e *Env) Page {
	return &Thumbnail{formPage: newFormPage(e, "thumbnail", "Thumbnail",
		"THUMBNAIL SELECTION", "which image becomes the cover, and what gets baked onto it",
		thumbnailSections)}
}

func thumbnailSections(e *Env) []components.Section {
	s := e.Settings

	picking := components.Section{Title: "Choose the source image", Note: "archives", Rows: []components.Row{
		components.Bind("Page order", orderSelect(e),
			"Natural compares digit runs numerically, so page2 precedes page10. Alphabetical is byte-wise.",
			liveFlag(e)),
		components.Bind("Cover selection", coverSelect(e),
			"Cover names are cover, folder, thumb, thumbnail and front, matched exactly: cover.png qualifies, coverpage.png does not.",
			liveFlag(e)),
	}}

	// Both overlay defaults are off in the core, because turning them on changes
	// how every existing archive thumbnail looks. The row text says so rather
	// than letting the user discover it.
	dressing := components.Section{Title: "Bake onto the thumbnail", Note: "overlay", Rows: []components.Row{
		components.Bind("Format-coloured border", toggler(
			func() bool { return s.OverlayBorder },
			func(v bool) { s.OverlayBorder = v }),
			"Amber for compressed archives, indigo for e-books, grey otherwise. Off by default: opting in restyles every archive thumbnail you already have.",
			liveFlag(e)),
		components.Bind("Corner format chip", toggler(
			func() bool { return s.OverlayLabel },
			func(v bool) { s.OverlayLabel = v }),
			"Writes the container type into the corner. The extension drops it by itself at sizes where the text would be unreadable.",
			liveFlag(e)),
	}}

	return []components.Section{picking, dressing}
}

func orderSelect(e *Env) components.Control {
	return components.NewSelect(
		func() int {
			if e.Settings.SortOrder == arcthumb.SortAlphabetical {
				return 1
			}
			return 0
		},
		func(i int) {
			if i == 1 {
				e.Settings.SortOrder = arcthumb.SortAlphabetical
				return
			}
			e.Settings.SortOrder = arcthumb.SortNatural
		},
		[]components.Option{
			{Label: "Natural", Detail: "page2.jpg before page10.jpg. The default."},
			{Label: "Alphabetical", Detail: "Byte-wise: page10.jpg before page2.jpg."},
		})
}

func coverSelect(e *Env) components.Control {
	return components.NewSelect(
		func() int { return int(e.Settings.CoverMode) },
		func(i int) { e.Settings.CoverMode = arcthumb.CoverMode(i) },
		[]components.Option{
			{Label: "Ignore", Detail: "Always take the first image by page order."},
			{Label: "Prefer", Detail: "Take a cover-named image when there is one, otherwise the first."},
			{Label: "Only", Detail: "Cover-named or no thumbnail at all, so a stray screenshot in a work archive never becomes the icon."},
		})
}
