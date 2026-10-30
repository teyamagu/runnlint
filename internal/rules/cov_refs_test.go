package rules

// Lines that are not covered on purpose:
// refs.go stepVisible returns false for a target outside the steps. Callers check the index or look the name up first, so the branch is reached only by a direct call, which TestStepVisibleOutOfRange makes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
)

// With map steps, a name that is not a step, a name used from the top-level if, a name of a later step, and an index out of range each report step-ref with their own sentence.
func TestStepRefMapStepsBranches(t *testing.T) {
	cases := []struct {
		name string
		src  string
		part string
	}{
		{"missing name", "desc: d\nsteps:\n  run:\n    exec:\n      command: echo ok\n  check:\n    test: steps.nope.exit_code == 0\n", "steps.nope はこの runbook にありません"},
		{"top-level if", "desc: d\nif: steps.run.exit_code == 0\nsteps:\n  run:\n    exec:\n      command: echo ok\n  check:\n    test: \"true\"\n", "steps は step の実行中に参照します"},
		{"later step", "desc: d\nsteps:\n  check:\n    test: steps.run.exit_code == 0\n  run:\n    exec:\n      command: echo ok\n", "steps.run はまだ実行されていません"},
		{"index out of range", "desc: d\nsteps:\n  run:\n    exec:\n      command: echo ok\n  check:\n    test: steps[9].exit_code == 0\n", "steps は map なので steps.名前 の形で参照します"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireMessage(t, tc.src, "step-ref", tc.part)
		})
	}
}

// stepVisible is false for a target outside the steps.
func TestStepVisibleOutOfRange(t *testing.T) {
	b := &runbook.Book{Steps: []runbook.Step{{Index: 0}, {Index: 1}}}
	if stepVisible(b, &b.Steps[1], 5) || stepVisible(b, &b.Steps[1], -1) {
		t.Fatal("a target outside the steps is visible")
	}
}

// A vars name that contains an environment placeholder is not judged by var-ref. includeVars without a document finds no caller.
func TestVarRefEnvPlaceholderNameAndNoDoc(t *testing.T) {
	got := lint(t, "desc: d\nvars:\n  a: 1\nsteps:\n  - dump: vars.${NAME}\n  - test: \"true\"\n")
	if hasRule(got, "var-ref") {
		t.Fatalf("%+v", got)
	}
	out := includeVars(&Context{})
	if out.included || out.unknown || out.maybe || len(out.names) != 0 {
		t.Fatalf("%+v", out)
	}
}

// When a caller passes include.vars as something other than a mapping, the passed variables are unknown, so var-ref does not report the included runbook.
func TestVarRefUnknownIncludeVars(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("parent.yml", "desc: p\nvars:\n  v:\n    token: a\nsteps:\n  - include:\n      path: child.yml\n      vars: \"{{ vars.v }}\"\n  - test: \"true\"\n")
	child := write("child.yml", "desc: c\nsteps:\n  - dump: vars.token\n  - test: \"true\"\n")
	refs := refsFor(t, []string{dir}, dir)
	if got := varRefMessages(t, child, refs); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

// Inside an inline include, parent.vars.<name> is not reported when name is in var-ref's ignore, and is reported otherwise.
func TestParentVarsIgnore(t *testing.T) {
	src := "desc: d\nsteps:\n  - include:\n      steps:\n        - dump: parent.vars.token\n  - test: \"true\"\n"
	cfg := defaultConfig(t)
	got, _ := Lint("book.yml", []byte(src), cfg)
	if msgs := messagesOf(got, "var-ref"); len(msgs) != 1 || !strings.Contains(msgs[0], "token") {
		t.Fatalf("%+v", got)
	}
	for i := range cfg.Rules {
		if cfg.Rules[i].ID == "var-ref" {
			cfg.Rules[i].Options = map[string]any{"ignore": []any{"token"}}
		}
	}
	got, _ = Lint("book.yml", []byte(src), cfg)
	if hasRule(got, "var-ref") {
		t.Fatalf("%+v", got)
	}
}
