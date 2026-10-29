package baseline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

func f(file string, line int, rule, msg string) finding.Finding {
	return finding.Finding{File: file, Line: line, Column: 1, Severity: finding.Error, Rule: rule, Message: msg}
}

// TestWriteThenFilterIgnoresLineShift checks that reloading a written baseline and applying it drops the same diagnostic even after its line moves, and leaves only the new diagnostic.
func TestWriteThenFilterIgnoresLineShift(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "runnlint-baseline.json")
	old := []finding.Finding{
		f("books/a.yml", 3, "var-ref", "vars.x は vars にありません"),
		f("books/a.yml", 9, "var-ref", "vars.x は vars にありません"),
		f("books/b.yml", 1, "require-desc", "runbook に desc がありません"),
	}
	if err := Write(path, old); err != nil {
		t.Fatal(err)
	}
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	now := []finding.Finding{
		f("./books/a.yml", 5, "var-ref", "vars.x は vars にありません"),
		f("books/a.yml", 11, "var-ref", "vars.x は vars にありません"),
		f("books/a.yml", 12, "var-ref", "vars.x は vars にありません"),
		f("books/a.yml", 13, "step-ref", "steps[3] はこの runbook にありません"),
	}
	got := b.Filter(now)
	if len(got.Findings) != 2 || got.Suppressed != 2 || got.Stale != 1 {
		t.Fatalf("%+v", got)
	}
	if got.Findings[0].Line != 12 || got.Findings[1].Rule != "step-ref" {
		t.Fatalf("%+v", got.Findings)
	}
}

// TestPathsAreRelativeToBaselineFile checks that a baseline stores paths relative to where the file sits, so they still match when the command runs from another directory.
func TestPathsAreRelativeToBaselineFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	path := filepath.Join(dir, "runnlint-baseline.json")
	if err := Write(path, []finding.Finding{f("sub/a.yml", 1, "r", "m")}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"file": "sub/a.yml"`) || strings.Contains(string(raw), `"line"`) {
		t.Fatalf("%s", raw)
	}
	t.Chdir(filepath.Join(dir, "sub"))
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Filter([]finding.Finding{f("a.yml", 7, "r", "m")}); len(got.Findings) != 0 {
		t.Fatalf("%+v", got)
	}
}

// TestWriteIsStable checks that output is ordered by file, rule, and message, and that the same key is folded into count.
func TestWriteIsStable(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "b.json")
	in := []finding.Finding{f("b.yml", 1, "z", "m"), f("a.yml", 2, "r", "m"), f("a.yml", 1, "r", "m")}
	if err := Write(path, in); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	s := string(raw)
	if strings.Index(s, `"a.yml"`) > strings.Index(s, `"b.yml"`) || !strings.Contains(s, `"count": 2`) || !strings.Contains(s, `"version": 2`) {
		t.Fatalf("%s", s)
	}
}

// TestLoadErrors checks that broken JSON, an unknown version, and a missing file fail to load.
func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.json")
	os.WriteFile(broken, []byte("{"), 0o644)
	future := filepath.Join(dir, "future.json")
	os.WriteFile(future, []byte(`{"version": 99, "findings": []}`), 0o644)
	old := filepath.Join(dir, "v1.json")
	os.WriteFile(old, []byte(`{"version": 1, "findings": []}`), 0o644)
	for _, path := range []string{broken, future, old, filepath.Join(dir, "missing.json")} {
		if _, err := Load(path); err == nil {
			t.Fatalf("%s を受けています", path)
		}
	}
	_, err := Load(old)
	if err == nil || !strings.Contains(err.Error(), "--write-baseline") {
		t.Fatalf("%v", err)
	}
}

// TestBaselineMatchesEnglishSentence checks that, even when the displayed sentence is Japanese, the sentence written to the baseline and used for matching is English.
func TestBaselineMatchesEnglishSentence(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "b.json")
	in := finding.Finding{File: "a.yml", Rule: "require-steps", Message: "steps がありません", Match: "steps is missing"}
	if err := Write(path, []finding.Finding{in}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "steps がありません") || !strings.Contains(string(raw), "steps is missing") {
		t.Fatalf("%s", raw)
	}
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	shown := in
	shown.Message = "別の表示"
	if got := b.Filter([]finding.Finding{shown}); len(got.Findings) != 0 || got.Suppressed != 1 {
		t.Fatalf("%+v", got)
	}
}
