package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Every pair of levels of the step-shape factors behaves as documented: a step runs exactly one main runner (a runner call, include, or
// exec); test, dump, and bind accompany it; desc, if, loop, defer, and force are annotations that count as neither; a step with runner:
// cannot carry a main runner or a companion, and runner: itself must be a non-empty mapping.
func TestStepShapePairwise(t *testing.T) {
	factors := []factor{
		{"mains", []string{"one", "none", "two"}},
		{"companions", []string{"none", "test", "dump-and-bind"}},
		{"runnerDef", []string{"none", "valid", "empty-map", "non-map"}},
		{"annot", []string{"none", "desc-and-if", "loop-and-force"}},
		{"style", []string{"list", "map"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		var lines []string
		switch p["annot"] {
		case "desc-and-if":
			lines = append(lines, "desc: s", "if: \"true\"")
		case "loop-and-force":
			lines = append(lines, "loop: 2", "force: true")
		}
		switch p["mains"] {
		case "one":
			lines = append(lines, "exec:", "  command: echo ok")
		case "two":
			lines = append(lines, "exec:", "  command: echo ok", "req:", "  /x:", "    get:", "      body: null")
		}
		switch p["companions"] {
		case "test":
			lines = append(lines, "test: \"true\"")
		case "dump-and-bind":
			lines = append(lines, "dump: vars.v", "bind:", "  w: vars.v")
		}
		switch p["runnerDef"] {
		case "valid":
			lines = append(lines, "runner:", "  sc: ssh://alice@example.com:22")
		case "empty-map":
			lines = append(lines, "runner: {}")
		case "non-map":
			lines = append(lines, "runner: ssh://alice@example.com:22")
		}
		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  v: 1\nrunners:\n  req: https://example.com\nsteps:\n")
		if p["style"] == "map" {
			if len(lines) == 0 {
				b.WriteString("  subject: {} #STEP\n")
			} else {
				b.WriteString("  subject: #STEP\n")
			}
			for _, l := range lines {
				b.WriteString("    " + l + "\n")
			}
			b.WriteString("  verify:\n    test: \"true\"\n")
		} else {
			if len(lines) == 0 {
				b.WriteString("  - {} #STEP\n")
			}
			for i, l := range lines {
				if i == 0 {
					b.WriteString("  - " + l + " #STEP\n")
				} else {
					b.WriteString("    " + l + "\n")
				}
			}
			b.WriteString("  - test: \"true\"\n")
		}
		src := b.String()
		got := lint(t, src)
		found := findRule(got, "step-shape")
		var want string
		hasRunner := p["runnerDef"] != "none"
		hasMain := p["mains"] != "none"
		hasCompanion := p["companions"] != "none"
		switch {
		case hasRunner && (hasMain || hasCompanion):
			want = "cannot share a step"
		case hasRunner && p["runnerDef"] != "valid":
			want = "mapping from runner name"
		case hasRunner:
			want = ""
		case p["mains"] == "two":
			want = "one main runner"
		case !hasMain && !hasCompanion:
			want = "no runner to run"
		}
		if want != "" {
			expected++
		} else {
			unexpected++
		}
		if want == "" {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, src)
			}
			return
		}
		if len(found) != 1 || found[0].Severity != finding.Error || !strings.Contains(found[0].Match, want) {
			t.Fatalf("%s\nwant %q, got %+v\n%s", describe(p), want, found, src)
		}
	})
	// Most shapes are wrong by construction, so only the valid combinations stay silent.
	if expected < 10 || unexpected < 5 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
