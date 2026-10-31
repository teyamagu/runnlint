package rules

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
)

// Loading the default config finds no duplicate ids, and every rule other than off has exactly one of uses or check. Registered builtins are included with uses, and require-desc, no-debug, and loop-until-with-test have only check.
func TestDefaultConfigMatchesRegistry(t *testing.T) {
	cfg := defaultConfig(t)
	seen := map[string]config.Rule{}
	for _, rule := range cfg.Rules {
		if _, ok := seen[rule.ID]; ok {
			t.Fatalf("id %s が重複しています", rule.ID)
		}
		seen[rule.ID] = rule
		if rule.Severity == "off" {
			continue
		}
		if rule.Uses != "" && rule.Check != nil {
			t.Fatalf("%s は uses と check を両方持っています", rule.ID)
		}
		if rule.Uses == "" && rule.Check == nil {
			t.Fatalf("%s に uses も check もありません", rule.ID)
		}
		if rule.Uses != "" && !KnownBuiltin(rule.Uses) {
			t.Fatalf("%s の uses %s は未登録です", rule.ID, rule.Uses)
		}
	}
	for name := range registry {
		rule, ok := seen[name]
		if !ok || rule.Uses != name {
			t.Fatalf("組み込み %s がデフォルトにありません", name)
		}
	}
	for _, id := range []string{"require-desc", "no-debug", "loop-until-with-test"} {
		rule, ok := seen[id]
		if !ok || rule.Uses != "" || rule.Check == nil {
			t.Fatalf("宣言的ルール %s がデフォルトにありません: %+v", id, rule)
		}
	}
	if seen["require-test-step"].Severity != "off" {
		t.Fatalf("require-test-step のデフォルトは off です: %s", seen["require-test-step"].Severity)
	}
}

// CheckConfig fails when an unknown uses is passed at error. The same uses succeeds when severity is off.
func TestCheckConfigRejectsUnknownBuiltin(t *testing.T) {
	err := CheckConfig(&config.Config{Rules: []config.Rule{{
		ID: "x", Uses: "no-such-rule", Severity: "error",
	}}})
	if err == nil {
		t.Fatal("未知の uses を受けています")
	}
	err = CheckConfig(&config.Config{Rules: []config.Rule{{
		ID: "x", Uses: "no-such-rule", Severity: "off",
	}}})
	if err != nil {
		t.Fatal(err)
	}
}
