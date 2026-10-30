package rules

import (
	"strings"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/parser/lexer"
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
	"gopkg.in/yaml.v3"
)

type siteKind int

const (
	siteTemplate siteKind = iota
	siteExpr
)

type site struct {
	book   *runbook.Book
	step   *runbook.Step
	node   *yaml.Node
	src    string
	field  string
	kind   siteKind
	braced bool
	// sharp means the expression has a # comment outside ( ) and { }. runn strips it and warns that it is deprecated.
	sharp bool
	// skipParse means do not parse this as an expression. An expression such as if wrapped in {{ }} is inspected by the template side.
	skipParse bool
	tree      *parser.Tree
	err       error
}

func analyze(root *runbook.Book) []site {
	var sites []site
	root.Walk(func(b *runbook.Book) {
		for _, e := range b.Root.Map {
			if e.Key == "steps" {
				continue
			}
			walkStrings(e.Value, b, nil, &sites)
		}
		collectBookExprs(b, &sites)
		for i := range b.Steps {
			step := &b.Steps[i]
			for _, e := range step.Value.Map {
				switch e.Key {
				case "include":
					walkIncludeStrings(e.Value, b, step, &sites)
				case "test", "bind":
					// collectStepExprs reads this as an expression. {{ inside it is not a template.
				default:
					walkStrings(e.Value, b, step, &sites)
				}
			}
			collectStepExprs(b, step, &sites)
		}
	})
	for i := range sites {
		if sites[i].err != nil || sites[i].skipParse {
			continue
		}
		src := sites[i].src
		if sites[i].kind == siteExpr {
			// runn strips # comments before evaluating only expressions such as test and if. It does not strip them inside {{ }}.
			sites[i].sharp = commentCol(src) >= 0
			src = trimSharpComment(src)
		}
		tree, err := parser.Parse(strings.TrimSpace(src))
		sites[i].tree = tree
		sites[i].err = err
	}
	return sites
}

func walkIncludeStrings(v runbook.Value, b *runbook.Book, step *runbook.Step, sites *[]site) {
	if v.Kind == runbook.KindString {
		walkStrings(v, b, step, sites)
		return
	}
	if v.Kind != runbook.KindMap {
		return
	}
	if _, inline := v.Get("steps"); inline {
		return
	}
	walkStrings(v, b, step, sites)
}

func walkStrings(v runbook.Value, b *runbook.Book, step *runbook.Step, sites *[]site) {
	switch v.Kind {
	case runbook.KindString:
		inners, err := extractTemplates(expandEnvPlaceholders(v.String))
		if err != nil {
			*sites = append(*sites, site{book: b, step: step, node: v.Node, src: v.String, field: "template", kind: siteTemplate, err: err})
			return
		}
		for _, inner := range inners {
			*sites = append(*sites, site{book: b, step: step, node: v.Node, src: inner, field: "template", kind: siteTemplate})
		}
	case runbook.KindSeq:
		for _, el := range v.Seq {
			walkStrings(el, b, step, sites)
		}
	case runbook.KindMap:
		for _, e := range v.Map {
			walkStrings(e.Value, b, step, sites)
		}
	}
}

func collectBookExprs(b *runbook.Book, sites *[]site) {
	if v, ok := b.Root.Get("if"); ok {
		addExpr(sites, b, nil, v, "if")
	}
	if loop, ok := b.Root.Get("loop"); ok && loop.Kind == runbook.KindMap {
		if v, ok := loop.Get("until"); ok {
			addExpr(sites, b, nil, v, "until")
		}
		if v, ok := loop.Get("count"); ok {
			addCountExpr(sites, b, nil, v)
		}
	}
}

func collectStepExprs(b *runbook.Book, step *runbook.Step, sites *[]site) {
	if v, ok := step.Value.Get("if"); ok {
		addExpr(sites, b, step, v, "if")
	}
	if v, ok := step.Value.Get("test"); ok {
		addExpr(sites, b, step, v, "test")
	}
	if v, ok := step.Value.Get("dump"); ok {
		if v.Kind == runbook.KindString {
			addExpr(sites, b, step, v, "dump")
		} else if v.Kind == runbook.KindMap {
			if expr, ok := v.Get("expr"); ok {
				addExpr(sites, b, step, expr, "dump")
			}
		}
	}
	if v, ok := step.Value.Get("bind"); ok && v.Kind == runbook.KindMap {
		for _, e := range v.Map {
			addBindExprs(sites, b, step, e.Value)
		}
	}
	if loop, ok := step.Value.Get("loop"); ok && loop.Kind == runbook.KindMap {
		if v, ok := loop.Get("until"); ok {
			addExpr(sites, b, step, v, "until")
		}
		if v, ok := loop.Get("count"); ok {
			addCountExpr(sites, b, step, v)
		}
	}
}

