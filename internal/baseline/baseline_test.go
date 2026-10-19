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

// 書き出した baseline を読み直して当てたとき、同じ診断は行がずれても除外され、新しい診断だけが残ることを確認する。
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

// baseline のパスはファイルの置き場所からの相対で持ち、別のディレクトリから実行しても一致することを確認する。
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

// 書き出しは file、rule、message の順に並べ、同じキーは count にまとめることを確認する。
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
	if strings.Index(s, `"a.yml"`) > strings.Index(s, `"b.yml"`) || !strings.Contains(s, `"count": 2`) || !strings.Contains(s, `"version": 1`) {
		t.Fatalf("%s", s)
	}
}

// 壊れた JSON、未知の version、存在しないファイルは読み込みに失敗することを確認する。
func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.json")
	os.WriteFile(broken, []byte("{"), 0o644)
	future := filepath.Join(dir, "future.json")
	os.WriteFile(future, []byte(`{"version": 2, "findings": []}`), 0o644)
	for _, path := range []string{broken, future, filepath.Join(dir, "missing.json")} {
		if _, err := Load(path); err == nil {
			t.Fatalf("%s を受けています", path)
		}
	}
}
