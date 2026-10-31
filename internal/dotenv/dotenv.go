// Package dotenv reads and writes .env files.
//
// Parsing is delegated to github.com/joho/godotenv. Writing picks the least
// surprising quoting for each value so the output survives a round trip
// through godotenv, Docker's --env-file and the Node dotenv package.
package dotenv

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/joho/godotenv"
)

// Entry is one KEY=value line, optionally preceded by a comment.
type Entry struct {
	Key     string
	Value   string
	Comment string
}

// Parse reads KEY=value pairs from r.
func Parse(r io.Reader) (map[string]string, error) {
	return godotenv.Parse(r)
}

// Quote returns value encoded for the right-hand side of a .env line.
func Quote(value string) (string, error) {
	switch {
	case value == "":
		return "", nil
	case isBare(value):
		return value, nil
	case !strings.ContainsAny(value, "'\n\r") && !strings.HasSuffix(value, `\`):
		// Single quotes are literal in every common dotenv dialect.
		return "'" + value + "'", nil
	case strings.HasSuffix(value, `\`), strings.HasSuffix(value, `"`):
		// godotenv cannot terminate a double-quoted value ending in either.
		return "", fmt.Errorf("value cannot be represented in a .env file (contains a quote or newline and ends with %q)", value[len(value)-1:])
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '$':
			b.WriteString(`\$`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String(), nil
}

// isBare reports whether value can be written without quotes: no
// whitespace, no variable expansion, no comment marker and no leading quote.
func isBare(value string) bool {
	if value[0] == '\'' || value[0] == '"' {
		return false
	}
	for _, r := range value {
		switch {
		case r <= ' ', r == '$', r == '#', r == '\'', r == '"', r == '`', r == 0x7f:
			return false
		}
	}
	return true
}

// Write renders entries as a .env file. header lines are written as
// comments at the top of the file.
func Write(w io.Writer, header []string, entries []Entry) error {
	bw := bufio.NewWriter(w)
	for _, h := range header {
		fmt.Fprintf(bw, "# %s\n", h)
	}
	if len(header) > 0 && len(entries) > 0 {
		fmt.Fprintln(bw)
	}
	for _, e := range entries {
		if e.Comment != "" {
			for _, line := range strings.Split(e.Comment, "\n") {
				fmt.Fprintf(bw, "# %s\n", line)
			}
		}
		q, err := Quote(e.Value)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Key, err)
		}
		fmt.Fprintf(bw, "%s=%s\n", e.Key, q)
	}
	return bw.Flush()
}
