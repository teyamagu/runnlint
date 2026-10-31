package rules

import (
	"regexp"
	"strings"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
	"gopkg.in/yaml.v3"
)

func init() {
	register("unused-runner", unusedRunner)
	register("require-test-step", requireTestStep)
	register("unused-bind", unusedBind)
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
// The default config leaves this rule off. A project enables it by setting severity in .runnlint-runn.yaml.
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

var bindIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var bindRootRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)[.\[]`)

// unusedBind reports a bind name that no expression in this file reads, and that no file including or needing this runbook reads either.
// A caller reads an included runbook's binds as steps.<n>.<name> or previous.<name>, and a needed runbook's binds as needs.<name>.<bind>, so any
// identifier or member name in a caller counts. A runbook with if: included that no file includes or needs by a fixed path is not judged, because its
// callers may be outside the scanned files. Neither is a file whose expressions do not all parse. A bind key such as a.b or a[0] assigns into an existing bind, so it is a reference to a, not a new bind.
func unusedBind(c *Context) {
	if c.Doc == nil || c.Doc.Book == nil {
		return
	}
	root := c.Doc.Book
	local, ok := bookNames(root, c.Sites)
	if !ok {
		return
	}
	type def struct {
		name string
		node *yaml.Node
	}
	var defs []def
	root.Walk(func(b *runbook.Book) {
		for _, step := range b.Steps {
			bind, ok := step.Value.Get("bind")
			if !ok || bind.Kind != runbook.KindMap {
				continue
			}
			for _, e := range bind.Map {
				switch {
				case bindIdentRe.MatchString(e.Key):
					defs = append(defs, def{name: e.Key, node: e.KeyNode})
				case bindRootRe.MatchString(e.Key):
					local[bindRootRe.FindStringSubmatch(e.Key)[1]] = struct{}{}
				}
			}
		}
	})
	if len(defs) == 0 {
		return
	}
	outer, found, unknown := c.Refs.ReferrerNames(root.File)
	if unknown {
		return
	}
	if !found {
		if cond, ok := root.Root.Get("if"); ok && cond.Kind == runbook.KindString && requiresIncluded(cond.String) {
			// Runs only when included, but no caller was read. Who reads the binds is unknown.
			return
		}
	}
	for _, d := range defs {
		if _, ok := local[d.name]; ok {
			continue
		}
		if _, ok := outer[d.name]; ok {
			continue
		}
		c.Add(d.node, message.BindUnused, d.name)
	}
}

// bookNames collects the names that book references: those in its parsed expression sites, plus those in include.vars values.
// runn evaluates a string in include.vars as an expression in the caller's store and keeps it as a string when that fails,
// so a value that parses as an expression is read for names and one that does not is skipped. Templates in include.vars are already sites.
func bookNames(book *runbook.Book, sites []site) (map[string]struct{}, bool) {
	names, ok := exprNames(sites)
	book.Walk(func(b *runbook.Book) {
		for _, step := range b.Steps {
			inc, found := step.Value.Get("include")
			if !found || inc.Kind != runbook.KindMap {
				continue
			}
			if _, inline := inc.Get("steps"); inline {
				continue
			}
			if vars, found := inc.Get("vars"); found {
				includeVarNames(vars, names)
			}
		}
		for _, r := range b.Runners {
			if r.Kind != runbook.KindInclude || r.Value.Kind != runbook.KindMap {
				continue
			}
			if vars, found := r.Value.Get("vars"); found {
				includeVarNames(vars, names)
			}
		}
	})
	return names, ok
}

func includeVarNames(v runbook.Value, names map[string]struct{}) {
	switch v.Kind {
	case runbook.KindString:
		src := expandEnvPlaceholders(v.String)
		if strings.Contains(src, "{{") {
			return
		}
		tree, err := parser.Parse(strings.TrimSpace(src))
		if err != nil {
			return
		}
		node := tree.Node
		nv := nameVisitor{names: names}
		ast.Walk(&node, &nv)
	case runbook.KindSeq:
		for _, el := range v.Seq {
			includeVarNames(el, names)
		}
	case runbook.KindMap:
		for _, e := range v.Map {
			includeVarNames(e.Value, names)
		}
	}
}

// exprNames collects every identifier and member name that the parsed sites reference.
// ok is false when a site has an expression that does not parse, because its references are unknown.
func exprNames(sites []site) (map[string]struct{}, bool) {
	names := map[string]struct{}{}
	ok := true
	for _, s := range sites {
		if s.err != nil {
			ok = false
			continue
		}
		if s.tree == nil {
			continue
		}
		node := s.tree.Node
		v := nameVisitor{names: names}
		ast.Walk(&node, &v)
	}
	return names, ok
}

type nameVisitor struct {
	names map[string]struct{}
}

func (v *nameVisitor) Visit(node *ast.Node) {
	switch n := (*node).(type) {
	case *ast.IdentifierNode:
		v.names[n.Value] = struct{}{}
	case *ast.MemberNode:
		if s, ok := n.Property.(*ast.StringNode); ok {
			v.names[s.Value] = struct{}{}
		}
	}
}
