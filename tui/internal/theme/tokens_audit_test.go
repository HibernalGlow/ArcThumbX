package theme_test

import (
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/HibernalGlow/ArcThumbX/tui/internal/theme"
)

// The component-slot audit covers what gets painted. These three families are
// the rest of §3's token system, and a token nothing reads is indistinguishable
// on screen from one everything honours: the preset author writes it, Build
// compiles it, and no component asks. So each family is checked against the
// source, with a positive control, because a scan that found nothing would
// otherwise report every token as unused.

func sources(t *testing.T) string {
	t.Helper()
	var all strings.Builder
	fsys := os.DirFS("..")
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, rerr := fs.ReadFile(fsys, path)
		if rerr != nil {
			t.Errorf("read %s: %v", path, rerr)
			return nil
		}
		all.Write(b)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if all.Len() == 0 {
		t.Fatal("the scan collected no source at all")
	}
	return all.String()
}

// roleFields maps each typography role's Go field to the brief's name for it.
func roleFields(t *testing.T) map[string]string {
	t.Helper()
	typ := reflect.TypeOf(theme.Typeography{})
	out := map[string]string{}
	for i := 0; i < typ.NumField(); i++ {
		out[typ.Field(i).Name] = strings.ToLower(typ.Field(i).Name)
	}
	if len(out) < 9 {
		t.Fatalf("only %d typography roles declared; the brief lists nine", len(out))
	}
	return out
}

func TestEveryTypeRoleIsCompiled(t *testing.T) {
	src := sources(t)
	// Positive control: the value role compiles into the row's value style. If
	// this is not found, the reader is blind and every verdict below is noise.
	if !regexp.MustCompile(`typ\.Value\b`).MatchString(src) {
		t.Fatal("control failed: typ.Value is compiled into a component but the scan missed it")
	}
	for field := range roleFields(t) {
		if regexp.MustCompile(`typ\.` + field + `\b`).MatchString(src) {
			continue
		}
		if unusedTypography[field] == "" {
			t.Errorf("role %s is declared by presets and compiled by no style", field)
		}
	}
}

// unusedTypography records a role kept because a preset sets it and the token
// table the brief demands includes it, even though no compiled style reads it.
var unusedTypography = map[string]string{}

func TestEverySpacingTokenIsRead(t *testing.T) {
	src := sources(t)
	if !regexp.MustCompile(`Space\.PadX\b`).MatchString(src) {
		t.Fatal("control failed: box padding reads Space.PadX but the scan missed it")
	}
	typ := reflect.TypeOf(theme.Spacing{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i).Name
		if regexp.MustCompile(`Space\.` + field + `\b`).MatchString(src) {
			continue
		}
		if unusedSpacing[field] == "" {
			t.Errorf("spacing token %s is declared and read by nothing", field)
		}
	}
}

var unusedSpacing = map[string]string{
	"LG": "the brief asks for xs–xl; phase one has no layout that needs a 5th step",
	"XL": "as above",
}

func TestEveryBorderWeightIsUsed(t *testing.T) {
	src := sources(t)
	if !regexp.MustCompile(`Borders\.Accent\b`).MatchString(src) {
		t.Fatal("control failed: the modal frames itself with Borders.Accent")
	}
	for _, w := range []string{"None", "Subtle", "Normal", "Strong", "Accent"} {
		if regexp.MustCompile(`Borders\.` + w + `\b`).MatchString(src) {
			continue
		}
		if unusedBorders[w] == "" {
			t.Errorf("border weight %s is compiled into no component", w)
		}
	}
}

// unusedBorders records a weight the brief requires the table to define but no
// phase-one component frames with: every band is a ground, and the one framed
// surface wants the accent edge. A preset that wants a frameless modal asks for
// it by putting NoEdge in that slot, so the weight is reachable by data.
var unusedBorders = map[string]string{
	"None": "defined so a preset can drop a frame; no component paints a borderless box yet",
}
