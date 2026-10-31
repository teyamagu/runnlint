package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Every pair of levels of the require-test-step factors behaves as documented when the rule is enabled at info: a runbook is reported on
// its steps line only when it has steps, its top-level if does not require included, and no step of the book or its inline includes has
// test without a runner call, include, exec, or runner:, whatever the step style. The default config leaves the rule off.
func TestRequireTestStepPairwise(t *testing.T) {
	factors := []factor{
		{"cond", []string{"none", "included", "included == true", "included && vars.flag", "included || vars.flag", "!included", "bool", "template", "env"}},
		{"verify", []string{"none", "test-only", "test-with-companions", "test-with-loop", "inline-child", "test-on-exec", "test-on-req", "test-on-include", "runner-def-with-test"}},
		{"count", []string{"one", "empty"}},
		{"steps", []string{"list", "map"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		mapStyle := p["steps"] == "map"
		var b strings.Builder
		b.WriteString("desc: d\n")
		switch p["cond"] {
		case "none":
		case "bool":
			b.WriteString("if: true\n")
		case "template":
			b.WriteString("if: \"{{ vars.flag }}\"\n")
		case "env":
			b.WriteString("if: '${RUNN_MATRIX_FLAG:-true}'\n")
		default:
			b.WriteString("if: '" + p["cond"] + "'\n")
		}
		b.WriteString("vars:\n  flag: true\nrunners:\n  req: https://example.com\n")
		if p["count"] == "empty" {
			if mapStyle {
				b.WriteString("steps: {}\n")
			} else {
				b.WriteString("steps: []\n")
			}
		} else {
			b.WriteString("steps:\n")
			n := 0
			writeStep := func(lines []string) {
				n++
				if mapStyle {
					b.WriteString("  s" + strings.Repeat("x", n) + ":\n")
					for _, l := range lines {
						b.WriteString("    " + l + "\n")
					}
					return
				}
				for i, l := range lines {
					if i == 0 {
						b.WriteString("  - " + l + "\n")
					} else {
						b.WriteString("    " + l + "\n")
					}
				}
			}
			writeStep([]string{"req:", "  /health:", "    get:", "      body: null"})
			switch p["verify"] {
			case "none":
				writeStep([]string{"dump: steps[0].res"})
			case "test-only":
				writeStep([]string{"test: steps[0].res.status == 200"})
			case "test-with-companions":
				writeStep([]string{"desc: verify", "if: \"true\"", "dump: steps[0].res", "bind:", "  status: steps[0].res.status", "test: status == 200"})
			case "test-with-loop":
				writeStep([]string{"loop:", "  count: 2", "test: steps[0].res.status == 200"})
			case "inline-child":
				writeStep([]string{"include:", "  steps:", "    - test: steps[0].res.status == 200"})
			case "test-on-exec":
				writeStep([]string{"exec:", "  command: echo ok", "test: current.exit_code == 0"})
			case "test-on-req":
				writeStep([]string{"req:", "  /again:", "    get:", "      body: null", "test: current.res.status == 200"})
			case "test-on-include":
				writeStep([]string{"include:", "  path: other.yml", "test: \"true\""})
			case "runner-def-with-test":
				writeStep([]string{"runner:", "  sc: ssh://alice@example.com:22", "test: \"true\""})
			}
		}
		src := b.String()
		got := lintRequireTestStep(t, src)
		found := findRule(got, "require-test-step")
		requires := p["cond"] == "included" || p["cond"] == "included == true" || p["cond"] == "included && vars.flag"
		verified := p["verify"] == "test-only" || p["verify"] == "test-with-companions" || p["verify"] == "test-with-loop" || p["verify"] == "inline-child"
		expect := p["count"] != "empty" && !requires && !verified
		if expect {
			expected++
		} else {
			unexpected++
		}
		if !expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, src)
			}
			if p["count"] == "empty" && !hasRule(got, "require-steps") {
				t.Fatalf("%s\nrequire-steps missing: %+v\n%s", describe(p), got, src)
			}
			return
		}
		want := lineOf(t, src, "steps:")
		if len(found) != 1 || found[0].Severity != finding.Info || found[0].Line != want {
			t.Fatalf("%s\nwant one info at line %d, got %+v\n%s", describe(p), want, found, src)
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
