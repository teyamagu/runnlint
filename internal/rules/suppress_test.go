package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

func rulesAt(findings []finding.Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, fmt.Sprintf("%s@%02d", f.Rule, f.Line))
	}
	return out
}

func onlyRules(t *testing.T, got []finding.Finding, want ...string) {
	t.Helper()
	have := rulesAt(got)
	if strings.Join(have, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v\n%+v", have, want, got)
	}
}

// A runnlint disable-line at the end of a line removes only that line's diagnostic for the named rule.
// Diagnostics for rules that were not named, and diagnostics on other lines, remain.
func TestDisableLineTrailing(t *testing.T) {
	got := lint(t, `desc: d
steps:
  - dump: x
    test: current.res.status == # runnlint disable-line rule:expr-syntax -- 直す予定
  - dump: x
    test: current.res.status ==
  - dump: vars.missing # runnlint disable-line rule:expr-syntax
`)
	onlyRules(t, got, "expr-syntax@06", "var-ref@07", "unused-suppression@07")
}

// A runnlint disable-line on a comment-only line applies to the next line that is neither empty nor a comment.
// Several rules may be listed, rule: may be omitted, and listing no rule removes every rule.
func TestDisableLineStandalone(t *testing.T) {
	got := lint(t, `desc: d
steps:
  - exec:
      # runnlint disable-line rule:var-ref, expr-syntax
      # yamllint disable-line rule:line-length
      command: echo "{{ vars.missing }} {{ 1 + }}"

  # runnlint disable-line -- 移行中なのですべて止める
  - dump: vars.other
    test: steps[5].res
`)
	onlyRules(t, got, "step-ref@10")
}

// A diagnostic for a multiline scalar is reported on the key line, so a suppression written on the key line removes it.
// A comment inside the block is part of the value, so it is not a suppression and is reported as unused-suppression. As an expression, it is stripped as a # comment.
func TestDisableLineWithBlockScalar(t *testing.T) {
	got := lint(t, `desc: d
steps:
  - dump: x
    test: | # runnlint disable-line rule:expr-syntax
      current.res.status ==
  - dump: x
    # runnlint disable-line rule:var-ref
    test: |
      vars.missing == 1
  - dump: x
    test: |
      current.res.status == 200 # runnlint disable-line rule:expr-syntax
      && true
`)
	onlyRules(t, got, "unused-suppression@12")
}

// # runnlint inside quotes is not treated as a suppression.
func TestDisableLineIgnoresQuoted(t *testing.T) {
	got := lint(t, `desc: d
steps:
  - dump: vars.missing
    test: '"# runnlint disable-line" != ""'
  - dump: "vars.other # runnlint disable-line"
`)
	onlyRules(t, got, "var-ref@03", "var-ref@05")
}

// An unknown rule, a yamllint rule, an unavailable directive, and a suppression that removes nothing are reported as unused-suppression.
func TestUnusedSuppression(t *testing.T) {
	got := lint(t, `desc: d # runnlint disable-line rule:no-such-rule
steps:
  - dump: x # runnlint disable-line rule:yamllint/line-length
  - dump: x # runnlint disable rule:var-ref
  - dump: x # runnlint disable-line
  - test: "true"
`)
	onlyRules(t, got, "unused-suppression@01", "unused-suppression@03", "unused-suppression@04", "unused-suppression@05")
	want := []string{"no-such-rule", "yamllint disable-line", "disable-line", "抑制する診断がありません"}
	for i, f := range got {
		if !strings.Contains(f.Message, want[i]) || f.Severity != finding.Warning {
			t.Fatalf("%d: %+v", i, f)
		}
	}
}

// It also applies to custom rules and declarative rules written only in YAML. A suppression that names a rule set to off is not reported.
func TestDisableLineCustomRuleAndOff(t *testing.T) {
	got := lintConfig(t, `
rules:
  - id: no-plain-password
    severity: error
    message: "plain"
    check:
      select: vars.password
      matches: '^\$\{.+\}$'
  - id: var-ref
    severity: off
`, `desc: d
vars:
  password: secret # runnlint disable-line rule:no-plain-password
steps:
  - dump: vars.missing # runnlint disable-line rule:var-ref
`)
	for _, f := range got {
		if f.Rule == "no-plain-password" || f.Rule == "unused-suppression" {
			t.Fatalf("%+v", got)
		}
	}
}

func TestParseDirectives(t *testing.T) {
	src := `a: 1 # runnlint disable-line rule:x rule:y,z -- reason # more
b: "# runnlint disable-line"
c: 'it''s # runnlint disable-line'
d: |
  # runnlint disable-line
e: x#runnlint disable-line
# runnlint disable-line

f: 2
`
	dirs := parseDirectives([]byte(src))
	if len(dirs) != 3 {
		t.Fatalf("%+v", dirs)
	}
	if dirs[0].line != 1 || dirs[0].target != 1 || strings.Join(dirs[0].rules, ",") != "x,y,z" || dirs[0].column != 6 {
		t.Fatalf("%+v", dirs[0])
	}
	if dirs[1].line != 5 || dirs[1].problem == "" {
		t.Fatalf("%+v", dirs[1])
	}
	if dirs[2].line != 7 || dirs[2].target != 9 || dirs[2].rules != nil {
		t.Fatalf("%+v", dirs[2])
	}
}
