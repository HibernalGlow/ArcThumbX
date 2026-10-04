// Package keymap is the only place that reads Bubble Tea key events. Pages see
// intents, which keeps them testable without synthesising terminal input and
// keeps a rebind in one file.
package keymap

import (
	"strings"

	"charm.land/bubbletea/v2"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/components"
)

// Action is an app-level intent produced from a key press.
type Action int

const (
	ActionNone Action = iota
	// Global tier: these are handled by the app, never by a page.
	ActionQuit
	ActionHelp
	ActionThemes
	ActionSave
	ActionRevert
	ActionPageNext
	ActionPagePrev

	// Focus tier: handed to the active page.
	ActionFocusNext
	ActionFocusPrev
	ActionStepBack
	ActionStepFwd
	ActionActivate
	ActionScrollUp
	ActionScrollDown
	ActionClose
)

func (a Action) String() string {
	switch a {
	case ActionQuit:
		return "quit"
	case ActionHelp:
		return "help"
	case ActionThemes:
		return "themes"
	case ActionSave:
		return "save"
	case ActionRevert:
		return "revert"
	case ActionPageNext:
		return "page-next"
	case ActionPagePrev:
		return "page-prev"
	case ActionFocusNext:
		return "focus-next"
	case ActionFocusPrev:
		return "focus-prev"
	case ActionStepBack:
		return "step-back"
	case ActionStepFwd:
		return "step-fwd"
	case ActionActivate:
		return "activate"
	case ActionScrollUp:
		return "scroll-up"
	case ActionScrollDown:
		return "scroll-down"
	case ActionClose:
		return "close"
	default:
		return "none"
	}
}

// IsGlobal reports whether the app handles the action itself. Pages get the
// rest, which is why the help bar can show a page's own hints beside these.
func (a Action) IsGlobal() bool {
	switch a {
	case ActionQuit, ActionHelp, ActionThemes, ActionPageNext, ActionPagePrev, ActionClose:
		return true
	default:
		return false
	}
}

// KeyMap binds the shared chord set. The layout follows the interaction
// hierarchy: enter and space are the primary act, tab and the arrows are
// secondary movement, esc and q are global.
type KeyMap struct{}

// Default is the shipped binding set.
func Default() KeyMap { return KeyMap{} }

// Resolve translates a key press into an action.
func (KeyMap) Resolve(msg tea.KeyPressMsg) Action {
	shift := msg.Mod&tea.ModShift != 0

	switch msg.Code {
	case tea.KeyEsc:
		return ActionClose
	case tea.KeyUp:
		return ActionFocusPrev
	case tea.KeyDown:
		return ActionFocusNext
	case tea.KeyLeft:
		return ActionStepBack
	case tea.KeyRight:
		return ActionStepFwd
	case tea.KeyEnter:
		return ActionActivate
	case tea.KeyTab:
		if shift {
			return ActionPagePrev
		}
		return ActionPageNext
	case tea.KeyHome:
		return ActionScrollUp
	case tea.KeyEnd:
		return ActionScrollDown
	case tea.KeyPgUp:
		return ActionScrollUp
	case tea.KeyPgDown:
		return ActionScrollDown
	}

	switch strings.ToLower(msg.Text) {
	case " ":
		return ActionActivate
	case "q":
		return ActionQuit
	case "?", "h":
		return ActionHelp
	case "t":
		return ActionThemes
	case "s":
		return ActionSave
	case "r":
		return ActionRevert
	case "j":
		return ActionFocusNext
	case "k":
		return ActionFocusPrev
	case "n":
		return ActionPageNext
	case "p":
		return ActionPagePrev
	case "l":
		return ActionStepFwd
	case "left", "right":
		return ActionNone
	}
	return ActionNone
}

// GlobalHelp is the shared part of the help bar, drawn in the app's fixed order.
func GlobalHelp() []components.HelpItem {
	return []components.HelpItem{
		{Key: "Tab", Action: "page", ID: "help:page"},
		{Key: "Up/Down", Action: "focus"},
		{Key: "Enter", Action: "edit", ID: "help:edit"},
		{Key: "T", Action: "theme", ID: "theme"},
		{Key: "?", Action: "help", ID: "help"},
		{Key: "Q", Action: "quit", ID: "quit"},
	}
}

// Reference is the full binding table, rendered by the help dialog and the About
// page from one source so the two can never disagree.
func Reference() []components.ListItem {
	return []components.ListItem{
		{Label: "Up / Down, J / K", Value: "move focus", Detail: "Focus the previous or next setting."},
		{Label: "Left / Right", Value: "adjust", Detail: "Step a selector, slider or lamp without cycling."},
		{Label: "Enter / Space", Value: "activate", Detail: "Toggle a lamp, cycle a selector, press a button."},
		{Label: "Tab / Shift+Tab", Value: "next page", Detail: "Move through the pages, wrapping at the ends."},
		{Label: "PgUp / PgDown", Value: "scroll", Detail: "Page the pane when a screen does not fit."},
		{Label: "T", Value: "presets", Detail: "Open the theme picker."},
		{Label: "? / H", Value: "help", Detail: "Show this reference."},
		{Label: "S", Value: "save", Detail: "Write the pending ArcThumbX settings."},
		{Label: "R", Value: "revert", Detail: "Discard pending edits."},
		{Label: "Esc", Value: "dismiss", Detail: "Close a dialog, or a pending edit note."},
		{Label: "Q", Value: "quit", Detail: "Leave the panel. Asks first when edits are pending."},
		{Label: "Mouse", Value: "click · wheel", Detail: "Click a row to focus, a value to edit, a wheel to scroll."},
	}
}
