package rules

import (
	"strings"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/runbook"
	"github.com/teyamagu/runnlint/internal/runnspec"
)

func init() {
	register("step-shape", stepShape)
	register("http-request", httpRequest)
	register("grpc-request", grpcRequest)
	register("db-request", dbRequest)
	register("exec-request", execRequest)
	register("ssh-request", sshRequest)
	register("cdp-request", cdpRequest)
	register("agent-request", agentRequest)
	register("include-request", includeRequest)
}

var httpMethods = map[string]struct{}{
	"GET": {}, "HEAD": {}, "POST": {}, "PUT": {}, "PATCH": {},
	"DELETE": {}, "OPTIONS": {}, "TRACE": {}, "CONNECT": {},
}

func stepShape(c *Context) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			checkStepFields(c, step)
			checkStepRunners(c, b, step)
		}
	}
}

func checkStepFields(c *Context, step runbook.Step) {
	checkStringField(c, step.Value, "desc")
	if v, ok := step.Value.Get("if"); ok && !boolOrString(v) {
		c.Add(v.Node, "if は式または bool です")
	}
	if v, ok := step.Value.Get("defer"); ok && !boolOrString(v) {
		c.Add(v.Node, "defer は bool または文字列です")
	}
	if v, ok := step.Value.Get("force"); ok && !boolOrString(v) {
		c.Add(v.Node, "force は bool または文字列です")
	}
	if v, ok := step.Value.Get("test"); ok && !boolOrString(v) {
		c.Add(v.Node, "test は式または bool です")
	}
	if v, ok := step.Value.Get("loop"); ok {
		checkLoop(c, v, true)
	}
	if v, ok := step.Value.Get("dump"); ok {
		checkDump(c, v)
	}
	if v, ok := step.Value.Get("bind"); ok {
		checkBind(c, v)
	}
}

func checkDump(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		return
	case runbook.KindMap:
		allowKeys(c, v, "dump", runnspec.DefKeys(runnspec.DefDump))
		if _, ok := v.Get("expr"); !ok {
			c.Add(v.Node, "dump.expr がありません")
		}
		if out, ok := v.Get("out"); ok && out.Kind != runbook.KindString {
			c.Add(out.Node, "dump.out は文字列です")
		}
		checkBoolish(c, v, "disableTrailingNewline", "disableMaskingSecrets")
	default:
		c.Add(v.Node, "dump は式または mapping です")
	}
}

func checkBind(c *Context, v runbook.Value) {
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "bind は mapping です")
		return
	}
	if len(v.Map) == 0 {
		c.Add(v.Node, "bind が空です")
	}
	for _, e := range v.Map {
		if e.Value.Kind != runbook.KindString {
			c.Add(e.Value.Node, "bind.%s は式です", e.Key)
		}
	}
}

func checkStepRunners(c *Context, b *runbook.Book, step runbook.Step) {
	known := b.RunnersAt(step.Index)
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
			_ = known
		}
	}
	if hasRunner && (mains > 0 || companions > 0) {
		node := step.Value.Node
		if v, ok := step.Value.Get("runner"); ok {
			node = v.Node
		}
		c.Add(node, "runner: は他の runner と同じ step に書けません")
		return
	}
	if hasRunner {
		def, ok := step.Value.Get("runner")
		if !ok || def.Kind != runbook.KindMap || len(def.Map) == 0 {
			node := step.Value.Node
			if ok {
				node = def.Node
			}
			c.Add(node, "runner: は runner 名から定義への mapping です")
		}
		return
	}
	if mains > 1 {
		c.Add(step.Value.Node, "step の main runner は1つです")
	}
	if mains == 0 && companions == 0 {
		c.Add(step.Value.Node, "step に実行する runner がありません")
	}
}

func httpRequest(c *Context) {
	eachUserRunner(c, runbook.KindHTTP, func(step runbook.Step, key string, v runbook.Value) {
		checkHTTP(c, v)
	})
}

func grpcRequest(c *Context) {
	eachUserRunner(c, runbook.KindGRPC, func(step runbook.Step, key string, v runbook.Value) {
		checkGRPC(c, v)
	})
}

func dbRequest(c *Context) {
	eachUserRunner(c, runbook.KindDB, func(step runbook.Step, key string, v runbook.Value) {
		checkDB(c, v)
	})
}

func sshRequest(c *Context) {
	eachUserRunner(c, runbook.KindSSH, func(step runbook.Step, key string, v runbook.Value) {
		checkCommandMap(c, v, "SSH")
	})
}

func cdpRequest(c *Context) {
	eachUserRunner(c, runbook.KindCDP, func(step runbook.Step, key string, v runbook.Value) {
		checkCDP(c, v)
	})
}

