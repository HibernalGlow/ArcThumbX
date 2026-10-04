// Command arcthumb-tui is ArcThumbX's terminal front end.
//
// It is a separate Go program on purpose: it reads and writes the same
// configuration file the Rust config app and the shell extension use, and it
// links nothing from the Rust core.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/app"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/arcthumb"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/render"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme/presets"
	"github.com/HibernalGlow/ArcThumbX/tui/internal/tuicfg"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "arcthumb-tui:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		flagPreset   = flag.String("theme", "", "preset id: "+joinIDs(theme.Names()))
		flagSettings = flag.String("settings", "", "path to the ArcThumbX settings file")
		flagOffline  = flag.Bool("offline", false, "use an in-memory fixture; never touch disk")
		flagReadOnly = flag.Bool("read-only", false, "refuse to write ArcThumbX settings")
		flagRender   = flag.String("render", "", "print one frame and exit: "+strings.Join(render.Pages(), "|"))
		flagDialog   = flag.String("dialog", "", "with --render, open a modal first: themes|help|quit")
		flagSize     = flag.String("size", "110x34", "COLSxROWS for --render")
		flagPlain    = flag.Bool("plain", false, "with --render, strip colour escapes")
		flagList     = flag.Bool("list-themes", false, "print the registered presets")
		flagVersion  = flag.Bool("version", false, "print the version")
	)
	flag.Parse()

	presets.Load()

	if *flagVersion {
		fmt.Println("arcthumb-tui " + buildVersion())
		return nil
	}
	if *flagList {
		for _, e := range theme.Registered() {
			fmt.Printf("%-18s %s\n", e.ID, e.Blurb)
		}
		return nil
	}

	prefs, prefsPath, err := loadPrefs(*flagOffline)
	if err != nil {
		return err
	}
	if *flagPreset != "" {
		if _, ok := theme.Make(theme.PresetID(*flagPreset)); !ok {
			return fmt.Errorf("unknown preset %q; try --list-themes", *flagPreset)
		}
		prefs.ThemeID = *flagPreset
	}
	store, err := pickStore(*flagOffline, *flagSettings)
	if err != nil {
		return err
	}

	if *flagRender != "" {
		return renderFrame(*flagRender, *flagDialog, *flagSize, *flagPlain, prefs, store, *flagPreset)
	}

	m := app.New(app.Options{
		Store:     store,
		Prefs:     prefs,
		PrefsPath: prefsPath,
		Version:   buildVersion(),
		ReadOnly:  *flagReadOnly,
	})
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}

// loadPrefs reads the front-end's own file. Offline mode skips it entirely so a
// fixture run cannot leave a preferences file behind.
func loadPrefs(offline bool) (tuicfg.Settings, string, error) {
	if offline {
		return tuicfg.Default(), "", nil
	}
	s, path, err := tuicfg.Load()
	if err != nil {
		// A missing or unreadable preferences file is not a reason to refuse to
		// start: fall back to defaults and keep writing disabled if the path
		// could not be resolved at all.
		return tuicfg.Default(), "", nil
	}
	return s, path, nil
}

func pickStore(offline bool, path string) (arcthumb.Store, error) {
	if path != "" {
		return arcthumb.NewFileStore(path), nil
	}
	if offline {
		return &arcthumb.MemoryStore{Writable: true}, nil
	}
	return arcthumb.DefaultStore()
}

func renderFrame(page, dialog, size string, plain bool, prefs tuicfg.Settings, store arcthumb.Store, preset string) error {
	cols, rows, err := parseSize(size)
	if err != nil {
		return err
	}
	var id theme.PresetID
	if preset != "" {
		id = theme.PresetID(preset)
	}
	frame := render.Frame(render.Options{
		Preset:  id,
		Page:    page,
		Dialog:  dialog,
		Width:   cols,
		Height:  rows,
		Store:   store,
		Prefs:   prefs,
		Version: buildVersion(),
	})
	out := frame.ANSI
	if plain {
		out = frame.Plain
	}
	fmt.Println(out)
	return nil
}

func parseSize(s string) (int, int, error) {
	w, h, ok := strings.Cut(s, "x")
	if !ok {
		return 0, 0, fmt.Errorf("--size wants COLSxROWS, got %q", s)
	}
	cols, err := strconv.Atoi(strings.TrimSpace(w))
	if err != nil {
		return 0, 0, fmt.Errorf("--size columns: %w", err)
	}
	rows, err := strconv.Atoi(strings.TrimSpace(h))
	if err != nil {
		return 0, 0, fmt.Errorf("--size rows: %w", err)
	}
	if cols < 30 || rows < 8 {
		return 0, 0, fmt.Errorf("--size %dx%d is too small to draw the panel", cols, rows)
	}
	return cols, rows, nil
}

// buildVersion reads the binary's own build info. Nothing is stamped by hand, so
// the string cannot be a value the build never applied.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

func joinIDs(ids []theme.PresetID) string {
	if len(ids) == 0 {
		return "see --list-themes"
	}
	presets.Load()
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, string(id))
	}
	return strings.Join(names, "|")
}
