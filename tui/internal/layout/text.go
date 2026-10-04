package layout

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
)

// Fit pads or truncates s to exactly w visible cells, keeping it left aligned.
// Styled input is safe: measurement goes through lipgloss, which ignores SGR.
func Fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	got := lipgloss.Width(s)
	switch {
	case got > w:
		return Truncate(s, w)
	case got < w:
		return s + strings.Repeat(" ", w-got)
	default:
		return s
	}
}

// FitRight aligns s to the right of the cell. Values in a setting row use this,
// which is what lets a column of numbers read as an instrument panel.
func FitRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	got := lipgloss.Width(s)
	if got > w {
		return Truncate(s, w)
	}
	return strings.Repeat(" ", w-got) + s
}

// Truncate cuts s to w visible cells, marking the cut. It never splits a wide
// rune, and it never cuts inside an escape sequence: values reaching here are
// already styled, and a chopped SGR would print "[38;2;" on the screen and shift
// every column downstream.
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	// Reserve a cell for the ellipsis when there is room for one.
	limit := w
	marker := ""
	if w > 1 {
		limit = w - 1
		marker = "…"
	}
	var (
		b       strings.Builder
		visible int
		i       int
	)
	runes := []rune(s)
	for i < len(runes) {
		r := runes[i]
		if r == 0x1b {
			// Copy the whole sequence: it costs no visible cells.
			j := i + 1
			if j < len(runes) && (runes[j] == '[' || runes[j] == ']' || runes[j] == '(') {
				j++
			}
			for j < len(runes) && !isFinal(j, runes) {
				j++
			}
			if j < len(runes) {
				j++
			}
			for k := i; k < j && k < len(runes); k++ {
				b.WriteRune(runes[k])
			}
			i = j
			continue
		}
		rw := runeWidth(r)
		if visible+rw > limit {
			break
		}
		b.WriteRune(r)
		visible += rw
		i++
	}
	return b.String() + marker
}

// isFinal reports whether runes[j] ends an escape sequence, using the ECMA-48
// rule that a CSI terminates on a byte in the range @ through ~.
func isFinal(j int, runes []rune) bool {
	r := runes[j]
	return r >= '@' && r <= '~'
}

func runeWidth(r rune) int {
	if !unicode.IsPrint(r) && r != ' ' {
		return 0
	}
	return lipgloss.Width(string(r))
}

// Lines splits rendering output into lines without losing trailing blanks, so a
// block can be pinned to a fixed height.
func Lines(s string) []string { return strings.Split(s, "\n") }

// JoinLines is the inverse of Lines, named so a page that composes strips and
// blocks does not reach for strings directly.
func JoinLines(lines []string) string { return strings.Join(lines, "\n") }

// BlockHeight counts the lines of a rendered block.
func BlockHeight(s string) int { return lipgloss.Height(s) }

// PadLeft indents every line of a block, the tool for nesting under a section.
func PadLeft(s string, n int) string {
	if n <= 0 {
		return s
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

// ClampTo keeps a block within h lines, dropping the overflow from the bottom.
// The shell uses it as the last guard before painting, so a tall page can never
// push the help bar off screen.
func ClampTo(s string, h int) string {
	if h <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= h {
		return s
	}
	return strings.Join(lines[:h], "\n")
}

// ColumnWidth picks the usable measure: the pane width minus a margin, never
// wider than the reader's preferred line length. Long prose in a 200-column
// terminal is unreadable, so the cap is a design token rather than a constant.
func ColumnWidth(pane, preferred int) int {
	w := pane
	if preferred > 0 && preferred < w {
		w = preferred
	}
	if w < 20 {
		w = 20
	}
	return w
}
