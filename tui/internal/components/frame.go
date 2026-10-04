package components

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/layout"
)

// Header is the instrument nameplate: brand at the left, live metadata at the
// right, one hairline beneath. It is the only full-width rule in the layout, and
// that is deliberate — with a single frame line, everything below it reads as a
// pane rather than as a stack of competing boxes.
type Header struct {
	// Brand is drawn in the title role, e.g. "ARC/THUMB".
	Brand string
	// Suffix is drawn in the accent colour, the amber X of the wordmark.
	Suffix string
	// Mode is the panel designation, e.g. "CONFIG".
	Mode string
	// Meta is right-aligned readout cells: preset name, cell size, version.
	Meta []string
	// Signal is an optional indicator lamp group.
	Signal *Meter
}

// HeaderHeight is the number of lines the header occupies, including its rule.
const HeaderHeight = 2

// View draws the header.
func (h *Header) View(c Ctx) string {
	t := c.T
	w := max(c.Rect.W, 20)

	brand := t.Components.Brand.Render(layout.Truncate(h.Brand, w/2))
	if h.Suffix != "" {
		// The wordmark's amber X needs its own cell of air: butted against the
		// word it reads as a typo rather than as the X of ArcThumbX.
		brand += " " + t.Components.NavBadge.Render(h.Suffix)
	}
	left := brand
	if h.Mode != "" {
		left += "  " + t.Components.Caption.Render(strings.ToUpper(h.Mode))
	}

	var cells []string
	for _, m := range h.Meta {
		cells = append(cells, t.Components.HeaderMeta.Render(m))
	}
	// The right-hand readout is decorative, so on a narrow terminal cells are
	// dropped from the front — the preset name is the one the picker already
	// shows — until it fits. Truncating the left half alone cannot work: a right
	// side wider than the line would push the total past the screen and wrap it.
	limit := max(w-6, 4)
	for len(cells) > 1 && lipgloss.Width(strings.Join(cells, "  ")) > limit {
		cells = cells[1:]
	}
	right := strings.Join(cells, "  ")
	if h.Signal != nil {
		if right != "" {
			right += "  "
		}
		right += h.Signal.View(t)
	}
	if lw := lipgloss.Width(right); lw > limit {
		right = layout.Truncate(right, limit)
	}
	// The two halves are sized from their own measured widths: padding the
	// left half to w-pad as well would make the line w+rightW long, and a header
	// one cell too wide wraps, which moves every row below it up by one.
	left = layout.Truncate(left, max(w-lipgloss.Width(right)-1, 4))
	pad := max(w-lipgloss.Width(left)-lipgloss.Width(right), 1)
	line1 := left + strings.Repeat(" ", pad) + right
	if rightW := lipgloss.Width(right); rightW == 0 {
		line1 = layout.Fit(left, w)
	}
	return line1 + "\n" + t.Rule(w, t.Colors.Border)
}

// HelpItem is one key/action pair. ID makes it clickable: a non-empty ID turns
// the hint into a real button, so the help bar doubles as the mouse user's
// menu.
type HelpItem struct {
	Key    string
	Action string
	ID     string
}

// HelpBar is the bottom key strip: the third information tier, always visible,
// never more than one line.
type HelpBar struct {
	ID    string
	Items []HelpItem
}

// View draws the strip.
func (h *HelpBar) View(c Ctx) string {
	t := c.T
	w := max(c.Rect.W, 20)
	x := c.Rect.X
	var parts []string
	for i, item := range h.Items {
		key := t.Components.HelpKey.Render(strings.ToUpper(item.Key))
		act := t.Components.HelpAction.Render(item.Action)
		cell := key + " " + act
		width := lipgloss.Width(cell)
		parts = append(parts, cell)
		if item.ID != "" {
			c.Register(Target{Kind: HitButton, Index: i, ID: item.ID},
				layout.R(x, c.Rect.Y, width, 1))
		}
		x += width
		if i != len(h.Items)-1 {
			sep := " " + t.Components.HelpDim.Render(t.Glyphs.Divider) + " "
			parts = append(parts, sep)
			x += 3
		}
	}
	return layout.Fit(strings.Join(parts, ""), w)
}