// addBindExprs collects strings inside mappings and arrays as expressions, the same way as runn's EvalAny.
func addBindExprs(sites *[]site, b *runbook.Book, step *runbook.Step, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		addExpr(sites, b, step, v, "bind")
	case runbook.KindSeq:
		for _, el := range v.Seq {
			addBindExprs(sites, b, step, el)
		}
	case runbook.KindMap:
		for _, e := range v.Map {
			addBindExprs(sites, b, step, e.Value)
		}
	}
}

func addCountExpr(sites *[]site, b *runbook.Book, step *runbook.Step, v runbook.Value) {
	if v.Kind != runbook.KindString || numOnlyRe.MatchString(strings.TrimSpace(v.String)) {
		return
	}
	addExpr(sites, b, step, v, "count")
}

func addExpr(sites *[]site, b *runbook.Book, step *runbook.Step, v runbook.Value, field string) {
	if v.Kind != runbook.KindString {
		return
	}
	src := expandEnvPlaceholders(v.String)
	braced := false
	skipParse := false
	if field == "test" || field == "bind" {
		// {{ inside the value is not a template. It is an expr-braces subject only when the whole value is wrapped.
		src, braced, skipParse = classifyExprBraces(src)
	} else if strings.Contains(src, "{{") {
		braced = true
		skipParse = true
	}
	*sites = append(*sites, site{
		book:      b,
		step:      step,
		node:      v.Node,
		src:       src,
		field:     field,
		kind:      siteExpr,
		braced:    braced,
		skipParse: skipParse,
	})
}

// classifyExprBraces decides how {{ is treated in test and bind.
// When the whole value is one {{ }}, the unwrapped inside is parsed as an expression and is an expr-braces subject.
// When the value starts with {{ but is not a single template, it is not parsed as an expression and only expr-braces is reported.
// Any other {{ is inside a string, so the whole value is parsed as an expression.
func classifyExprBraces(src string) (parseSrc string, braced, skipParse bool) {
	s := strings.TrimSpace(src)
	if !strings.HasPrefix(s, "{{") {
		return src, false, false
	}
	if inner, ok := soleTemplate(s); ok {
		return inner, true, false
	}
	return src, true, true
}

// soleTemplate returns the inside when all of s is one {{ }}. s starts with {{.
func soleTemplate(s string) (string, bool) {
	inner, end, ok := scanTemplate(s, len("{{"))
	if !ok {
		return "", false
	}
	if strings.TrimSpace(s[end:]) != "" {
		return "", false
	}
	return inner, true
}

// envPlaceholder is the identifier substituted into an expression in place of ${...}.
const envPlaceholder = "__runnlint_env__"

