package dotenv

import (
	"bytes"
	"strings"
	"testing"
)

func TestQuoteRoundTrip(t *testing.T) {
	values := []string{
		"",
		"simple",
		"postgres://user:p%40ss@localhost:5432/db?sslmode=disable",
		"with spaces",
		"  leading and trailing  ",
		"hash # not a comment",
		"value#nospace",
		"$HOME and ${PATH}",
		`price=$5`,
		`double "quotes" inside`,
		`back\slash`,
		`trailing\`,
		"it's got an apostrophe",
		"multi\nline\nvalue",
		"windows\r\nline",
		`mixed 'single' and "double" and $VAR and \n literal`,
		`a\$b`,
		"unicode: データベース 🔐",
		"-----BEGIN PRIVATE " + "KEY-----\nMIIEvQIBADANBg\n-----END PRIVATE KEY-----",
		`"starts with quote`,
		`'starts with single`,
	}

	var entries []Entry
	for i, v := range values {
		entries = append(entries, Entry{Key: keyFor(i), Value: v, Comment: "note for " + keyFor(i)})
	}

	var buf bytes.Buffer
	if err := Write(&buf, []string{"generated"}, entries); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, buf.String())
	}
	for i, v := range values {
		if got[keyFor(i)] != v {
			t.Errorf("round trip of %q: got %q", v, got[keyFor(i)])
		}
	}
}

func TestQuoteUnrepresentable(t *testing.T) {
	for _, v := range []string{"it's\\", "line\nends\\", `it's "quoted"`} {
		if _, err := Quote(v); err == nil {
			t.Errorf("Quote(%q) should fail", v)
		}
	}
}

func TestQuoteStyle(t *testing.T) {
	tests := map[string]string{
		"plain":       "plain",
		"a b":         "'a b'",
		"$X":          "'$X'",
		"it's":        `"it's"`,
		"one\ntwo":    `"one\ntwo"`,
		"https://x/y": "https://x/y",
	}
	for in, want := range tests {
		got, err := Quote(in)
		if err != nil || got != want {
			t.Errorf("Quote(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestWriteMultilineComment(t *testing.T) {
	var buf bytes.Buffer
	Write(&buf, nil, []Entry{{Key: "K", Value: "v", Comment: "line one\nline two"}})
	want := "# line one\n# line two\nK=v\n"
	if buf.String() != want {
		t.Errorf("got %q want %q", buf.String(), want)
	}
}

func keyFor(i int) string {
	return "KEY_" + strings.Repeat("X", i)
}
