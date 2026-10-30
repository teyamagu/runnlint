package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Every pair of levels of the included-unreferenced factors behaves as documented: a runbook whose top-level if is true only when included
// (included, included == true in either order, or joined with &&) is reported on the if line when it is inside the judged scope and no other
// file includes it by a string path, a map path, an include runner, or a dynamic path that can match it. A self-include, a needs entry, a
// dynamic path that cannot match, and conditions that can hold on their own (||, !, a template, no if) do not count.
func TestIncludedUnreferencedPairwise(t *testing.T) {
	factors := []factor{
		{"cond", []string{"included", "included == true", "true == included", "included && vars.flag", "vars.flag && included", "included || vars.flag", "!included", "none", "template"}},
		{"referrer", []string{"none", "static-string", "static-map-path", "runner-include", "dynamic-match", "dynamic-nomatch", "self-include", "needs-only"}},
		{"scope", []string{"in-scope", "out-of-scope", "nil-refs"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		target := "desc: target\n"
		switch p["cond"] {
		case "none":
		case "template":
			target += "if: \"{{ included }}\"\n"
		default:
			target += "if: '" + p["cond"] + "'\n"
		}
		target += "vars:\n  flag: true\nsteps:\n"
		if p["referrer"] == "self-include" {
			target += "  - include: target.yml\n"
		} else {
			target += "  - exec:\n      command: echo ok\n"
		}
		target += "  - test: \"true\"\n"

		var caller string
		switch p["referrer"] {
		case "static-string":
			caller = "desc: c\nsteps:\n  - include: target.yml\n"
		case "static-map-path":
			caller = "desc: c\nsteps:\n  - include:\n      path: target.yml\n"
		case "runner-include":
			caller = "desc: c\nrunners:\n  inc:\n    path: target.yml\nsteps:\n  - inc:\n      vars: {}\n"
		case "dynamic-match":
			caller = "desc: c\nvars:\n  n: target\nsteps:\n  - include:\n      path: \"{{ vars.n }}.yml\"\n"
		case "dynamic-nomatch":
			// A dynamic path matches every file that ends with the text after its last dynamic part, so only a different suffix fails to match.
			caller = "desc: c\nvars:\n  n: target\nsteps:\n  - include:\n      path: \"{{ vars.n }}.yaml\"\n"
		case "needs-only":
			caller = "desc: c\nneeds:\n  t: target.yml\nsteps:\n  - exec:\n      command: echo ok\n"
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
		var refs *Refs
		switch p["scope"] {
		case "in-scope":
			refs = refsFor(t, []string{dir}, dir)
		case "out-of-scope":
			refs = refsFor(t, nil, dir)
		}
		got, is := LintWith(targetPath, []byte(target), defaultConfig(t), refs)
		if !is {
			t.Fatalf("not a runbook:\n%s", target)
		}
		found := findRule(got, "included-unreferenced")

		requires := map[string]bool{"included": true, "included == true": true, "true == included": true, "included && vars.flag": true, "vars.flag && included": true}[p["cond"]]
		referenced := map[string]bool{"static-string": true, "static-map-path": true, "runner-include": true, "dynamic-match": true}[p["referrer"]]
		expect := p["scope"] == "in-scope" && requires && !referenced
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
		want := lineOf(t, target, "if:")
		if len(found) != 1 || found[0].Severity != finding.Warning || found[0].Line != want {
			t.Fatalf("%s\nwant one warning at line %d, got %+v\n%s", describe(p), want, found, target)
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
