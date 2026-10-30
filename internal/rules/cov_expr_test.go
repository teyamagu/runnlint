package rules

import (
	"testing"
)

// An unclosed {{ in a template reports expr-syntax with the unclosed sentence. A dump mapping's expr is read as an expression, so an undefined variable in it reports var-ref.
func TestTemplateUnclosedAndDumpExpr(t *testing.T) {
	got := lint(t, "desc: \"{{ a\"\nsteps:\n  - test: \"true\"\n")
	found := findRule(got, "expr-syntax")
	if len(found) != 1 || found[0].Message != "{{ が閉じられていません" || found[0].Line != 1 {
		t.Fatalf("%+v", got)
	}
	got = lint(t, "desc: d\nsteps:\n  - dump:\n      expr: vars.missing\n      out: out.txt\n  - test: \"true\"\n")
	if msgs := messagesOf(got, "var-ref"); len(msgs) != 1 || msgs[0] != "vars.missing は vars にありません" {
		t.Fatalf("%+v", got)
	}
}

// An if wrapped in {{ }} reports expr-braces for if. A test that starts with one {{ }} but continues after it reports expr-braces only, without expr-syntax.
func TestBracedIfAndTrailingTextAfterTemplate(t *testing.T) {
	got := lint(t, "desc: d\nvars:\n  x: true\nsteps:\n  - dump: x\n    if: \"{{ vars.x }}\"\n  - test: \"true\"\n")
	if msgs := messagesOf(got, "expr-braces"); len(msgs) != 1 || msgs[0] != "if は式なので {{ }} で囲みません" {
		t.Fatalf("%+v", got)
	}
	got = lint(t, "desc: d\nsteps:\n  - test: \"{{ 1 }} and more\"\n")
	if msgs := messagesOf(got, "expr-braces"); len(msgs) != 1 || msgs[0] != "test は式なので {{ }} で囲みません" {
		t.Fatalf("%+v", got)
	}
	if hasRule(got, "expr-syntax") {
		t.Fatalf("%+v", got)
	}
}

// A test with an unterminated quote cannot be lexed, so no comment is stripped and expr-syntax reports the parse error.
// An escaped quote inside a quoted string inside {{ }} does not end the template.
func TestLexErrorAndEscapedQuoteInTemplate(t *testing.T) {
	got := lint(t, "desc: d\nsteps:\n  - test: \"'abc\"\n")
	if !hasRule(got, "expr-syntax") {
		t.Fatalf("%+v", got)
	}
	got = lint(t, "desc: '{{ \"a\\\"b\" }}'\nsteps:\n  - test: \"true\"\n")
	if hasRule(got, "expr-syntax") {
		t.Fatalf("%+v", got)
	}
}

// A step index that is an expression such as steps[vars.i] cannot be resolved, so step-ref does not judge it. inspect of a nil tree is empty.
func TestDynamicStepIndexAndNilTree(t *testing.T) {
	got := lint(t, "desc: d\nvars:\n  i: 0\nsteps:\n  - exec:\n      command: echo ok\n  - test: steps[vars.i].exit_code == 0\n")
	if hasRule(got, "step-ref") || hasRule(got, "expr-syntax") {
		t.Fatalf("%+v", got)
	}
	v := inspect(nil)
	if len(v.idents) != 0 || len(v.steps) != 0 || len(v.vars) != 0 {
		t.Fatalf("%+v", v)
	}
}

// A test that starts with {{ but never closes it is not parsed as an expression: only expr-braces is reported.
func TestUnclosedTemplateInTest(t *testing.T) {
	got := lint(t, "desc: d\nsteps:\n  - test: \"{{ 1 +\"\n")
	if msgs := messagesOf(got, "expr-braces"); len(msgs) != 1 || msgs[0] != "test は式なので {{ }} で囲みません" {
		t.Fatalf("%+v", got)
	}
	if hasRule(got, "expr-syntax") {
		t.Fatalf("%+v", got)
	}
}
