package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

func unusedBindFindings(t *testing.T, path string, refs *Refs) []finding.Finding {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := LintWith(path, src, defaultConfig(t), refs)
	return findRule(got, "unused-bind")
}

// When a directory is scanned, a bind that neither this file nor any caller reads reports unused-bind as warning on the bind key's line.
// A caller reads an included runbook's bind as steps.<n>.<name>, also as a bare expression in include.vars, and a needed runbook's bind as needs.<name>.<bind>.
func TestUnusedBindAcrossFiles(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join(moduleRoot(t), "internal", "rules", "testdata", "unusedbind"))
	if err != nil {
		t.Fatal(err)
	}
	refs := refsFor(t, []string{dir}, dir)
	cases := []struct {
		file  string
		names []string
		lines []int
	}{
		{"child.yml", []string{"orphan"}, []int{10}},
		{"needed.yml", []string{"crumbs"}, []int{8}},
		{"toplevel.yml", []string{"unused"}, []int{8}},
		{"dyn.yml", []string{"other"}, []int{9}},
		{"solo-included.yml", nil, nil},
		{"parent.yml", nil, nil},
		{"dynamic-parent.yml", nil, nil},
	}
	for _, tc := range cases {
		got := unusedBindFindings(t, filepath.Join(dir, tc.file), refs)
		if len(got) != len(tc.names) {
			t.Fatalf("%s: %+v", tc.file, got)
		}
		for i, f := range got {
			if f.Severity != finding.Warning || f.Line != tc.lines[i] || !strings.Contains(f.Message, tc.names[i]) {
				t.Fatalf("%s: %+v", tc.file, f)
			}
		}
	}
}

// Linting a file alone, without reading callers, skips a runbook with if: included and still reports a runbook that is not included.
func TestUnusedBindWithoutCallers(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join(moduleRoot(t), "internal", "rules", "testdata", "unusedbind"))
	if err != nil {
		t.Fatal(err)
	}
	if got := unusedBindFindings(t, filepath.Join(dir, "child.yml"), nil); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	if got := unusedBindFindings(t, filepath.Join(dir, "toplevel.yml"), nil); len(got) != 1 {
		t.Fatalf("%+v", got)
	}
}

// A bind inside an inline include that the same file reads is used. A file with an expression that does not parse is not judged.
func TestUnusedBindInlineAndBrokenExpression(t *testing.T) {
	inline := lint(t, `
desc: inline
steps:
  - include:
      steps:
        - exec:
            command: echo ok
          bind:
            inner: current.stdout
  - test: steps[0].steps[0].inner != ""
`)
	if hasRule(inline, "unused-bind") {
		t.Fatalf("%+v", inline)
	}
	broken := lint(t, `
desc: broken
steps:
  - exec:
      command: echo ok
    bind:
      maybe: current.stdout
  - test: "maybe =="
`)
	if hasRule(broken, "unused-bind") || !hasRule(broken, "expr-syntax") {
		t.Fatalf("%+v", broken)
	}
}

// runn expands {{ }} in mapping keys too, such as an HTTP path or a body field name. A bind read only from a key is used,
// a variable referenced only from a key is still checked by var-ref, and a template in a key that does not parse reports expr-syntax on the key's line.
func TestTemplatesInMappingKeys(t *testing.T) {
	got := lint(t, `
desc: key template
runners:
  req: https://example.com
steps:
  - exec:
      command: echo 11
    bind:
      items: current.stdout
      field: current.stdout
  - req:
      /documents?filters={{ items }}&page={{ vars.missing }}:
        post:
          body:
            application/json:
              "{{ field }}": 1
              "{{ 1 + }}": 2
  - test: steps[1].res.status == 400
`)
	if hasRule(got, "unused-bind") {
		t.Fatalf("%+v", got)
	}
	if msgs := messagesOf(got, "var-ref"); len(msgs) != 1 || !strings.Contains(msgs[0], "vars.missing") {
		t.Fatalf("%+v", got)
	}
	syntax := findRule(got, "expr-syntax")
	if len(syntax) != 1 || syntax[0].Line != 17 {
		t.Fatalf("%+v", got)
	}
}
