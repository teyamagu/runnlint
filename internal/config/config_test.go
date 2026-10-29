package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// TestLoadOverride checks that a config extending the defaults, with var-ref off and ban-debug added, keeps var-ref's uses and require-steps, and adds ban-debug's check.
func TestLoadOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	body := []byte("version: 1\nrules:\n  - id: var-ref\n    severity: off\n  - id: ban-debug\n    severity: error\n    message: debug は禁止です\n    check:\n      select: debug\n      exists: false\n")
	if err := os.WriteFile(filepath.Join(dir, ".runnlint-runn.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, rule := range cfg.Rules {
		if rule.ID == "var-ref" {
			found = true
			if rule.Severity != finding.Off {
				t.Fatalf("severity = %s", rule.Severity)
			}
			if rule.Uses != "var-ref" {
				t.Fatalf("uses が消えました: %+v", rule)
			}
		}
		if rule.ID == "require-steps" && rule.Uses != "require-steps" {
			t.Fatalf("デフォルトルールが消えました: %+v", rule)
		}
	}
	if !found {
		t.Fatal("var-ref がありません")
	}
	var custom bool
	for _, rule := range cfg.Rules {
		if rule.ID == "ban-debug" && rule.Check != nil && rule.Check.Select == "debug" {
			custom = true
		}
	}
	if !custom {
		t.Fatal("追加ルールがマージされていません")
	}
}

// TestIgnore checks that a literal matches the walked path and paths under it. A trailing / is stripped only for descendants; an exact path match keeps the trailing /.
// A pattern matches a base name only when the whole pattern matches that name. *.gen.yml matches; dir/*.yml matches only files directly under dir.
// ** matches only a walked path and spans intermediate directories. A pattern with ./, or one that contains /, misses when the walked path string differs.
func TestIgnore(t *testing.T) {
	cases := []struct {
		pat  string
		path string
		want bool
	}{
		{"examples/fixtures/openapi.yml", "examples/fixtures/openapi.yml", true},
		{"examples/fixtures/openapi.yml", "examples/valid/openapi.yml", false},
		{"examples/fixtures/openapi.yml", "/repo/examples/fixtures/openapi.yml", false},
		{"examples/bad.yml", "examples/bad.yml", true},
		{"examples/bad", "examples/bad/a.yml", true},
		{"examples/bad", "examples/bad-extra/a.yml", false},
		{"examples/bad/", "examples/bad/a.yml", true},
		{"examples/bad/", "examples/bad", false},
		{"examples/fixtures/openapi.yml/", "examples/fixtures/openapi.yml", false},
		{"dir/*.yml", "a.yml", false},
		{"dir/*.yml", "books/dir/a.yml", false},
		{"./examples/bad", "examples/bad/a.yml", false},
		{"openapi.yml", "examples/valid/openapi.yml", true},
		{"openapi.yml", "/repo/examples/valid/openapi.yml", true},
		{"vendor", "vendor/a/b.yml", true},
		{"vendor", "foo/vendor", true},
		{"vendor", "/repo/vendor/a.yml", false},
		{"*.gen.yml", "foo.gen.yml", true},
		{"*.gen.yml", "dir/foo.gen.yml", true},
		{"*.gen.yml", "/repo/dir/foo.gen.yml", true},
		{"dir/*.yml", "dir/a.yml", true},
		{"dir/*.yml", "dir/sub/a.yml", false},
		{"vendor/**", "vendor/a.yml", true},
		{"vendor/**", "vendor/a/b.yml", true},
		{"vendor/**", "other/vendor/a.yml", false},
	}
	for _, c := range cases {
		cfg := &Config{Ignore: []string{c.pat}}
		if got := cfg.Ignored(c.path); got != c.want {
			t.Errorf("pattern %q path %q = %v, want %v", c.pat, c.path, got, c.want)
		}
	}
	kept := &Config{Ignore: []string{"examples/bad.yml", "vendor/**", "*.gen.yml"}}
	if kept.Ignored("examples/valid/http.yml") {
		t.Fatal("examples/valid/http.yml は対象です")
	}
}
