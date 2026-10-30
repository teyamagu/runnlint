package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// unusedBindCase turns one row of the unused-bind matrix into a target runbook, an optional caller runbook, and the expected outcome.
type unusedBindCase struct {
	target   string
	caller   string
	withRefs bool
	expect   bool
}

func buildUnusedBindCase(p pick) unusedBindCase {
	mapStyle := p["steps"] == "map"

	var t strings.Builder
	t.WriteString("desc: target\n")
	if c := p["cond"]; c != "" {
		t.WriteString("if: '" + c + "'\n")
	}
	t.WriteString("vars:\n  flag: true\n  k: token\n")
	t.WriteString("runners:\n  req: https://example.com\n")
	t.WriteString("steps:\n")

	command := "echo ok"
	if p["local"] == "template-value" {
		command = "\"echo {{ token }}\""
	}
	var binds []string
	switch p["key"] {
	case "plain":
		binds = []string{"token: current.stdout"}
	case "plain+path":
		binds = []string{"token: current.stdout", "token.sub: current.stdout"}
	case "plain+index":
		binds = []string{"token: current.stdout", "token[0]: current.stdout"}
	case "template":
		binds = []string{"\"{{ vars.k }}\": current.stdout"}
	case "hyphen":
		binds = []string{"foo-bar: current.stdout"}
	}
	first := []string{"exec:", "  command: " + command, "bind:"}
	for _, b := range binds {
		first = append(first, "  "+b)
	}
	writeStep := func(name string, lines []string) {
		if mapStyle {
			t.WriteString("  " + name + ":\n")
			for _, l := range lines {
				t.WriteString("    " + l + "\n")
			}
			return
		}
		for i, l := range lines {
			if i == 0 {
				t.WriteString("  - " + l + "\n")
			} else {
				t.WriteString("    " + l + "\n")
			}
		}
	}
	writeStep("run", first)

	switch p["local"] {
	case "none", "template-value":
	case "test":
		writeStep("ref", []string{"test: token != \"\""})
	case "if":
		writeStep("ref", []string{"dump: 1", "if: token != \"\""})
	case "dump":
		writeStep("ref", []string{"dump: token"})
	case "dump-expr":
		writeStep("ref", []string{"dump:", "  expr: token"})
	case "bind-value":
		writeStep("ref", []string{"exec:", "  command: echo again", "bind:", "  token: upper(token)"})
	case "until":
		writeStep("ref", []string{"exec:", "  command: echo again", "loop:", "  count: 2", "  until: token != \"\""})
	case "template-key":
		writeStep("ref", []string{"req:", "  /x/{{ token }}:", "    get:", "      body: null"})
	case "inline-child":
		writeStep("ref", []string{"include:", "  steps:", "    - test: token != \"\""})
	case "previous-member":
		writeStep("ref", []string{"test: previous.token != null"})
	case "broken":
		writeStep("ref", []string{"test: \"token ==\""})
	}
	target := t.String()

	var caller string
	switch p["caller"] {
	case "none":
	case "static-none":
		caller = "desc: c\nsteps:\n  - include: target.yml\n"
	case "static-member":
		caller = "desc: c\nsteps:\n  - include: target.yml\n  - test: steps[0].token != null\n"
	case "static-previous":
		caller = "desc: c\nsteps:\n  - include: target.yml\n  - test: previous.token != null\n"
	case "static-include-vars":
		caller = "desc: c\nsteps:\n  - include: target.yml\n  - include:\n      path: target.yml\n      vars:\n        x: steps[0].token\n"
	case "static-template":
		caller = "desc: c\nsteps:\n  - include: target.yml\n  - exec:\n      command: \"echo {{ steps[0].token }}\"\n"
	case "static-broken":
		caller = "desc: c\nsteps:\n  - include: target.yml\n  - test: \"1 +\"\n"
	case "dynamic-ref":
		caller = "desc: c\nvars:\n  n: target\nsteps:\n  - include:\n      path: \"{{ vars.n }}.yml\"\n  - test: steps[0].token != null\n"
	case "dynamic-none":
		caller = "desc: c\nvars:\n  n: target\nsteps:\n  - include:\n      path: \"{{ vars.n }}.yml\"\n"
	case "needs-ref":
		caller = "desc: c\nneeds:\n  t: target.yml\nsteps:\n  - dump: needs.t.token\n"
	case "needs-none":
		caller = "desc: c\nneeds:\n  t: target.yml\nsteps:\n  - exec:\n      command: echo\n"
	}

	// The oracle follows the documented contract, not the implementation.
	hasDef := strings.HasPrefix(p["key"], "plain")
	localUsed := p["key"] == "plain+path" || p["key"] == "plain+index" || (p["local"] != "none" && p["local"] != "broken")
	localUnknown := p["local"] == "broken"
	withRefs := p["refs"] == "with-refs"
	c := p["caller"]
	found := withRefs && (strings.HasPrefix(c, "static-") || strings.HasPrefix(c, "needs-"))
	outerUnknown := withRefs && c == "static-broken"
	outerUsed := withRefs && (c == "static-member" || c == "static-previous" || c == "static-include-vars" || c == "static-template" || c == "dynamic-ref" || c == "needs-ref")
	requires := p["cond"] == "included" || p["cond"] == "included == true" || p["cond"] == "included && vars.flag"
	expect := hasDef && !localUnknown && !outerUnknown && !localUsed && !outerUsed && !(!found && requires)
	return unusedBindCase{target: target, caller: caller, withRefs: withRefs, expect: expect}
}

