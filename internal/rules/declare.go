package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
	"gopkg.in/yaml.v3"
)

type match struct {
	value   runbook.Value
	keyNode *yaml.Node
	key     string
	path    string
}

// whereCond is one where entry with its parsed path and compiled regular expression.
// For a path that started with $, segs holds the rest and the entry is looked up from the runbook root once per file.
type whereCond struct {
	cond config.Cond
	re   *regexp.Regexp
	segs []runbook.Seg
}

func runCheck(c *Context) {
	check := c.Rule.Check
	if check == nil || c.Doc == nil {
		return
	}
	matchRe, err := compileRe(check.Matches)
	if err != nil {
		c.Add(c.Doc.Root.Node, message.ReMatches, err.Error())
		return
	}
	notRe, err := compileRe(check.NotMatch)
	if err != nil {
		c.Add(c.Doc.Root.Node, message.ReNotMatches, err.Error())
		return
	}
	keyRe, err := compileRe(check.KeyMatches)
	if err != nil {
		c.Add(c.Doc.Root.Node, message.ReKeyMatches, err.Error())
		return
	}
	keyNotRe, err := compileRe(check.KeyNotMatch)
	if err != nil {
		c.Add(c.Doc.Root.Node, message.ReKeyNot, err.Error())
		return
	}
	selSegs, err := runbook.ParsePath(check.Select)
	if err != nil {
		c.AddError(c.Doc.Root.Node, message.Wrap(message.PathBad, err, c.Rule.ID, "select", check.Select))
		return
	}
	var gates, filters []whereCond
	for _, cond := range check.Where {
		re, err := compileRe(cond.Matches)
		if err != nil {
			c.Add(c.Doc.Root.Node, message.ReWhere, err.Error())
			return
		}
		segs, err := runbook.ParsePath(cond.Path)
		if err != nil {
			c.AddError(c.Doc.Root.Node, message.Wrap(message.PathBad, err, c.Rule.ID, "where.path", cond.Path))
			return
		}
		wc := whereCond{cond: cond, re: re, segs: segs}
		if len(segs) > 0 && !segs[0].Quoted && segs[0].Key == "$" {
			wc.segs = segs[1:]
			gates = append(gates, wc)
			continue
		}
		filters = append(filters, wc)
	}

	root := c.Doc.Root
	for _, wc := range gates {
		if !condHolds(root, wc) {
			return
		}
	}

	var found []match
	for _, m := range selectSegs(root, selSegs) {
		if whereMatch(m.value, filters) {
			found = append(found, m)
		}
	}
	if check.Exists != nil && *check.Exists && len(found) == 0 {
		node, path := deepest(root, selSegs)
		c.report(node.Node, path, "", runbook.Value{}, message.PathMissing, check.Select)
		return
	}
	if check.Exists != nil && !*check.Exists && len(found) > 0 {
		for _, m := range found {
			c.report(m.value.Node, m.path, m.key, m.value, message.PathPresent, m.path)
		}
		return
	}

	for _, m := range found {
		if check.Forbid {
			c.report(m.value.Node, m.path, m.key, m.value, message.CondMatch)
			continue
		}
		if id, args, onKey, ok := violate(m.value, m.key, check, matchRe, notRe, keyRe, keyNotRe); ok {
			node := m.value.Node
			if onKey && m.keyNode != nil {
				node = m.keyNode
			}
			c.report(node, m.path, m.key, m.value, id, args...)
		}
	}
}

// report uses the rule's own message when one is set. Otherwise it uses the built-in sentence.
func (c *Context) report(node *yaml.Node, path, key string, v runbook.Value, id message.ID, args ...any) {
	if strings.TrimSpace(c.Rule.Message) != "" {
		c.AddText(node, render(c, path, key, v))
		return
	}
	c.Add(node, id, args...)
}

func compileRe(expr string) (*regexp.Regexp, error) {
	if expr == "" {
		return nil, nil
	}
	return regexp.Compile(expr)
}