func agentRequest(c *Context) {
	eachUserRunner(c, runbook.KindAgent, func(step runbook.Step, key string, v runbook.Value) {
		checkAgentStep(c, v)
	})
}

func execRequest(c *Context) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			v, ok := step.Value.Get("exec")
			if !ok {
				continue
			}
			checkExec(c, v)
		}
	}
}

func includeRequest(c *Context) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			v, ok := step.Value.Get("include")
			if !ok {
				continue
			}
			checkInclude(c, v)
		}
	}
}

type stepFn func(step runbook.Step, key string, v runbook.Value)

func eachUserRunner(c *Context, kind runbook.RunnerKind, fn stepFn) {
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
				fn(step, e.Key, e.Value)
			}
		}
	}
}

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

func checkHTTP(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap || len(v.Map) != 1 {
		c.Add(v.Node, "HTTP リクエストは path を1つだけ持つ mapping です")
		return
	}
	path := v.Map[0]
	if path.Key == "" {
		c.Add(path.KeyNode, "HTTP の path が空です")
	} else if !strings.Contains(path.Key, "{{") && !strings.HasPrefix(path.Key, "/") && !strings.Contains(path.Key, "://") {
		c.AddAt(finding.Warning, path.KeyNode, "HTTP の path %s は / から始めます", path.Key)
	}
	methods := path.Value
	if methods.Kind != runbook.KindMap || len(methods.Map) != 1 {
		c.Add(methods.Node, "HTTP リクエストのメソッドは1つです")
		return
	}
	method := methods.Map[0]
	name := strings.ToUpper(method.Key)
	if _, ok := httpMethods[name]; !ok {
		c.Add(method.KeyNode, "HTTP メソッド %s は使えません", method.Key)
	}
	if method.Value.Kind != runbook.KindMap {
		c.Add(method.Value.Node, "メソッド %s の値は mapping です", method.Key)
		return
	}
	allowKeys(c, method.Value, "HTTP リクエスト", map[string]struct{}{
		"headers": {}, "body": {}, "useCookie": {}, "trace": {},
	})
	if headers, ok := method.Value.Get("headers"); ok {
		checkHeaders(c, headers)
	}
	if cookie, ok := method.Value.Get("useCookie"); ok && !boolOrTemplate(cookie) {
		c.Add(cookie.Node, "useCookie は bool です")
	}
	if trace, ok := method.Value.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, "trace は bool です")
	}
	checkHTTPBody(c, name, method.Value)
}

func checkHeaders(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "headers は mapping です")
		return
	}
	for _, e := range v.Map {
		switch e.Value.Kind {
		case runbook.KindString:
		case runbook.KindSeq:
			for _, el := range e.Value.Seq {
				if el.Kind != runbook.KindString {
					c.Add(el.Node, "header %s の要素は文字列です", e.Key)
				}
			}
		default:
			c.Add(e.Value.Node, "header %s の値は文字列または文字列の配列です", e.Key)
		}
	}
}

func checkHTTPBody(c *Context, method string, req runbook.Value) {
	body, ok := req.Get("body")
	needsBody := method == "POST" || method == "PATCH"
	if !ok || body.Kind == runbook.KindNull {
		if needsBody {
			node := req.Node
			if ok {
				node = body.Node
			}
			c.Add(node, "%s には media type 付きの body が必要です", method)
		}
		return
	}
	if body.Kind == runbook.KindString && strings.Contains(body.String, "{{") {
		return
	}
	if body.Kind != runbook.KindMap || len(body.Map) != 1 {
		c.Add(body.Node, "body の media type は1つです")
		return
	}
	media := body.Map[0]
	if strings.TrimSpace(media.Key) == "" {
		c.Add(media.KeyNode, "body の media type が空です")
		return
	}
	if needsBody && media.Value.Kind == runbook.KindNull {
		c.Add(media.Value.Node, "%s の body が null です", method)
	}
	checkMediaPayload(c, media.Key, media.Value)
}

func checkMediaPayload(c *Context, media string, payload runbook.Value) {
	base := media
	if i := strings.Index(base, ";"); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	switch base {
	case "text/plain":
		if payload.Kind != runbook.KindString && payload.Kind != runbook.KindNull {
			c.Add(payload.Node, "text/plain の body は文字列です")
		}
	case "application/x-www-form-urlencoded":
		if payload.Kind != runbook.KindMap {
			c.Add(payload.Node, "application/x-www-form-urlencoded の body は mapping です")
		}
	case "multipart/form-data":
		if payload.Kind != runbook.KindMap && payload.Kind != runbook.KindSeq {
			c.Add(payload.Node, "multipart/form-data の body は mapping または配列です")
		}
	case "application/json":
		return
	default:
		if payload.Kind == runbook.KindMap {
			if _, ok := payload.Get("filename"); !ok {
				c.Add(payload.Node, "media type %s の mapping body には filename が必要です", base)
			}
		}
	}
}

