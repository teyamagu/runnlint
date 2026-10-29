package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRunSkipsNonRunbook checks that walking a directory that contains YAML with neither steps nor runners omits that file name and exits 0.
func TestRunSkipsNonRunbook(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.yml"), []byte("desc: ok\nsteps:\n  - test: 'true'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.yml"), []byte("title: memo\nbody: hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	code, err := Run(context.Background(), Options{
		Paths:      []string{"."},
		NoYamllint: true,
		Stdout:     &stdout,
		Stderr:     &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte("notes.yml")) {
		t.Fatalf("runbook ではない YAML を検査しています: %s", stdout.String())
	}
}

// TestRunBadRunbook checks that linting a file with an undefined runner as JSON includes runner-defined and exits 1.
func TestRunBadRunbook(t *testing.T) {
	dir := t.TempDir()
	body := []byte("desc: bad\nsteps:\n  - nope:\n      /:\n        get:\n          body: null\n")
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stdout bytes.Buffer
	code, err := Run(context.Background(), Options{
		Paths:      []string{"bad.yml"},
		Format:     "json",
		NoYamllint: true,
		Stdout:     &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("code = %d\n%s", code, stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("runner-defined")) {
		t.Fatalf("json = %s", stdout.String())
	}
}
