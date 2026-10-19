package rules

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/teyamagu/runnlint/internal/runbook"
)

func init() {
	register("included-unreferenced", includedUnreferenced)
}

// Refs は、lint の対象と参照元のファイルにある include の参照先。
// runbook ごとの検査だけでは分からない「誰から include されるか」を、検査の前にまとめて集める。
type Refs struct {
	scopes  []string
	static  map[string][]IncludeRef
	dynamic []dynamicRef
}

// IncludeRef は1件の include。
type IncludeRef struct {
	// From は include する側のファイル。
	From string
	// Vars は include.vars で渡す変数名。KnownVars が false なら、展開しないと分からない。
	Vars      map[string]struct{}
	KnownVars bool
	// Dynamic は、パスに {{ }} や ${...} を含み、このファイルを include するかもしれないだけの参照。
	Dynamic bool
}

type dynamicRef struct {
	re  *regexp.Regexp
	ref IncludeRef
}

// NewRefs は scopes の中のファイルを判定の対象にする Refs を作る。
// scopes は、参照元を残らず読んだディレクトリかファイル。
func NewRefs(scopes ...string) *Refs {
	r := &Refs{static: map[string][]IncludeRef{}}
	for _, s := range scopes {
		if abs, err := filepath.Abs(s); err == nil {
			r.scopes = append(r.scopes, abs)
		}
	}
	return r
}

// Add は path の YAML から include の参照を読む。runbook でなければ何もしない。
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

// InScope は path の参照元をすべて読んだと言えるか。
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

// Callers は path を include しうる参照。動的な参照は、照合しうるものをすべて含む。
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

// resolveLocal は include や needs のパスを、book のあるディレクトリからのファイルパスにする。
// {{ }} や ${...} を含むもの、URL、空文字は解決できないので false。
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

// dynamicPattern は {{ }} や ${...} を含むパスを、最後の動的な部分より後ろの文字列で終わるパスの正規表現にする。
// 値が分からない部分の前は、相対パスの解決や .. の扱いが決まらないので照合に使わない。指摘しない側に倒す。
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

// requiresIncluded は、トップレベルの if が include されたときにしか真にならない形か。
// included、included == true と、それらを && でつないだ式を対象にする。|| や ! を含む形は単体でも動くので対象外。
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
	c.Add(cond.Node, "if: %s の runbook ですが、走査した範囲のどの runbook からも include されていません", strings.TrimSpace(cond.String))
}
