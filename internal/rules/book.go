package rules

import (
	"regexp"
	"strings"

	"github.com/teyamagu/runnlint/internal/runbook"
	"github.com/teyamagu/runnlint/internal/runnspec"
	"gopkg.in/yaml.v3"
)

func init() {
	register("duplicate-key", duplicateKey)
	register("unknown-key", unknownKey)
	register("book-shape", bookShape)
	register("require-steps", requireSteps)
	register("label-format", labelFormat)
	register("defer-experimental", deferExperimental)
}

var labelRe = regexp.MustCompile(runnspec.LabelPattern())

func duplicateKey(c *Context) {
	if c.Doc == nil {
		return
	}
	for _, d := range c.Doc.Dups {
		c.Add(d.Node, "キー %s が重複しています", d.Key)
	}
}

func allowedTopLevel(key string) bool {
	if _, ok := runnspec.TopLevelKeys()[key]; ok || key == "<<" {
		return true
	}
	for _, prefix := range runnspec.ExtensionPrefixes() {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func unknownKey(c *Context) {
	for _, b := range c.Books() {
		for _, e := range b.Root.Map {
			if allowedTopLevel(e.Key) {
				continue
			}
			c.Add(e.KeyNode, "トップレベルのキー %s は runbook のフィールドではありません", e.Key)
		}
	}
}

func requireSteps(c *Context) {
	for _, b := range c.Books() {
		steps, ok := b.Root.Get("steps")
		if !ok || b.Style == runbook.StyleMissing || len(b.Steps) == 0 {
			node := b.Root.Node
			if ok {
				node = steps.Node
			}
			c.Add(node, "steps がありません")
		}
	}
}

func labelFormat(c *Context) {
	for _, b := range c.Books() {
		labels, ok := b.Root.Get("labels")
		if !ok {
			continue
		}
		if labels.Kind != runbook.KindSeq {
			continue
		}
		for _, el := range labels.Seq {
			if el.Kind != runbook.KindString || !labelRe.MatchString(el.String) {
				c.Add(el.Node, "label に使えない文字があります: %s", el.Display())
			}
		}
	}
}

func deferExperimental(c *Context) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			if d, ok := step.Value.Get("defer"); ok {
				c.Add(nodeOf(d, step), "defer は実験的機能です")
			}
		}
	}
}

func bookShape(c *Context) {
	for _, b := range c.Books() {
		checkStringField(c, b.Root, "desc")
		checkSeqStrings(c, b.Root, "labels")
		checkMapStrings(c, b.Root, "needs")
		checkMap(c, b.Root, "runners")
		checkMap(c, b.Root, "vars")
		checkSecrets(c, b.Root)
		checkMapStrings(c, b.Root, "hostRules")
		checkBoolOrString(c, b.Root, "debug")
		checkBoolOrString(c, b.Root, "skipTest")
		checkBoolOrString(c, b.Root, "force")
		checkBoolOrString(c, b.Root, "trace")
		checkBoolOrString(c, b.Root, "if")
		if v, ok := b.Root.Get("interval"); ok && !validDuration(v) {
			c.Add(v.Node, "interval は時間です")
		}
		if v, ok := b.Root.Get("loop"); ok {
			checkLoop(c, v, false)
		}
		if v, ok := b.Root.Get("concurrency"); ok {
			checkConcurrency(c, v)
		}
	}
}

func checkStringField(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if v.Kind != runbook.KindString {
		c.Add(v.Node, "%s は文字列です", key)
	}
}

func checkSeqStrings(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if v.Kind != runbook.KindSeq {
		c.Add(v.Node, "%s は文字列の配列です", key)
		return
	}
	for _, el := range v.Seq {
		if el.Kind != runbook.KindString {
			c.Add(el.Node, "%s の要素は文字列です", key)
		}
	}
}

func checkMap(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "%s は mapping です", key)
	}
}

func checkMapStrings(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "%s は mapping です", key)
		return
	}
	for _, e := range v.Map {
		if e.Value.Kind != runbook.KindString {
			c.Add(e.Value.Node, "%s.%s は文字列です", key, e.Key)
		}
	}
}

func checkSecrets(c *Context, root runbook.Value) {
	v, ok := root.Get("secrets")
	if !ok {
		return
	}
	if v.Kind != runbook.KindSeq {
		c.Add(v.Node, "secrets は文字列の配列です")
		return
	}
	for _, el := range v.Seq {
		if !nonEmptyString(el) {
			c.Add(el.Node, "secrets の要素は空でない文字列です")
		}
	}
}

func checkBoolOrString(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if !boolOrString(v) {
		c.Add(v.Node, "%s は bool または文字列です", key)
	}
}

func checkConcurrency(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		if strings.TrimSpace(v.String) == "" {
			c.Add(v.Node, "concurrency が空です")
		}
	case runbook.KindSeq:
		if len(v.Seq) == 0 {
			c.Add(v.Node, "concurrency が空です")
		}
		for _, el := range v.Seq {
			if !nonEmptyString(el) {
				c.Add(el.Node, "concurrency の要素は空でない文字列です")
			}
		}
	default:
		c.Add(v.Node, "concurrency は文字列または文字列の配列です")
	}
}

func checkLoop(c *Context, v runbook.Value, step bool) {
	switch v.Kind {
	case runbook.KindNumber:
		if strings.HasPrefix(v.Number, "-") {
			c.Add(v.Node, "loop の回数は 0 以上です")
		}
	case runbook.KindString:
		return
	case runbook.KindMap:
		allowed := runnspec.DefKeys(runnspec.DefLoop)
		if _, ok := v.Get("count"); !ok {
			c.Add(v.Node, "loop.count がありません")
		}
		for _, e := range v.Map {
			if _, ok := allowed[e.Key]; !ok {
				c.Add(e.KeyNode, "loop にキー %s はありません", e.Key)
				continue
			}
			switch e.Key {
			case "count":
				if e.Value.Kind == runbook.KindNumber && strings.HasPrefix(e.Value.Number, "-") {
					c.Add(e.Value.Node, "loop.count は 0 以上です")
				}
				if e.Value.Kind != runbook.KindNumber && e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, "loop.count は数値または式です")
				}
			case "interval", "minInterval", "maxInterval":
				if !validDuration(e.Value) {
					c.Add(e.Value.Node, "loop.%s は時間です", e.Key)
				}
			case "jitter", "multiplier":
				if e.Value.Kind != runbook.KindNumber && e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, "loop.%s は数値です", e.Key)
				}
			case "until":
				if e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, "loop.until は式です")
				}
			}
		}
	default:
		where := "runbook"
		if step {
			where = "step"
		}
		c.Add(v.Node, "%s の loop は数値、式、または mapping です", where)
	}
}

func nodeOf(v runbook.Value, step runbook.Step) *yaml.Node {
	if v.Node != nil {
		return v.Node
	}
	if step.NameNode != nil {
		return step.NameNode
	}
	return step.Value.Node
}