// Dialog is a modal that interrupts the page beneath it. It is the one place in
// the design system allowed a full box, because a modal's job is to be
// unmistakably separate.
type Dialog struct {
	ID    string
	Title string
	// Lines is pre-rendered body text.
	Lines []string
	// Body, when set, renders the body itself (the keymap reference and the
	// palette preview use this) and is given the interior rect.
	Body func(Ctx) string
	// BodyHeight is how many lines Body needs.
	BodyHeight int
	Buttons    []Button
	Selected   int

	// buttonY is where the button row landed in the last frame, so a click can
	// be resolved without re-deriving the box geometry.
	buttonY int
}

// Move cycles the highlighted answer.
func (d *Dialog) Move(dir int) bool {
	if len(d.Buttons) == 0 || dir == 0 {
		return false
	}
	n := len(d.Buttons)
	i := (d.Selected + dir) % n
	if i < 0 {
		i += n
	}
	if i == d.Selected {
		return false
	}
	d.Selected = i
	return true
}

// Activate presses the highlighted answer and reports which one it was.
func (d *Dialog) Activate() (int, bool) {
	if len(d.Buttons) == 0 {
		return 0, false
	}
	if d.Selected < 0 || d.Selected >= len(d.Buttons) {
		d.Selected = 0
	}
	return d.Selected, true
}

// IndexOf maps an answer's ID to its position, or -1. A click on an answer has
// to go through the same commit path as Enter, otherwise the two input methods
// implement different dialogs.
func (d *Dialog) IndexOf(id string) int {
	for i, b := range d.Buttons {
		if b.ID == id {
			return i
		}
	}
	return -1
}

// View draws the scrim plus the box, centred.
//
// Terminals have no alpha channel, so there is no such thing here as a dimmed
// backdrop: the modal takes the whole ground and the page beneath it is not
// drawn at all. That also means no stale click targets survive underneath it.
func (d *Dialog) View(c Ctx) string {
	t := c.T
	w := max(c.Rect.W, 24)
	h := max(c.Rect.H, 7)

	content := []string{
		t.Components.Heading.Render(layout.Truncate(d.Title, w-10)),
		t.Components.SectionRule.Render(strings.Repeat(t.Glyphs.Rule, max(w-10, 4))),
	}
	body := d.Lines
	innerW := max(w-10, 20)
	innerH := max(d.BodyHeight, 1)
	if d.Body != nil {
		// Measure-only pass: the box is centred from the content it contains, and
		// the content depends on where the box is. Registering hits here would put
		// a body row several cells above where it is painted, so a click would
		// select the wrong item and look like it worked.
		body = layout.Lines(d.Body(Ctx{T: t, Rect: layout.R(0, 0, innerW, innerH)}))
	}
	// Fit the body to what is left of the box after title, buttons and their
	// breathing room, so a long help text scrolls off the bottom rather than
	// pushing the answers off screen.
	budget := h - HeaderHeight - 6
	if len(body) > budget && budget > 0 {
		body = body[:budget]
	}
	content = append(content, body...)
	content = append(content, "")

	bar := &ButtonBar{ID: d.ID + ":buttons", Buttons: d.Buttons, Hover: d.Selected}
	content = append(content, bar.View(Ctx{
		T:    t,
		Rect: layout.R(0, 0, max(w-10, 20), 1),
	}))

	// The frame comes from the theme's Dialog slot: the accent border and the
	// interior ground are a preset's decision. This component only owns geometry.
	box := t.Components.Dialog.Render(strings.Join(content, "\n"))

	// The buttons are the last line of content, so their absolute row follows
	// from the content length and the box's own top border + padding. Deriving it
	// from boxH instead put them on the border row: a click then hit the scrim
	// and did nothing, while everything still looked right.
	boxW, boxH := lipgloss.Width(box), lipgloss.Height(box)
	originX := c.Rect.X + (w-boxW)/2
	originY := c.Rect.Y + (h-boxH)/2
	if originX < c.Rect.X {
		originX = c.Rect.X
	}
	if originY < c.Rect.Y {
		originY = c.Rect.Y
	}
	const boxChromeAboveContent = 2 // top border, then top padding
	d.buttonY = originY + boxChromeAboveContent + len(content) - 1

	// Re-register the button row at its true absolute position: bar.View drew
	// it into a scratch rect, and only the centring above knows where it lands.
	if c.Hits != nil {
		// Paint order decides precedence and the last entry wins, so the scrim
		// goes in first: it must swallow clicks meant for the page underneath,
		// but never the body or the answers drawn on top of it.
		c.Hits.Add(layout.HitID("dialog:scrim"), layout.R(c.Rect.X, c.Rect.Y, w, h),
			Target{Kind: HitRow, ID: d.ID + ":scrim"})
		if d.Body != nil {
			// Content index 2, after the title and the rule under it.
			bodyY := originY + boxChromeAboveContent + 2
			d.Body(Ctx{T: t, Hits: c.Hits,
				Rect: layout.R(originX+3, bodyY, innerW, innerH)})
		}
		x := originX + 3
		for i, btn := range d.Buttons {
			width := len([]rune(" " + strings.ToUpper(btn.Label) + " "))
			c.Hits.Add(layout.HitID("button:"+btn.ID), layout.R(x, d.buttonY, width, 1),
				Target{Kind: HitButton, Index: i, ID: btn.ID})
			x += width + 1
		}
	}

	ground := t.Components.Scrim.Width(w).Height(h).
		Align(lipgloss.Center).AlignVertical(lipgloss.Center)
	return ground.Render(box)
}

