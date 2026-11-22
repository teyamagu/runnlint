package runbook

import (
	"fmt"
	"testing"
)

// TestRunnersAtMatchesWalk checks the cached runner map against the original walk.
func TestRunnersAtMatchesWalk(t *testing.T) {
	sources := []string{
		"steps:\n  - test: \"true\"\n",
		"runners:\n  req: https://example.com\nsteps:\n  - req:\n      /a:\n        get:\n          body: null\n",
		"steps:\n  - runner:\n      sc: ssh://u@h\n  - sc:\n      command: ls\n  - runner:\n      db: mysql://h/db\n  - db:\n      query: select 1\n",
		"steps:\n  one:\n    runner:\n      sc: ssh://u@h\n  two:\n    sc:\n      command: ls\n",
		"steps:\n  - 1\n  - runner:\n      sc: ssh://u@h\n  - sc:\n      command: ls\n",
		"steps:\n  - include:\n      path: child.yml\n      steps:\n        - runner:\n            inner: https://example.com\n        - inner:\n            /a:\n              get:\n                body: null\n",
	}
	for _, src := range sources {
		doc, err := Parse([]byte(src))
		if err != nil || doc.Book == nil {
			t.Fatalf("parse: %v\n%s", err, src)
		}
		doc.Book.Walk(func(b *Book) {
			for stepIndex := 0; stepIndex <= len(b.Steps)+1; stepIndex++ {
				if diff := runnerDiff(b.collectRunners(stepIndex), b.RunnersAt(stepIndex)); diff != "" {
					t.Fatalf("index %d: %s\n%s", stepIndex, diff, src)
				}
			}
			for _, s := range b.Steps {
				got, gok := b.StepByName(s.Name)
				want, wok := stepByNameWalk(b, s.Name)
				if gok != wok || got.Index != want.Index || got.Name != want.Name {
					t.Fatalf("name %q got %+v %v want %+v %v", s.Name, got, gok, want, wok)
				}
			}
			if _, ok := b.StepByName("missing"); ok {
				t.Fatal("missing name found")
			}
		})
	}
}

func stepByNameWalk(b *Book, name string) (Step, bool) {
	for _, s := range b.Steps {
		if s.Name == name {
			return s, true
		}
	}
	return Step{}, false
}

func runnerDiff(want, got map[string]Runner) string {
	if len(want) != len(got) {
		return fmt.Sprintf("len %d want %d", len(got), len(want))
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok || g.Kind != w.Kind || g.FromStep != w.FromStep || g.Name != w.Name {
			return fmt.Sprintf("%s got %+v want %+v", name, g, w)
		}
	}
	return ""
}
