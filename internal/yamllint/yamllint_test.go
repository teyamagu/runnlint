package yamllint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
)

// TestParseOutput checks that two yamllint lines become file, line, column, severity, and yamllint/rule. A malformed line and empty output produce no diagnostic.
func TestParseOutput(t *testing.T) {
	out := "book.yml:12:3: [error] trailing spaces (trailing-spaces)\nbook.yml:4:1: [warning] missing document start (document-start)\nnot a finding\n"
	got := ParseOutput(out)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].File != "book.yml" || got[0].Line != 12 || got[0].Column != 3 || got[0].Severity != finding.Error || got[0].Rule != "yamllint/trailing-spaces" {
		t.Fatalf("%+v", got[0])
	}
	if got[1].Severity != finding.Warning || got[1].Rule != "yamllint/document-start" {
		t.Fatalf("%+v", got[1])
	}
	if len(ParseOutput("")) != 0 {
		t.Fatal("空の出力")
	}
}

// TestResolveConfig checks that an absolute path is returned unchanged. A relative path that exists in the config directory becomes absolute. A relative path that does not exist is returned unchanged.
func TestResolveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".yamllint.yaml")
	if err := os.WriteFile(path, []byte("extends: default\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveConfig(dir, path); got != path {
		t.Fatalf("abs = %s", got)
	}
	if got := resolveConfig(dir, ".yamllint.yaml"); got != path {
		t.Fatalf("rel = %s", got)
	}
	if got := resolveConfig(dir, "missing.yaml"); got != "missing.yaml" {
		t.Fatalf("missing = %s", got)
	}
}

// TestRunMissingBinary checks that a missing yamllint command with missing set to error yields one error from the yamllint rule. With missing off, yamllint disabled, or no target files, there is neither a diagnostic nor an error.
func TestRunMissingBinary(t *testing.T) {
	ctx := context.Background()
	bin := "runnlint-no-such-yamllint"
	got, err := Run(ctx, config.Yamllint{Bin: bin, Missing: "error"}, "", []string{"a.yml"})
	if err != nil || len(got) != 1 || got[0].Rule != "yamllint" || got[0].Severity != finding.Error {
		t.Fatalf("err=%v %+v", err, got)
	}
	got, err = Run(ctx, config.Yamllint{Bin: bin, Missing: "off"}, "", []string{"a.yml"})
	if err != nil || got != nil {
		t.Fatalf("off: err=%v %+v", err, got)
	}
	disabled := false
	got, err = Run(ctx, config.Yamllint{Enabled: &disabled, Bin: bin}, "", []string{"a.yml"})
	if err != nil || got != nil {
		t.Fatalf("disabled: err=%v %+v", err, got)
	}
	got, err = Run(ctx, config.Yamllint{Bin: bin}, "", nil)
	if err != nil || got != nil {
		t.Fatalf("no files: err=%v %+v", err, got)
	}
}

// TestIsLintFailure checks that exit code 1 makes isLintFailure true. Exit code 2 and a nil error make it false.
func TestIsLintFailure(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 1")
	err := cmd.Run()
	if !isLintFailure(err) {
		t.Fatalf("%v", err)
	}
	cmd = exec.Command("sh", "-c", "exit 2")
	if isLintFailure(cmd.Run()) {
		t.Fatal("exit 2 を lint 失敗と扱っています")
	}
	if isLintFailure(nil) {
		t.Fatal("nil")
	}
}
