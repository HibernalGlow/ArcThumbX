package arcthumb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrNotWired marks a capability this build can report on but not perform. The
// UI renders such a row as read-only disclosure instead of a control that would
// silently do nothing.
var ErrNotWired = errors.New("arcthumb: not wired in the TUI on this platform")

// Store is the seam between the TUI and however ArcThumbX persists the shared
// settings. Keeping it an interface is what lets a Windows registry backend, or
// a future IPC client for the Rust helper, drop in without touching a page.
type Store interface {
	// Kind names the backend for the status line: "file", "registry", …
	Kind() string
	// Location is the human-readable place settings live.
	Location() string
	// Load returns the effective settings plus anything the file carried that
	// this build could not use.
	Load() (Settings, Ignored, error)
	// Save writes settings back. Implementations must be atomic: the shell
	// extension reads this file on every request.
	Save(Settings) error
}

// Regenerator is an optional capability: dropping the host's thumbnail cache
// so it re-asks the extension. Asserted for, never assumed.
type Regenerator interface {
	Regenerate(ctx context.Context) error
}

// FileStore reads and writes the macOS settings file that the Quick Look
// extension reads, at the same path the Rust helper uses:
//
//	$HOME/Library/Containers/com.citrussoda.ArcThumb.thumbnail/Data/
//	    Library/Application Support/ArcThumb/settings
type FileStore struct {
	path string
}

// ExtensionBundleID is the sandboxed extension whose container holds the file.
const ExtensionBundleID = "com.citrussoda.ArcThumb.thumbnail"

// SettingsDirRelPath is the container-relative location of the file.
const SettingsRelPath = "Library/Application Support/ArcThumb/settings"

// MacOSSettingsPath resolves the container file under home, mirroring
// settings_path() in the Rust helper.
func MacOSSettingsPath(home string) string {
	return filepath.Join(home, "Library/Containers", ExtensionBundleID, "Data",
		filepath.FromSlash(SettingsRelPath))
}

// NewFileStore points a store at an explicit path, which is how tests and
// --settings exercise the real write protocol without redirecting $HOME.
func NewFileStore(path string) *FileStore { return &FileStore{path: path} }

func (*FileStore) Kind() string { return "file" }

func (s *FileStore) Location() string { return s.path }

func (s *FileStore) Load() (Settings, Ignored, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		// No file yet simply means nobody has saved settings.
		return Default(), Ignored{}, nil
	}
	if err != nil {
		return Default(), Ignored{}, err
	}
	s2, ig := Parse(string(data))
	return s2.Clamp(), ig, nil
}

// Save writes atomically: a temp file in the same directory, then rename. The
// extension must never observe a half-written file.
func (s *FileStore) Save(want Settings) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "settings.tmp.*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.WriteString(Marshal(want.Clamp())); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, s.path); err != nil {
		return err
	}
	return nil
}

// Regenerate asks Quick Look to drop its cached thumbnails, the same supported
// route the macOS GUI takes. It changes no settings, and the UI only offers it
// behind a confirmation.
func (*FileStore) Regenerate(ctx context.Context) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("%w: qlmanage is macOS only", ErrNotWired)
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/qlmanage", "-r", "cache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qlmanage -r cache: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RegistryStore stands where the Windows backend will go: HKCU\Software\ArcThumb
// through the same key names. It is deliberately inert — reading the registry
// needs the platform API, and guessing at it from a macOS box would be worse
// than saying so.
type RegistryStore struct{}

func (*RegistryStore) Kind() string     { return "registry" }
func (*RegistryStore) Location() string { return `HKCU\Software\ArcThumb` }
func (*RegistryStore) Load() (Settings, Ignored, error) {
	return Default(), Ignored{}, fmt.Errorf("%w: registry backend", ErrNotWired)
}
func (*RegistryStore) Save(Settings) error {
	return fmt.Errorf("%w: registry backend", ErrNotWired)
}

// DefaultStore picks the backend for the current platform, honouring
// ARCThumbX_SETTINGS so a demo or test run can point elsewhere.
func DefaultStore() (Store, error) {
	if p := strings.TrimSpace(os.Getenv("ARCTHUMB_X_SETTINGS")); p != "" {
		return NewFileStore(p), nil
	}
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve settings path: %w", err)
		}
		return NewFileStore(MacOSSettingsPath(home)), nil
	default:
		return &RegistryStore{}, nil
	}
}

// MemoryStore is the offline backend: it holds factory defaults in RAM and
// never touches the disk. Used by --offline and by the render fixtures.
type MemoryStore struct {
	S Settings
	// Writable lets a fixture prove the save path ran.
	Writable bool
}

func (m *MemoryStore) Kind() string     { return "memory" }
func (m *MemoryStore) Location() string { return "in-memory (no file)" }
func (m *MemoryStore) Load() (Settings, Ignored, error) {
	if m.S == (Settings{}) {
		m.S = Default()
	}
	return m.S.Clamp(), Ignored{}, nil
}
func (m *MemoryStore) Save(s Settings) error {
	if !m.Writable {
		return fmt.Errorf("%w: memory store is read-only", ErrNotWired)
	}
	m.S = s.Clamp()
	return nil
}

// Check that the backend chosen at runtime really satisfies the interfaces the
// UI asserts for.
var (
	_ Store       = (*FileStore)(nil)
	_ Store       = (*RegistryStore)(nil)
	_ Store       = (*MemoryStore)(nil)
	_ Regenerator = (*FileStore)(nil)
)