func checkGRPC(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap || len(v.Map) != 1 {
		c.Add(v.Node, "gRPC リクエストは service/method を1つだけ持つ mapping です")
		return
	}
	call := v.Map[0]
	if !strings.Contains(call.Key, "{{") && !validGRPCMethod(call.Key) {
		c.Add(call.KeyNode, "gRPC のメソッド %s は package.Service/Method の形です", call.Key)
	}
	if call.Value.Kind != runbook.KindMap {
		c.Add(call.Value.Node, "gRPC メソッドの値は mapping です")
		return
	}
	allowKeys(c, call.Value, "gRPC リクエスト", map[string]struct{}{
		"headers": {}, "message": {}, "messages": {}, "timeout": {}, "trace": {},
	})
	if headers, ok := call.Value.Get("headers"); ok {
		checkHeaders(c, headers)
	}
	_, hasMessage := call.Value.Get("message")
	_, hasMessages := call.Value.Get("messages")
	if hasMessage && hasMessages {
		c.Add(call.Value.Node, "message と messages は同時に指定できません")
	}
	if msg, ok := call.Value.Get("message"); ok {
		if msg.Kind == runbook.KindString && strings.Contains(msg.String, "{{") {
			// 展開後に mapping になる
		} else if msg.Kind != runbook.KindMap {
			c.Add(msg.Node, "message は mapping です")
		}
	}
	if msgs, ok := call.Value.Get("messages"); ok {
		checkGRPCMessages(c, msgs)
	}
	if timeout, ok := call.Value.Get("timeout"); ok && !validDuration(timeout) {
		c.Add(timeout.Node, "timeout は時間です")
	}
	if trace, ok := call.Value.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, "trace は bool です")
	}
}

func validGRPCMethod(name string) bool {
	name = strings.TrimPrefix(name, "/")
	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return false
	}
	if parts[len(parts)-1] == "" || strings.Join(parts[:len(parts)-1], "/") == "" {
		return false
	}
	return true
}

func checkGRPCMessages(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindSeq {
		c.Add(v.Node, "messages は配列です")
		return
	}
	for _, el := range v.Seq {
		switch el.Kind {
		case runbook.KindMap:
		case runbook.KindString:
			if el.String != "receive" && el.String != "close" && !strings.Contains(el.String, "{{") {
				c.Add(el.Node, "messages の文字列は receive または close です")
			}
		default:
			c.Add(el.Node, "messages の要素は mapping、receive、close のいずれかです")
		}
	}
}

func checkDB(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "DB リクエストは mapping です")
		return
	}
	allowKeys(c, v, "DB リクエスト", map[string]struct{}{"query": {}, "trace": {}})
	query, ok := v.Get("query")
	if !ok || query.Kind != runbook.KindString || strings.TrimSpace(query.String) == "" {
		node := v.Node
		if ok {
			node = query.Node
		}
		c.Add(node, "query は空でない文字列です")
	}
	if trace, ok := v.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, "trace は bool です")
	}
}

func checkExec(c *Context, v runbook.Value) {
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "exec は mapping です")
		return
	}
	allowKeys(c, v, "exec", runnspec.DefKeys(runnspec.DefExec))
	checkCommand(c, v)
	if stdin, ok := v.Get("stdin"); ok && stdin.Kind != runbook.KindString {
		c.Add(stdin.Node, "stdin は文字列です")
	}
	if sh, ok := v.Get("shell"); ok {
		if sh.Kind != runbook.KindString {
			c.Add(sh.Node, "shell は文字列です")
		} else if !runbook.ContainsTemplate(sh.String) && sh.String != "bash" && sh.String != "sh" && !strings.Contains(sh.String, "{0}") {
			c.Add(sh.Node, "shell は bash、sh、または {0} を含むコマンドです")
		}
	}
	if bg, ok := v.Get("background"); ok && !boolOrTemplate(bg) {
		c.Add(bg.Node, "background は bool です")
	}
	if live, ok := v.Get("liveOutput"); ok && !boolOrTemplate(live) {
		c.Add(live.Node, "liveOutput は bool です")
	}
	if env, ok := v.Get("env"); ok {
		if env.Kind != runbook.KindMap {
			c.Add(env.Node, "env は mapping です")
		} else {
			for _, e := range env.Map {
				if e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, "env.%s は文字列です", e.Key)
				}
			}
		}
	}
}

func checkCommandMap(c *Context, v runbook.Value, what string) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "%s リクエストは mapping です", what)
		return
	}
	allowKeys(c, v, what, map[string]struct{}{"command": {}})
	checkCommand(c, v)
}

