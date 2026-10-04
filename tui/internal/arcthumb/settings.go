// Package arcthumb models the settings ArcThumbX shares between its config
// front end and the shell / Quick Look extension.
//
// This is a re-implementation of the wire format only, kept honest by the
// parity tests in settings_parity_test.go: the Rust side is the single source
// of truth at src/settings.rs and
// src/bin/arcthumb-config/settings_store.rs. Nothing here links to Rust, and
// nothing here needs to change when the core changes — unless a key is added,
// in which case the only cost is a new field.
package arcthumb

import (
	"strconv"
	"strings"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/kv"
)

// SortOrder picks how images inside an archive are ordered before one is
// chosen as the cover.
type SortOrder int

const (
	// SortNatural compares digit runs numerically, so page2.jpg precedes
	// page10.jpg. Factory default.
	SortNatural SortOrder = iota
	// SortAlphabetical is a plain byte-wise sort.
	SortAlphabetical
)

// Value is the canonical on-disk string, mirroring as_registry_value().
func (s SortOrder) Value() string {
	if s == SortAlphabetical {
		return "alphabetical"
	}
	return "natural"
}

func (s SortOrder) String() string { return s.Value() }

// Label is the display form used in the UI.
func (s SortOrder) Label() string {
	if s == SortAlphabetical {
		return "Alphabetical"
	}
	return "Natural"
}

// CoverMode controls how cover-named images (cover.*, folder.*, thumb.*,
// thumbnail.*, front.*) are treated when picking an archive thumbnail.
type CoverMode int

const (
	// CoverIgnore always takes the first image by sort order.
	CoverIgnore CoverMode = iota
	// CoverPrefer takes a cover-named image when present. Factory default.
	CoverPrefer
	// CoverOnly uses a cover-named image or produces no thumbnail at all.
	CoverOnly
)

func (c CoverMode) Value() string {
	switch c {
	case CoverIgnore:
		return "ignore"
	case CoverOnly:
		return "only"
	default:
		return "prefer"
	}
}

func (c CoverMode) String() string { return c.Value() }

func (c CoverMode) Label() string {
	switch c {
	case CoverIgnore:
		return "Ignore cover names"
	case CoverOnly:
		return "Cover-named only"
	default:
		return "Prefer cover name"
	}
}

// supportedImageExts mirrors SUPPORTED_IMAGE_EXTS. Index i is bit i of the
// mask, so this list is append-only: reordering it would silently reinterpret
// every user's saved config.
var supportedImageExts = []string{
	".jpg", ".jpeg", ".png", ".gif", ".bmp",
	".tiff", ".tif", ".webp", ".ico", ".avif", ".jxl",
}

// SupportedArchiveExts mirrors SUPPORTED_ARCHIVE_EXTS, same append-only rule.
var supportedArchiveExts = []string{
	"zip", "cbz", "rar", "cbr", "7z", "cb7",
	"tar", "cbt", "epub", "fb2", "mobi", "azw", "azw3",
}

// SupportedImageExts returns the decodeable image extensions in bit order.
// AVIF and JXL are the last two bits; the Rust core only lists them when a
// backend that can decode them is compiled in, which is true of every
// ArcThumbX build that ships.
func SupportedImageExts() []string { return append([]string(nil), supportedImageExts...) }

// SupportedArchiveExts returns the openable container extensions in bit order.
func SupportedArchiveExts() []string { return append([]string(nil), supportedArchiveExts...) }

// ImageMaskAll and ArchiveMaskAll are the factory defaults and the clamp: bits
// above the table length are ignored by the core, so saving them would be a lie.
func ImageMaskAll() uint32   { return maskAll(len(supportedImageExts)) }
func ArchiveMaskAll() uint32 { return maskAll(len(supportedArchiveExts)) }

func maskAll(n int) uint32 {
	if n >= 32 {
		return 0xFFFFFFFF
	}
	return (uint32(1) << uint(n)) - 1
}

