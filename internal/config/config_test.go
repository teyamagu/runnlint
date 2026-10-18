package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// 既定を継承する設定で var-ref を off にし ban-debug を足して読み込んだとき、var-ref の uses は残り、require-steps も残り、ban-debug の check が加わることを確認する。
func TestLoadOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	body := []byte("version: 1\nrules:\n  - id: var-ref\n    severity: off\n  - id: ban-debug\n    severity: error\n    message: debug は禁止です\n    check:\n      select: debug\n      exists: false\n")
	if err := os.WriteFile(filepath.Join(dir, ".runnlint.yaml"), body, 0o644); err != nil {
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

// examples/bad.yml、vendor 配下、*.gen.yml に合うパスで Ignored が true になることを確認する。examples/valid/http.yml では false になることを確認する。
func TestIgnore(t *testing.T) {
	cfg := &Config{Ignore: []string{"examples/bad.yml", "vendor/**", "*.gen.yml"}}
	if !cfg.Ignored("examples/bad.yml") {
		t.Fatal("exact")
	}
	if !cfg.Ignored("vendor/a/b.yml") {
		t.Fatal("glob")
	}
	if !cfg.Ignored("dir/foo.gen.yml") && !cfg.Ignored("foo.gen.yml") {
		t.Fatal("basename")
	}
	if cfg.Ignored("examples/valid/http.yml") {
		t.Fatal("http.yml は対象です")
	}
}
