package theme

// Factory builds a fresh compiled theme. Presets are registered as factories so
// no two screens can ever share a mutated instance, and so a preset stays a
// plain function of no arguments.
type Factory func() *Theme

// Entry is one registered preset plus its position in the picker.
type Entry struct {
	ID    PresetID
	Name  string
	Blurb string
	Make  Factory
}

// DefaultPreset is what ArcThumbX TUI opens with. Retro Future A is the
// product's visual identity, not a decoration layered on top of it.
const DefaultPreset PresetID = "retro_future"

var entries []Entry

// Register adds a preset. Call it once per preset from Load; the order of the
// calls is the order shown in the theme picker.
func Register(e Entry) {
	if e.Make == nil || e.ID == "" {
		panic("theme: preset registered without an ID or factory")
	}
	for _, prev := range entries {
		if prev.ID == e.ID {
			panic("theme: duplicate preset " + string(e.ID))
		}
	}
	entries = append(entries, e)
}

// Registered lists the presets in picker order.
func Registered() []Entry {
	out := make([]Entry, len(entries))
	copy(out, entries)
	return out
}

// Names lists preset IDs, for validation and for the help text.
func Names() []PresetID {
	out := make([]PresetID, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}

// Make compiles a preset by ID.
func Make(id PresetID) (*Theme, bool) {
	for _, e := range entries {
		if e.ID == id {
			return e.Make(), true
		}
	}
	return nil, false
}

// MustMake compiles a preset, falling back to the default preset when the ID is
// unknown. A stale config value must never stop the TUI from starting.
func MustMake(id PresetID) *Theme {
	if t, ok := Make(id); ok {
		return t
	}
	if t, ok := Make(DefaultPreset); ok {
		return t
	}
	panic("theme: no presets registered; call presets.Load()")
}
