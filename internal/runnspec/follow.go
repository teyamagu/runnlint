package runnspec

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
)

// Canon puts notation-definition bytes into a form that can be compared aside from newline differences.
func Canon(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.TrimRight(b, "\n")
}

// Changed returns the names of files whose imported definition differs from the latest.
// The result is empty when only the tag moved forward and the contents are the same.
func Changed(currentSchema, currentCDP, latestSchema, latestCDP []byte) []string {
	var out []string
	if !bytes.Equal(Canon(currentSchema), Canon(latestSchema)) {
		out = append(out, "runbook.schema.yaml")
	}
	if !bytes.Equal(Canon(currentCDP), Canon(latestCDP)) {
		out = append(out, "cdpfn.src")
	}
	return out
}

// WriteNotation writes the tag and the two definition files into dir.
func WriteNotation(dir, tag string, schema, cdp []byte) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return message.New(message.TagEmpty)
	}
	if len(bytes.TrimSpace(schema)) == 0 || len(bytes.TrimSpace(cdp)) == 0 {
		return message.New(message.SpecEmpty)
	}
	files := []struct {
		name string
		body []byte
	}{
		{"VERSION", []byte(tag + "\n")},
		{"runbook.schema.yaml", withTrailingNL(Canon(schema))},
		{"cdpfn.src", withTrailingNL(Canon(cdp))},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.name), file.body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func withTrailingNL(b []byte) []byte {
	return append(append([]byte{}, b...), '\n')
}