// expandEnvPlaceholders matches runn expanding ${...} in values to environment variables before it reads YAML.
// The value depends on the runtime environment, so it is not read and is replaced with an identifier. An unset variable does not cause a false syntax finding.
// A nest such as ${A:-${B}} is replaced as one placeholder, outer and all. An unclosed ${ is left as-is.
func expandEnvPlaceholders(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			if end := envEnd(s, i+2); end > 0 {
				b.WriteString(envPlaceholder)
				i = end
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func envEnd(s string, start int) int {
	depth := 1
	for j := start; j < len(s); j++ {
		switch s[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return -1
}

// trimSharpComment, like runn's trimDeprecatedComment, removes from a # outside brackets through the end of the line.
// A # inside a string literal stays, because lexing makes it part of the string. A # inside ( ) or { } stays as a closure argument.
// runn slices the byte string using a column counted in characters. Here the cut is made in characters.
func trimSharpComment(cond string) string {
	if !strings.Contains(cond, "#") || commentCol(cond) < 0 {
		return cond
	}
	var kept []string
	for _, line := range strings.Split(cond, "\n") {
		if strings.HasPrefix(strings.Trim(line, " "), "#") {
			continue
		}
		if col := commentCol(line); col > 0 {
			kept = append(kept, strings.TrimRight(string([]rune(line)[:col]), " "))
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// commentCol is the character offset of a # treated as a comment. It is -1 when there is none or lexing fails.
func commentCol(s string) int {
	tokens, err := lexer.Lex(file.NewSource(s))
	if err != nil {
		return -1
	}
	inBrace, inParen := false, false
	for _, t := range tokens {
		switch {
		case t.Kind == lexer.Bracket && t.Value == "{":
			inBrace = true
		case t.Kind == lexer.Bracket && t.Value == "}":
			inBrace = false
		case t.Kind == lexer.Bracket && t.Value == "(":
			inParen = true
		case t.Kind == lexer.Bracket && t.Value == ")":
			inParen = false
		case t.Kind == lexer.Operator && t.Value == "#" && !inBrace && !inParen:
			return t.From
		}
	}
	return -1
}

func extractTemplates(s string) ([]string, error) {
	var out []string
	for i := 0; i < len(s); i++ {
		if i+1 >= len(s) || s[i] != '{' || s[i+1] != '{' {
			continue
		}
		inner, end, ok := scanTemplate(s, i+2)
		if !ok {
			return nil, message.New(message.ExprUnclosed)
		}
		out = append(out, inner)
		i = end - 1
	}
	return out, nil
}

// scanTemplate looks for the closing }} from start. A }} inside quotes or inside the expression's ( ) [ ] { } is not the end.
// When brackets are unbalanced and no close is found, the first }} outside quotes is the end, as in runn.
func scanTemplate(s string, start int) (string, int, bool) {
	if inner, end, ok := scanTemplateDepth(s, start, true); ok {
		return inner, end, true
	}
	return scanTemplateDepth(s, start, false)
}

func scanTemplateDepth(s string, start int, nested bool) (string, int, bool) {
	in := byte(0)
	depth := 0
	for j := start; j < len(s); {
		c := s[j]
		if in != 0 {
			if c == '\\' && in != '`' && j+1 < len(s) {
				j += 2
				continue
			}
			if c == in {
				in = 0
			}
			j++
			continue
		}
		switch {
		case c == '"' || c == '\'' || c == '`':
			in = c
		case c == '}' && depth == 0 && j+1 < len(s) && s[j+1] == '}':
			return s[start:j], j + 2, true
		case !nested:
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth == 0 {
				return "", 0, false
			}
			depth--
		}
		j++
	}
	return "", 0, false
}

type refVisitor struct {
	idents []string
	steps  []stepRef
	vars   []string
	parent []string
}

type stepRef struct {
	index *int
	name  string
	ok    bool
}

func (v *refVisitor) Visit(node *ast.Node) {
	switch n := (*node).(type) {
	case *ast.IdentifierNode:
		v.idents = append(v.idents, n.Value)
	case *ast.MemberNode:
		if id, ok := n.Node.(*ast.IdentifierNode); ok {
			switch id.Value {
			case "steps":
				v.steps = append(v.steps, stepSelector(n.Property))
			case "vars":
				if s, ok := n.Property.(*ast.StringNode); ok {
					v.vars = append(v.vars, s.Value)
				}
			}
		}
		if inner, ok := n.Node.(*ast.MemberNode); ok {
			if id, ok := inner.Node.(*ast.IdentifierNode); ok && id.Value == "parent" {
				if prop, ok := inner.Property.(*ast.StringNode); ok && prop.Value == "vars" {
					if name, ok := n.Property.(*ast.StringNode); ok {
						v.parent = append(v.parent, name.Value)
					}
				}
			}
		}
	}
}

func stepSelector(n ast.Node) stepRef {
	switch p := n.(type) {
	case *ast.IntegerNode:
		i := p.Value
		return stepRef{index: &i, ok: true}
	case *ast.StringNode:
		return stepRef{name: p.Value, ok: true}
	default:
		return stepRef{}
	}
}

func inspect(tree *parser.Tree) refVisitor {
	var v refVisitor
	if tree == nil {
		return v
	}
	node := tree.Node
	ast.Walk(&node, &v)
	return v
}

func stepInLoop(s site) bool {
	if s.book != nil && s.book.LoopEnabled() {
		return true
	}
	if s.step == nil {
		return false
	}
	_, ok := s.step.Value.Get("loop")
	return ok
}
