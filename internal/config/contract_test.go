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

// TestExtendsNoneReplacesDefaults checks that a config with extends: none and one rule loads as that single rule. yamllint bin, missing, and enabled stay at their defaults.
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

// TestExtendsNoneRejectsDuplicateID checks that an extends: NONE config that repeats an id fails to load and the error says the rule is duplicated.
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

// TestLaterRuleOverlaysEarlier checks that a config extending the defaults and repeating an id keeps one rule, keeps the later message and select, and uses error when severity is omitted.
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

// TestOmittedSeverityBecomesError checks that overriding var-ref with only ignore, and no severity, sets severity to error, leaves uses as var-ref, and keeps token in ignore.
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

// TestCheckReplacesBuiltin checks that loading duplicate-key with only a check clears uses and replaces it with a check whose select is desc.
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

// TestValidateRuleShape checks that each invalid config below fails and that the error contains the expected wording.
func TestValidateRuleShape(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// A config that sets uses and check together fails, and the error says they cannot both be set.
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
			// A check whose select is only whitespace fails, and the error mentions select.
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
			// A config that sets severity to fatal fails, and the error mentions severity.
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
			// A config that sets yamllint.missing to fatal fails, and the error mentions yamllint.missing.
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

// TestOffRuleNeedsNoImplementation checks that a rule with neither uses nor check and severity off loads, and that its uses and check stay empty.
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

// TestYamllintFieldsMerge checks that a config setting only yamllint enabled and missing changes those two fields and leaves bin as yamllint.
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

// TestWhereAcceptsSingleMapping checks that a where written as one mapping, not an array, becomes one condition and keeps path and exists.
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

// TestDiscoverWalksToParentAndPrefersYAML checks that an empty config path in a child directory selects the parent's .runnlint-runn.yaml and does not read the .runnlint-runn.yml or .runnlint.yaml beside it.
func TestDiscoverWalksToParentAndPrefersYAML(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, ".runnlint-runn.yaml", `
version: 1
extends: none
rules:
  - id: from-dot
    severity: off
`)
	writeConfig(t, root, ".runnlint-runn.yml", `
version: 1
extends: none
rules:
  - id: from-yml
    severity: off
`)
	writeConfig(t, root, ".runnlint.yaml", `
version: 1
extends: none
rules:
  - id: from-legacy
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
	if len(cfg.Rules) != 1 || cfg.Rules[0].ID != "from-dot" || len(cfg.Warnings) != 0 {
		t.Fatalf("%+v warnings=%v", cfg.Rules, cfg.Warnings)
	}
}

// TestDiscoverIgnoresLegacyNames checks that a legacy name such as .runnlint.yaml is not read even when its contents are a runnlint config, that no warning is emitted, and that the defaults are used.
func TestDiscoverIgnoresLegacyNames(t *testing.T) {
	for _, name := range []string{".runnlint.yaml", ".runnlint.yml", "runnlint.yaml", "runnlint.yml"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, name, "extends: none\nrules:\n  - id: from-legacy\n    severity: off\n")
			t.Chdir(root)
			cfg, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			if cfg.SourceDir != "" || len(cfg.Warnings) != 0 {
				t.Fatalf("dir=%s warnings=%v", cfg.SourceDir, cfg.Warnings)
			}
			for _, rule := range cfg.Rules {
				if rule.ID == "from-legacy" {
					t.Fatal("従来の名前を読んでいます")
				}
			}
		})
	}
}

// TestLoadJSONConfig checks that a JSON config file loads and that extends: none leaves only the require-steps rule.
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

// TestDiscoverRejectsForeignUniqueName checks that a discovered .runnlint-runn.yaml with the wrong shape fails instead of being skipped.
func TestDiscoverRejectsForeignUniqueName(t *testing.T) {
	foreign := map[string]string{
		"未知のキー":         "linters:\n  - runbook\n",
		"rules の型":      "rules:\n  no-debug: true\n",
		"未知の version":   "version: 3\nrules: []\n",
		"YAML として壊れている": "rules: [\n",
	}
	for name, body := range foreign {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, ".runnlint-runn.yaml", body)
			t.Chdir(root)
			_, err := Load("")
			if err == nil {
				t.Fatal("形の違う設定を読んでいます")
			}
		})
	}
}

// TestExplicitConfigWarnsUnknownKeys checks that a file passed with --config still loads when it has unknown keys, and that only a warning is kept.
func TestExplicitConfigWarnsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".runnlint.yaml", "extends: default\nignores:\n  - vendor\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0].Error(), "ignores") {
		t.Fatalf("%v", cfg.Warnings)
	}
}

// TestPrintedDefaultPassesShapeCheck checks that placing --print-config output in .runnlint-runn.yaml loads with no warning.
func TestPrintedDefaultPassesShapeCheck(t *testing.T) {
	root := t.TempDir()
	raw, err := DefaultYAML()
	if err != nil {
		t.Fatal(err)
	}
	writeConfig(t, root, ".runnlint-runn.yaml", string(raw))
	t.Chdir(root)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Warnings) != 0 || cfg.SourceDir != root {
		t.Fatalf("%v %s", cfg.Warnings, cfg.SourceDir)
	}
}

// TestNestedUnknownKeyWarnsAndLoads checks that a discovered config whose only problem is an unknown key inside rules loads and emits a warning.
func TestNestedUnknownKeyWarnsAndLoads(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, ".runnlint-runn.yaml", "extends: none\nrules:\n  - id: mine\n    severity: off\n    sevrity: error\n")
	t.Chdir(root)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].ID != "mine" || len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0].Error(), "sevrity") {
		t.Fatalf("%+v", cfg)
	}
}
