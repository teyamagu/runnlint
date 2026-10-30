package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Every pair of levels of the loop-index factors behaves as documented: a bare identifier i in any expression site is reported unless the
// step has a loop (a count or a mapping) or the runbook has a top-level loop; a top-level if sees only the top-level loop; another
// identifier, a member named i, and the string "i" are not the index.
func TestLoopIndexPairwise(t *testing.T) {
	factors := []factor{
		{"site", []string{"test", "dump", "template-value", "bind-value", "top-if"}},
		{"loop", []string{"none", "step-count", "step-map-until", "top-loop"}},
		{"ident", []string{"i", "j", "member-i", "string-i"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		expr := map[string]string{"i": "i == 0", "j": "j == 0", "member-i": "vars.i == 0", "string-i": "len('i') == 1"}[p["ident"]]
		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  i: 1\n  j: 1\n")
		if p["loop"] == "top-loop" {
			b.WriteString("loop:\n  count: 2\n")
		}
		if p["site"] == "top-if" {
			b.WriteString("if: " + expr + " #SITE\n")
		}
		b.WriteString("steps:\n  - exec:\n      command: echo ok\n")
		switch p["loop"] {
		case "step-count":
			b.WriteString("    loop: 2\n")
		case "step-map-until":
			b.WriteString("    loop:\n      count: 2\n      until: current.exit_code == 0\n")
		}
		switch p["site"] {
		case "test":
			b.WriteString("    test: " + expr + " #SITE\n")
		case "dump":
			b.WriteString("    dump: " + exprOperand(expr) + " #SITE\n")
		case "template-value":
			b.WriteString("    bind:\n      x: \"{{ " + exprOperand(expr) + " }}\" #SITE\n")
		case "bind-value":
			b.WriteString("    bind:\n      x: " + exprOperand(expr) + " #SITE\n")
		}
		b.WriteString("  - test: steps[0].exit_code == 0\n")
		src := b.String()
		got := lint(t, src)
		found := findRule(got, "loop-index")
		inLoop := p["loop"] == "top-loop" || (p["site"] != "top-if" && p["loop"] != "none")
		expect := p["ident"] == "i" && !inLoop
		if expect {
			expected++
		} else {
			unexpected++
		}
		if !expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, src)
			}
			return
		}
		line := lineOf(t, src, "#SITE")
		if len(found) != 1 || found[0].Severity != finding.Warning || found[0].Line != line {
			t.Fatalf("%s\nwant one warning at line %d, got %+v\n%s", describe(p), line, found, src)
		}
	})
	if expected < 5 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}

// Every pair of levels of the previous-step factors behaves as documented: the identifier previous is reported on the first step and in
// top-level fields, which run before any step, and not on a later step; a member named previous and the string "previous" are not the
// identifier.
func TestPreviousStepPairwise(t *testing.T) {
	factors := []factor{
		{"site", []string{"test", "dump", "template-value", "bind-value", "top-if", "top-until"}},
		{"position", []string{"first", "second"}},
		{"ident", []string{"previous", "member-previous", "string-previous"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		expr := map[string]string{"previous": "previous.exit_code == 0", "member-previous": "vars.previous == 0", "string-previous": "len('previous') == 8"}[p["ident"]]
		plain := exprOperand(expr)
		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  previous: 1\n")
		if p["site"] == "top-if" {
			b.WriteString("if: " + expr + " #SITE\n")
		}
		if p["site"] == "top-until" {
			b.WriteString("loop:\n  count: 2\n  until: " + expr + " #SITE\n")
		}
		b.WriteString("steps:\n")
		write := func(withSite bool) {
			b.WriteString("  - exec:\n      command: echo ok\n")
			if !withSite {
				return
			}
			switch p["site"] {
			case "test":
				b.WriteString("    test: " + expr + " #SITE\n")
			case "dump":
				b.WriteString("    dump: " + plain + " #SITE\n")
			case "template-value":
				b.WriteString("    bind:\n      x: \"{{ " + plain + " }}\" #SITE\n")
			case "bind-value":
				b.WriteString("    bind:\n      x: " + plain + " #SITE\n")
			}
		}
		if p["position"] == "first" {
			write(true)
			write(false)
		} else {
			write(false)
			write(true)
		}
		b.WriteString("  - test: steps[0].exit_code == 0\n")
		src := b.String()
		got := lint(t, src)
		found := findRule(got, "previous-step")
		topLevel := p["site"] == "top-if" || p["site"] == "top-until"
		expect := p["ident"] == "previous" && (topLevel || p["position"] == "first")
		if expect {
			expected++
		} else {
			unexpected++
		}
		if !expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, src)
			}
			return
		}
		line := lineOf(t, src, "#SITE")
		if len(found) != 1 || found[0].Severity != finding.Warning || found[0].Line != line {
			t.Fatalf("%s\nwant one warning at line %d, got %+v\n%s", describe(p), line, found, src)
		}
	})
	if expected < 5 || unexpected < 5 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}

// exprOperand drops the trailing comparison of an expression such as "i == 0" or "len('i') == 1", leaving the operand for dump and bind.
func exprOperand(expr string) string {
	if i := strings.LastIndex(expr, " == "); i >= 0 {
		return expr[:i]
	}
	return expr
}
