package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runRaw runs with yamllint disabled unless the test enables it, and returns the exit code, stdout, stderr, and error.
func runRaw(t *testing.T, opt Options) (int, string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	opt.Stdout = &stdout
	opt.Stderr = &stderr
	code, err := Run(context.Background(), opt)
	return code, stdout.String(), stderr.String(), err
}

// A rules file whose language is neither ja nor en fails with exit 2 before linting.
func TestCovRunRejectsConfigLanguage(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{".runnlint-runn.yaml": "language: fr\n", "a.yml": "desc: a\nsteps:\n  - test: \"true\"\n"})
	code, _, _, err := runRaw(t, Options{Paths: []string{"a.yml"}, NoYamllint: true})
	if code != 2 || err == nil || !strings.Contains(err.Error(), "fr") {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

// Configuration warnings are printed to stderr before linting. A rule whose uses is unknown fails with exit 2.
func TestCovRunWarningsAndCheckConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{
		".runnlint-runn.yaml": "rules:\n  - id: var-ref\n    severity: warning\n    typo: 1\n",
		"a.yml":               "desc: a\nsteps:\n  - test: \"true\"\n",
	})
	code, _, stderr, err := runRaw(t, Options{Paths: []string{"a.yml"}, NoYamllint: true})
	if code != 0 || err != nil || !strings.HasPrefix(stderr, "warning: ") || !strings.Contains(stderr, "typo") {
		t.Fatalf("code=%d err=%v\n%s", code, err, stderr)
	}
	writeFiles(t, dir, map[string]string{".runnlint-runn.yaml": "rules:\n  - id: x\n    uses: no-such\n"})
	code, _, _, err = runRaw(t, Options{Paths: []string{"a.yml"}, NoYamllint: true})
	if code != 2 || err == nil || !strings.Contains(err.Error(), "no-such") {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

// With no paths, the current directory is scanned.
func TestCovRunDefaultsToCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{"sub/a.yml": "steps:\n  - test: \"true\"\n"})
	code, out := runText(t, Options{})
	if code != 0 || !strings.Contains(out, "sub/a.yml:1:1: warning: [require-desc]") {
		t.Fatalf("code=%d\n%s", code, out)
	}
}

// When yamllint is enabled and the command is missing, the configured severity is reported. When the command exits with an unexpected code, the run fails with exit 2 and the English sentence under language en.
func TestCovRunYamllint(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{
		".runnlint-runn.yaml": "yamllint:\n  bin: runnlint-no-such-yamllint\n  missing: info\n",
		"a.yml":               "desc: a\nsteps:\n  - test: \"true\"\n",
	})
	code, out, _, err := runRaw(t, Options{Paths: []string{"a.yml"}})
	if code != 0 || err != nil || !strings.Contains(out, "a.yml:1:1: info: [yamllint]") {
		t.Fatalf("code=%d err=%v\n%s", code, err, out)
	}
	script := filepath.Join(dir, "fake-yamllint")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho broken\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, dir, map[string]string{".runnlint-runn.yaml": "language: en\nyamllint:\n  bin: " + script + "\n"})
	code, _, _, err = runRaw(t, Options{Paths: []string{"a.yml"}})
	if code != 2 || err == nil || err.Error() != "yamllint failed: broken" {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

// Writing the baseline into a directory that does not exist fails with exit 2. A baseline entry that no longer occurs is reported as already fixed on stderr.
func TestCovRunBaselineWriteErrorAndStale(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{
		"a.yml":         "steps:\n  - test: \"true\"\n",
		"baseline.json": `{"version": 2, "findings": [{"file": "a.yml", "rule": "require-desc", "message": "runbook に desc がありません", "count": 1}, {"file": "gone.yml", "rule": "require-desc", "message": "runbook に desc がありません", "count": 1}]}`,
	})
	code, _, _, err := runRaw(t, Options{Paths: []string{"a.yml"}, NoYamllint: true, WriteBaseline: filepath.Join(dir, "missing", "b.json")})
	if code != 2 || err == nil || !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("code=%d err=%v", code, err)
	}
	code, out, stderr, err := runRaw(t, Options{Paths: []string{"a.yml"}, NoYamllint: true, Baseline: "baseline.json"})
	if code != 0 || err != nil || out != "" || !strings.Contains(stderr, "既存の 1 件") || !strings.Contains(stderr, "1 件は解消済み") {
		t.Fatalf("code=%d err=%v out=%q\n%s", code, err, out, stderr)
	}
}

// Runbooks under .git, vendor, and node_modules are skipped by the directory walk and are not read as referrers either.
func TestCovRunSkipsSpecialDirectories(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	bad := "desc: x\nsteps:\n  - nope:\n      query: SELECT 1\n"
	writeFiles(t, dir, map[string]string{
		".git/a.yml":         bad,
		"vendor/b.yml":       bad,
		"node_modules/c.yml": bad,
		"books/parent.yml":   "desc: p\nsteps:\n  - include: ../vendor/child.yml\n",
		"vendor/child.yml":   "desc: c\nif: included\nsteps:\n  - test: \"true\"\n",
		"books/orphan.yml":   "desc: o\nif: included\nsteps:\n  - test: \"true\"\n",
	})
	code, out := runText(t, Options{Paths: []string{"."}})
	if code != 0 || strings.Contains(out, "runner-defined") || strings.Count(out, "[included-unreferenced]") != 1 || !strings.Contains(out, "orphan.yml") {
		t.Fatalf("code=%d\n%s", code, out)
	}
}

// An --include-root that is a file is rejected, and under language en the error is in English. A root that cannot be walked fails too.
func TestCovBuildRefsErrors(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{"a.yml": "desc: a\nsteps:\n  - test: \"true\"\n", "root/x.yml": "desc: x\nsteps:\n  - test: \"true\"\n"})
	code, _, _, err := runRaw(t, Options{Paths: []string{"a.yml"}, NoYamllint: true, IncludeRoots: []string{"a.yml"}, Lang: "en"})
	if code != 2 || err == nil || err.Error() != "--include-root a.yml is not a directory" {
		t.Fatalf("code=%d err=%v", code, err)
	}
	code, _, _, err = runRaw(t, Options{Paths: []string{"a.yml"}, NoYamllint: true, IncludeRoots: []string{"a.yml"}})
	if code != 2 || err == nil || err.Error() != "--include-root a.yml はディレクトリではありません" {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read everything")
	}
	locked := filepath.Join(dir, "root", "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)
	code, _, _, err = runRaw(t, Options{Paths: []string{"a.yml"}, NoYamllint: true, IncludeRoots: []string{"root"}})
	if code != 2 || err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

// A directory that cannot be read stops the walk with exit 2. A file passed directly that cannot be read fails when referrers are collected.
func TestCovUnreadableInputs(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read everything")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{"books/a.yml": "desc: a\nsteps:\n  - test: \"true\"\n", "secret.yml": "desc: s\nsteps:\n  - test: \"true\"\n"})
	locked := filepath.Join(dir, "books", "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)
	code, _, _, err := runRaw(t, Options{Paths: []string{"books"}, NoYamllint: true})
	if code != 2 || err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if err := os.Chmod(filepath.Join(dir, "secret.yml"), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(dir, "secret.yml"), 0o644)
	code, _, _, err = runRaw(t, Options{Paths: []string{"secret.yml"}, NoYamllint: true})
	if code != 2 || err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("code=%d err=%v", code, err)
	}
}
