package arcthumb_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
)

// The literal expectations below were transcribed from the Rust producer, not
// invented: src/settings.rs for the tables and defaults,
// src/bin/arcthumb-config/settings_store.rs for the wire format. RustSource_*
// re-reads that producer at test time so a transcription that drifts fails here
// instead of quietly writing a file the extension misreads.

func TestMarshalIsByteExactForDefaults(t *testing.T) {
	want := strings.Join([]string{
		"# ArcThumb — Finder thumbnail extension settings.",
		"# Written by ArcThumb.app; safe to edit, but the extension",
		"# re-reads this file on every request.",
		"sort_order = natural",
		"cover_mode = prefer",
		"enabled_image_exts = 2047",
		"enabled_archive_exts = 8191",
		"overlay_border = 0",
		"overlay_label = 0",
		"log_enabled = 0",
		"",
	}, "\n")

	got := arcthumb.Marshal(arcthumb.Default())
	if got != want {
		t.Fatalf("wire format drifted from the Rust producer\n got: %q\nwant: %q", got, want)
	}
}

func TestMasksMatchTableLengths(t *testing.T) {
	img := len(arcthumb.SupportedImageExts())
	arc := len(arcthumb.SupportedArchiveExts())
	if img != 11 {
		t.Errorf("image table length = %d, want 11 (.jpg … .jxl)", img)
	}
	if arc != 13 {
		t.Errorf("archive table length = %d, want 13 (zip … azw3)", arc)
	}
	if got, want := arcthumb.ImageMaskAll(), uint32((1<<uint(img))-1); got != want {
		t.Errorf("ImageMaskAll() = %d, want %d", got, want)
	}
	if got, want := arcthumb.ArchiveMaskAll(), uint32((1<<uint(arc))-1); got != want {
		t.Errorf("ArchiveMaskAll() = %d, want %d", got, want)
	}
}

// TestParseToleratesBadInput is the guard that a corrupt file costs a line and
// not the whole configuration, which is the extension's stated behaviour.
func TestParseToleratesBadInput(t *testing.T) {
	text := strings.Join([]string{
		"# comment",
		"",
		"nonsense without separator",
		"sort_order = alpha",
		"cover_mode = ONLY",
		"enabled_image_exts = 99999999999999",
		"overlay_border = yes",
		"log_enabled = maybe",
		"brand_new_key = whatever",
	}, "\n")

	s, ignored := arcthumb.Parse(text)
	if s.SortOrder != arcthumb.SortAlphabetical {
		t.Errorf("alias alpha: sort = %v, want alphabetical", s.SortOrder)
	}
	if s.CoverMode != arcthumb.CoverOnly {
		t.Errorf("case-insensitive ONLY: cover = %v, want only", s.CoverMode)
	}
	if s.EnabledImageExts != arcthumb.Default().EnabledImageExts {
		t.Errorf("overflowing mask should be rejected, got %d", s.EnabledImageExts)
	}
	if !s.OverlayBorder {
		t.Error(`overlay_border = yes should parse true`)
	}
	if s.LogEnabled {
		t.Error(`log_enabled = maybe must parse false, matching parse_flag`)
	}
	if len(ignored.UnknownKeys) != 1 || ignored.UnknownKeys[0] != "brand_new_key" {
		t.Errorf("unknown keys = %v, want exactly [brand_new_key]", ignored.UnknownKeys)
	}
	if len(ignored.Malformed) != 1 {
		t.Errorf("malformed lines = %v, want one", ignored.Malformed)
	}

	clamped := s.Clamp()
	if clamped.EnabledImageExts&^arcthumb.ImageMaskAll() != 0 {
		t.Error("Clamp left bits above the table set")
	}
}

func TestRoundTrip(t *testing.T) {
	in := arcthumb.Settings{
		SortOrder:          arcthumb.SortAlphabetical,
		CoverMode:          arcthumb.CoverIgnore,
		EnabledImageExts:   0b10000000111,
		EnabledArchiveExts: 0b1000000000011,
		OverlayBorder:      true,
		OverlayLabel:       false,
		LogEnabled:         true,
	}
	out, _ := arcthumb.Parse(arcthumb.Marshal(in))
	if out != in {
		t.Fatalf("round trip changed state:\n in %+v\nout %+v", in, out)
	}
}

func TestExtBits(t *testing.T) {
	mask := uint32(0)
	for i := 0; i < len(arcthumb.SupportedImageExts()); i += 2 {
		mask = arcthumb.ExtSet(mask, i, true)
	}
	if got := arcthumb.EnabledCount(mask, len(arcthumb.SupportedImageExts())); got != 6 {
		t.Errorf("EnabledCount = %d, want 6", got)
	}
	if arcthumb.ExtSet(mask, 40, true) != mask {
		t.Error("out-of-range bit must be ignored")
	}
}

