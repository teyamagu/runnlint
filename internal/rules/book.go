package rules

import (
	"regexp"
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
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
		c.Add(d.Node, message.DupKey, d.Key)
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
			c.Add(e.KeyNode, message.UnknownBookKey, e.Key)
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
			c.Add(node, message.StepsMissing)
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
				c.Add(el.Node, message.LabelChars, el.Display())
			}
		}
	}
}

func deferExperimental(c *Context) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			if d, ok := step.Value.Get("defer"); ok {
				c.Add(nodeOf(d, step), message.DeferExper)
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
			c.Add(v.Node, message.IntervalDur)
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
		c.Add(v.Node, message.FieldString, key)
	}
}

func checkSeqStrings(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if v.Kind != runbook.KindSeq {
		c.Add(v.Node, message.FieldStrings, key)
		return
	}
	for _, el := range v.Seq {
		if el.Kind != runbook.KindString {
			c.Add(el.Node, message.FieldElem, key)
		}
	}
}

func checkMap(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.FieldMap, key)
	}
}

func checkMapStrings(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.FieldMap, key)
		return
	}
	for _, e := range v.Map {
		if e.Value.Kind != runbook.KindString {
			c.Add(e.Value.Node, message.FieldMapString, key, e.Key)
		}
	}
}

func checkSecrets(c *Context, root runbook.Value) {
	v, ok := root.Get("secrets")
	if !ok {
		return
	}
	if v.Kind != runbook.KindSeq {
		c.Add(v.Node, message.SecretsArray)
		return
	}
	for _, el := range v.Seq {
		if !nonEmptyString(el) {
			c.Add(el.Node, message.SecretsElem)
		}
	}
}

func checkBoolOrString(c *Context, root runbook.Value, key string) {
	v, ok := root.Get(key)
	if !ok {
		return
	}
	if !boolOrString(v) {
		c.Add(v.Node, message.FieldBoolOrStr, key)
	}
}

func checkConcurrency(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		if strings.TrimSpace(v.String) == "" {
			c.Add(v.Node, message.ConcEmpty)
		}
	case runbook.KindSeq:
		if len(v.Seq) == 0 {
			c.Add(v.Node, message.ConcEmpty)
		}
		for _, el := range v.Seq {
			if !nonEmptyString(el) {
				c.Add(el.Node, message.ConcElem)
			}
		}
	default:
		c.Add(v.Node, message.ConcType)
	}
}

func checkLoop(c *Context, v runbook.Value, step bool) {
	switch v.Kind {
	case runbook.KindNumber:
		if strings.HasPrefix(v.Number, "-") {
			c.Add(v.Node, message.LoopCountNeg)
		}
	case runbook.KindString:
		return
	case runbook.KindMap:
		allowed := runnspec.DefKeys(runnspec.DefLoop)
		if _, ok := v.Get("count"); !ok {
			c.Add(v.Node, message.LoopCountMiss)
		}
		for _, e := range v.Map {
			if _, ok := allowed[e.Key]; !ok {
				c.Add(e.KeyNode, message.LoopUnknownKey, e.Key)
				continue
			}
			switch e.Key {
			case "count":
				if e.Value.Kind == runbook.KindNumber && strings.HasPrefix(e.Value.Number, "-") {
					c.Add(e.Value.Node, message.LoopCountMin)
				}
				if e.Value.Kind != runbook.KindNumber && e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, message.LoopCountType)
				}
			case "interval", "minInterval", "maxInterval":
				if !validDuration(e.Value) {
					c.Add(e.Value.Node, message.LoopDuration, e.Key)
				}
			case "jitter", "multiplier":
				if e.Value.Kind != runbook.KindNumber && e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, message.LoopNumber, e.Key)
				}
			case "until":
				if e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, message.LoopUntil)
				}
			}
		}
	default:
		where := "runbook"
		if step {
			where = "step"
		}
		c.Add(v.Node, message.LoopType, where)
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
