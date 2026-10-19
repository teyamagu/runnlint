package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

func writeConfig(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// extends: none でルールを1つだけ書いた設定を読み込んだとき、ルールがその1件だけになることを確認する。yamllint の bin、missing、有効状態は既定のままであることを確認する。
func TestExtendsNoneReplacesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", `
version: 1
extends: none
rules:
  - id: only
    severity: warning
    check:
      select: desc
      exists: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].ID != "only" || cfg.Rules[0].Severity != finding.Warning {
		t.Fatalf("%+v", cfg.Rules)
	}
	if cfg.Yamllint.Bin != "yamllint" || cfg.Yamllint.Missing != string(finding.Warning) || !cfg.Yamllint.EnabledOrDefault() {
		t.Fatalf("%+v", cfg.Yamllint)
	}
}

// extends: NONE の設定に同じ id を2回書いたとき、読み込みが失敗しメッセージに「重複」が含まれることを確認する。
func TestExtendsNoneRejectsDuplicateID(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", `
version: 1
extends: NONE
rules:
  - id: a
    severity: off
  - id: a
    severity: off
`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "重複") {
		t.Fatal(err)
	}
}

// 既定を継承する設定で同じ id を2回書いたとき、ルールは1件になり、後の message と select が残り、省略した severity は error になることを確認する。
func TestLaterRuleOverlaysEarlier(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", `
version: 1
rules:
  - id: extra
    message: first
    check:
      select: desc
      exists: true
  - id: extra
    message: second
    check:
      select: debug
      exists: false
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	var found int
	for _, rule := range cfg.Rules {
		if rule.ID != "extra" {
			continue
		}
		found++
		if rule.Message != "second" || rule.Check == nil || rule.Check.Select != "debug" || rule.Severity != finding.Error || rule.Uses != "" {
			t.Fatalf("%+v", rule)
		}
	}
	if found != 1 {
		t.Fatalf("extra = %d", found)
	}
}

// severity を書かずに var-ref へ ignore だけ上書きしたとき、深刻度が error になり、uses は var-ref のまま、ignore に token が残ることを確認する。
func TestOmittedSeverityBecomesError(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", `
version: 1
rules:
  - id: var-ref
    options:
      ignore:
        - token
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range cfg.Rules {
		if rule.ID != "var-ref" {
			continue
		}
		if rule.Severity != finding.Error || rule.Uses != "var-ref" {
			t.Fatalf("%+v", rule)
		}
		raw, _ := rule.Options["ignore"].([]any)
		if len(raw) != 1 || raw[0] != "token" {
			t.Fatalf("%#v", rule.Options)
		}
		return
	}
	t.Fatal("var-ref がありません")
}

// duplicate-key に check だけを書いて読み込んだとき、uses が空になり select が desc の check に替わることを確認する。
func TestCheckReplacesBuiltin(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", `
version: 1
rules:
  - id: duplicate-key
    check:
      select: desc
      exists: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range cfg.Rules {
		if rule.ID == "duplicate-key" {
			if rule.Uses != "" || rule.Check == nil || rule.Check.Select != "desc" {
				t.Fatalf("%+v", rule)
			}
			return
		}
	}
	t.Fatal("duplicate-key がありません")
}

// 次の不正な設定を読み込んだとき、いずれも失敗し、エラーに期待する語句が含まれることを確認する。
func TestValidateRuleShape(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// uses と check を同時に書いた設定を読み込んだとき、失敗しエラーに「同時」が含まれることを確認する。
			name: "uses and check",
			body: `
version: 1
extends: none
rules:
  - id: x
    uses: duplicate-key
    check:
      select: desc
      exists: true
`,
			want: "同時",
		},
		{
			// select が空白だけの check を読み込んだとき、失敗しエラーに select が含まれることを確認する。
			name: "empty select",
			body: `
version: 1
extends: none
rules:
  - id: x
    check:
      select: "  "
      exists: true
`,
			want: "select",
		},
		{
			// severity に fatal を書いた設定を読み込んだとき、失敗しエラーに severity が含まれることを確認する。
			name: "bad severity",
			body: `
version: 1
extends: none
rules:
  - id: x
    severity: fatal
    uses: duplicate-key
`,
			want: "severity",
		},
		{
			// yamllint.missing に fatal を書いた設定を読み込んだとき、失敗しエラーに yamllint.missing が含まれることを確認する。
			name: "bad yamllint missing",
			body: `
version: 1
extends: none
yamllint:
  missing: fatal
rules:
  - id: x
    severity: off
`,
			want: "yamllint.missing",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeConfig(t, dir, ".runnlint.yaml", tc.body)
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatal(err)
			}
		})
	}
}

// uses も check も無く severity が off のルールだけを読み込んだとき、成功しそのルールの uses と check が空のままであることを確認する。
func TestOffRuleNeedsNoImplementation(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", `
version: 1
extends: none
rules:
  - id: paused
    severity: off
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Uses != "" || cfg.Rules[0].Check != nil {
		t.Fatalf("%+v", cfg.Rules[0])
	}
}