// TestFileStoreWritesAtomicallyAndRereads drives the real write protocol: a
// file the extension can only ever see complete.
func TestFileStoreWritesAtomicallyAndRereads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings")
	store := arcthumb.NewFileStore(path)

	s, ignored, err := store.Load()
	if err != nil {
		t.Fatalf("missing file must load as defaults: %v", err)
	}
	if s != arcthumb.Default() {
		t.Errorf("missing file gave %+v", s)
	}
	if !ignored.Empty() {
		t.Errorf("defaults carry no ignored keys, got %v", ignored)
	}

	want := arcthumb.Default()
	want.OverlayLabel = true
	want.EnabledImageExts = arcthumb.ExtSet(want.EnabledImageExts, 3, false)
	if err := store.Save(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, _, err := store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got != want {
		t.Fatalf("reload mismatch: want %+v got %+v", want, got)
	}

	// The temp file must be gone: a leftover settings.tmp.* in the container is
	// noise the extension never asked for.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "settings.tmp.") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestMacOSPathMirrorsRust(t *testing.T) {
	got := arcthumb.MacOSSettingsPath("/Users/tester")
	want := "/Users/tester/Library/Containers/com.citrussoda.ArcThumb.thumbnail/Data/" +
		"Library/Application Support/ArcThumb/settings"
	if got != want {
		t.Fatalf("path drift, the extension would read a different file:\n got %s\nwant %s", got, want)
	}
}

func TestRegistryStoreRefusesRatherThanLying(t *testing.T) {
	var store arcthumb.Store = &arcthumb.RegistryStore{}
	if _, _, err := store.Load(); err == nil {
		t.Fatal("registry backend must report ErrNotWired, not defaults as if read")
	}
	if err := store.Save(arcthumb.Default()); err == nil {
		t.Fatal("registry backend must refuse a write")
	}
}

// TestSourceParityWithRust compares the Go tables against the Rust source text.
// Set ARCTHUMB_TUI_PARITY_REQUIRED=1 to turn a missing source into a failure.
func TestSourceParityWithRust(t *testing.T) {
	root := repoRoot(t)
	settings := readSource(t, root, "src/settings.rs")
	store := readSource(t, root, "src/bin/arcthumb-config/settings_store.rs")

	img := extractArray(t, settings, "SUPPORTED_IMAGE_EXTS")
	arc := extractArray(t, settings, "SUPPORTED_ARCHIVE_EXTS")

	if diff := stringDiff(arcthumb.SupportedImageExts(), img); diff != "" {
		t.Errorf("image table differs from Rust SUPPORTED_IMAGE_EXTS (bit order is load-bearing):\n%s", diff)
	}
	if diff := stringDiff(arcthumb.SupportedArchiveExts(), arc); diff != "" {
		t.Errorf("archive table differs from Rust SUPPORTED_ARCHIVE_EXTS:\n%s", diff)
	}

	// Key names and their written order, taken from the to_text body and
	// compared against what Marshal actually emits when every optional key is
	// set. Reading the consumer's own output keeps this from becoming a second
	// copy of the expectation.
	rustOrder := rustWriteOrder(t, store)
	full := arcthumb.Default()
	full.Language = "zh"
	full.Theme = "light"
	got := writtenKeys(arcthumb.Marshal(full))
	if diff := stringDiff(got, rustOrder); diff != "" {
		t.Errorf("Rust writes keys in a different order than Marshal:\n%s", diff)
	}

	// Every header sentence must appear, so a user who cats either file sees the
	// same explanation.
	for _, line := range arcthumb.Header() {
		core := strings.TrimSuffix(strings.TrimPrefix(line, "ArcThumb"), ".")
		if core == "" {
			continue
		}
		if !strings.Contains(store, strings.TrimSpace(firstWords(line, 3))) {
			t.Errorf("header sentence %q is not the one the Rust writer emits", line)
		}
	}
}

