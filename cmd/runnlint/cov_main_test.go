package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --list-rules with a rules file that cannot be loaded exits 2 and prints the error in Japanese, or in English with --lang en.
func TestCovListRulesLoadError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, ".runnlint-runn.yaml", "version: 2\n")
	code, _, stderr := capture(t, []string{"--list-rules"})
	if code != 2 || !strings.Contains(stderr, "version は 1 です") {
		t.Fatalf("code=%d\n%s", code, stderr)
	}
	code, _, stderr = capture(t, []string{"--list-rules", "--lang", "en"})
	if code != 2 || !strings.Contains(stderr, "has version 1") {
		t.Fatalf("code=%d\n%s", code, stderr)
	}
}

// --list-rules applies --lang, rejects a config language that is neither ja nor en, prints configuration warnings in the chosen language, and exits 2 when a rule's uses is unknown.
func TestCovListRulesLanguageWarningsAndCheck(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, ".runnlint-runn.yaml", "language: fr\n")
	code, _, stderr := capture(t, []string{"--list-rules"})
	if code != 2 || !strings.Contains(stderr, "fr") {
		t.Fatalf("code=%d\n%s", code, stderr)
	}
	code, stdout, _ := capture(t, []string{"--list-rules", "--lang", "ja"})
	if code != 0 || !strings.Contains(stdout, "var-ref\twarning\tbuiltin:var-ref") {
		t.Fatalf("code=%d\n%s", code, stdout)
	}
	explicit := writeFile(t, dir, "with-unknown.yaml", "version: 1\nowner: me\n")
	code, stdout, stderr = capture(t, []string{"--list-rules", "--lang", "en", "-c", explicit})
	if code != 0 || !strings.HasPrefix(stderr, "warning: ") || !strings.Contains(stderr, "owner") || !strings.Contains(stdout, "require-desc\twarning\tcheck") {
		t.Fatalf("code=%d\n%s\n%s", code, stdout, stderr)
	}
	writeFile(t, dir, ".runnlint-runn.yaml", "rules:\n  - id: x\n    uses: no-such\n")
	code, _, stderr = capture(t, []string{"--list-rules"})
	if code != 2 || !strings.Contains(stderr, "no-such") {
		t.Fatalf("code=%d\n%s", code, stderr)
	}
}

// --lang=en in one argument switches the display to English. --include-root may be repeated on the command line.
func TestCovLangEqualsAndIncludeRoot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "a.yml", "desc: a\nsteps:\n  - nope:\n      query: SELECT 1\n")
	code, stdout, _ := capture(t, []string{"--lang=en", "--no-yamllint", "a.yml"})
	if code != 1 || !strings.Contains(stdout, "error: [runner-defined] runner nope is not in runners or a preceding runner:") {
		t.Fatalf("code=%d\n%s", code, stdout)
	}
	if err := os.MkdirAll(filepath.Join(dir, "books", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "books"), "orphan.yml", "desc: o\nif: included\nsteps:\n  - test: \"true\"\n")
	writeFile(t, filepath.Join(dir, "books", "sub"), "inner.yml", "desc: i\nif: included\nsteps:\n  - test: \"true\"\n")
	code, stdout, stderr := capture(t, []string{"--no-yamllint", "--include-root", "books", "--include-root", "books/sub", "books/orphan.yml"})
	if code != 0 || strings.Count(stdout, "[included-unreferenced]") != 1 {
		t.Fatalf("code=%d\n%s\n%s", code, stdout, stderr)
	}
	var list stringList
	if err := list.Set("a"); err != nil || list.String() != "a" {
		t.Fatalf("%v %s", err, list.String())
	}
	if err := list.Set("b"); err != nil || list.String() != "a,b" {
		t.Fatalf("%v %s", err, list.String())
	}
}