// Settings is the shared configuration. Field set and defaults match
// src/settings.rs.
type Settings struct {
	SortOrder SortOrder
	CoverMode CoverMode

	// EnabledImageExts and EnabledArchiveExts are bitmasks over the
	// extension tables above.
	EnabledImageExts   uint32
	EnabledArchiveExts uint32

	// OverlayBorder bakes the format-coloured frame into archive
	// thumbnails; OverlayLabel bakes the corner format chip. Both default
	// off because they change how every existing thumbnail looks.
	OverlayBorder bool
	OverlayLabel  bool

	// LogEnabled turns on the extension's diagnostic log.
	LogEnabled bool

	// Language and Theme are the config panel's own front-end choices, stored in
	// this same file. Empty means "never chosen", which is what lets the OS
	// locale still decide on a fresh install — so an unset key must stay absent
	// rather than being written as an empty value.
	Language string
	Theme    string
}

// Default is the factory configuration: natural sort, prefer covers,
// everything decodable enabled, overlays and logging off.
func Default() Settings {
	return Settings{
		SortOrder:          SortNatural,
		CoverMode:          CoverPrefer,
		EnabledImageExts:   ImageMaskAll(),
		EnabledArchiveExts: ArchiveMaskAll(),
	}
}

// Clamp drops mask bits outside the supported tables, the same guard the core
// applies in to_core() before handing settings to the thumbnailer.
func (s Settings) Clamp() Settings {
	s.EnabledImageExts &= ImageMaskAll()
	s.EnabledArchiveExts &= ArchiveMaskAll()
	return s
}

// Header is the comment block the Rust writer emits. Reproduced exactly so a
// file the TUI wrote is indistinguishable from one the GUI wrote.
func Header() []string {
	return []string{
		"ArcThumb — Finder thumbnail extension settings.",
		"Written by ArcThumb.app; safe to edit, but the extension",
		"re-reads this file on every request.",
	}
}

// Marshal renders the on-disk form. Key order is fixed and matches
// StoredSettings::to_text, so a diff between a GUI-written and a
// TUI-written file stays empty.
func Marshal(s Settings) string {
	items := []kv.Item{
		{Key: "sort_order", Value: s.SortOrder.Value()},
		{Key: "cover_mode", Value: s.CoverMode.Value()},
		{Key: "enabled_image_exts", Value: strconv.FormatUint(uint64(s.EnabledImageExts), 10)},
		{Key: "enabled_archive_exts", Value: strconv.FormatUint(uint64(s.EnabledArchiveExts), 10)},
		{Key: "overlay_border", Value: kv.Flag(s.OverlayBorder)},
		{Key: "overlay_label", Value: kv.Flag(s.OverlayLabel)},
		{Key: "log_enabled", Value: kv.Flag(s.LogEnabled)},
	}
	// Same conditional tail the Rust writer emits: an unset preference writes no
	// line at all, so a save from here cannot pin a choice the user never made.
	if s.Language != "" {
		items = append(items, kv.Item{Key: "language", Value: s.Language})
	}
	if s.Theme != "" {
		items = append(items, kv.Item{Key: "theme", Value: s.Theme})
	}
	return kv.Encode(Header(), items)
}

// Ignored reports what the file contained that this build could not use, so
// the UI can disclose loss instead of hiding it.
type Ignored struct {
	UnknownKeys []string
	Malformed   []string
}

// Empty reports whether nothing was dropped.
func (i Ignored) Empty() bool { return len(i.UnknownKeys) == 0 && len(i.Malformed) == 0 }

// Parse reads the on-disk form, starting from the factory defaults. Unknown
// keys and malformed lines are skipped and reported, never fatal: a bad line
// must not cost the user every thumbnail in the finder.
func Parse(text string) (Settings, Ignored) {
	s := Default()
	doc := kv.Decode(text)
	// Applied in file order so a duplicated key keeps its final value, which
	// is what the Rust loop does.
	for _, it := range doc.Items {
		apply(&s, it.Key, it.Value)
	}
	return s, ignored(doc)
}