// Every pair of levels of the unused-bind factors behaves as documented: a bind is reported only when it has a plain name, nothing in the
// file reads it (expressions, templates in values and keys, inline includes, path-form bind keys), no caller read as a referrer reads it,
// neither side has an unparsable expression, and the runbook is not an include-only part whose callers were not read.
func TestUnusedBindPairwise(t *testing.T) {
	factors := []factor{
		{"cond", []string{"", "included", "included == true", "included && vars.flag", "included || vars.flag", "vars.flag"}},
		{"key", []string{"plain", "plain+path", "plain+index", "template", "hyphen"}},
		{"local", []string{"none", "test", "if", "dump", "dump-expr", "bind-value", "until", "template-value", "template-key", "inline-child", "previous-member", "broken"}},
		{"caller", []string{"none", "static-none", "static-member", "static-previous", "static-include-vars", "static-template", "static-broken", "dynamic-ref", "dynamic-none", "needs-ref", "needs-none"}},
		{"refs", []string{"with-refs", "single-file"}},
		{"steps", []string{"list", "map"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		c := buildUnusedBindCase(p)
		if c.expect {
			expected++
		} else {
			unexpected++
		}
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "target.yml")
		if err := os.WriteFile(targetPath, []byte(c.target), 0o644); err != nil {
			t.Fatal(err)
		}
		if c.caller != "" {
			if err := os.WriteFile(filepath.Join(dir, "caller.yml"), []byte(c.caller), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		var refs *Refs
		if c.withRefs {
			refs = refsFor(t, []string{dir}, dir)
		}
		got, is := LintWith(targetPath, []byte(c.target), defaultConfig(t), refs)
		if !is {
			t.Fatalf("not a runbook:\n%s", c.target)
		}
		found := findRule(got, "unused-bind")
		if !c.expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, c.target)
			}
			return
		}
		want := lineOf(t, c.target, "token: current.stdout")
		if len(found) != 1 || found[0].Severity != finding.Warning || found[0].Line != want || !strings.Contains(found[0].Message, "token") {
			t.Fatalf("%s\nwant one warning at line %d, got %+v\n%s", describe(p), want, found, c.target)
		}
	})
	// A report needs every factor at a non-reading level, so only the base-choice rows and a few pairwise rows expect one.
	if expected < 5 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