// TestSourceParityDetectsDrift is the positive control for the gate above: the
// same comparison must go red when a table is altered, otherwise a passing parity
// test would prove nothing.
func TestSourceParityDetectsDrift(t *testing.T) {
	root := repoRoot(t)
	settings := readSource(t, root, "src/settings.rs")
	rust := extractArray(t, settings, "SUPPORTED_IMAGE_EXTS")

	shuffled := append([]string(nil), rust...)
	shuffled[0], shuffled[1] = shuffled[1], shuffled[0]
	if diff := stringDiff(shuffled, rust); diff == "" {
		t.Fatal("positive control failed: a swapped table compared as equal")
	}
	short := rust[:len(rust)-1]
	if diff := stringDiff(short, rust); diff == "" {
		t.Fatal("positive control failed: a truncated table compared as equal")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// tui/internal/arcthumb -> repo root is three levels up.
	root := filepath.Clean(filepath.Join(dir, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "Cargo.toml")); err != nil {
		if os.Getenv("ARCTHUMB_TUI_PARITY_REQUIRED") == "1" {
			t.Fatalf("Rust sources not found at %s and parity is required: %v", root, err)
		}
		t.Skipf("Rust sources not found at %s; source-parity gate skipped", root)
	}
	return root
}

func readSource(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// extractArray pulls the string literals out of a Rust `pub const NAME: &[&str] = &[…]`,
// ignoring the `#[cfg(...)]` attributes that sit between entries: those carry
// feature names like "wic", and treating them as extensions would make this gate
// compare the table against a phantom list.
func extractArray(t *testing.T, src, name string) []string {
	t.Helper()
	idx := strings.Index(src, "pub const "+name)
	if idx < 0 {
		t.Fatalf("Rust const %s not found", name)
	}
	start := strings.Index(src[idx:], "&[")
	if start < 0 {
		t.Fatalf("%s: array opening not found", name)
	}
	body := src[idx+start:]
	end := strings.Index(body, "];")
	if end < 0 {
		t.Fatalf("%s: array close not found", name)
	}
	var kept []string
	for _, line := range strings.Split(body[:end], "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#[") {
			continue
		}
		kept = append(kept, line)
	}
	literals := regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(strings.Join(kept, "\n"), -1)
	out := make([]string, 0, len(literals))
	for _, m := range literals {
		out = append(out, m[1])
	}
	if len(out) == 0 {
		t.Fatalf("%s: no literals extracted", name)
	}
	return out
}

// rustWriteOrder returns the keys in the order the Rust writer emits them, read
// out of the to_text body only — scanning the whole file would also match the
// parser's arms and the CLI's output strings.
func rustWriteOrder(t *testing.T, store string) []string {
	t.Helper()
	start := strings.Index(store, "pub fn to_text")
	if start < 0 {
		t.Fatal("to_text not found")
	}
	end := strings.Index(store[start:], "pub fn parse")
	if end < 0 {
		t.Fatal("parse not found after to_text")
	}
	body := store[start : start+end]
	matches := regexp.MustCompile(`"([a-z_]+) = `).FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	if len(out) == 0 {
		t.Fatal("no keys found in to_text")
	}
	return out
}

func stringDiff(got, want []string) string {
	var b strings.Builder
	n := len(got)
	if len(want) > n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		g, w := "<missing>", "<missing>"
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			b.WriteString("index " + strconv.Itoa(i) + ": got " + g + ", want " + w + "\n")
		}
	}
	return b.String()
}

func firstWords(s string, n int) string {
	parts := strings.Fields(s)
	if len(parts) < n {
		n = len(parts)
	}
	return strings.Join(parts[:n], " ")
}

// writtenKeys lists the keys of a marshalled file in order, skipping comments.
// The parity gate reads the consumer's own output instead of a hand-copied list,
// so the expectation cannot drift from the code it is checking.
func writtenKeys(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, _, ok := strings.Cut(line, " = "); ok {
			out = append(out, k)
		}
	}
	return out
}

// TestOptionalKeysArePreserved: the desktop panel stores its language and theme
// rocker in this same file. A save from the TUI must not drop them, and an unset
// one must not be written as an empty value — that would turn "never chosen, let
// the OS decide" into a pinned choice the user never made.
func TestOptionalKeysArePreserved(t *testing.T) {
	s, ignored := arcthumb.Parse("language = zh_CN\ntheme = LIGHT\n")
	if s.Language != "zh" {
		t.Errorf("language = %q, want the canonical zh tag", s.Language)
	}
	if s.Theme != "light" {
		t.Errorf("theme = %q, want light (from_tag lower-cases)", s.Theme)
	}
	if !ignored.Empty() {
		t.Errorf("language and theme are known keys, got %+v", ignored)
	}

	text := arcthumb.Marshal(s)
	if !strings.Contains(text, "language = zh\n") || !strings.Contains(text, "theme = light\n") {
		t.Fatalf("optional keys lost on write:\n%s", text)
	}

	if out := arcthumb.Marshal(arcthumb.Default()); strings.Contains(out, "language") || strings.Contains(out, "theme") {
		t.Errorf("unset preferences were written:\n%s", out)
	}

	// A tag neither front end recognises is dropped rather than stored, matching
	// the Rust `if Theme::from_tag(value).is_some()` guard.
	bad, _ := arcthumb.Parse("language = klingon\ntheme = sepia\n")
	if bad.Language != "" || bad.Theme != "" {
		t.Errorf("unrecognised tags were accepted: language=%q theme=%q", bad.Language, bad.Theme)
	}
}