func checkCommand(c *Context, v runbook.Value) {
	cmd, ok := v.Get("command")
	if !ok || cmd.Kind != runbook.KindString || strings.TrimSpace(cmd.String) == "" {
		node := v.Node
		if ok {
			node = cmd.Node
		}
		c.Add(node, "command は空でない文字列です")
	}
}

func checkCDP(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "CDP リクエストは mapping です")
		return
	}
	allowKeys(c, v, "CDP リクエスト", map[string]struct{}{"actions": {}})
	actions, ok := v.Get("actions")
	if !ok || actions.Kind != runbook.KindSeq || len(actions.Seq) == 0 {
		node := v.Node
		if ok {
			node = actions.Node
		}
		c.Add(node, "actions は空でない配列です")
		return
	}
	for _, el := range actions.Seq {
		switch el.Kind {
		case runbook.KindString:
			args, known := cdpArgs[el.String]
			if !known {
				c.Add(el.Node, "CDP アクション %s はありません", el.String)
				continue
			}
			if len(args) > 0 {
				c.Add(el.Node, "CDP アクション %s には引数が必要です", el.String)
			}
		case runbook.KindMap:
			if len(el.Map) != 1 {
				c.Add(el.Node, "CDP アクションはキーを1つだけ持つ mapping です")
				continue
			}
			act := el.Map[0]
			args, known := cdpArgs[act.Key]
			if !known {
				c.Add(act.KeyNode, "CDP アクション %s はありません", act.Key)
				continue
			}
			switch act.Value.Kind {
			case runbook.KindString:
				if len(args) == 0 {
					c.Add(act.Value.Node, "CDP アクション %s は引数を取りません", act.Key)
				} else if len(args) > 1 {
					c.Add(act.Value.Node, "CDP アクション %s は %s を mapping で指定します", act.Key, strings.Join(args, ", "))
				}
			case runbook.KindMap:
				allowed := map[string]struct{}{}
				for _, name := range args {
					allowed[name] = struct{}{}
				}
				if len(allowed) == 0 && len(act.Value.Map) > 0 {
					c.Add(act.Value.Node, "CDP アクション %s は引数を取りません", act.Key)
					continue
				}
				allowKeys(c, act.Value, "CDP アクション "+act.Key, allowed)
			default:
				c.Add(act.Value.Node, "CDP アクション %s の引数は文字列または mapping です", act.Key)
			}
		default:
			c.Add(el.Node, "CDP アクションは文字列または mapping です")
		}
	}
}

func checkAgentStep(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, "agent リクエストは mapping です")
		return
	}
	allowKeys(c, v, "agent リクエスト", map[string]struct{}{"prompt": {}, "clearContext": {}})
	prompt, ok := v.Get("prompt")
	if !ok || prompt.Kind != runbook.KindString {
		node := v.Node
		if ok {
			node = prompt.Node
		}
		c.Add(node, "prompt は文字列です")
	}
	if cc, ok := v.Get("clearContext"); ok && !boolOrTemplate(cc) {
		c.Add(cc.Node, "clearContext は bool です")
	}
}

func checkInclude(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		if strings.TrimSpace(v.String) == "" {
			c.Add(v.Node, "include の path が空です")
		}
	case runbook.KindMap:
		_, path := v.Get("path")
		_, steps := v.Get("steps")
		if path && steps {
			c.Add(v.Node, "include では path と steps を同時に指定できません")
		}
		if !path && !steps {
			c.Add(v.Node, "include には path または steps が必要です")
		}
		if path {
			allowKeys(c, v, "include", map[string]struct{}{"path": {}, "vars": {}, "skipTest": {}, "force": {}})
			if p, ok := v.Get("path"); ok && !nonEmptyString(p) {
				c.Add(p.Node, "include.path は空でない文字列です")
			}
		}
		if steps && !path {
			allowKeys(c, v, "include", map[string]struct{}{
				"steps": {}, "desc": {}, "runners": {}, "vars": {}, "skipTest": {}, "force": {},
			})
		}
		if vars, ok := v.Get("vars"); ok && vars.Kind != runbook.KindMap {
			c.Add(vars.Node, "include.vars は mapping です")
		}
		if runners, ok := v.Get("runners"); ok && steps && runners.Kind != runbook.KindMap {
			c.Add(runners.Node, "include.runners は mapping です")
		}
		if desc, ok := v.Get("desc"); ok && desc.Kind != runbook.KindString {
			c.Add(desc.Node, "include.desc は文字列です")
		}
		for _, key := range []string{"skipTest", "force"} {
			if field, ok := v.Get(key); ok && !boolOrTemplate(field) {
				c.Add(field.Node, "include.%s は bool です", key)
			}
		}
	default:
		c.Add(v.Node, "include は path または mapping です")
	}
}
