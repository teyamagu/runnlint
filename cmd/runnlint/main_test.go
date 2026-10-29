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

// TestVersion checks that --version prints runnlint and Version on stdout and exits 0.
func TestVersion(t *testing.T) {
	code, stdout, stderr := capture(t, []string{"--version"})
	if code != 0 || stderr != "" || stdout != "runnlint "+version.Version+"\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// TestHelpAndUnknownFlag checks that -h prints usage on stderr and exits 0. An unknown flag exits 2.
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

// TestPrintEmbeddedConfig checks that --print-config prints the embedded rules on stdout. --print-yamllint-config prints the embedded yamllint config.
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

// TestListRules checks that --list-rules, in a directory with no config file, lists duplicate-key as builtin and require-desc as check.
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

// TestCommandExitCodes checks that a runbook with an error exits 1 and a warning-only runbook exits 0. A config with an invalid severity exits 2 without linting.
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

// TestBaselineFlags checks that passing a file written by --write-baseline to --baseline hides the existing error and exits 0.
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
	raw, err := os.ReadFile(filepath.Join(dir, "baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ありません") || !strings.Contains(string(raw), `"version": 2`) {
		t.Fatalf("%s", raw)
	}
}

// TestLangFlag checks that --lang en switches the display to English, and that a --lang value other than ja or en exits 2.
func TestLangFlag(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	body := []byte("desc: x\n")
	if err := os.WriteFile(filepath.Join(dir, "book.yml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := capture(t, []string{"--no-yamllint", "--lang", "en", "book.yml"})
	if code != 1 || !strings.Contains(stdout, "steps is missing") || strings.Contains(stdout, "steps がありません") {
		t.Fatalf("code = %d\n%s\n%s", code, stdout, stderr)
	}
	code, stdout, stderr = capture(t, []string{"--no-yamllint", "book.yml"})
	if code != 1 || !strings.Contains(stdout, "steps がありません") {
		t.Fatalf("code = %d\n%s\n%s", code, stdout, stderr)
	}
	code, _, stderr = capture(t, []string{"--lang", "fr", "book.yml"})
	if code != 2 || !strings.Contains(stderr, "language") {
		t.Fatalf("code = %d\n%s", code, stderr)
	}
	code, _, stderr = capture(t, []string{"--lang", "en", "-h"})
	if code != 0 || !strings.Contains(stderr, "Usage: runnlint") {
		t.Fatalf("code = %d\n%s", code, stderr)
	}
}
