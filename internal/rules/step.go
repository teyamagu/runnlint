package rules

import (
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
)

func init() {
	register("step-shape", stepShape)
}

var httpMethods = map[string]struct{}{
	"GET": {}, "HEAD": {}, "POST": {}, "PUT": {}, "PATCH": {},
	"DELETE": {}, "OPTIONS": {}, "TRACE": {}, "CONNECT": {},
}

func stepShape(c *Context) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			checkStepFields(c, step)
			checkStepRunners(c, step)
		}
	}
}

// checkStepFields follows appendStep: if, desc, defer, force, loop, test, dump, then bind.
func checkStepFields(c *Context, step runbook.Step) {
	if v, ok := step.Value.Get("if"); ok && !boolOrString(v) {
		c.Add(v.Node, message.IfExpr)
	}
	checkStringField(c, step.Value, "desc")
	if v, ok := step.Value.Get("defer"); ok && !boolOrString(v) {
		c.Add(v.Node, message.DeferType)
	}
	if v, ok := step.Value.Get("force"); ok && !boolOrString(v) {
		c.Add(v.Node, message.ForceType)
	}
	if v, ok := step.Value.Get("loop"); ok {
		checkLoop(c, v, true)
	}
	if v, ok := step.Value.Get("test"); ok && !boolOrString(v) {
		c.Add(v.Node, message.TestExpr)
	}
	if v, ok := step.Value.Get("dump"); ok {
		checkDump(c, v)
	}
	if v, ok := step.Value.Get("bind"); ok {
		checkBind(c, v)
	}
}

func checkStepRunners(c *Context, step runbook.Step) {
	var mains int
	var companions int
	hasRunner := false
	for _, e := range step.Value.Map {
		switch e.Key {
		case "if", "desc", "loop", "defer", "force", "<<":
			continue
		case "runner":
			hasRunner = true
		case "test", "dump", "bind":
			companions++
		default:
			mains++
		}
	}
	if hasRunner && (mains > 0 || companions > 0) {
		node := step.Value.Node
		if v, ok := step.Value.Get("runner"); ok {
			node = v.Node
		}
		c.Add(node, message.RunnerColon)
		return
	}
	if hasRunner {
		def, ok := step.Value.Get("runner")
		if !ok || def.Kind != runbook.KindMap || len(def.Map) == 0 {
			node := step.Value.Node
			if ok {
				node = def.Node
			}
			c.Add(node, message.RunnerMap)
		}
		return
	}
	if mains > 1 {
		c.Add(step.Value.Node, message.MainOne)
	}
	if mains == 0 && companions == 0 {
		c.Add(step.Value.Node, message.MainMissing)
	}
}

func eachUserRunner(c *Context, kind runbook.RunnerKind, fn func(runbook.Value)) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			known := b.RunnersAt(step.Index)
			for _, e := range step.Value.Map {
				if isStepKeyword(e.Key) {
					continue
				}
				r, ok := known[e.Key]
				if !ok {
					continue
				}
				got := r.Kind
				if got == runbook.KindUnknown {
					got = guessStep(e.Value)
				}
				if got != kind {
					continue
				}
				fn(e.Value)
			}
		}
	}
}

// guessStep follows appendStep when the runner value is still a template.
// query is DB, actions is CDP, prompt is agent, command is SSH, then one path and one method is HTTP or gRPC.
func guessStep(v runbook.Value) runbook.RunnerKind {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return runbook.KindUnknown
	}
	if v.Kind != runbook.KindMap {
		return runbook.KindUnknown
	}
	if _, ok := v.Get("query"); ok {
		return runbook.KindDB
	}
	if _, ok := v.Get("actions"); ok {
		return runbook.KindCDP
	}
	if _, ok := v.Get("prompt"); ok {
		return runbook.KindAgent
	}
	if _, ok := v.Get("command"); ok {
		return runbook.KindSSH
	}
	if len(v.Map) == 1 && v.Map[0].Value.Kind == runbook.KindMap && len(v.Map[0].Value.Map) == 1 {
		method := v.Map[0].Value.Map[0].Key
		if _, ok := httpMethods[strings.ToUpper(method)]; ok {
			return runbook.KindHTTP
		}
		key := v.Map[0].Key
		if strings.HasPrefix(key, "/") {
			return runbook.KindHTTP
		}
		if strings.Contains(key, "/") {
			return runbook.KindGRPC
		}
	}
	return runbook.KindUnknown
}