func violate(v runbook.Value, key string, check *config.Check, matchRe, notRe, keyRe, keyNotRe *regexp.Regexp) (message.ID, []any, bool, bool) {
	if check.Kind != "" && runbook.KindName(v.Kind) != check.Kind {
		return message.KindMismatch, []any{runbook.KindName(v.Kind), check.Kind}, false, true
	}
	if check.Equals != nil && check.Equals.Set && !equalValue(v, check.Equals.Value) {
		return message.ValueNotEq, []any{fmt.Sprint(check.Equals.Value)}, false, true
	}
	if check.NotEquals != nil && check.NotEquals.Set && equalValue(v, check.NotEquals.Value) {
		return message.ValueIs, []any{fmt.Sprint(check.NotEquals.Value)}, false, true
	}
	text := scalarText(v)
	if matchRe != nil && (v.Kind != runbook.KindString && v.Kind != runbook.KindNumber || !matchRe.MatchString(text)) {
		return message.ValuePattern, nil, false, true
	}
	if notRe != nil && (v.Kind == runbook.KindString || v.Kind == runbook.KindNumber) && notRe.MatchString(text) {
		return message.ValueForbid, nil, false, true
	}
	if keyRe != nil && !keyRe.MatchString(key) {
		return message.KeyPattern, nil, true, true
	}
	if keyNotRe != nil && keyNotRe.MatchString(key) {
		return message.KeyForbid, nil, true, true
	}
	if len(check.OneOf) > 0 && !oneOf(v, check.OneOf) {
		return message.ValueOneOf, nil, false, true
	}
	if check.MinLen != nil && v.Len() < *check.MinLen {
		return message.LenUnder, []any{*check.MinLen}, false, true
	}
	if check.MaxLen != nil && v.Len() > *check.MaxLen {
		return message.LenOver, []any{*check.MaxLen}, false, true
	}
	return "", nil, false, false
}

func whereMatch(v runbook.Value, conds []whereCond) bool {
	for _, wc := range conds {
		if !condHolds(v, wc) {
			return false
		}
	}
	return true
}

// condHolds reports whether one where entry holds for v. For a root entry, v is the runbook root.
func condHolds(v runbook.Value, wc whereCond) bool {
	cond := wc.cond
	target := v
	if cond.Path != "" {
		next, ok := v.LookupSegs(wc.segs)
		if cond.Exists != nil && *cond.Exists && !ok {
			return false
		}
		if cond.Exists != nil && !*cond.Exists {
			return !ok
		}
		if !ok {
			return false
		}
		target = next
	} else if cond.Exists != nil {
		return false
	}
	if cond.Kind != "" && runbook.KindName(target.Kind) != cond.Kind {
		return false
	}
	if cond.Equals != nil && cond.Equals.Set && !equalValue(target, cond.Equals.Value) {
		return false
	}
	if wc.re != nil && !wc.re.MatchString(scalarText(target)) {
		return false
	}
	return true
}

func oneOf(v runbook.Value, values []any) bool {
	for _, want := range values {
		if equalValue(v, want) {
			return true
		}
	}
	return false
}

func equalValue(v runbook.Value, want any) bool {
	if want == nil {
		return v.Kind == runbook.KindNull
	}
	switch w := want.(type) {
	case bool:
		return v.Kind == runbook.KindBool && v.Bool == w
	case string:
		return v.Kind == runbook.KindString && v.String == w
	case int:
		return v.Kind == runbook.KindNumber && v.Number == strconv.Itoa(w)
	case int64:
		return v.Kind == runbook.KindNumber && v.Number == strconv.FormatInt(w, 10)
	case float64:
		if v.Kind != runbook.KindNumber {
			return false
		}
		f, err := strconv.ParseFloat(v.Number, 64)
		return err == nil && f == w
	default:
		return scalarText(v) == fmt.Sprint(want)
	}
}

func scalarText(v runbook.Value) string {
	switch v.Kind {
	case runbook.KindString:
		return v.String
	case runbook.KindNumber:
		return v.Number
	case runbook.KindBool:
		return strconv.FormatBool(v.Bool)
	case runbook.KindNull:
		return "null"
	default:
		return ""
	}
}

// selectPath returns every value that the select path selects. A path that cannot be parsed selects nothing.
func selectPath(root runbook.Value, path string) []match {
	segs, err := runbook.ParsePath(path)
	if err != nil {
		return nil
	}
	return selectSegs(root, segs)
}

