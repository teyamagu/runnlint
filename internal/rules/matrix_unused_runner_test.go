package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Every pair of levels of the unused-runner factors behaves as documented: a runner defined in runners: or in a step's runner: is reported
// only when no step of the book or its inline includes calls it, whatever the step style, the runner kind, or where the call sits,
// and a runner with a reserved name is left to runner-reserved.
func TestUnusedRunnerPairwise(t *testing.T) {
	factors := []factor{
		{"def", []string{"runners-string", "runners-map", "step-runner"}},
		{"name", []string{"normal", "reserved"}},
		{"usage", []string{"none", "after", "before", "inline-child", "twice"}},
		{"steps", []string{"list", "map"}},
		{"kind", []string{"ssh", "http"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		mapStyle := p["steps"] == "map"
		name := map[string]string{"ssh": "sc", "http": "req"}[p["kind"]]
		if p["name"] == "reserved" {
			name = "test"
		}
		var def string
		switch p["kind"] {
		case "ssh":
			if p["def"] == "runners-map" {
				def = name + ":\n    host: example.com\n    user: alice"
			} else {
				def = name + ": ssh://alice@example.com:22"
			}
		case "http":
			if p["def"] == "runners-map" {
				def = name + ":\n    endpoint: https://example.com"
			} else {
				def = name + ": https://example.com"
			}
		}
		var call []string
		if p["kind"] == "ssh" {
			call = []string{name + ":", "  command: hostname"}
		} else {
			call = []string{name + ":", "  /health:", "    get:", "      body: null"}
		}
		var b strings.Builder
		b.WriteString("desc: d\n")
		if p["def"] != "step-runner" {
			b.WriteString("runners:\n  " + strings.ReplaceAll(def, "\n    ", "\n    ") + "\n")
		}
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
		filler := []string{"exec:", "  command: echo ok"}
		// A reserved runner name cannot be called from a step (the key would be the test keyword), so usage levels call nothing for it.
		callable := p["name"] == "normal"
		usage := p["usage"]
		if !callable {
			usage = "none"
		}
		if usage == "before" {
			writeStep(call)
		}
		if p["def"] == "step-runner" {
			writeStep([]string{"runner:", "  " + strings.ReplaceAll(def, "\n    ", "\n      ")})
		} else {
			writeStep(filler)
		}
		switch usage {
		case "after", "twice":
			writeStep(call)
			if usage == "twice" {
				writeStep(call)
			}
		case "inline-child":
			inner := []string{"include:", "  steps:"}
			for i, l := range call {
				if i == 0 {
					inner = append(inner, "    - "+l)
				} else {
					inner = append(inner, "      "+l)
				}
			}
			writeStep(inner)
		}
		writeStep([]string{"test: \"true\""})
		src := b.String()
		got := lint(t, src)
		found := findRule(got, "unused-runner")
		expect := callable && p["usage"] == "none"
		if expect {
			expected++
		} else {
			unexpected++
		}
		if !expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, src)
			}
			if p["name"] == "reserved" && !hasRule(got, "runner-reserved") {
				t.Fatalf("%s\nrunner-reserved missing: %+v\n%s", describe(p), got, src)
			}
			return
		}
		want := lineOf(t, src, "  "+name+":")
		if len(found) != 1 || found[0].Severity != finding.Warning || found[0].Line != want || !strings.Contains(found[0].Message, name) {
			t.Fatalf("%s\nwant one warning at line %d, got %+v\n%s", describe(p), want, found, src)
		}
	})
	if expected < 3 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
