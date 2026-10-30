package report

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

// Diagnostics at the same file and line are ordered by column before rule.
func TestCovWriteSortsByColumn(t *testing.T) {
	findings := []finding.Finding{
		{File: "a.yml", Line: 1, Column: 9, Severity: finding.Error, Rule: "a-rule", Message: "late"},
		{File: "a.yml", Line: 1, Column: 2, Severity: finding.Error, Rule: "z-rule", Message: "early"},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "text", findings); err != nil {
		t.Fatal(err)
	}
	want := "a.yml:1:2: error: [z-rule] early\na.yml:1:9: error: [a-rule] late\n"
	if buf.String() != want {
		t.Fatalf("got\n%s", buf.String())
	}
}

// A writer that fails makes Write return that error in text, github, and json formats. With no diagnostics, text and github write nothing and succeed.
func TestCovWriteErrors(t *testing.T) {
	one := []finding.Finding{{File: "a.yml", Line: 1, Column: 1, Severity: finding.Warning, Rule: "r", Message: "m"}}
	for _, format := range []string{"text", "github", "json"} {
		err := Write(failWriter{}, format, one)
		if err == nil || !strings.Contains(err.Error(), "closed") {
			t.Fatalf("%s: %v", format, err)
		}
	}
	for _, format := range []string{"text", "github"} {
		if err := Write(failWriter{}, format, nil); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
	}
}

// HasError is false for warnings and info only, and for no diagnostics.
func TestCovHasErrorFalse(t *testing.T) {
	if HasError(nil) {
		t.Fatal("nil has an error")
	}
	if HasError([]finding.Finding{{Severity: finding.Warning}, {Severity: finding.Info}}) {
		t.Fatal("warning and info count as error")
	}
}
