package rules

import (
	"sort"
	"strings"
	"testing"
)

// Every pair of levels of the expression factors behaves as documented for expr-syntax and expr-braces: test and bind read {{ only as a
// wrapper (a sole {{ }} reports expr-braces and its inside is parsed; a value that starts with {{ but is not one template reports
// expr-braces alone); if, until, dump, and loop.count report expr-braces for any {{ and have their templates parsed; a plain string such as
// an exec command has only its templates parsed; an unset ${...} and a # comment outside brackets never cause a syntax error.
func TestExprSyntaxAndBracesPairwise(t *testing.T) {
	factors := []factor{
		{"field", []string{"test", "bind-value", "if-step", "if-top", "until", "dump", "count", "template-value"}},
		{"content", []string{"valid", "broken", "whole-valid", "whole-broken", "partial-valid", "partial-broken", "env", "sharp"}},
		{"style", []string{"plain", "double-quoted", "block"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		field, content, style := p["field"], p["content"], p["style"]
		exprField := field != "template-value"
		var text string
		switch {
		case field == "count":
			text = map[string]string{"valid": "vars.n", "broken": "1 +", "whole-valid": "{{ vars.n }}", "whole-broken": "{{ 1 + }}", "partial-valid": "{{ vars.n }} + 1", "partial-broken": "{{ 1 + }} + 1", "env": "${RUNN_MATRIX_UNSET}", "sharp": "vars.n # c"}[content]
		case exprField:
			text = map[string]string{"valid": "vars.x == 1", "broken": "vars.x ==", "whole-valid": "{{ vars.x == 1 }}", "whole-broken": "{{ vars.x == }}", "partial-valid": "{{ vars.x }} == 1", "partial-broken": "{{ vars.x == }} == 1", "env": "vars.x == ${RUNN_MATRIX_UNSET}", "sharp": "vars.x == 1 # c"}[content]
		default:
			text = map[string]string{"valid": "echo {{ vars.x }}", "broken": "echo {{ vars.x == }}", "whole-valid": "{{ vars.x }}", "whole-broken": "{{ 1 + }}", "partial-valid": "{{ vars.x }} ok", "partial-broken": "{{ 1 + }} ok", "env": "echo ${RUNN_MATRIX_UNSET}", "sharp": "echo # c"}[content]
		}
		// A plain scalar cannot start with {, so those contents fall back to a double-quoted scalar; a # after a space in a plain scalar is a YAML comment.
		if style == "plain" && strings.HasPrefix(text, "{{") {
			style = "double-quoted"
		}
		// keyIndent is the column of the field's key, so a block scalar's content is indented two deeper.
		keyIndent := map[string]int{"test": 4, "if-step": 4, "dump": 4, "bind-value": 6, "until": 6, "count": 6, "template-value": 6, "if-top": 0}[field]
		var value string
		switch style {
		case "double-quoted":
			value = `"` + strings.ReplaceAll(text, `"`, `\"`) + `"`
		case "block":
			value = "|\n" + strings.Repeat(" ", keyIndent+2) + text
		default:
			value = text
		}
		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  x: 1\n  n: 2\n")
		if field == "if-top" {
			b.WriteString("if: " + value + " #SITE\n")
		}
		b.WriteString("steps:\n  - exec:\n      command: echo ok\n")
		switch field {
		case "test":
			b.WriteString("  - exec:\n      command: echo again\n    test: " + value + " #SITE\n")
		case "bind-value":
			b.WriteString("  - exec:\n      command: echo again\n    bind:\n      out: " + value + " #SITE\n  - dump: out\n")
		case "if-step":
			b.WriteString("  - exec:\n      command: echo again\n    if: " + value + " #SITE\n")
		case "until":
			b.WriteString("  - exec:\n      command: echo again\n    loop:\n      count: 2\n      until: " + value + " #SITE\n")
		case "dump":
			b.WriteString("  - dump: " + value + " #SITE\n")
		case "count":
			b.WriteString("  - exec:\n      command: echo again\n    loop:\n      count: " + value + " #SITE\n")
		case "template-value":
			b.WriteString("  - exec:\n      command: " + value + " #SITE\n")
		case "if-top":
			b.WriteString("  - dump: vars.x\n")
		}
		b.WriteString("  - test: steps[0].exit_code == 0\n")
		src := b.String()
		if style == "block" {
			src = strings.ReplaceAll(src, " #SITE", "")
		}
		got := lint(t, src)
		var gotRules []string
		for _, f := range got {
			if f.Rule == "expr-syntax" || f.Rule == "expr-braces" {
				gotRules = append(gotRules, f.Rule)
			}
		}
		var want []string
		switch {
		case field == "test" || field == "bind-value":
			switch content {
			case "broken":
				want = []string{"expr-syntax"}
			case "whole-valid", "partial-valid", "partial-broken":
				want = []string{"expr-braces"}
			case "whole-broken":
				want = []string{"expr-braces", "expr-syntax"}
			}
		case exprField:
			switch content {
			case "broken":
				want = []string{"expr-syntax"}
			case "whole-valid", "partial-valid":
				want = []string{"expr-braces"}
			case "whole-broken", "partial-broken":
				want = []string{"expr-braces", "expr-syntax"}
			}
		default:
			switch content {
			case "broken", "whole-broken", "partial-broken":
				want = []string{"expr-syntax"}
			}
		}
		if len(want) > 0 {
			expected++
		} else {
			unexpected++
		}
		sort.Strings(want)
		sort.Strings(gotRules)
		if strings.Join(want, ",") != strings.Join(gotRules, ",") {
			t.Fatalf("%s\nwant %v, got %v: %+v\n%s", describe(p), want, gotRules, got, src)
		}
		var line int
		if style == "block" {
			line = lineOf(t, src, map[string]string{"test": "test: |", "bind-value": "out: |", "if-step": "if: |", "if-top": "if: |", "until": "until: |", "dump": "dump: |", "count": "count: |", "template-value": "command: |"}[field])
		} else {
			line = lineOf(t, src, "#SITE")
		}
		for _, f := range got {
			if (f.Rule == "expr-syntax" || f.Rule == "expr-braces") && f.Line != line {
				t.Fatalf("%s\n%s at line %d, want line %d\n%s", describe(p), f.Rule, f.Line, line, src)
			}
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
