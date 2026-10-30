package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWarningAndInfoDoNotFail checks that a runbook without desc yields a require-desc warning and exits 0. A defer-only runbook yields a defer-experimental info and also exits 0.
func TestWarningAndInfoDoNotFail(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	warn := []byte("steps:\n  - test: \"true\"\n")
	if err := os.WriteFile(filepath.Join(dir, "warn.yml"), warn, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	code, err := Run(context.Background(), Options{
		Paths:      []string{"warn.yml"},
		NoYamllint: true,
		Stdout:     &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || !bytes.Contains(stdout.Bytes(), []byte("warning: [require-desc]")) {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}

	stdout.Reset()
	info := []byte("desc: deferred\nsteps:\n  - defer: true\n    test: \"true\"\n")
	if err := os.WriteFile(filepath.Join(dir, "info.yml"), info, 0o644); err != nil {
		t.Fatal(err)
	}
	code, err = Run(context.Background(), Options{
		Paths:      []string{"info.yml"},
		NoYamllint: true,
		Stdout:     &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || !bytes.Contains(stdout.Bytes(), []byte("info: [defer-experimental]")) {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}
}

// TestIgnoreAppliesOnlyToDirectoryWalk checks that a directory walk omits a file listed in ignore and exits 0. Passing that same file by path lints it and exits 1.
func TestIgnoreAppliesOnlyToDirectoryWalk(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cfg := []byte("extends: default\nignore:\n  - bad.yml\n")
	if err := os.WriteFile(filepath.Join(dir, ".runnlint-runn.yaml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	bad := []byte("desc: bad\nsteps:\n  - nope:\n      query: SELECT 1\n")
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	ok := []byte("desc: ok\nsteps:\n  - test: \"true\"\n")
	if err := os.WriteFile(filepath.Join(dir, "ok.yml"), ok, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code, err := Run(context.Background(), Options{
		Paths:      []string{"."},
		NoYamllint: true,
		Stdout:     &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || bytes.Contains(stdout.Bytes(), []byte("bad.yml")) {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}

	stdout.Reset()
	code, err = Run(context.Background(), Options{
		Paths:      []string{"bad.yml"},
		NoYamllint: true,
		Stdout:     &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !bytes.Contains(stdout.Bytes(), []byte("runner-defined")) {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}
}

// TestBrokenRunbookWithRootKeyIsReported checks that a directory walk reports a broken multi-document YAML that has steps and exits 1. A file with neither steps nor runners is omitted.
func TestBrokenRunbookWithRootKeyIsReported(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	broken := []byte("steps:\n  - test: \"true\"\n---\ndesc: extra\n")
	if err := os.WriteFile(filepath.Join(dir, "broken.yml"), broken, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.yml"), []byte("title: memo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	code, err := Run(context.Background(), Options{
		Paths:      []string{"."},
		NoYamllint: true,
		Stdout:     &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("broken.yml")) || bytes.Contains(stdout.Bytes(), []byte("notes.yml")) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

// TestRunFailures checks that a missing path and the output format xml exit 2. Linting a runbook that has an error in github format includes ::error and exits 1.
func TestRunFailures(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	code, err := Run(context.Background(), Options{
		Paths:      []string{"missing.yml"},
		NoYamllint: true,
		Stdout:     &bytes.Buffer{},
	})
	if err == nil || code != 2 {
		t.Fatalf("code = %d err = %v", code, err)
	}

	code, err = Run(context.Background(), Options{
		Paths:      []string{"."},
		Format:     "xml",
		NoYamllint: true,
		Stdout:     &bytes.Buffer{},
	})
	if err == nil || code != 2 {
		t.Fatalf("code = %d err = %v", code, err)
	}

	var stdout bytes.Buffer
	body := []byte("desc: bad\nsteps:\n  - nope:\n      query: SELECT 1\n")
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	code, err = Run(context.Background(), Options{
		Paths:      []string{"bad.yml"},
		Format:     "github",
		NoYamllint: true,
		Stdout:     &stdout,
	})
	if err != nil || code != 1 || !bytes.Contains(stdout.Bytes(), []byte("::error file=bad.yml,")) {
		t.Fatalf("code = %d err = %v\n%s", code, err, stdout.String())
	}
}

// TestBaselineReportsOnlyNewFindings checks that writing existing diagnostics with --write-baseline and then passing --baseline hides them and exits 0. Adding a new violation reports only that diagnostic and exits 1.
func TestBaselineReportsOnlyNewFindings(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	bad := "desc: bad\nsteps:\n  - nope:\n      query: SELECT 1\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code, err := Run(context.Background(), Options{
		Paths:         []string{"."},
		NoYamllint:    true,
		WriteBaseline: "runnlint-baseline.json",
		Stdout:        &stdout,
		Stderr:        &stderr,
	})
	if err != nil || code != 0 {
		t.Fatalf("code = %d err = %v\n%s", code, err, stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "runnlint-baseline.json")); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code, err = Run(context.Background(), Options{
		Paths:      []string{"."},
		NoYamllint: true,
		Baseline:   "runnlint-baseline.json",
		Stdout:     &stdout,
		Stderr:     &stderr,
	})
	if err != nil || code != 0 || stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("baseline")) {
		t.Fatalf("code = %d err = %v\nstdout:\n%s\nstderr:\n%s", code, err, stdout.String(), stderr.String())
	}

	// Add a line at the top so existing violations shift, and add a new violation.
	bad = "# shifted\n" + bad + "  - other:\n      query: SELECT 2\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	code, err = Run(context.Background(), Options{
		Paths:      []string{"."},
		NoYamllint: true,
		Baseline:   "runnlint-baseline.json",
		Stdout:     &stdout,
		Stderr:     &stderr,
	})
	if err != nil || code != 1 || bytes.Contains(stdout.Bytes(), []byte("nope")) || !bytes.Contains(stdout.Bytes(), []byte("other")) {
		t.Fatalf("code = %d err = %v\n%s", code, err, stdout.String())
	}
}

// TestMissingBaselineFails checks that a missing baseline fails with exit code 2.
func TestMissingBaselineFails(t *testing.T) {
	t.Chdir(t.TempDir())
	code, err := Run(context.Background(), Options{Paths: []string{"."}, NoYamllint: true, Baseline: "missing.json", Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil || code != 2 {
		t.Fatalf("code = %d err = %v", code, err)
	}
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func runText(t *testing.T, opt Options) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	opt.NoYamllint = true
	opt.Stdout = &stdout
	opt.Stderr = &stderr
	code, err := Run(context.Background(), opt)
	if err != nil {
		t.Fatalf("%v\n%s", err, stderr.String())
	}
	return code, stdout.String()
}

// TestIncludedUnreferencedScope checks that a directory walk reports included-unreferenced only for an if: included runbook that nothing includes.
// A file excluded by ignore is still read as a referrer. A file passed directly is not judged. Adding --include-root extends the range that is judged.
func TestIncludedUnreferencedScope(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	child := "desc: child\nif: included\nsteps:\n  - test: \"true\"\n"
	writeFiles(t, dir, map[string]string{
		".runnlint-runn.yaml":    "ignore:\n  - books/ignored.yml\n",
		"books/parent.yml":       "desc: parent\nsteps:\n  - include: child.yml\n",
		"books/ignored.yml":      "desc: ignored\nsteps:\n  - include: from-ignored.yml\n",
		"books/child.yml":        child,
		"books/from-ignored.yml": child,
		"books/orphan.yml":       child,
		"books/sub/inner.yml":    child,
		"other/parent.yml":       "desc: other\nsteps:\n  - include: ../books/sub/inner.yml\n",
	})

	_, out := runText(t, Options{Paths: []string{"books"}})
	if strings.Count(out, "[included-unreferenced]") != 2 || !strings.Contains(out, "orphan.yml:2:") || !strings.Contains(out, "inner.yml:2:") {
		t.Fatalf("%s", out)
	}

	_, out = runText(t, Options{Paths: []string{"books/orphan.yml"}})
	if strings.Contains(out, "[included-unreferenced]") {
		t.Fatalf("直接渡したファイルを判定しています:\n%s", out)
	}

	_, out = runText(t, Options{Paths: []string{"books/orphan.yml"}, IncludeRoots: []string{"books"}})
	if !strings.Contains(out, "orphan.yml:2:") {
		t.Fatalf("--include-root の中のファイルを判定していません:\n%s", out)
	}

	_, out = runText(t, Options{Paths: []string{"books"}, IncludeRoots: []string{"other"}})
	if strings.Count(out, "[included-unreferenced]") != 1 || strings.Contains(out, "inner.yml") {
		t.Fatalf("%s", out)
	}
}

// TestIncludedUnreferencedConfigAndBaseline checks that included-unreferenced is silent when severity is off, and that a baseline can suppress it as an existing diagnostic. A missing --include-root fails with 2.
func TestIncludedUnreferencedConfigAndBaseline(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{
		"books/orphan.yml": "desc: child\nif: included\nsteps:\n  - test: \"true\"\n",
	})
	_, out := runText(t, Options{Paths: []string{"books"}, WriteBaseline: "baseline.json"})
	if out != "" {
		t.Fatalf("%s", out)
	}
	code, out := runText(t, Options{Paths: []string{"books"}, Baseline: "baseline.json"})
	if code != 0 || out != "" {
		t.Fatalf("code = %d\n%s", code, out)
	}
	writeFiles(t, dir, map[string]string{".runnlint-runn.yaml": "rules:\n  - id: included-unreferenced\n    severity: off\n"})
	_, out = runText(t, Options{Paths: []string{"books"}})
	if strings.Contains(out, "included-unreferenced") {
		t.Fatalf("%s", out)
	}
	code, err := Run(context.Background(), Options{Paths: []string{"books"}, NoYamllint: true, IncludeRoots: []string{"missing"}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if code != 2 || err == nil {
		t.Fatalf("code = %d err = %v", code, err)
	}
}

// TestInlineSuppressionBeforeBaseline checks that an inline suppression applies before the baseline. A suppressed diagnostic is not written into the baseline, and an unused suppression's unused-suppression can itself be suppressed by the baseline.
func TestInlineSuppressionBeforeBaseline(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{
		"book.yml": "desc: d\nsteps:\n  - dump: vars.a # runnlint disable-line rule:var-ref\n  - dump: x # runnlint disable-line rule:var-ref\n",
	})
	if _, out := runText(t, Options{Paths: []string{"."}, WriteBaseline: "baseline.json"}); out != "" {
		t.Fatalf("%s", out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"rule": "var-ref"`) || !strings.Contains(string(raw), `"rule": "unused-suppression"`) {
		t.Fatalf("%s", raw)
	}
	if code, out := runText(t, Options{Paths: []string{"."}, Baseline: "baseline.json"}); code != 0 || out != "" {
		t.Fatalf("code = %d\n%s", code, out)
	}
}

// TestIncludedUnreferencedExampleCommand checks that passing examples/bad/included-unreferenced.yml directly does not report included-unreferenced, and that adding --include-root examples does.
func TestIncludedUnreferencedExampleCommand(t *testing.T) {
	t.Chdir(moduleRoot(t))
	path := "examples/bad/included-unreferenced.yml"
	var stdout bytes.Buffer
	code, err := Run(context.Background(), Options{Paths: []string{path}, NoYamllint: true, Stdout: &stdout})
	if err != nil || code != 0 || stdout.Len() != 0 {
		t.Fatalf("code = %d err = %v\n%s", code, err, stdout.String())
	}
	stdout.Reset()
	code, err = Run(context.Background(), Options{
		Paths:        []string{path},
		IncludeRoots: []string{"examples"},
		NoYamllint:   true,
		Stdout:       &stdout,
	})
	if err != nil || code != 0 || strings.Count(stdout.String(), "[included-unreferenced]") != 1 || strings.Contains(stdout.String(), "[unused-suppression]") {
		t.Fatalf("code = %d err = %v\n%s", code, err, stdout.String())
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod が見つかりません")
		}
		dir = parent
	}
}

// TestLegacyConfigNameIsIgnored checks that .runnlint.yaml is not discovered. Another tool's config produces no warning, and linting uses the defaults.
func TestLegacyConfigNameIsIgnored(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{
		".runnlint.yaml": "linters:\n  enable: [all]\n",
		"book.yml":       "desc: d\nsteps:\n  - test: \"true\"\n",
	})
	var stdout, stderr bytes.Buffer
	code, err := Run(context.Background(), Options{Paths: []string{"."}, NoYamllint: true, Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || stderr.Len() != 0 {
		t.Fatalf("code = %d err = %v\n%s\n%s", code, err, stdout.String(), stderr.String())
	}
}

// TestRuleFilesInWalkAndArgument checks that a rule with files applies only to matching files, both when the directory is walked and when the file is passed as an argument.
func TestRuleFilesInWalkAndArgument(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{
		".runnlint-runn.yaml": "extends: none\nrules:\n  - id: integrations-only-include\n    severity: error\n    files: ['integrations/**']\n    check:\n      select: 'steps.*'\n      where:\n        - path: include\n          exists: false\n      forbid: true\n",
		"integrations/a.yml":  "desc: d\nsteps:\n  - test: \"true\"\n",
		"books/a.yml":         "desc: d\nsteps:\n  - test: \"true\"\n",
	})
	var stdout bytes.Buffer
	code, err := Run(context.Background(), Options{Paths: []string{"."}, NoYamllint: true, Stdout: &stdout})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(stdout.String(), "integrations/a.yml") || strings.Contains(stdout.String(), "books/a.yml") {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}

	stdout.Reset()
	code, err = Run(context.Background(), Options{Paths: []string{"books/a.yml"}, NoYamllint: true, Stdout: &stdout})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || stdout.Len() != 0 {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}

	stdout.Reset()
	code, err = Run(context.Background(), Options{Paths: []string{"integrations/a.yml"}, NoYamllint: true, Stdout: &stdout})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(stdout.String(), "[integrations-only-include]") {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}
}
