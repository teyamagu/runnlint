package rules

import "testing"

// The three rules in unused.go do nothing without a document.
func TestUnusedRulesWithoutDocument(t *testing.T) {
	for _, fn := range []Builtin{unusedRunner, requireTestStep, unusedBind} {
		c := &Context{}
		fn(c)
		if len(c.out) != 0 {
			t.Fatalf("%+v", c.out)
		}
	}
}

// include.vars values decide whether a bind is read: a bare expression in an include runner's vars counts, a template is left to the template sites,
// a value that does not parse as an expression is ignored, and strings inside a list are read too.
func TestUnusedBindIncludeVarsForms(t *testing.T) {
	const head = "desc: d\nsteps:\n  - exec:\n      command: echo ok\n    bind:\n      token: current.stdout\n"
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"include runner vars", "desc: d\nrunners:\n  child:\n    path: other.yml\n    vars:\n      x: token\nsteps:\n  - exec:\n      command: echo ok\n    bind:\n      token: current.stdout\n  - test: \"true\"\n", false},
		{"template in include vars", head + "  - include:\n      path: other.yml\n      vars:\n        x: \"{{ token }}\"\n  - test: \"true\"\n", false},
		{"unparsable include vars", head + "  - include:\n      path: other.yml\n      vars:\n        x: hello world\n  - test: \"true\"\n", true},
		{"list in include vars", head + "  - include:\n      path: other.yml\n      vars:\n        xs:\n          - token\n  - test: \"true\"\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lint(t, tc.src)
			found := findRule(got, "unused-bind")
			if tc.want && (len(found) != 1 || found[0].Message != "bind token はどこからも参照されていません") {
				t.Fatalf("%+v", got)
			}
			if !tc.want && len(found) != 0 {
				t.Fatalf("%+v", got)
			}
		})
	}
}
