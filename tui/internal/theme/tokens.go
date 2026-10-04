package theme

import (
	"reflect"
)

// ColorToken reads a colour token by its exported field name. It exists for
// self-reporting: the About palette preview and the coverage test both need to
// walk the token set without a hand-maintained list that would drift the first
// time a token is added.
func (t *Theme) ColorToken(name string) Color {
	v := reflect.ValueOf(t.Colors).FieldByName(name)
	if !v.IsValid() {
		return Inherit
	}
	return Color(v.String())
}

// TokenNames lists the colour token field names in declaration order.
func TokenNames() []string {
	typ := reflect.TypeOf(Colors{})
	out := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		out = append(out, typ.Field(i).Name)
	}
	return out
}

// MissingColorTokens reports which colour tokens inherit from the terminal
// rather than declaring a value. For Terminal Default that is the point, so this
// is a report, not an error; callers compare it against what they require.
func (t *Theme) MissingColorTokens() []string {
	var out []string
	for _, name := range TokenNames() {
		if t.ColorToken(name) == Inherit {
			out = append(out, name)
		}
	}
	return out
}

// DeclaredColorTokens counts the colour tokens with a value of their own.
func (t *Theme) DeclaredColorTokens() int {
	return len(TokenNames()) - len(t.MissingColorTokens())
}

// RoleNames lists the typographic roles, for the same self-reporting reason.
func RoleNames() []string {
	typ := reflect.TypeOf(Typeography{})
	out := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		out = append(out, typ.Field(i).Name)
	}
	return out
}
