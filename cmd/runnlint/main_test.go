package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/version"
)

func capture(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	stdout, stderr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	code := run(args)
	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = stdout, stderr
	var outBuf, errBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, outR); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(&errBuf, errR); err != nil {
		t.Fatal(err)
	}
	return code, outBuf.String(), errBuf.String()
}

// --version を渡したとき、標準出力が runnlint と Version になり、終了コードが 0 であることを確認する。
func TestVersion(t *testing.T) {
	code, stdout, stderr := capture(t, []string{"--version"})
	if code != 0 || stderr != "" || stdout != "runnlint "+version.Version+"\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// -h を渡して実行したとき、使い方が標準エラーに出て終了コードが 0 になることを確認する。未知のフラグでは終了コードが 2 になることを確認する。
func TestHelpAndUnknownFlag(t *testing.T) {
	code, _, stderr := capture(t, []string{"-h"})
	if code != 0 {
		t.Fatalf("code = %d\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "runnlint") {
		t.Fatalf("usage = %s", stderr)
	}

	code, _, stderr = capture(t, []string{"--not-a-flag"})
	if code != 2 {
		t.Fatalf("code = %d\n%s", code, stderr)
	}
}

// --print-config を実行したとき、埋め込みルール定義が標準出力に出ることを確認する。--print-yamllint-config では yamllint 用の埋め込み設定が出ることを確認する。
func TestPrintEmbeddedConfig(t *testing.T) {
	code, stdout, stderr := capture(t, []string{"--print-config"})
	if code != 0 {
		t.Fatalf("code = %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "id: duplicate-key") || !strings.Contains(stdout, "id: require-desc") {
		t.Fatalf("config = %s", stdout)
	}

	code, stdout, stderr = capture(t, []string{"--print-yamllint-config"})
	if code != 0 {
		t.Fatalf("code = %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "document-start") {
		t.Fatalf("yamllint config = %s", stdout)
	}
}

// 設定ファイルの無いディレクトリで --list-rules を実行したとき、duplicate-key が builtin、require-desc が check として既定の一覧に出ることを確認する。
func TestListRules(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	code, stdout, stderr := capture(t, []string{"--list-rules"})
	if code != 0 {
		t.Fatalf("code = %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "duplicate-key\terror\tbuiltin:duplicate-key") {
		t.Fatalf("builtins = %s", stdout)
	}
	if !strings.Contains(stdout, "require-desc\twarning\tcheck") {
		t.Fatalf("checks = %s", stdout)
	}
}

// error のある runbook を検査したとき終了コードが 1、warning だけの runbook では 0 になることを確認する。severity が不正な設定では検査せず 2 になることを確認する。
func TestCommandExitCodes(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	bad := []byte("desc: bad\nsteps:\n  - nope:\n      query: SELECT 1\n")
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := capture(t, []string{"--no-yamllint", "bad.yml"})
	if code != 1 {
		t.Fatalf("code = %d\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "error: [runner-defined]") {
		t.Fatalf("stdout = %s", stdout)
	}

	warn := []byte("steps:\n  - test: \"true\"\n")
	if err := os.WriteFile(filepath.Join(dir, "warn.yml"), warn, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = capture(t, []string{"--no-yamllint", "warn.yml"})
	if code != 0 {
		t.Fatalf("code = %d\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "warning: [require-desc]") {
		t.Fatalf("stdout = %s", stdout)
	}

	cfg := []byte("version: 1\nrules:\n  - id: broken\n    severity: fatal\n    uses: duplicate-key\n")
	if err := os.WriteFile(filepath.Join(dir, ".runnlint-runn.yaml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = capture(t, []string{"--no-yamllint", "bad.yml"})
	if code != 2 {
		t.Fatalf("code = %d\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "severity") {
		t.Fatalf("stderr = %s", stderr)
	}
}

// --write-baseline で書き出したファイルを --baseline に渡すと、既存の error が出ず終了コードが 0 になることを確認する。
func TestBaselineFlags(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	bad := []byte("desc: bad\nsteps:\n  - nope:\n      query: SELECT 1\n")
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := capture(t, []string{"--no-yamllint", "--write-baseline", "baseline.json", "bad.yml"})
	if code != 0 || stdout != "" || !strings.Contains(stderr, "baseline.json") {
		t.Fatalf("code = %d\n%s\n%s", code, stdout, stderr)
	}
	code, stdout, stderr = capture(t, []string{"--no-yamllint", "--baseline", "baseline.json", "bad.yml"})
	if code != 0 || stdout != "" {
		t.Fatalf("code = %d\n%s\n%s", code, stdout, stderr)
	}
}
