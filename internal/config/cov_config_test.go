package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
	"gopkg.in/yaml.v3"
)

func englishOf(err error) string {
	return message.Localize(message.EN, err)
}

// where that is a sequence of non-mappings, or a scalar, fails to decode with the YAML type error.
func TestCovWhereListDecodeErrors(t *testing.T) {
	for _, raw := range []string{"where:\n  - 1\n", "where: 1\n"} {
		var c Check
		err := yaml.Unmarshal([]byte(raw), &c)
		if err == nil || !strings.Contains(err.Error(), "cannot unmarshal") {
			t.Fatalf("%q: %v", raw, err)
		}
	}
}

// Load fails for a path that cannot be read. With --config, a file whose rules field has the wrong type is warned about and then fails to parse with the path as prefix.
func TestCovLoadReadAndParseErrors(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil || !strings.HasPrefix(englishOf(err), "cannot read the rules file: ") {
		t.Fatalf("%v", err)
	}
	path := writeConfig(t, t.TempDir(), "explicit.yaml", "version: 1\nrules: 1\n")
	_, err = Load(path)
	if err == nil || !strings.HasPrefix(englishOf(err), path+": ") || !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Fatalf("%v", err)
	}
}

// An empty rules file loads the defaults. A rules file whose top level is a sequence is rejected as not a mapping.
func TestCovLoadEmptyAndSequence(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeConfig(t, dir, UniqueName, "")
	cfg, err := Load("")
	if err != nil || len(cfg.Rules) == 0 || cfg.Version != 1 {
		t.Fatalf("err=%v rules=%d", err, len(cfg.Rules))
	}
	writeConfig(t, dir, UniqueName, "- a\n")
	_, err = Load("")
	if err == nil || !strings.HasSuffix(englishOf(err), ": the top level is not a mapping") {
		t.Fatalf("%v", err)
	}
}

// merge takes language and yamllint.config from the user file, and a rule written twice merges its options key by key, the later value winning.
func TestCovMergeLanguageYamllintAndOptions(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "rules.yaml", strings.Join([]string{
		"version: 1",
		"language: en",
		"yamllint:",
		"  config: lint.yaml",
		"rules:",
		"  - id: var-ref",
		"    severity: warning",
		"    options:",
		"      ignore: [a]",
		"      extra: 1",
		"  - id: var-ref",
		"    severity: warning",
		"    options:",
		"      ignore: [b]",
		"",
	}, "\n"))
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != "en" || cfg.Yamllint.Config != "lint.yaml" || cfg.Yamllint.Bin != "yamllint" {
		t.Fatalf("%+v", cfg)
	}
	for _, rule := range cfg.Rules {
		if rule.ID != "var-ref" {
			continue
		}
		ignore, _ := rule.Options["ignore"].([]any)
		if len(ignore) != 1 || ignore[0] != "b" || rule.Options["extra"] != 1 {
			t.Fatalf("%+v", rule.Options)
		}
		return
	}
	t.Fatal("var-ref not found")
}

// Validate rejects a rule without an id and a rule that has neither uses nor check.
func TestCovValidateIDAndUses(t *testing.T) {
	err := (&Config{Rules: []Rule{{Severity: finding.Error, Uses: "x"}}}).Validate()
	if err == nil || englishOf(err) != "a rule has an empty id" {
		t.Fatalf("%v", err)
	}
	err = (&Config{Rules: []Rule{{ID: "r", Severity: finding.Warning}}}).Validate()
	if err == nil || englishOf(err) != "rule r needs uses or check" {
		t.Fatalf("%v", err)
	}
}

// A ** pattern may be followed by a slash, mix with * and ?, and contain characters that are escaped in the regular expression.
func TestCovGlobMatch(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/openapi.yml", "a/b/openapi.yml", true},
		{"**/openapi.yml", "openapi.yml", true},
		{"a/**/b*.yml", "a/x/y/book.yml", true},
		{"a/**/b*.yml", "a/x/c.yml", false},
		{"a/**/?.yml", "a/x/c.yml", true},
		{"a/**/?.yml", "a/x/c.yaml", false},
		{"a.b/**", "a.b/c.yml", true},
		{"a.b/**", "axb/c.yml", false},
		{"(x)/**", "(x)/c.yml", true},
	}
	for _, tc := range cases {
		if got := globMatch(tc.pattern, tc.path); got != tc.want {
			t.Fatalf("%s vs %s: %v", tc.pattern, tc.path, got)
		}
	}
	if !(&Config{Ignore: []string{"**/gen.yml"}}).Ignored(filepath.Join("x", "gen.yml")) {
		t.Fatal("Ignored does not use globMatch for **")
	}
}

// discover returns nothing when no rules file exists up to the filesystem root, and finds .runnlint-runn.yml when the .yaml name is absent.
func TestCovDiscover(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if got := discover(); got != "" && !strings.HasSuffix(got, UniqueName) && !strings.HasSuffix(got, ".runnlint-runn.yml") {
		t.Fatalf("%s", got)
	}
	path := writeConfig(t, dir, ".runnlint-runn.yml", "version: 1\n")
	if got := discover(); got != path {
		t.Fatalf("%s want %s", got, path)
	}
	if err := os.Mkdir(filepath.Join(dir, UniqueName), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := discover(); got != path {
		t.Fatalf("a directory named %s was chosen: %s", UniqueName, got)
	}
}
