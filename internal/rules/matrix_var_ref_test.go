package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Every pair of levels of the var-ref factors behaves as documented: a vars.* reference in any expression site (including a template in a
// mapping key and in include.vars) is reported unless the runbook defines it, var-ref's options.ignore lists it, or a caller read as a
// referrer passes it in include.vars (a dynamic include counts only for what it might pass). A runbook is not judged when a caller's
// include.vars cannot be read, or when its top-level if requires included and no fixed-path caller was found.
func TestVarRefPairwise(t *testing.T) {
	factors := []factor{
		{"site", []string{"test", "dump", "template-value", "template-key", "bind-value", "until", "include-vars-template"}},
		{"defined", []string{"none", "own-vars", "ignore-option"}},
		{"cond", []string{"none", "included"}},
		{"refs", []string{"with-refs", "single-file"}},
		{"caller", []string{"none", "static-passes", "static-not-passing", "static-unknown", "dynamic-passes", "dynamic-not-passing"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		var b strings.Builder
		b.WriteString("desc: target\n")
		if p["cond"] == "included" {
			b.WriteString("if: included\n")
		}
		if p["defined"] == "own-vars" {
			b.WriteString("vars:\n  secret: 1\n")
		}
		b.WriteString("runners:\n  req: https://example.com\nsteps:\n")
		switch p["site"] {
		case "test":
			b.WriteString("  - exec:\n      command: echo ok\n    test: vars.secret != null\n")
		case "dump":
			b.WriteString("  - dump: vars.secret\n")
		case "template-value":
			b.WriteString("  - exec:\n      command: \"echo {{ vars.secret }}\"\n")
		case "template-key":
			b.WriteString("  - req:\n      \"/x/{{ vars.secret }}\":\n        get:\n          body: null\n")
		case "bind-value":
			b.WriteString("  - exec:\n      command: echo ok\n    bind:\n      s: vars.secret\n  - dump: s\n")
		case "until":
			b.WriteString("  - exec:\n      command: echo ok\n    loop:\n      count: 2\n      until: vars.secret != null\n")
		case "include-vars-template":
			b.WriteString("  - include:\n      path: ../valid/http.yml\n      vars:\n        x: \"{{ vars.secret }}\"\n")
		}
		b.WriteString("  - test: \"true\"\n")
		target := b.String()

		var caller string
		switch p["caller"] {
		case "static-passes":
			caller = "desc: c\nsteps:\n  - include:\n      path: target.yml\n      vars:\n        secret: 1\n"
		case "static-not-passing":
			caller = "desc: c\nsteps:\n  - include: target.yml\n"
		case "static-unknown":
			caller = "desc: c\nvars:\n  all:\n    secret: 1\nsteps:\n  - include:\n      path: target.yml\n      vars: \"{{ vars.all }}\"\n"
		case "dynamic-passes":
			caller = "desc: c\nvars:\n  n: target\nsteps:\n  - include:\n      path: \"{{ vars.n }}.yml\"\n      vars:\n        secret: 1\n"
		case "dynamic-not-passing":
			caller = "desc: c\nvars:\n  n: target\nsteps:\n  - include:\n      path: \"{{ vars.n }}.yml\"\n"
		}

		dir := t.TempDir()
		targetPath := filepath.Join(dir, "target.yml")
		if err := os.WriteFile(targetPath, []byte(target), 0o644); err != nil {
			t.Fatal(err)
		}
		if caller != "" {
			if err := os.WriteFile(filepath.Join(dir, "caller.yml"), []byte(caller), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cfg := defaultConfig(t)
		if p["defined"] == "ignore-option" {
			for i := range cfg.Rules {
				if cfg.Rules[i].ID == "var-ref" {
					cfg.Rules[i].Options = map[string]any{"ignore": []any{"secret"}}
				}
			}
		}
		var refs *Refs
		if p["refs"] == "with-refs" {
			refs = refsFor(t, []string{dir}, dir)
		}
		got, is := LintWith(targetPath, []byte(target), cfg, refs)
		if !is {
			t.Fatalf("not a runbook:\n%s", target)
		}
		found := findRule(got, "var-ref")

		withRefs := p["refs"] == "with-refs"
		c := p["caller"]
		staticFound := withRefs && strings.HasPrefix(c, "static-")
		unknown := (withRefs && c == "static-unknown") || (p["cond"] == "included" && !staticFound)
		defined := p["defined"] != "none" || (withRefs && (c == "static-passes" || c == "dynamic-passes"))
		expect := !unknown && !defined
		if expect {
			expected++
		} else {
			unexpected++
		}
		if !expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, target)
			}
			return
		}
		want := lineOf(t, target, "vars.secret")
		if len(found) != 1 || found[0].Severity != finding.Warning || found[0].Line != want || !strings.Contains(found[0].Message, "vars.secret") {
			t.Fatalf("%s\nwant one warning at line %d, got %+v\n%s", describe(p), want, found, target)
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