func ignored(doc kv.Doc) Ignored {
	var out Ignored
	for _, k := range doc.Keys() {
		if !knownKey(k) {
			out.UnknownKeys = append(out.UnknownKeys, k)
		}
	}
	out.Malformed = doc.Malformed
	return out
}

func knownKey(k string) bool {
	switch k {
	case "sort_order", "cover_mode", "enabled_image_exts", "enabled_archive_exts",
		"overlay_border", "overlay_label", "log_enabled", "language", "theme":
		return true
	}
	return false
}

// apply sets one recognised key. A recognised key with an unparseable value
// leaves the current value alone, which is the Rust behaviour for enums and
// masks; flags simply read as off.
func apply(s *Settings, key, value string) {
	switch key {
	case "sort_order":
		switch strings.ToLower(value) {
		case "alphabetical", "alpha":
			s.SortOrder = SortAlphabetical
		case "natural", "nat":
			s.SortOrder = SortNatural
		}
	case "cover_mode":
		switch strings.ToLower(value) {
		case "ignore":
			s.CoverMode = CoverIgnore
		case "prefer":
			s.CoverMode = CoverPrefer
		case "only":
			s.CoverMode = CoverOnly
		}
	case "enabled_image_exts":
		if n, err := strconv.ParseUint(value, 10, 32); err == nil {
			s.EnabledImageExts = uint32(n)
		}
	case "enabled_archive_exts":
		if n, err := strconv.ParseUint(value, 10, 32); err == nil {
			s.EnabledArchiveExts = uint32(n)
		}
	case "overlay_border":
		s.OverlayBorder = kv.ParseFlag(value)
	case "overlay_label":
		s.OverlayLabel = kv.ParseFlag(value)
	case "log_enabled":
		s.LogEnabled = kv.ParseFlag(value)
	case "language":
		// A tag the panel would not recognise is dropped rather than stored,
		// matching the Rust `if Locale::from_tag(value).is_some()` guard.
		if tag, ok := LocaleTag(value); ok {
			s.Language = tag
		}
	case "theme":
		if tag, ok := ThemeTag(value); ok {
			s.Theme = tag
		}
	}
}

// LocaleTag normalises a language tag the way Locale::from_tag does: lower-case,
// underscores folded to hyphens, then matched by prefix. It returns the canonical
// short tag the Rust side writes back.
func LocaleTag(value string) (string, bool) {
	tag := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
	switch {
	case strings.HasPrefix(tag, "zh"), strings.HasPrefix(tag, "chinese"), tag == "cn":
		return "zh", true
	case strings.HasPrefix(tag, "ja"), strings.HasPrefix(tag, "japanese"):
		return "ja", true
	case strings.HasPrefix(tag, "en"), strings.HasPrefix(tag, "english"):
		return "en", true
	default:
		return "", false
	}
}

// ThemeTag mirrors Theme::from_tag: trim, lower-case, and only dark or light.
func ThemeTag(value string) (string, bool) {
	switch tag := strings.ToLower(strings.TrimSpace(value)); tag {
	case "dark", "light":
		return tag, true
	default:
		return "", false
	}
}

// ExtEnabled reports whether bit i of mask is on.
func ExtEnabled(mask uint32, i int) bool {
	if i < 0 || i > 31 {
		return false
	}
	return mask&(uint32(1)<<uint(i)) != 0
}

// ExtSet returns mask with bit i forced to on or off.
func ExtSet(mask uint32, i int, on bool) uint32 {
	if i < 0 || i > 31 {
		return mask
	}
	bit := uint32(1) << uint(i)
	if on {
		return mask | bit
	}
	return mask &^ bit
}

// EnabledCount reports how many supported bits are on, clamped to the table.
func EnabledCount(mask uint32, tableSize int) int {
	n := 0
	for i := 0; i < tableSize; i++ {
		if ExtEnabled(mask, i) {
			n++
		}
	}
	return n
}