// selectSegs returns every value that segs selects. A value reached twice, which is possible with repeated **, is kept once.
func selectSegs(root runbook.Value, segs []runbook.Seg) []match {
	all := walkPath(root, segs, "", "", nil)
	seen := map[string]struct{}{}
	out := all[:0]
	for _, m := range all {
		if _, ok := seen[m.path]; ok {
			continue
		}
		seen[m.path] = struct{}{}
		out = append(out, m)
	}
	return out
}

func isWild(seg runbook.Seg, name string) bool {
	return !seg.Quoted && seg.Key == name
}

// pathKey renders a key for path. A key that contains a dot is written in double quotes, the way select would write it.
func pathKey(key string) string {
	if strings.Contains(key, ".") {
		return "\"" + key + "\""
	}
	return key
}

func joinPath(acc, key string) string {
	if acc == "" {
		return pathKey(key)
	}
	return acc + "." + pathKey(key)
}

func walkPath(v runbook.Value, segs []runbook.Seg, acc, key string, keyNode *yaml.Node) []match {
	if len(segs) == 0 {
		if acc == "" {
			acc = "$"
		}
		return []match{{value: v, key: key, keyNode: keyNode, path: acc}}
	}
	seg := segs[0]
	if isWild(seg, "**") {
		if len(segs) == 1 {
			return leaves(v, acc, key, keyNode)
		}
		out := walkPath(v, segs[1:], acc, key, keyNode)
		forEachChild(v, acc, func(child runbook.Value, childAcc, childKey string, childKeyNode *yaml.Node) {
			out = append(out, walkPath(child, segs, childAcc, childKey, childKeyNode)...)
		})
		return out
	}
	if isWild(seg, "*") {
		var out []match
		forEachChild(v, acc, func(child runbook.Value, childAcc, childKey string, childKeyNode *yaml.Node) {
			out = append(out, walkPath(child, segs[1:], childAcc, childKey, childKeyNode)...)
		})
		return out
	}
	if v.Kind != runbook.KindMap {
		return nil
	}
	for _, e := range v.Map {
		if e.Key == seg.Key {
			return walkPath(e.Value, segs[1:], joinPath(acc, e.Key), e.Key, e.KeyNode)
		}
	}
	return nil
}

// forEachChild calls fn for each mapping value or sequence element of v. A scalar has no children.
func forEachChild(v runbook.Value, acc string, fn func(child runbook.Value, acc, key string, keyNode *yaml.Node)) {
	switch v.Kind {
	case runbook.KindSeq:
		for i, el := range v.Seq {
			idx := strconv.Itoa(i)
			fn(el, acc+"["+idx+"]", idx, nil)
		}
	case runbook.KindMap:
		for _, e := range v.Map {
			fn(e.Value, joinPath(acc, e.Key), e.Key, e.KeyNode)
		}
	}
}

// leaves returns every scalar at or below v. It is the meaning of a trailing **.
func leaves(v runbook.Value, acc, key string, keyNode *yaml.Node) []match {
	if v.Kind != runbook.KindSeq && v.Kind != runbook.KindMap {
		if acc == "" {
			acc = "$"
		}
		return []match{{value: v, key: key, keyNode: keyNode, path: acc}}
	}
	var out []match
	forEachChild(v, acc, func(child runbook.Value, childAcc, childKey string, childKeyNode *yaml.Node) {
		out = append(out, leaves(child, childAcc, childKey, childKeyNode)...)
	})
	return out
}

// deepest follows segs as far as plain keys exist. It stops before * and **. It is the position for an exists: true diagnostic.
func deepest(root runbook.Value, segs []runbook.Seg) (runbook.Value, string) {
	cur := root
	acc := ""
	for _, seg := range segs {
		if isWild(seg, "*") || isWild(seg, "**") || cur.Kind != runbook.KindMap {
			return cur, acc
		}
		next, ok := cur.Get(seg.Key)
		if !ok {
			return cur, acc
		}
		acc = joinPath(acc, seg.Key)
		cur = next
	}
	return cur, acc
}

func render(c *Context, path, key string, v runbook.Value) string {
	msg := c.Rule.Message
	tpl, err := template.New("msg").Option("missingkey=zero").Parse(msg)
	if err != nil {
		return msg
	}
	var buf bytes.Buffer
	data := map[string]any{
		"path":  path,
		"key":   key,
		"value": scalarText(v),
		"rule":  c.Rule.ID,
	}
	if err := tpl.Execute(&buf, data); err != nil {
		return msg
	}
	return buf.String()
}
