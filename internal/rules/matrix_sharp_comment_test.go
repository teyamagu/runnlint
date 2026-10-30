package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// sharpValue writes an expression with the chosen comment in the chosen YAML scalar style. A # after a space in a plain scalar is a YAML
// comment and never reaches runn, so for the plain style a leading comment becomes a YAML comment line before the key.
func sharpValue(expr, pos, style string) (string, bool) {
	inValue := false
	var text string
	switch pos {
	case "none":
		text = expr
	case "leading-line":
		text = "# note\n" + expr
		inValue = true
	case "trailing":
		text = expr + " # note"
		inValue = true
	case "in-string":
		text = expr + ` && "#tag" != ""`
	case "closure-paren":
		text = expr + " && all([1, 2], # > 0)"
	case "closure-brace":
		text = expr + " && len(filter([1, 2], {# > 1})) == 1"
	case "double-slash":
		text = expr + " // note"
	}
	switch style {
	case "block":
		return "|\n" + indentLines(text, "      "), inValue
	case "double-quoted":
		return `"` + strings.ReplaceAll(strings.ReplaceAll(text, `"`, `\"`), "\n", `\n`) + `"`, inValue
	default: // plain: a newline cannot appear, and a # after a space is a YAML comment, so the closure # follows the comma directly
		if pos == "leading-line" {
			return expr, false
		}
		return strings.ReplaceAll(text, "], # > 0)", "],# > 0)"), false
	}
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

// Every pair of levels of the expr-sharp-comment factors behaves as documented: a # comment outside brackets inside the value of test,
// a step or top-level if, until, a bind value, or dump reports info on that field's line, once per field; a # in a string literal, the
// closure # inside ( ) or { }, a // comment, and a YAML comment after a plain scalar are not reported; the expression still parses.
func TestExprSharpCommentPairwise(t *testing.T) {
	factors := []factor{
		{"field", []string{"test", "if-step", "if-top", "until", "bind-value", "dump-string", "dump-expr"}},
		{"pos", []string{"none", "leading-line", "trailing", "in-string", "closure-paren", "closure-brace", "double-slash"}},
		{"style", []string{"block", "double-quoted", "plain"}},
		{"count", []string{"one-step", "two-steps"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		expr := "steps[0].exit_code == 0"
		if p["field"] == "dump-string" || p["field"] == "dump-expr" || p["field"] == "bind-value" {
			expr = "steps[0].stdout"
		}
		value, inValue := sharpValue(expr, p["pos"], p["style"])
		var b strings.Builder
		b.WriteString("desc: d\n")
		if p["field"] == "if-top" {
			b.WriteString("if: " + strings.ReplaceAll(value, "\n      ", "\n  ") + "\n")
		}
		b.WriteString("steps:\n  - exec:\n      command: echo ok\n")
		steps := 1
		if p["count"] == "two-steps" {
			steps = 2
		}
		marker := "MARK"
		for i := 0; i < steps; i++ {
			switch p["field"] {
			case "test":
				b.WriteString("  - exec:\n      command: echo again\n    test: " + value + " #" + marker + "\n")
			case "if-step":
				b.WriteString("  - exec:\n      command: echo again\n    if: " + value + " #" + marker + "\n")
			case "until":
				b.WriteString("  - exec:\n      command: echo again\n    loop:\n      count: 2\n      until: " + strings.ReplaceAll(value, "\n      ", "\n        ") + " #" + marker + "\n")
			case "bind-value":
				b.WriteString("  - exec:\n      command: echo again\n    bind:\n      out: " + strings.ReplaceAll(value, "\n      ", "\n        ") + " #" + marker + "\n")
			case "dump-string":
				b.WriteString("  - dump: " + strings.ReplaceAll(value, "\n      ", "\n      ") + " #" + marker + "\n")
			case "dump-expr":
				b.WriteString("  - dump:\n      expr: " + strings.ReplaceAll(value, "\n      ", "\n        ") + " #" + marker + "\n")
			case "if-top":
				b.WriteString("  - dump: steps[0].stdout\n")
			}
		}
		b.WriteString("  - test: steps[0].exit_code == 0\n")
		src := b.String()
		// The trailing " #MARK" is a YAML comment for plain and double-quoted scalars; for block scalars it would be inside the value, so drop it there.
		if p["style"] == "block" {
			src = strings.ReplaceAll(src, " #"+marker, "")
		}
		got := lint(t, src)
		found := findRule(got, "expr-sharp-comment")
		wantN := 0
		if inValue {
			wantN = steps
			if p["field"] == "if-top" {
				wantN = 1
			}
		}
		if wantN > 0 {
			expected++
		} else {
			unexpected++
		}
		if hasRule(got, "expr-syntax") {
			t.Fatalf("%s\nexpr-syntax reported: %+v\n%s", describe(p), got, src)
		}
		if len(found) != wantN {
			t.Fatalf("%s\nwant %d findings, got %+v\n%s", describe(p), wantN, found, src)
		}
		fieldKey := map[string]string{"test": "test:", "if-step": "if:", "if-top": "if:", "until": "until:", "bind-value": "out:", "dump-string": "dump:", "dump-expr": "expr:"}[p["field"]]
		lines := linesOf(src, fieldKey)
		for i, f := range found {
			if f.Severity != finding.Info || f.Line != lines[i] {
				t.Fatalf("%s\nfinding %d at line %d, want line %d: %+v\n%s", describe(p), i, f.Line, lines[i], f, src)
			}
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
