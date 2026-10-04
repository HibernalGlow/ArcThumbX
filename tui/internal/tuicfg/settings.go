// Package tuicfg holds the TUI's own preferences: the frontend-only settings
// that decide how the panel looks and behaves, as opposed to the ArcThumbX
// settings it edits.
//
// It is stored in the same "key = value" dialect as the ArcThumbX settings
// file, on purpose. One format means one parser, and a user who learned where
// the app's own knobs live already knows how to read this one.
package tuicfg

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/kv"
)

// Settings are the frontend preferences. Every field here is editable from
// inside the TUI: a preference nobody can change does not belong in this file.
type Settings struct {
	// ThemeID is a theme.PresetID. Unknown values fall back to the default
	// preset at load time rather than failing to start.
	ThemeID string

	// MouseEnabled turns cell-motion tracking on. Some users want the
	// terminal's own text selection back, which mouse tracking steals.
	MouseEnabled bool

	// ShowSidebar keeps the navigation rail. Below the width breakpoint the
	// shell already folds it into the header tabs, so this is the manual
	// override for a wide terminal.
	ShowSidebar bool

	// ContentWidth caps the measure of long-form pages. A paragraph across
	// two hundred columns is unreadable, so the cap is a preference.
	ContentWidth int

	// ConfirmOnQuit asks before discarding unsaved edits.
	ConfirmOnQuit bool

	// LastPage restores the screen the user was on.
	LastPage string
}

// Default is the shipped configuration: the product's own identity theme, mouse
// on, sidebar on, a 96-column measure and a quit guard.
func Default() Settings {
	return Settings{
		ThemeID:       "retro_future",
		MouseEnabled:  true,
		ShowSidebar:   true,
		ContentWidth:  96,
		ConfirmOnQuit: true,
		LastPage:      "dashboard",
	}
}

// MinContentWidth and MaxContentWidth bound the slider in General.
const (
	MinContentWidth = 60
	MaxContentWidth = 160
)

// Normalize clamps values into range so a hand-edited file cannot produce a
// layout that cannot be drawn.
func (s Settings) Normalize() Settings {
	if s.ContentWidth < MinContentWidth {
		s.ContentWidth = MinContentWidth
	}
	if s.ContentWidth > MaxContentWidth {
		s.ContentWidth = MaxContentWidth
	}
	if s.ThemeID == "" {
		s.ThemeID = Default().ThemeID
	}
	if s.LastPage == "" {
		s.LastPage = Default().LastPage
	}
	return s
}

// Header explains the file to whoever finds it.
func Header() []string {
	return []string{
		"ArcThumbX TUI — front-end preferences.",
		"Written by arcthumb-tui; safe to edit while it is closed.",
		"These are not the ArcThumbX thumbnail settings.",
	}
}

// Marshal renders the on-disk form.
func Marshal(s Settings) string {
	return kv.Encode(Header(), []kv.Item{
		{Key: "theme", Value: s.ThemeID},
		{Key: "mouse", Value: kv.Flag(s.MouseEnabled)},
		{Key: "sidebar", Value: kv.Flag(s.ShowSidebar)},
		{Key: "content_width", Value: strconv.Itoa(s.ContentWidth)},
		{Key: "confirm_quit", Value: kv.Flag(s.ConfirmOnQuit)},
		{Key: "last_page", Value: s.LastPage},
	})
}

// Parse reads the on-disk form, ignoring anything it does not recognise.
func Parse(text string) Settings {
	s := Default()
	doc := kv.Decode(text)
	for _, it := range doc.Items {
		switch it.Key {
		case "theme":
			s.ThemeID = it.Value
		case "mouse":
			s.MouseEnabled = kv.ParseFlag(it.Value)
		case "sidebar":
			s.ShowSidebar = kv.ParseFlag(it.Value)
		case "content_width":
			if n, err := strconv.Atoi(it.Value); err == nil {
				s.ContentWidth = n
			}
		case "confirm_quit":
			s.ConfirmOnQuit = kv.ParseFlag(it.Value)
		case "last_page":
			s.LastPage = it.Value
		}
	}
	return s.Normalize()
}

// Dir returns the preferences directory, honouring ARCThumbX_TUI_CONFIG so
// tests and a second, isolated instance can redirect it.
func Dir() (string, error) {
	if p := strings.TrimSpace(os.Getenv("ARCTHUMB_X_TUI_CONFIG")); p != "" {
		return p, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "arcthumb-tui"), nil
}

// Path is the preferences file inside Dir.
func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "settings"), nil
}

// Load reads the preferences, returning defaults when no file exists yet.
func Load() (Settings, string, error) {
	p, err := Path()
	if err != nil {
		return Default().Normalize(), "", err
	}
	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return Default().Normalize(), p, nil
	case err != nil:
		return Default().Normalize(), p, err
	}
	return Parse(string(data)).Normalize(), p, nil
}

// Save writes the preferences to Path().
func Save(s Settings) error {
	p, err := Path()
	if err != nil {
		return err
	}
	return SaveAt(p, s)
}

// SaveAt writes the preferences to an explicit path. The running app always uses
// this with the path it loaded from: recomputing the location at save time would
// make --settings and an injected test directory silently write somewhere else.
func SaveAt(p string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "settings.tmp.*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.WriteString(Marshal(s.Normalize())); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, p)
}
