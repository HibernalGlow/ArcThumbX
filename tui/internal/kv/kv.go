// Package kv reads and writes the flat "key = value" dialect ArcThumbX uses
// for its settings files.
//
// The dialect is deliberately not JSON (see the note at the top of
// src/bin/arcthumb-config/settings_store.rs): an unrecognised key is ignored
// rather than failing the whole file, which is what lets an older Quick Look
// extension read a newer config. Both the ArcThumbX settings file and the TUI's
// own preferences file are this format, so they share one parser and one set of
// tolerance rules.
package kv

import (
	"strings"
)

// Item is one "key = value" line in the order it must be written.
type Item struct {
	Key   string
	Value string
}

// Doc is a decoded file: the recognised pairs in file order, plus everything
// that was skipped. Keeping the skipped lines in the result is what lets the
// TUI disclose "2 unrecognised keys ignored" instead of swallowing them.
type Doc struct {
	// Comments holds the leading '#' block verbatim, so a rewritten file
	// still explains itself to whoever cats it.
	Comments []string
	Items    []Item

	// Malformed holds lines with no '=' separator, and Blank lines are
	// dropped silently.
	Malformed []string
}

// Value returns the last value written for key, or "" when absent.
func (d Doc) Value(key string) (string, bool) {
	for i := len(d.Items) - 1; i >= 0; i-- {
		if d.Items[i].Key == key {
			return d.Items[i].Value, true
		}
	}
	return "", false
}

// Keys lists the distinct keys present, in first-seen order.
func (d Doc) Keys() []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(d.Items))
	for _, it := range d.Items {
		if !seen[it.Key] {
			seen[it.Key] = true
			out = append(out, it.Key)
		}
	}
	return out
}

// Decode parses the dialect. It never fails: a corrupt line costs that line,
// never the file, matching the Rust parser's behaviour.
func Decode(text string) Doc {
	var d Doc
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "#"):
			d.Comments = append(d.Comments, trimmed)
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			d.Malformed = append(d.Malformed, trimmed)
			continue
		}
		d.Items = append(d.Items, Item{Key: strings.TrimSpace(key), Value: strings.TrimSpace(value)})
	}
	return d
}

// Encode writes a header comment block followed by the items in order, each
// terminated by LF. Trailing newline is always present: the file is meant to be
// readable with cat and diffable with git.
func Encode(header []string, items []Item) string {
	var b strings.Builder
	for _, h := range header {
		b.WriteString("# ")
		b.WriteString(h)
		b.WriteByte('\n')
	}
	for _, it := range items {
		b.WriteString(it.Key)
		b.WriteString(" = ")
		b.WriteString(it.Value)
		b.WriteByte('\n')
	}
	return b.String()
}

// Flag renders a bool the way ArcThumbX does: 0 or 1, never true/false.
func Flag(on bool) string {
	if on {
		return "1"
	}
	return "0"
}

// ParseFlag reads a bool. Anything that is not an accepted truthy token is off,
// which is the Rust parse_flag rule ("1"|"true"|"yes"|"on").
func ParseFlag(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
