package theme

// Glyphs is the glyph token set. Every mark a component draws — section
// marker, focus pointer, state lamp, slider fill, signal ramp, divider — is a
// token, so a preset can change the whole terminal personality without
// touching component code, and an ASCII-only terminal keeps working.
type Glyphs struct {
	// Section prefixes a section legend.
	Section string
	// Rule is the repeated glyph of a horizontal hairline.
	Rule string
	// Grid is the faint vertical divider between panes.
	Grid string
	// Focus marks the focused row.
	Focus string
	// Bullet is the list item marker when unfocused.
	Bullet string

	// On and Off are the two states of an indicator lamp.
	On  string
	Off string
	// Check and Cross are set/unset marks.
	Check string
	Cross string

	// SliderFull and SliderEmpty draw an instrument-style value bar.
	SliderFull  string
	SliderEmpty string

	// Signal is an ascending load meter ramp, e.g. ▁▃▅▇.
	Signal []string

	// Prev and Next wrap the current choice of a cycling selector.
	Prev string
	Next string

	// Terminal is the CRT/power indicator in the header.
	Terminal string
	// Link joins the header brand, e.g. the "/" of ARC/THUMB.
	Link string
	// Divider separates items on the help bar.
	Divider string

	// DialogClose is the corner mark of a modal.
	DialogClose string
}

// UnicodeGlyphs is the default set: clean geometric marks, no box noise.
func UnicodeGlyphs() Glyphs {
	return Glyphs{
		Section:     "▌",
		Rule:        "─",
		Grid:        "│",
		Focus:       "▸",
		Bullet:      "·",
		On:          "●",
		Off:         "○",
		Check:       "●",
		Cross:       "○",
		SliderFull:  "▓",
		SliderEmpty: "░",
		Signal:      []string{"▁", "▂", "▄", "▇"},
		Prev:        "‹",
		Next:        "›",
		Terminal:    "◍",
		Link:        "/",
		Divider:     "·",
		DialogClose: "×",
	}
}

// ASCIIGlyphs degrades every mark to a byte that exists in every terminal
// font. High-contrast and minimal setups use it.
func ASCIIGlyphs() Glyphs {
	return Glyphs{
		Section:     ">",
		Rule:        "-",
		Grid:        "|",
		Focus:       ">",
		Bullet:      ".",
		On:          "*",
		Off:         "-",
		Check:       "*",
		Cross:       "-",
		SliderFull:  "#",
		SliderEmpty: ".",
		Signal:      []string{"_", "=", "#", "#"},
		Prev:        "<",
		Next:        ">",
		Terminal:    "o",
		Link:        "/",
		Divider:     "|",
		DialogClose: "x",
	}
}

// Merge copies the non-empty fields of over onto base, so a preset can adopt a
// standard set and retune two or three marks.
func (g Glyphs) Merge(over Glyphs) Glyphs {
	pick := func(dst *string, src string) {
		if src != "" {
			*dst = src
		}
	}
	pick(&g.Section, over.Section)
	pick(&g.Rule, over.Rule)
	pick(&g.Grid, over.Grid)
	pick(&g.Focus, over.Focus)
	pick(&g.Bullet, over.Bullet)
	pick(&g.On, over.On)
	pick(&g.Off, over.Off)
	pick(&g.Check, over.Check)
	pick(&g.Cross, over.Cross)
	pick(&g.SliderFull, over.SliderFull)
	pick(&g.SliderEmpty, over.SliderEmpty)
	pick(&g.Prev, over.Prev)
	pick(&g.Next, over.Next)
	pick(&g.Terminal, over.Terminal)
	pick(&g.Link, over.Link)
	pick(&g.Divider, over.Divider)
	pick(&g.DialogClose, over.DialogClose)
	if len(over.Signal) > 0 {
		g.Signal = over.Signal
	}
	return g
}

// WithASCIIRule is a shorthand presets use when they keep Unicode marks but
// want the rules drawn as hyphens.
func (g Glyphs) WithASCIIRule() Glyphs {
	return g.Merge(Glyphs{Rule: "-"})
}
