package theme_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// consumers walks the front end's own source and collects every
// Components.<field> selector outside this package. Reflection can say what a
// theme declares; only the call sites say what a component actually paints.
func consumers(t *testing.T) map[string]int {
	t.Helper()
	// The walk and the read must use the same file system: paths from a DirFS are
	// relative to that root, so handing one to the cwd-based opener parses
	// nothing and reports every slot as dead.
	fsys := os.DirFS("..")
	fset := token.NewFileSet()
	out := map[string]int{}
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Skip the declaring package (its assignments are not uses) and the test
		// binaries (a test that reads a slot does not put it on screen).
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
			strings.HasPrefix(path, "theme/") || strings.Contains(path, "/theme/") {
			return nil
		}
		src, rerr := fs.ReadFile(fsys, path)
		if rerr != nil {
			t.Errorf("read %s: %v", path, rerr)
			return nil
		}
		f, perr := parser.ParseFile(fset, path, src, 0)
		if perr != nil {
			t.Errorf("parse %s: %v", path, perr)
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Components" {
				out[sel.Sel.Name]++
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("the scan found no Components use at all; it is blind, not the code")
	}
	// Positive control: the help bar is painted by every frame. If the scan
	// cannot see it, "unused" findings mean nothing.
	if out["HelpBar"] == 0 {
		t.Fatal("control failed: HelpBar is painted on every frame but the scan missed it")
	}
	return out
}

// declaredSlots reflects the component-style struct, so the list cannot drift
// from the type the way a hand-copied one would.
func declaredSlots() []string {
	typ := reflect.TypeOf(theme.Components{})
	out := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		out = append(out, typ.Field(i).Name)
	}
	sort.Strings(out)
	return out
}

func TestComponentSlotAudit(t *testing.T) {
	used := consumers(t)
	declared := map[string]bool{}
	for _, f := range declaredSlots() {
		declared[f] = true
	}

	// Abstractions the brief names by hand, and the slots that carry them. A
	// named abstraction with no call site is a style every preset compiles and no
	// component paints: the theme layer would be decorative, not load-bearing.
	painted := map[string][]string{
		"tab":             {"TabBar", "Tab", "TabActive"},
		"list item":       {"ListItem", "ListItemActive"},
		"button":          {"Button", "ButtonPrimary", "ButtonHover"},
		"status":          {"StatusOn", "StatusOff", "StatusWarn", "StatusError", "StatusInfo"},
		"help bar":        {"HelpBar", "HelpKey", "HelpAction", "HelpDim"},
		"inspector strip": {"Inspector", "Description"},
		"dialog":          {"Dialog", "Scrim"},
		"section":         {"Section", "SectionRule"},
		"setting row":     {"Label", "Value", "ValueActive"},
		"slider":          {"SliderTrack", "SliderFill"},
		"toggle":          {"ToggleOn", "ToggleOff"},
		"select":          {"Select", "SelectArrows"},
		"header":          {"Header", "Brand", "HeaderMeta"},
		"sidebar":         {"Nav", "NavItem", "NavItemActive"},
	}
	for name, slots := range painted {
		hit := 0
		for _, s := range slots {
			if used[s] > 0 {
				hit++
			}
		}
		if hit == 0 {
			t.Errorf("%q is a named abstraction no component reads (slots %v)", name, slots)
		}
	}

	// These three are also named by the brief, and phase one has nothing that
	// would paint them: no free-text setting, and a design rule that forbids
	// boxy panels. They must exist as slots a preset can restyle, so the check is
	// existence plus a stated reason, never a silent use.
	declaredOnly := map[string][]string{
		"panel": {"Panel"},
		"card":  {"Card"},
		"input": {"Input", "InputActive"},
	}
	for name, slots := range declaredOnly {
		for _, s := range slots {
			if !declared[s] {
				t.Errorf("%q has no slot %s: the brief asks for the abstraction", name, s)
			}
			if used[s] == 0 && reserved[s] == "" {
				t.Errorf("%s is declared but painted by nothing and has no recorded reason", s)
			}
		}
	}

	var dead []string
	for _, f := range declaredSlots() {
		if used[f] == 0 && reserved[f] == "" {
			dead = append(dead, f)
		}
	}
	if len(dead) > 0 {
		t.Errorf("slots every preset compiles but nothing paints: %v", dead)
	}
}

// reserved is the account of slots that exist without a call site, and why. A
// slot missing from this table and unused is dead weight, not a plan, and the
// audit fails.
var reserved = map[string]string{
	"Panel":       "inset surface the brief requires; phase one paints bands instead, per the no-boxes rule",
	"Card":        "elevated inset the brief requires; nothing in phase one is a card",
	"Input":       "text field the brief requires; no free-text setting exists yet",
	"InputActive": "the focused variant of the above",
}
