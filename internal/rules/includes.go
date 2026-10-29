package rules

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
)

func init() {
	register("included-unreferenced", includedUnreferenced)
}

// Refs holds include targets found in the files being linted and in their callers.
// It collects, before checks run, who includes a runbook, which a per-runbook check cannot see.
type Refs struct {
	scopes  []string
	static  map[string][]IncludeRef
	dynamic []dynamicRef
}

// IncludeRef is one include.
type IncludeRef struct {
	// From is the file that includes the target.
	From string
	// Vars are variable names passed in include.vars. When KnownVars is false, they are unknown until expansion.
	Vars      map[string]struct{}
	KnownVars bool
	// Dynamic is a reference whose path contains {{ }} or ${...} and may only possibly include this file.
	Dynamic bool
}

type dynamicRef struct {
	re  *regexp.Regexp
	ref IncludeRef
}

// NewRefs builds a Refs that judges files inside scopes.
// scopes are directories or files whose callers were all read.
func NewRefs(scopes ...string) *Refs {
	r := &Refs{static: map[string][]IncludeRef{}}
	for _, s := range scopes {
		if abs, err := filepath.Abs(s); err == nil {
			r.scopes = append(r.scopes, abs)
		}
	}
	return r
}

// Add reads include references from the YAML at path. It does nothing when the file is not a runbook.
func (r *Refs) Add(path string, src []byte) {
	if r == nil {
		return
	}
	doc, err := runbook.Parse(src)
	if err != nil || doc.Book == nil {
		return
	}
	from, err := filepath.Abs(path)
	if err != nil {
		return
	}
	doc.Book.SetLocation(path, filepath.Dir(path))
	doc.Book.Walk(func(b *runbook.Book) {
		for _, step := range b.Steps {
			inc, ok := step.Value.Get("include")
			if !ok {
				continue
			}
			switch inc.Kind {
			case runbook.KindString:
				r.addRef(b, from, inc, runbook.Value{})
			case runbook.KindMap:
				if p, ok := inc.Get("path"); ok {
					vars, _ := inc.Get("vars")
					r.addRef(b, from, p, vars)
				}
			}
		}
		for _, runner := range b.Runners {
			if runner.Kind != runbook.KindInclude || runner.Value.Kind != runbook.KindMap {
				continue
			}
			if p, ok := runner.Value.Get("path"); ok {
				vars, _ := runner.Value.Get("vars")
				r.addRef(b, from, p, vars)
			}
		}
	})
}

func (r *Refs) addRef(b *runbook.Book, from string, path, vars runbook.Value) {
	if path.Kind != runbook.KindString {
		return
	}
	ref := IncludeRef{From: from}
	switch vars.Kind {
	case runbook.KindNull:
		ref.KnownVars = true
	case runbook.KindMap:
		ref.KnownVars = true
		ref.Vars = map[string]struct{}{}
		for _, e := range vars.Map {
			ref.Vars[e.Key] = struct{}{}
		}
	}
	target, ok := resolveLocal(b, path.String)
	if ok {
		if abs, err := filepath.Abs(target); err == nil {
			r.static[abs] = append(r.static[abs], ref)
		}
		return
	}
	if re := dynamicPattern(path.String); re != nil {
		ref.Dynamic = true
		r.dynamic = append(r.dynamic, dynamicRef{re: re, ref: ref})
	}
}

// InScope reports whether every caller of path was read.
func (r *Refs) InScope(path string) bool {
	if r == nil {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, s := range r.scopes {
		if abs == s || strings.HasPrefix(abs, s+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// Callers returns references that may include path. Dynamic references include every one that can match.
func (r *Refs) Callers(path string) []IncludeRef {
	if r == nil {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	var out []IncludeRef
	for _, ref := range r.static[abs] {
		if ref.From != abs {
			out = append(out, ref)
		}
	}
	slash := filepath.ToSlash(abs)
	for _, d := range r.dynamic {
		if d.ref.From != abs && d.re.MatchString(slash) {
			out = append(out, d.ref)
		}
	}
	return out
}

// resolveLocal turns an include or needs path into a file path relative to the directory of book.
// A path that contains {{ }} or ${...}, a URL, or an empty string cannot be resolved, so it returns false.
func resolveLocal(b *runbook.Book, raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || runbook.ContainsTemplate(s) {
		return "", false
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return "", false
	}
	if filepath.IsAbs(s) {
		return filepath.Clean(s), true
	}
	dir := b.Dir
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, s), true
}

var dynamicPartRe = regexp.MustCompile(`\{\{.*?\}\}|\$\{[^}]*\}+`)

// dynamicPattern turns a path that contains {{ }} or ${...} into a regexp for paths that end with the text after the last dynamic part.
// Text before an unknown value is not used for matching, because relative resolution and .. are not determined. Err on the side of not reporting.
func dynamicPattern(raw string) *regexp.Regexp {
	s := filepath.ToSlash(strings.TrimSpace(raw))
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return nil
	}
	locs := dynamicPartRe.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return nil
	}
	suffix := s[locs[len(locs)-1][1]:]
	return regexp.MustCompile(regexp.QuoteMeta(suffix) + `$`)
}

// requiresIncluded reports whether a top-level if is true only when the runbook is included.
// It matches included, included == true, and expressions that join those with &&. Forms that contain || or ! can run on their own, so they are excluded.
func requiresIncluded(cond string) bool {
	src := strings.TrimSpace(trimSharpComment(expandEnvPlaceholders(cond)))
	if src == "" || strings.Contains(src, "{{") {
		return false
	}
	tree, err := parser.Parse(src)
	if err != nil {
		return false
	}
	return needsIncluded(tree.Node)
}

func needsIncluded(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.IdentifierNode:
		return v.Value == "included"
	case *ast.BinaryNode:
		switch v.Operator {
		case "&&", "and":
			return needsIncluded(v.Left) || needsIncluded(v.Right)
		case "==":
			if b, ok := v.Right.(*ast.BoolNode); ok && b.Value {
				return needsIncluded(v.Left)
			}
			if b, ok := v.Left.(*ast.BoolNode); ok && b.Value {
				return needsIncluded(v.Right)
			}
		}
	}
	return false
}

func includedUnreferenced(c *Context) {
	if c.Refs == nil || c.Doc == nil || c.Doc.Book == nil {
		return
	}
	b := c.Doc.Book
	cond, ok := b.Root.Get("if")
	if !ok || cond.Kind != runbook.KindString || !requiresIncluded(cond.String) {
		return
	}
	if !c.Refs.InScope(b.File) || len(c.Refs.Callers(b.File)) > 0 {
		return
	}
	c.Add(cond.Node, message.IncludedNone, strings.TrimSpace(cond.String))
}
