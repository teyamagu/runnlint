package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

func sample() []finding.Finding {
	return []finding.Finding{
		{File: "b.yml", Line: 2, Column: 1, Severity: finding.Warning, Rule: "var-ref", Message: "未定義"},
		{File: "a.yml", Line: 4, Column: 3, Severity: finding.Error, Rule: "http-request", Message: "body がありません"},
		{File: "a.yml", Line: 4, Column: 3, Severity: finding.Info, Rule: "defer-experimental", Message: "実験的"},
	}
}

// TestWriteTextSortsAndFormats checks that diagnostics written as text are ordered by file, line, and column. At the same position, rule names are lexicographic, so defer-experimental comes before http-request.
func TestWriteTextSortsAndFormats(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "text", sample()); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := strings.Join([]string{
		"a.yml:4:3: info: [defer-experimental] 実験的",
		"a.yml:4:3: error: [http-request] body がありません",
		"b.yml:2:1: warning: [var-ref] 未定義",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

// TestWriteGitHubLevels checks that github format writes error as ::error, warning as ::warning, and info as ::notice.
func TestWriteGitHubLevels(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "github", sample()); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, line := range []string{
		"::error file=a.yml,line=4,col=3::[http-request] body がありません",
		"::notice file=a.yml,line=4,col=3::[defer-experimental] 実験的",
		"::warning file=b.yml,line=2,col=1::[var-ref] 未定義",
	} {
		if !strings.Contains(got, line) {
			t.Fatalf("missing %s\n%s", line, got)
		}
	}
}

// TestWriteJSON checks that an empty format is written as text. json writes the diagnostics as an array that keeps rule, severity, and line.
func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "", sample()[:1]); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "warning: [var-ref]") {
		t.Fatalf("empty format = %s", buf.String())
	}

	buf.Reset()
	if err := Write(&buf, "json", sample()[:1]); err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["rule"] != "var-ref" || items[0]["severity"] != "warning" || items[0]["line"].(float64) != 2 {
		t.Fatalf("%s", buf.String())
	}
}

// TestWriteRejectsUnknownFormat checks that passing xml as the format writes nothing and the error message contains xml.
func TestWriteRejectsUnknownFormat(t *testing.T) {
	err := Write(&bytes.Buffer{}, "xml", nil)
	if err == nil || !strings.Contains(err.Error(), "xml") {
		t.Fatalf("%v", err)
	}
}

// TestHasError checks that HasError is false when there are no diagnostics and when there are only warnings, and true when an error is included.
func TestHasError(t *testing.T) {
	if HasError(nil) || HasError(sample()[:1]) {
		t.Fatal("warning を error と扱っています")
	}
	if !HasError(sample()) {
		t.Fatal("error を見落としています")
	}
}
