package rules

import (
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
)

func init() {
	register("expr-syntax", exprSyntax)
	register("expr-braces", exprBraces)
	register("step-ref", lintStepRefs)
	register("var-ref", varRef)
	register("loop-index", loopIndex)
	register("previous-step", previousStep)
}

func exprSyntax(c *Context) {
	for _, s := range c.Sites {
		if s.err == nil {
			continue
		}
		c.AddError(s.node, s.err)
	}
}

func exprBraces(c *Context) {
	for _, s := range c.Sites {
		if s.kind != siteExpr || !s.braced {
			continue
		}
		c.Add(s.node, message.ExprNotBrace, s.field)
	}
}

func lintStepRefs(c *Context) {
	for _, s := range c.Sites {
		if s.err != nil || s.tree == nil || s.book == nil {
			continue
		}
		info := inspect(s.tree)
		for _, ref := range info.steps {
			if !ref.ok || strings.Contains(ref.name, envPlaceholder) {
				continue
			}
			checkOneStepRef(c, s, ref)
		}
	}
}

func checkOneStepRef(c *Context, s site, ref stepRef) {
	b := s.book
	if ref.index != nil {
		if b.Style == runbook.StyleMap {
			c.Add(s.node, message.StepsMapRef, indexHint(b, *ref.index))
			return
		}
		if *ref.index < 0 || *ref.index >= len(b.Steps) {
			c.Add(s.node, message.StepsIndexMiss, *ref.index)
			return
		}
		if s.step == nil && s.field != "until" {
			c.Add(s.node, message.StepsDuring)
			return
		}
		if s.step != nil && !stepVisible(b, s.step, *ref.index) {
			c.Add(s.node, message.StepsIndexEarly, *ref.index)
		}
		return
	}
	if b.Style == runbook.StyleList {
		c.Add(s.node, message.StepsArrayRef)
		return
	}
	target, ok := b.StepByName(ref.name)
	if !ok {
		c.Add(s.node, message.StepsNameMiss, ref.name)
		return
	}
	if s.step == nil && s.field != "until" {
		c.Add(s.node, message.StepsDuring)
		return
	}
	if s.step != nil && !stepVisible(b, s.step, target.Index) {
		c.Add(s.node, message.StepsNameEarly, ref.name)
	}
}

func indexHint(b *runbook.Book, index int) string {
	if index >= 0 && index < len(b.Steps) && b.Steps[index].Name != "" {
		return b.Steps[index].Name
	}
	return "名前"
}

// stepVisible reports whether current can refer to the result of target.
// A defer step runs after ordinary steps, and later ones in the file run first.
func stepVisible(b *runbook.Book, current *runbook.Step, target int) bool {
	if target == current.Index {
		return true
	}
	if target < 0 || target >= len(b.Steps) {
		return false
	}
	dst := b.Steps[target]
	if !current.Defer {
		return !dst.Defer && target < current.Index
	}
	if !dst.Defer {
		return true
	}
	return target > current.Index
}

func varRef(c *Context) {
	ignore := optionSet(c.Rule.Options, "ignore")
	outer := includeVars(c)
	for _, s := range c.Sites {
		if s.err != nil || s.tree == nil || s.book == nil {
			continue
		}
		info := inspect(s.tree)
		names := s.book.VarNames()
		root := s.book.Parent == nil
		if root && outer.unknown {
			continue
		}
		for _, name := range info.vars {
			if _, ok := ignore[name]; ok || strings.Contains(name, envPlaceholder) {
				continue
			}
			if _, ok := names[name]; ok {
				continue
			}
			if _, ok := outer.names[name]; ok && root {
				continue
			}
			c.Add(s.node, message.VarMissing, name)
		}
		if root {
			if outer.included || outer.maybe {
				// Parent vars differ per caller, so they are not checked when a caller may exist.
				continue
			}
			for _, name := range info.parent {
				c.Add(s.node, message.ParentVarsInside, name)
			}
			continue
		}
		parentVars := s.book.Parent.VarNames()
		for _, name := range info.parent {
			if _, ok := ignore[name]; ok {
				continue
			}
			if _, ok := parentVars[name]; !ok {
				c.Add(s.node, message.ParentVarsMiss, name)
			}
		}
	}
}

// outerVars are variables passed by include callers outside this file.
type outerVars struct {
	// names are variables that some caller passes in include.vars.
	names map[string]struct{}
	// included reports whether a caller was found.
	included bool
	// unknown reports whether the passed variables are unknown. When they are, var-ref is not reported.
	unknown bool
	// maybe reports whether this file may be the target of a dynamic include.
	maybe bool
}

// includeVars collects variables that runbooks including this file pass in.
// Even when no caller is found, a runbook with if: included is assumed to run via include, so the passed variables are treated as unknown.
func includeVars(c *Context) outerVars {
	out := outerVars{names: map[string]struct{}{}}
	if c.Doc == nil || c.Doc.Book == nil {
		return out
	}
	b := c.Doc.Book
	for _, ref := range c.Refs.Callers(b.File) {
		// A dynamic reference may only possibly include this file, so only variables it might pass are added.
		if !ref.Dynamic {
			out.included = true
			if !ref.KnownVars {
				out.unknown = true
			}
		} else {
			out.maybe = true
		}
		for name := range ref.Vars {
			out.names[name] = struct{}{}
		}
	}
	if !out.included {
		if cond, ok := b.Root.Get("if"); ok && cond.Kind == runbook.KindString && requiresIncluded(cond.String) {
			out.included = true
			out.unknown = true
		}
	}
	return out
}

func loopIndex(c *Context) {
	for _, s := range c.Sites {
		if s.err != nil || s.tree == nil || stepInLoop(s) {
			continue
		}
		info := inspect(s.tree)
		for _, id := range info.idents {
			if id == "i" {
				c.Add(s.node, message.LoopIndex)
				break
			}
		}
	}
}

func previousStep(c *Context) {
	for _, s := range c.Sites {
		if s.err != nil || s.tree == nil {
			continue
		}
		if s.step != nil && s.step.Index != 0 {
			continue
		}
		info := inspect(s.tree)
		for _, id := range info.idents {
			if id == "previous" {
				c.Add(s.node, message.NoPrevious)
				break
			}
		}
	}
}
