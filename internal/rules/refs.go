package rules

import (
	"strings"

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
		if s.err == nil || s.braced {
			continue
		}
		c.Add(s.node, "%s", s.err.Error())
	}
}

func exprBraces(c *Context) {
	for _, s := range c.Sites {
		if s.kind != siteExpr || !s.braced {
			continue
		}
		c.Add(s.node, "%s は式なので {{ }} で囲みません", s.field)
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
			c.Add(s.node, "steps は map なので steps.%s の形で参照します", indexHint(b, *ref.index))
			return
		}
		if *ref.index < 0 || *ref.index >= len(b.Steps) {
			c.Add(s.node, "steps[%d] はこの runbook にありません", *ref.index)
			return
		}
		if s.step == nil && s.field != "until" {
			c.Add(s.node, "steps は step の実行中に参照します")
			return
		}
		if s.step != nil && !stepVisible(b, s.step, *ref.index) {
			c.Add(s.node, "steps[%d] はまだ実行されていません", *ref.index)
		}
		return
	}
	if b.Style == runbook.StyleList {
		c.Add(s.node, "steps は配列なので steps[n] の形で参照します")
		return
	}
	target, ok := b.StepByName(ref.name)
	if !ok {
		c.Add(s.node, "steps.%s はこの runbook にありません", ref.name)
		return
	}
	if s.step == nil && s.field != "until" {
		c.Add(s.node, "steps は step の実行中に参照します")
		return
	}
	if s.step != nil && !stepVisible(b, s.step, target.Index) {
		c.Add(s.node, "steps.%s はまだ実行されていません", ref.name)
	}
}

func indexHint(b *runbook.Book, index int) string {
	if index >= 0 && index < len(b.Steps) && b.Steps[index].Name != "" {
		return b.Steps[index].Name
	}
	return "名前"
}

// stepVisible は current から target の結果を参照できるか。
// defer step は通常 step のあと、ファイルでは後ろにあるものから先に実行される。
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
			c.Add(s.node, "vars.%s は vars にありません", name)
		}
		if root {
			if outer.included {
				// 親の vars は include 元ごとに違うので、include 元があるときは調べない。
				continue
			}
			for _, name := range info.parent {
				c.Add(s.node, "parent.vars.%s は include された runbook の中で参照します", name)
			}
			continue
		}
		parentVars := s.book.Parent.VarNames()
		for _, name := range info.parent {
			if _, ok := ignore[name]; ok {
				continue
			}
			if _, ok := parentVars[name]; !ok {
				c.Add(s.node, "parent.vars.%s は親の vars にありません", name)
			}
		}
	}
}

// outerVars は、ファイルの外の include 元から渡される変数。
type outerVars struct {
	// names は、どれかの include 元が include.vars で渡す変数。
	names map[string]struct{}
	// included は include 元が見つかったか。
	included bool
	// unknown は、渡される変数が分からないか。分からなければ var-ref を出さない。
	unknown bool
}

// includeVars は、このファイルを include する runbook が渡す変数を集める。
// include 元が見つからなくても、if: included の runbook は include されて動く前提なので、渡される変数が分からないとする。
func includeVars(c *Context) outerVars {
	out := outerVars{names: map[string]struct{}{}}
	if c.Doc == nil || c.Doc.Book == nil {
		return out
	}
	b := c.Doc.Book
	for _, ref := range c.Refs.Callers(b.File) {
		out.included = true
		if !ref.KnownVars {
			out.unknown = true
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
				c.Add(s.node, "i は loop の中で使える添字です")
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
				c.Add(s.node, "先頭の step には previous がありません")
				break
			}
		}
	}
}
