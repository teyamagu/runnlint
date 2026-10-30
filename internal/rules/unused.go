package rules

import (
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
)

func init() {
	register("unused-runner", unusedRunner)
	register("require-test-step", requireTestStep)
}

// unusedRunner reports a runner in runners: or in a step's runner: that no step of the book, including its inline includes, calls.
// Runners with a reserved name are left to runner-reserved.
func unusedRunner(c *Context) {
	for _, b := range c.Books() {
		used := map[string]struct{}{}
		collectCalledRunners(b, used)
		for _, r := range b.Runners {
			if reservedRunner(r.Name) {
				continue
			}
			if _, ok := used[r.Name]; !ok {
				c.Add(r.NameNode, message.RunnerUnused, r.Name)
			}
		}
		for _, step := range b.Steps {
			def, ok := step.Value.Get("runner")
			if !ok || def.Kind != runbook.KindMap {
				continue
			}
			for _, e := range def.Map {
				if reservedRunner(e.Key) {
					continue
				}
				if _, ok := used[e.Key]; !ok {
					c.Add(e.KeyNode, message.RunnerUnused, e.Key)
				}
			}
		}
	}
}

// collectCalledRunners adds the runner names called by the steps of b and of its inline includes.
// A called runner is any step key that is not a step keyword, the same reading runner-defined uses.
func collectCalledRunners(b *runbook.Book, used map[string]struct{}) {
	b.Walk(func(book *runbook.Book) {
		for _, step := range book.Steps {
			for _, e := range step.Value.Map {
				if isStepKeyword(e.Key) {
					continue
				}
				used[e.Key] = struct{}{}
			}
		}
	})
}

// requireTestStep reports a runbook that can run on its own but has no step with only test.
// A step that attaches test to a request checks one response. A step with only test names what the scenario verifies.
// Runbooks whose top-level if is true only when included are verified by their caller. Runbooks without steps are left to require-steps.
func requireTestStep(c *Context) {
	if c.Doc == nil || c.Doc.Book == nil {
		return
	}
	b := c.Doc.Book
	if len(b.Steps) == 0 {
		return
	}
	if cond, ok := b.Root.Get("if"); ok && cond.Kind == runbook.KindString && requiresIncluded(cond.String) {
		return
	}
	found := false
	b.Walk(func(book *runbook.Book) {
		for _, step := range book.Steps {
			if isTestOnly(step) {
				found = true
			}
		}
	})
	if found {
		return
	}
	node := b.Root.Node
	for _, e := range b.Root.Map {
		if e.Key == "steps" {
			node = e.KeyNode
		}
	}
	c.Add(node, message.TestStepMissing)
}

// isTestOnly reports whether a step has test and nothing that produces a result: no runner call, include, exec, or runner:.
func isTestOnly(step runbook.Step) bool {
	hasTest := false
	for _, e := range step.Value.Map {
		switch e.Key {
		case "test":
			hasTest = true
		case "desc", "if", "loop", "defer", "force", "dump", "bind":
			continue
		default:
			return false
		}
	}
	return hasTest
}
