package rules

import (
	"strconv"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Every pair of levels of the step-ref factors behaves as documented: an index reference on map steps and a name reference on list steps
// are errors; a target that does not exist is an error; a reference from the top-level if is an error while a top-level loop.until is
// allowed; from a step, the target must be itself or one that has already run, where a defer step runs after ordinary steps and later
// defer steps run before earlier ones; the diagnostic sits on the referencing field's line.
func TestStepRefPairwise(t *testing.T) {
	factors := []factor{
		{"style", []string{"list", "map"}},
		{"form", []string{"index", "name"}},
		{"target", []string{"self", "earlier", "later", "missing"}},
		{"site", []string{"test", "template-value", "top-if", "top-until"}},
		{"defer", []string{"none", "current", "target", "both"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		mapStyle := p["style"] == "map"
		names := []string{"a", "b", "c"}
		targetIndex := map[string]int{"self": 1, "earlier": 0, "later": 2, "missing": 9}[p["target"]]
		var ref string
		if p["form"] == "index" {
			ref = "steps[" + strconv.Itoa(targetIndex) + "]"
		} else if targetIndex == 9 {
			ref = "steps.nope"
		} else {
			ref = "steps." + names[targetIndex]
		}
		expr := ref + ".exit_code == 0"
		currentDefer := p["defer"] == "current" || p["defer"] == "both"
		targetDefer := p["defer"] == "target" || p["defer"] == "both"
		if p["target"] == "self" && targetDefer {
			currentDefer = true
		}
		var b strings.Builder
		b.WriteString("desc: d\n")
		if p["site"] == "top-if" {
			b.WriteString("if: " + expr + "\n")
		}
		if p["site"] == "top-until" {
			b.WriteString("loop:\n  count: 2\n  until: " + expr + "\n")
		}
		b.WriteString("steps:\n")
		for i, name := range names {
			lines := []string{"exec:", "  command: echo " + name}
			deferred := (i == 1 && currentDefer) || (i == targetIndex && i != 1 && targetDefer)
			if deferred {
				lines = append(lines, "defer: true")
			}
			if i == 1 {
				switch p["site"] {
				case "test":
					lines = append(lines, "test: "+expr+" #SITE")
				case "template-value":
					lines = []string{"exec:", "  command: \"echo {{ " + ref + ".exit_code }}\" #SITE"}
					if deferred {
						lines = append(lines, "defer: true")
					}
				}
			}
			if mapStyle {
				b.WriteString("  " + name + ":\n")
				for _, l := range lines {
					b.WriteString("    " + l + "\n")
				}
			} else {
				for j, l := range lines {
					if j == 0 {
						b.WriteString("  - " + l + "\n")
					} else {
						b.WriteString("    " + l + "\n")
					}
				}
			}
		}
		src := b.String()
		got := lint(t, src)
		found := findRule(got, "step-ref")

		var want string
		switch {
		case p["form"] == "index" && mapStyle:
			want = "steps is a map"
		case p["form"] == "name" && !mapStyle:
			want = "steps is an array"
		case p["target"] == "missing":
			want = "is not in this runbook"
		case p["site"] == "top-if":
			want = "while a step is running"
		case p["site"] == "top-until":
			want = ""
		case p["target"] == "self":
			want = ""
		default:
			visible := false
			if !currentDefer {
				visible = !targetDefer && targetIndex < 1
			} else if !targetDefer {
				visible = true
			} else {
				visible = targetIndex > 1
			}
			if !visible {
				want = "has not run yet"
			}
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
		var line int
		switch p["site"] {
		case "top-if":
			line = lineOf(t, src, "if: ")
		case "top-until":
			line = lineOf(t, src, "until: ")
		default:
			line = lineOf(t, src, "#SITE")
		}
		if len(found) != 1 || found[0].Severity != finding.Error || found[0].Line != line || !strings.Contains(found[0].Match, want) {
			t.Fatalf("%s\nwant %q at line %d, got %+v\n%s", describe(p), want, line, found, src)
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
