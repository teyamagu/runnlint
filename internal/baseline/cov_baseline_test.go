package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
)

// An entry whose count is 0 or missing counts as one occurrence, so one matching diagnostic is suppressed and a second one remains.
func TestCovLoadCountDefaultsToOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	raw := `{"version": 2, "findings": [{"file": "a.yml", "rule": "r", "message": "m", "count": 0}, {"file": "b.yml", "rule": "r", "message": "m"}]}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	res := b.Filter([]finding.Finding{f(filepath.Join(dir, "a.yml"), 1, "r", "m"), f(filepath.Join(dir, "a.yml"), 2, "r", "m"), f(filepath.Join(dir, "b.yml"), 1, "r", "m")})
	if res.Suppressed != 2 || len(res.Findings) != 1 || res.Findings[0].Line != 2 || res.Stale != 0 {
		t.Fatalf("%+v", res)
	}
}

// Write orders entries by file, then rule, then message, so two messages of the same rule in one file are sorted by message.
func TestCovWriteSortsByMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	findings := []finding.Finding{
		f(filepath.Join(dir, "a.yml"), 1, "r", "zeta"),
		f(filepath.Join(dir, "a.yml"), 2, "r", "alpha"),
		f(filepath.Join(dir, "a.yml"), 3, "q", "omega"),
	}
	if err := Write(path, findings); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out file
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Findings) != 3 || out.Findings[0].Rule != "q" || out.Findings[1].Message != "alpha" || out.Findings[2].Message != "zeta" {
		t.Fatalf("%+v", out.Findings)
	}
}

// Writing into a directory that does not exist fails with the baseline-write sentence that wraps the OS error.
func TestCovWriteFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "baseline.json")
	err := Write(path, nil)
	if err == nil || !strings.HasPrefix(message.Localize(message.EN, err), "cannot write the baseline: ") {
		t.Fatalf("%v", err)
	}
}
