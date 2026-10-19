package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// desc が無い runbook を検査したとき require-desc の warning が出て終了コードは 0 であることを確認する。defer だけの runbook では defer-experimental の info が出て、やはり 0 であることを確認する。
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

// ignore に入れたファイルをディレクトリ走査したとき、結果に出ず終了コードが 0 になることを確認する。同じファイルをパス指定したときは検査され、終了コードが 1 になることを確認する。
func TestIgnoreAppliesOnlyToDirectoryWalk(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cfg := []byte("extends: default\nignore:\n  - bad.yml\n")
	if err := os.WriteFile(filepath.Join(dir, ".runnlint.yaml"), cfg, 0o644); err != nil {
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

// 複数 document で steps を持つ壊れた YAML をディレクトリ走査したとき、そのファイルが診断されて終了コードが 1 になることを確認する。steps も runners も無いファイルは結果に出ないことを確認する。
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

// 存在しないパスと、出力形式 xml では終了コードが 2 になることを確認する。github 形式で error のある runbook を検査したとき、出力に ::error が含まれ終了コードが 1 になることを確認する。
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

// --write-baseline で既存の診断を書き出したあと --baseline を渡すと、既存の診断は出ず終了コードが 0 になることを確認する。新しい違反を足すと、その診断だけが出て 1 になることを確認する。
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

	// 先頭に行を足して既存の違反の行をずらし、新しい違反を足す。
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

// 存在しない baseline を渡すと、終了コード 2 で失敗することを確認する。
func TestMissingBaselineFails(t *testing.T) {
	t.Chdir(t.TempDir())
	code, err := Run(context.Background(), Options{Paths: []string{"."}, NoYamllint: true, Baseline: "missing.json", Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil || code != 2 {
		t.Fatalf("code = %d err = %v", code, err)
	}
}
