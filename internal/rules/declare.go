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
	var whereRe []*regexp.Regexp
	for _, cond := range check.Where {
		re, err := compileRe(cond.Matches)
		if err != nil {
			c.Add(c.Doc.Root.Node, message.ReWhere, err.Error())
			return
		}
		whereRe = append(whereRe, re)
	}

	found := selectPath(c.Doc.Root, check.Select)
	if check.Exists != nil && *check.Exists && len(found) == 0 {
		node, path := deepest(c.Doc.Root, splitPath(check.Select))
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
		if !whereMatch(m.value, check.Where, whereRe) {
			continue
		}
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

func whereMatch(v runbook.Value, conds []config.Cond, res []*regexp.Regexp) bool {
	for i, cond := range conds {
		target := v
		if cond.Path != "" {
			next, ok := v.Lookup(cond.Path)
			if cond.Exists != nil && *cond.Exists && !ok {
				return false
			}
			if cond.Exists != nil && !*cond.Exists {
				if ok {
					return false
				}
				continue
			}
			if !ok {
				return false
			}
			target = next
		} else if cond.Exists != nil {
			return false
		}
		if cond.Equals != nil && cond.Equals.Set && !equalValue(target, cond.Equals.Value) {
			return false
		}
		if i < len(res) && res[i] != nil {
			text := scalarText(target)
			if !res[i].MatchString(text) {
				return false
			}
		}
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

func selectPath(root runbook.Value, path string) []match {
	return walkPath(root, splitPath(path), "", "", nil)
}

func splitPath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

func walkPath(v runbook.Value, segs []string, acc, key string, keyNode *yaml.Node) []match {
	if len(segs) == 0 {
		if acc == "" {
			acc = "$"
		}
		return []match{{value: v, key: key, keyNode: keyNode, path: acc}}
	}
	seg := segs[0]
	if seg == "*" {
		switch v.Kind {
		case runbook.KindSeq:
			var out []match
			for i, el := range v.Seq {
				next := acc + "[" + strconv.Itoa(i) + "]"
				out = append(out, walkPath(el, segs[1:], next, strconv.Itoa(i), nil)...)
			}
			return out
		case runbook.KindMap:
			var out []match
			for _, e := range v.Map {
				next := e.Key
				if acc != "" {
					next = acc + "." + e.Key
				}
				out = append(out, walkPath(e.Value, segs[1:], next, e.Key, e.KeyNode)...)
			}
			return out
		default:
			return nil
		}
	}
	if v.Kind != runbook.KindMap {
		return nil
	}
	var entry runbook.Entry
	found := false
	for _, e := range v.Map {
		if e.Key == seg {
			entry = e
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	next := seg
	if acc != "" {
		next = acc + "." + seg
	}
	return walkPath(entry.Value, segs[1:], next, seg, entry.KeyNode)
}

func deepest(root runbook.Value, segs []string) (runbook.Value, string) {
	cur := root
	acc := ""
	for _, seg := range segs {
		if seg == "*" || cur.Kind != runbook.KindMap {
			return cur, acc
		}
		next, ok := cur.Get(seg)
		if !ok {
			return cur, acc
		}
		if acc == "" {
			acc = seg
		} else {
			acc += "." + seg
		}
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
