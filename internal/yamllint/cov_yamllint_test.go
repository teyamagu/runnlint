package yamllint

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
)

// fakeYamllint writes a shell script that prints body to stdout and exits with code, and returns its path.
func fakeYamllint(t *testing.T, dir, body string, code int) string {
	t.Helper()
	path := filepath.Join(dir, "yamllint")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > \"" + filepath.Join(dir, "args.txt") + "\"\nprintf '%s' '" + body + "'\nexit " + itoa(code) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// With an empty bin, yamllint is looked up on PATH. With an empty missing, a missing command is a warning.
func TestCovRunDefaultsBinAndMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := Run(context.Background(), config.Yamllint{}, "", []string{"a.yml"})
	if err != nil || len(got) != 1 || got[0].Severity != finding.Warning || got[0].Rule != "yamllint" {
		t.Fatalf("err=%v %+v", err, got)
	}
}

// A yamllint that exits 1 with parsable output yields those diagnostics. The built-in config is passed with -d and the files follow.
func TestCovRunParsesOutput(t *testing.T) {
	dir := t.TempDir()
	bin := fakeYamllint(t, dir, "a.yml:3:1: [error] trailing spaces (trailing-spaces)", 1)
	got, err := Run(context.Background(), config.Yamllint{Bin: bin}, "", []string{"a.yml", "b.yml"})
	if err != nil || len(got) != 1 || got[0].Rule != "yamllint/trailing-spaces" || got[0].Line != 3 {
		t.Fatalf("err=%v %+v", err, got)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(args), "-f parsable -d ") || !strings.HasSuffix(string(args), " a.yml b.yml") {
		t.Fatalf("args = %s", args)
	}
}

// With yamllint.config set, the config path is resolved against the config directory and passed with -c. A clean run with exit 0 yields no diagnostics.
func TestCovRunPassesConfig(t *testing.T) {
	dir := t.TempDir()
	bin := fakeYamllint(t, dir, "", 0)
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "lint.yaml"), []byte("extends: default\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Run(context.Background(), config.Yamllint{Bin: bin, Config: "lint.yaml"}, cfgDir, []string{"a.yml"})
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v %+v", err, got)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(args) != "-f parsable -c "+filepath.Join(cfgDir, "lint.yaml")+" a.yml" {
		t.Fatalf("args = %s", args)
	}
}

// A yamllint that exits with a code other than 1 fails with its output as the reason, with the Go error when it printed nothing, and still yields diagnostics when the output has parsable lines.
func TestCovRunFailures(t *testing.T) {
	dir := t.TempDir()
	bin := fakeYamllint(t, dir, "usage: yamllint [-h]", 2)
	_, err := Run(context.Background(), config.Yamllint{Bin: bin}, "", []string{"a.yml"})
	if err == nil || message.Localize(message.EN, err) != "yamllint failed: usage: yamllint [-h]" {
		t.Fatalf("%v", err)
	}
	bin = fakeYamllint(t, dir, "", 3)
	_, err = Run(context.Background(), config.Yamllint{Bin: bin}, "", []string{"a.yml"})
	if err == nil || message.Localize(message.EN, err) != "yamllint failed: exit status 3" {
		t.Fatalf("%v", err)
	}
	bin = fakeYamllint(t, dir, "a.yml:1:1: [warning] missing document start (document-start)", 2)
	got, err := Run(context.Background(), config.Yamllint{Bin: bin}, "", []string{"a.yml"})
	if err != nil || len(got) != 1 || got[0].Rule != "yamllint/document-start" {
		t.Fatalf("err=%v %+v", err, got)
	}
}