// yamllint の enabled と missing だけを書いた設定を読み込んだとき、その2項目だけ替わり bin は yamllint のままであることを確認する。
func TestYamllintFieldsMerge(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", `
version: 1
yamllint:
  enabled: false
  missing: error
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Yamllint.EnabledOrDefault() || cfg.Yamllint.Missing != "error" || cfg.Yamllint.Bin != "yamllint" {
		t.Fatalf("%+v", cfg.Yamllint)
	}
}

// where を配列ではなく mapping 1件で書いた設定を読み込んだとき、条件が1件になり path と exists が保たれることを確認する。
func TestWhereAcceptsSingleMapping(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", `
version: 1
extends: none
rules:
  - id: w
    check:
      select: steps.*
      where:
        path: test
        exists: true
      forbid: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	where := cfg.Rules[0].Check.Where
	if len(where) != 1 || where[0].Path != "test" || where[0].Exists == nil || !*where[0].Exists {
		t.Fatalf("%+v", where)
	}
}

// 子ディレクトリで設定パスを空にして読み込んだとき、親の .runnlint.yaml が選ばれ runnlint.yml のルールは読み込まれないことを確認する。
func TestDiscoverWalksToParentAndPrefersDotYAML(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, ".runnlint.yaml", `
version: 1
extends: none
rules:
  - id: from-dot
    severity: off
`)
	writeConfig(t, root, "runnlint.yml", `
version: 1
extends: none
rules:
  - id: from-plain
    severity: off
`)
	sub := filepath.Join(root, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].ID != "from-dot" {
		t.Fatalf("%+v", cfg.Rules)
	}
}

// JSON の設定ファイルを読み込んだとき成功し、extends: none のとおりルールが require-steps の1件だけになることを確認する。
func TestLoadJSONConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "rules.json", `{
  "version": 1,
  "extends": "none",
  "rules": [
    {"id": "require-steps", "uses": "require-steps", "severity": "error"}
  ]
}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Uses != "require-steps" {
		t.Fatalf("%+v", cfg.Rules)
	}
}

// 探索で見つけた .runnlint.yaml が runnlint のルールファイルの形でない（未知のキー、型の違い、未知の version）とき、
// 警告を残して読み飛ばし、同じディレクトリの次の候補や親ディレクトリ、無ければデフォルトを使うことを確認する。
func TestDiscoverSkipsForeignConfig(t *testing.T) {
	foreign := map[string]string{
		"未知のキー":         "linters:\n  - runbook\n",
		"rules の型":      "rules:\n  no-debug: true\n",
		"未知の version":   "version: 3\nrules: []\n",
		"YAML として壊れている": "rules: [\n",
	}
	for name, body := range foreign {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, ".runnlint.yaml", body)
			t.Chdir(root)
			cfg, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], ".runnlint.yaml") || !strings.Contains(cfg.Warnings[0], ".runnlint-runn.yaml") {
				t.Fatalf("%v", cfg.Warnings)
			}
			if len(cfg.Rules) == 0 || cfg.SourceDir != "" {
				t.Fatalf("デフォルトになっていません: %+v", cfg)
			}

			writeConfig(t, root, "runnlint.yml", "extends: none\nrules:\n  - id: from-plain\n    severity: off\n")
			cfg, err = Load("")
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Rules) != 1 || cfg.Rules[0].ID != "from-plain" || len(cfg.Warnings) != 1 {
				t.Fatalf("%+v", cfg)
			}
		})
	}
}

// .runnlint-runn.yaml は従来の名前より先に選ばれることを確認する。固有の名前なので、形が違えば読み飛ばさず失敗する。
func TestDiscoverPrefersUniqueName(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, ".runnlint.yaml", "extends: none\nrules:\n  - id: from-legacy\n    severity: off\n")
	writeConfig(t, root, ".runnlint-runn.yaml", "extends: none\nrules:\n  - id: from-unique\n    severity: off\n")
	t.Chdir(root)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].ID != "from-unique" {
		t.Fatalf("%+v", cfg.Rules)
	}
	writeConfig(t, root, ".runnlint-runn.yaml", "linters: []\n")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "linters") {
		t.Fatalf("err = %v", err)
	}
}

// --config で渡したファイルは、未知のキーがあっても従来どおり読み込み、警告だけを残すことを確認する。
func TestExplicitConfigWarnsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", "extends: default\nignores:\n  - vendor\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "ignores") {
		t.Fatalf("%v", cfg.Warnings)
	}
}

// --print-config の出力を従来の名前で置いたとき、読み飛ばさず警告も出さないことを確認する。
func TestPrintedDefaultPassesShapeCheck(t *testing.T) {
	root := t.TempDir()
	raw, err := DefaultYAML()
	if err != nil {
		t.Fatal(err)
	}
	writeConfig(t, root, ".runnlint.yaml", string(raw))
	t.Chdir(root)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Warnings) != 0 || cfg.SourceDir != root {
		t.Fatalf("%v %s", cfg.Warnings, cfg.SourceDir)
	}
}
