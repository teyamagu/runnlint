package rules

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
	"gopkg.in/yaml.v3"
)

// book-shape reports needs that is not a mapping, a secrets element that is not a string, and in a loop mapping an unknown key, a negative count, and a jitter that is not a number. A loop given as an expression is accepted.
func TestBookShapeRemainingBranches(t *testing.T) {
	cases := []struct {
		name string
		top  string
		part string
	}{
		{"needs not map", "needs: 1\n", "needs は mapping です"},
		{"secrets elem", "secrets:\n  - 1\n", "secrets の要素は空でない文字列です"},
		{"loop unknown key", "loop:\n  count: 1\n  nope: 1\n", "loop にキー nope はありません"},
		{"loop negative count", "loop:\n  count: -1\n", "loop.count は 0 以上です"},
		{"loop jitter", "loop:\n  count: 1\n  jitter: true\n", "loop.jitter は数値です"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireMessage(t, "desc: d\n"+tc.top+"steps:\n  - test: \"true\"\n", "book-shape", tc.part)
		})
	}
	rejectMessage(t, "desc: d\nloop: \"{{ vars.n }}\"\nvars:\n  n: 2\nsteps:\n  - test: \"true\"\n", "book-shape", "loop")
}

// duplicateKey does nothing without a document. nodeOf falls back to the step name's node, then to the step value's node, when the field has no node.
func TestDuplicateKeyNilDocAndNodeOf(t *testing.T) {
	c := &Context{}
	duplicateKey(c)
	if len(c.out) != 0 {
		t.Fatalf("%+v", c.out)
	}
	name := &yaml.Node{Line: 3}
	value := &yaml.Node{Line: 4}
	field := &yaml.Node{Line: 5}
	step := runbook.Step{NameNode: name, Value: runbook.Value{Node: value}}
	if nodeOf(runbook.Value{Node: field}, step) != field {
		t.Fatal("field node is not preferred")
	}
	if nodeOf(runbook.Value{}, step) != name {
		t.Fatal("step name node is not used")
	}
	step.NameNode = nil
	if nodeOf(runbook.Value{}, step) != value {
		t.Fatal("step value node is not used")
	}
}

// A concurrency element that is not a non-empty string reports book-shape.
func TestConcurrencyElement(t *testing.T) {
	requireMessage(t, "desc: d\nconcurrency:\n  - 1\nsteps:\n  - test: \"true\"\n", "book-shape", "concurrency の要素は空でない文字列です")
}
