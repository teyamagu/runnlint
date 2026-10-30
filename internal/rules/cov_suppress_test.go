package rules

// Lines that are not covered on purpose:
// suppress.go suppressedBy returns false for a finding whose rule is unused-suppression. Such findings are created after suppression runs, so none is ever passed in.

import (
	"strings"
	"testing"
)

// A comment-only directive on the last line has no following line, so it suppresses nothing and is reported as unused.
// A # after the rule list ends the list, so rule:var-ref still suppresses var-ref.
func TestDirectiveAtEndAndCommentAfterRules(t *testing.T) {
	got := lint(t, "desc: d\nsteps:\n  - dump: x\n  - test: \"true\"\n# runnlint disable-line rule:var-ref\n")
	found := findRule(got, "unused-suppression")
	if len(found) != 1 || found[0].Line != 5 || !strings.Contains(found[0].Message, "var-ref") {
		t.Fatalf("%+v", got)
	}
	got = lint(t, "desc: d\nsteps:\n  - dump: vars.missing # runnlint disable-line rule:var-ref # note\n  - test: \"true\"\n")
	if hasRule(got, "var-ref") || hasRule(got, "unused-suppression") {
		t.Fatalf("%+v", got)
	}
}

// An escaped quote inside a double-quoted value does not end the quote, so the directive after it is still read.
// The value a"b is not an expression, so without the directive expr-syntax is reported, and with it the diagnostic is suppressed and the directive is used.
func TestDirectiveAfterEscapedQuote(t *testing.T) {
	const line = "  - dump: \"a\\\"b\""
	plain := lint(t, "desc: d\nsteps:\n"+line+"\n  - test: \"true\"\n")
	if found := findRule(plain, "expr-syntax"); len(found) != 1 || found[0].Line != 3 {
		t.Fatalf("%+v", plain)
	}
	got := lint(t, "desc: d\nsteps:\n"+line+" # runnlint disable-line rule:expr-syntax\n  - test: \"true\"\n")
	if hasRule(got, "expr-syntax") || hasRule(got, "unused-suppression") {
		t.Fatalf("%+v", got)
	}
}

// A block scalar after "-   |" with extra spaces, and after "- |" alone, starts a block whose inner comment is not a suppression.
func TestBlockIndentListForms(t *testing.T) {
	for _, src := range []string{
		"desc: d\nvars:\n  a:\n    -   |\n      x # runnlint disable-line rule:var-ref\nsteps:\n  - test: \"true\"\n",
		"desc: d\nvars:\n  a:\n    - |\n      x # runnlint disable-line rule:var-ref\nsteps:\n  - test: \"true\"\n",
	} {
		got := lint(t, src)
		found := findRule(got, "unused-suppression")
		if len(found) != 1 || found[0].Line != 5 || !strings.Contains(found[0].Message, "複数行の値の中のコメントは抑制になりません") {
			t.Fatalf("%+v", got)
		}
	}
}

// With unused-suppression off, a directive that suppresses nothing is silently ignored.
func TestUnusedSuppressionOff(t *testing.T) {
	yml := "version: 1\nrules:\n  - id: unused-suppression\n    severity: off\n"
	got := lintConfig(t, yml, "desc: d\nsteps:\n  - dump: x # runnlint disable-line rule:no-such-rule\n  - test: \"true\"\n")
	if hasRule(got, "unused-suppression") {
		t.Fatalf("%+v", got)
	}
}
