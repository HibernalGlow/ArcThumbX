package arcthumb

// PanelChoice is one value the desktop configuration panel's keycaps can
// select, as it is stored in the shared settings file.
type PanelChoice struct {
	// Tag is the value written. The empty tag means "write no key at all", so
	// the panel falls back to its own default instead of an explicit choice.
	Tag string
	// Name is how the panel labels the value. Language names are endonyms: the
	// Rust front end deliberately never translates them, because a Chinese user
	// looks for 中文 rather than for that word spelled in the active language.
	Name string
}

// PanelLocaleChoices mirrors the Rust Locale keycaps in their own order, so the
// first real option here is the first keycap there.
func PanelLocaleChoices() []PanelChoice {
	return []PanelChoice{
		{Tag: "", Name: "Not chosen"},
		{Tag: "zh", Name: "中文"},
		{Tag: "en", Name: "English"},
		{Tag: "ja", Name: "日本語"},
	}
}

// PanelThemeChoices mirrors the Rust Theme rocker.
func PanelThemeChoices() []PanelChoice {
	return []PanelChoice{
		{Tag: "", Name: "Not chosen"},
		{Tag: "dark", Name: "Dark"},
		{Tag: "light", Name: "Light"},
	}
}