// Shell is the application frame: header, navigation rail, page pane, inspector
// strip and help bar, with a modal replacing everything when one is open.
//
// It owns all geometry decisions, including the responsive one, so no page ever
// asks how wide the terminal is.
type Shell struct {
	Header    Header
	Nav       *Sidebar
	Folded    *Tabs
	Inspector Inspector
	Help      HelpBar

	// Content draws the page into the pane it is given.
	Content func(Ctx) string

	// Overlay sits above everything when non-nil.
	Overlay *Dialog

	// ShowNav is the user's preference; FoldAt is the width below which the
	// rail becomes a tab strip no matter what the preference says.
	ShowNav bool
	FoldAt  int

	// Pane is filled in by View so the app can route wheel events at the
	// region the user is actually pointing at.
	Pane layout.Rect
}

// FoldAtDefault is the width below which a rail plus a readable measure no
// longer fit side by side.
const FoldAtDefault = 84

// View renders exactly c.Rect.H lines. Pinning the height is what stops the
// help bar from jittering off screen when a page grows by one line.
func (s *Shell) View(c Ctx) string {
	t := c.T
	w := max(c.Rect.W, 20)
	h := max(c.Rect.H, 6)

	if s.Overlay != nil {
		if c.Hits != nil {
			c.Hits.Reset()
		}
		return s.Overlay.View(Ctx{T: t, Rect: layout.R(c.Rect.X, c.Rect.Y, w, h), Hits: c.Hits})
	}

	// The bottom block is fixed: one rule, the inspector readout, the help bar.
	folded := !s.ShowNav || w < s.foldAt()
	footerH := 3
	contentH := h - HeaderHeight - footerH
	if contentH < 2 {
		contentH = 2
	}

	navW := 0
	if !folded {
		navW = DefaultNavWidth
		if s.Nav != nil && s.Nav.Width > 0 {
			navW = s.Nav.Width
		}
		// The separator column and the pane's gutter are reserved here rather
		// than appended at draw time: two extra cells painted outside the
		// allocation are what makes every line one cell too wide to fit.
		navW += 2
	}
	paneW := max(w-navW, 20)
	paneX := c.Rect.X + navW

	// The header is split rather than appended whole, because the height clamp
	// at the end counts lines: a single element holding two rows would push the
	// help bar one line past the bottom.
	lines := layout.Lines(s.Header.View(Ctx{T: t, Rect: layout.R(c.Rect.X, c.Rect.Y, w, HeaderHeight)}))
	for i := range lines {
		lines[i] = paintBand(t.Components.Header, lines[i], w)
	}

	// A folded rail costs the pane one line, which is why the tab strip is laid
	// out inside the band rather than added to the header.
	tabH := 0
	if folded && s.Folded != nil {
		tabH = 1
	}
	band := make([]string, 0, contentH)
	if tabH == 1 {
		// A folded rail is still chrome, so it sits on the tab bar's lifted
		// ground rather than the page canvas that starts below it.
		band = append(band, paintBand(t.Components.TabBar, s.Folded.View(Ctx{
			T: t, Rect: layout.R(c.Rect.X, c.Rect.Y+HeaderHeight, w, 1), Hits: c.Hits,
		}), w))
	}
	pageH := max(contentH-tabH, 1)
	s.Pane = layout.R(paneX, c.Rect.Y+HeaderHeight+tabH, paneW, pageH)
	page := ""
	if s.Content != nil {
		page = s.Content(Ctx{
			T: t, Rect: s.Pane, Hits: c.Hits, Width: paneW,
		})
	}
	pageLines := layout.Lines(page)

	var railLines []string
	if !folded && s.Nav != nil {
		rail := s.Nav.View(Ctx{
			T: t, Rect: layout.R(c.Rect.X, c.Rect.Y+HeaderHeight, navW-2, pageH), Hits: c.Hits,
		})
		railLines = layout.Lines(rail)
		for len(railLines) < pageH {
			railLines = append(railLines, "")
		}
		if len(railLines) > pageH {
			railLines = railLines[:pageH]
		}
	}
	for len(pageLines) < pageH {
		pageLines = append(pageLines, "")
	}
	if len(pageLines) > pageH {
		pageLines = pageLines[:pageH]
	}
	for i := 0; i < pageH; i++ {
		switch {
		case folded:
			// No rail, so the pane owns the whole width and the chrome bands
			// are the only lifted surfaces.
			band = append(band, paintBand(t.Components.Root, pageLines[i], w))
		default:
			rail := ""
			if i < len(railLines) {
				rail = railLines[i]
			}
			// Each half is painted to its own reserved width, because a single
			// Style over the joined line would put the pane's ground under the
			// rail. Together the three parts are exactly w cells.
			band = append(band, paintBand(t.Components.Nav, rail, navW-2)+
				paintBand(t.Components.Root,
					t.Components.Grid.Render(t.Glyphs.Grid)+" "+pageLines[i], w-navW+2))
		}
	}
	lines = append(lines, band...)

	// Footer: rule, inspector readout, help bar.
	footY := c.Rect.Y + HeaderHeight + len(band)
	lines = append(lines, paintBand(t.Components.Root, t.FrameRule(w), w))
	lines = append(lines, paintBand(t.Components.Inspector, s.Inspector.View(Ctx{
		T: t, Rect: layout.R(c.Rect.X, footY+1, w, 1), Hits: c.Hits,
	}), w))
	lines = append(lines, paintBand(t.Components.HelpBar, s.Help.View(Ctx{
		T: t, Rect: layout.R(c.Rect.X, footY+2, w, 1), Hits: c.Hits,
	}), w))

	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (s *Shell) foldAt() int {
	if s.FoldAt > 0 {
		return s.FoldAt
	}
	return FoldAtDefault
}

// paintBand stretches an existing component style across the full width behind
// an already composed line. Every strip the shell draws goes through this rather
// than a colour picked here: a terminal only shows a theme's ground where
// something paints it, and a component that names a colour token directly is the
// coupling the theme layer exists to prevent. Inherit compiles to no SGR, so a
// preset that inherits its terminal's palette is untouched by this.
func paintBand(s lipgloss.Style, line string, w int) string {
	if w <= 0 {
		return line
	}
	return s.Width(w).ColorWhitespace(true).Render(line)
}
