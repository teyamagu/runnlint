package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
)

// Every pair of levels of the suppression factors behaves as documented: a "# runnlint disable-line" at the end of a line, on the line
// before, or on the line before with a blank line between, suppresses the named rules (or every rule when none is named) on the target
// line; one inside a block scalar or inside quotes is not a directive; a rule that is named but suppresses nothing, an unknown rule, a
// yamllint rule, and a bare directive that suppresses nothing are reported as unused-suppression; a reason after -- changes nothing.
func TestSuppressionPairwise(t *testing.T) {
	factors := []factor{
		{"position", []string{"trailing", "previous-line", "blank-between", "inside-block", "inside-quotes"}},
		{"rules", []string{"exact", "other", "all", "two-including", "unknown", "yamllint"}},
		{"target", []string{"var-ref", "expr-syntax", "none"}},
		{"reason", []string{"with", "without"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		id := p["target"]
		if id == "none" {
			id = "var-ref"
		}
		spec := map[string]string{"exact": "rule:" + id, "other": "rule:no-debug", "all": "", "two-including": "rule:no-debug," + id, "unknown": "rule:nope", "yamllint": "rule:yamllint/line-length"}[p["rules"]]
		directive := strings.TrimSpace("# runnlint disable-line " + spec)
		if p["reason"] == "with" {
			directive += " -- because"
		}
		var expr string
		switch p["target"] {
		case "var-ref":
			expr = "vars.missing == 1"
		case "expr-syntax":
			expr = "1 +"
		default:
			expr = "vars.ok == 1"
		}
		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  ok: 1\nsteps:\n  - exec:\n      command: echo ok\n")
		switch p["position"] {
		case "trailing":
			b.WriteString("    test: \"" + expr + "\" " + directive + " #TARGET\n")
		case "previous-line":
			b.WriteString("    " + directive + " #DIRECTIVE\n    test: \"" + expr + "\" #TARGET\n")
		case "blank-between":
			b.WriteString("    " + directive + " #DIRECTIVE\n\n    test: \"" + expr + "\" #TARGET\n")
		case "inside-block":
			b.WriteString("    test: | #TARGET\n      " + expr + " " + directive + "\n")
		case "inside-quotes":
			b.WriteString("    test: \"" + expr + " " + directive + "\" #TARGET\n")
		}
		b.WriteString("  - test: steps[0].exit_code == 0\n")
		src := b.String()
		if p["position"] == "inside-block" {
			src = strings.Replace(src, " #TARGET", "", 1)
		}
		got := lint(t, src)
		var gotList []string
		for _, f := range got {
			switch f.Rule {
			case "var-ref", "expr-syntax":
				gotList = append(gotList, f.Rule)
			case "unused-suppression":
				gotList = append(gotList, f.Rule+" "+f.Match)
			}
		}
		diag := ""
		if p["target"] != "none" {
			diag = p["target"]
		}
		var want []string
		unused := func(id message.ID, args ...any) {
			want = append(want, "unused-suppression "+en(id, args...))
		}
		switch p["position"] {
		case "inside-quotes":
			if diag != "" {
				want = append(want, diag)
			}
		case "inside-block":
			if diag != "" {
				want = append(want, diag)
			}
			unused(message.SuppressInBlock)
		default:
			suppressed := false
			switch p["rules"] {
			case "exact":
				if diag != "" {
					suppressed = true
				} else {
					unused(message.SuppressNoHit, id)
				}
			case "other":
				unused(message.SuppressNoHit, "no-debug")
			case "all":
				if diag != "" {
					suppressed = true
				} else {
					unused(message.SuppressNone)
				}
			case "two-including":
				unused(message.SuppressNoHit, "no-debug")
				if diag != "" {
					suppressed = true
				} else {
					unused(message.SuppressNoHit, id)
				}
			case "unknown":
				unused(message.SuppressNoRule, "nope")
			case "yamllint":
				unused(message.SuppressYAML, "yamllint/line-length", "line-length")
			}
			if diag != "" && !suppressed {
				want = append(want, diag)
			}
		}
		if len(want) > 0 {
			expected++
		} else {
			unexpected++
		}
		sort.Strings(want)
		sort.Strings(gotList)
		if strings.Join(want, "\n") != strings.Join(gotList, "\n") {
			t.Fatalf("%s\nwant:\n%s\ngot:\n%s\n%s", describe(p), strings.Join(want, "\n"), strings.Join(gotList, "\n"), src)
		}
		// The directive's own diagnostics sit on the directive line; a suppressed-nothing bare directive on the target line is on that line.
		for _, f := range got {
			if f.Rule != "unused-suppression" {
				continue
			}
			var wantLine int
			switch p["position"] {
			case "previous-line", "blank-between":
				wantLine = lineOf(t, src, "#DIRECTIVE")
			case "inside-block":
				wantLine = lineOf(t, src, directive)
			default:
				wantLine = lineOf(t, src, "#TARGET")
			}
			if f.Line != wantLine {
				t.Fatalf("%s\nunused-suppression at line %d, want %d\n%s", describe(p), f.Line, wantLine, src)
			}
		}
	})
	if expected < 10 || unexpected < 5 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
