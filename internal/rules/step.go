package rules

import (
	"strings"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
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
		c.Add(v.Node, message.IfExpr)
	}
	if v, ok := step.Value.Get("defer"); ok && !boolOrString(v) {
		c.Add(v.Node, message.DeferType)
	}
	if v, ok := step.Value.Get("force"); ok && !boolOrString(v) {
		c.Add(v.Node, message.ForceType)
	}
	if v, ok := step.Value.Get("test"); ok && !boolOrString(v) {
		c.Add(v.Node, message.TestExpr)
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
			c.Add(v.Node, message.DumpExprMiss)
		}
		if out, ok := v.Get("out"); ok && out.Kind != runbook.KindString {
			c.Add(out.Node, message.DumpOutType)
		}
		checkBoolish(c, v, "disableTrailingNewline", "disableMaskingSecrets")
	default:
		c.Add(v.Node, message.DumpType)
	}
}

func checkBind(c *Context, v runbook.Value) {
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.BindType)
		return
	}
	if len(v.Map) == 0 {
		c.Add(v.Node, message.BindEmpty)
	}
	// Values are not limited to string expressions. runn evaluates mappings and arrays recursively and binds anything else as-is.
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
		c.Add(v.Node, message.HTTPOnePath)
		return
	}
	path := v.Map[0]
	if path.Key == "" {
		c.Add(path.KeyNode, message.HTTPPathEmpty)
	} else if optionBool(c.Rule.Options, "require-leading-slash") && !runbook.ContainsTemplate(path.Key) && !strings.HasPrefix(path.Key, "/") && !strings.Contains(path.Key, "://") {
		// runn joins endpoint and path, so a path such as graphql still works. Enable this in options only when paths should be aligned.
		c.AddAt(finding.Warning, path.KeyNode, message.HTTPPathSlash, path.Key)
	}
	methods := path.Value
	if methods.Kind != runbook.KindMap || len(methods.Map) != 1 {
		c.Add(methods.Node, message.HTTPOneMethod)
		return
	}
	method := methods.Map[0]
	name := strings.ToUpper(method.Key)
	if _, ok := httpMethods[name]; !ok {
		c.Add(method.KeyNode, message.HTTPMethodBad, method.Key)
	}
	if method.Value.Kind != runbook.KindMap {
		c.Add(method.Value.Node, message.HTTPMethodMap, method.Key)
		return
	}
	allowKeys(c, method.Value, "HTTP リクエスト", map[string]struct{}{
		"headers": {}, "body": {}, "useCookie": {}, "trace": {},
	})
	if headers, ok := method.Value.Get("headers"); ok {
		checkHeaders(c, headers)
	}
	if cookie, ok := method.Value.Get("useCookie"); ok && !boolOrTemplate(cookie) {
		c.Add(cookie.Node, message.UseCookieBool)
	}
	if trace, ok := method.Value.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, message.TraceBool)
	}
	checkHTTPBody(c, name, method.Value)
}

func checkHeaders(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind == runbook.KindNull {
		// headers: with no value is the same as having no headers.
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.HeadersMap)
		return
	}
	for _, e := range v.Map {
		switch e.Value.Kind {
		case runbook.KindString:
		case runbook.KindSeq:
			for _, el := range e.Value.Seq {
				if el.Kind != runbook.KindString {
					c.Add(el.Node, message.HeaderElem, e.Key)
				}
			}
		default:
			c.Add(e.Value.Node, message.HeaderValue, e.Key)
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
			c.Add(node, message.BodyRequired, method)
		}
		return
	}
	if body.Kind == runbook.KindString && strings.Contains(body.String, "{{") {
		return
	}
	if body.Kind != runbook.KindMap || len(body.Map) != 1 {
		c.Add(body.Node, message.BodyOneType)
		return
	}
	media := body.Map[0]
	if strings.TrimSpace(media.Key) == "" {
		c.Add(media.KeyNode, message.BodyTypeEmpty)
		return
	}
	if needsBody && media.Value.Kind == runbook.KindNull {
		c.Add(media.Value.Node, message.BodyNull, method)
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
			c.Add(payload.Node, message.BodyText)
		}
	case "application/x-www-form-urlencoded":
		if payload.Kind != runbook.KindMap {
			c.Add(payload.Node, message.BodyForm)
		}
	case "multipart/form-data":
		if payload.Kind != runbook.KindMap && payload.Kind != runbook.KindSeq {
			c.Add(payload.Node, message.BodyMulti)
		}
	case "application/json":
		return
	default:
		if payload.Kind == runbook.KindMap {
			if _, ok := payload.Get("filename"); !ok {
				c.Add(payload.Node, message.BodyFilename, base)
			}
		}
	}
}

func checkGRPC(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap || len(v.Map) != 1 {
		c.Add(v.Node, message.GRPCOne)
		return
	}
	call := v.Map[0]
	if !strings.Contains(call.Key, "{{") && !validGRPCMethod(call.Key) {
		c.Add(call.KeyNode, message.GRPCShape, call.Key)
	}
	if call.Value.Kind != runbook.KindMap {
		c.Add(call.Value.Node, message.GRPCMap)
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
		c.Add(call.Value.Node, message.GRPCMsgBoth)
	}
	if msg, ok := call.Value.Get("message"); ok {
		if msg.Kind == runbook.KindString && strings.Contains(msg.String, "{{") {
			// expands to a mapping
		} else if msg.Kind != runbook.KindMap {
			c.Add(msg.Node, message.GRPCMessage)
		}
	}
	if msgs, ok := call.Value.Get("messages"); ok {
		checkGRPCMessages(c, msgs)
	}
	if timeout, ok := call.Value.Get("timeout"); ok && !validDuration(timeout) {
		c.Add(timeout.Node, message.TimeoutDur)
	}
	if trace, ok := call.Value.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, message.TraceBool)
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
		c.Add(v.Node, message.MessagesArray)
		return
	}
	for _, el := range v.Seq {
		switch el.Kind {
		case runbook.KindMap:
		case runbook.KindString:
			if el.String != "receive" && el.String != "close" && !strings.Contains(el.String, "{{") {
				c.Add(el.Node, message.MessagesWord)
			}
		default:
			c.Add(el.Node, message.MessagesElem)
		}
	}
}

func checkDB(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.DBMap)
		return
	}
	allowKeys(c, v, "DB リクエスト", map[string]struct{}{"query": {}, "trace": {}})
	query, ok := v.Get("query")
	if !ok || query.Kind != runbook.KindString || strings.TrimSpace(query.String) == "" {
		node := v.Node
		if ok {
			node = query.Node
		}
		c.Add(node, message.QueryEmpty)
	}
	if trace, ok := v.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, message.TraceBool)
	}
}

func checkExec(c *Context, v runbook.Value) {
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.ExecMap)
		return
	}
	allowKeys(c, v, "exec", runnspec.DefKeys(runnspec.DefExec))
	checkCommand(c, v)
	if stdin, ok := v.Get("stdin"); ok && stdin.Kind != runbook.KindString {
		c.Add(stdin.Node, message.StdinString)
	}
	if sh, ok := v.Get("shell"); ok {
		if sh.Kind != runbook.KindString {
			c.Add(sh.Node, message.ShellString)
		} else if !runbook.ContainsTemplate(sh.String) && sh.String != "bash" && sh.String != "sh" && !strings.Contains(sh.String, "{0}") {
			c.Add(sh.Node, message.ShellValue)
		}
	}
	if bg, ok := v.Get("background"); ok && !boolOrTemplate(bg) {
		c.Add(bg.Node, message.BackgroundBool)
	}
	if live, ok := v.Get("liveOutput"); ok && !boolOrTemplate(live) {
		c.Add(live.Node, message.LiveBool)
	}
	if env, ok := v.Get("env"); ok {
		if env.Kind != runbook.KindMap {
			c.Add(env.Node, message.EnvMap)
		} else {
			for _, e := range env.Map {
				if e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, message.EnvString, e.Key)
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
		c.Add(v.Node, message.RequestMap, what)
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
		c.Add(node, message.CommandEmpty)
	}
}

func checkCDP(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.CDPMap)
		return
	}
	allowKeys(c, v, "CDP リクエスト", map[string]struct{}{"actions": {}})
	actions, ok := v.Get("actions")
	if !ok || actions.Kind != runbook.KindSeq || len(actions.Seq) == 0 {
		node := v.Node
		if ok {
			node = actions.Node
		}
		c.Add(node, message.ActionsEmpty)
		return
	}
	for _, el := range actions.Seq {
		switch el.Kind {
		case runbook.KindString:
			args, known := cdpArgs[el.String]
			if !known {
				c.Add(el.Node, message.CDPUnknown, el.String)
				continue
			}
			if len(args) > 0 {
				c.Add(el.Node, message.CDPArgsNeed, el.String)
			}
		case runbook.KindMap:
			if len(el.Map) != 1 {
				c.Add(el.Node, message.CDPOneKey)
				continue
			}
			act := el.Map[0]
			args, known := cdpArgs[act.Key]
			if !known {
				c.Add(act.KeyNode, message.CDPUnknown, act.Key)
				continue
			}
			switch act.Value.Kind {
			case runbook.KindString:
				if len(args) == 0 {
					c.Add(act.Value.Node, message.CDPNoArgs, act.Key)
				} else if len(args) > 1 {
					c.Add(act.Value.Node, message.CDPArgMap, act.Key, strings.Join(args, ", "))
				}
			case runbook.KindMap:
				allowed := map[string]struct{}{}
				for _, name := range args {
					allowed[name] = struct{}{}
				}
				if len(allowed) == 0 && len(act.Value.Map) > 0 {
					c.Add(act.Value.Node, message.CDPNoArgs, act.Key)
					continue
				}
				allowKeys(c, act.Value, "CDP アクション "+act.Key, allowed)
			default:
				c.Add(act.Value.Node, message.CDPArgType, act.Key)
			}
		default:
			c.Add(el.Node, message.CDPElemType)
		}
	}
}

func checkAgentStep(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.AgentMap)
		return
	}
	allowKeys(c, v, "agent リクエスト", map[string]struct{}{"prompt": {}, "clearContext": {}})
	prompt, ok := v.Get("prompt")
	if !ok || prompt.Kind != runbook.KindString {
		node := v.Node
		if ok {
			node = prompt.Node
		}
		c.Add(node, message.PromptString)
	}
	if cc, ok := v.Get("clearContext"); ok && !boolOrTemplate(cc) {
		c.Add(cc.Node, message.ClearBool)
	}
}

func checkInclude(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		if strings.TrimSpace(v.String) == "" {
			c.Add(v.Node, message.IncludeEmpty)
		}
	case runbook.KindMap:
		_, path := v.Get("path")
		_, steps := v.Get("steps")
		if path && steps {
			c.Add(v.Node, message.IncludeBoth)
		}
		if !path && !steps {
			c.Add(v.Node, message.IncludeNeed)
		}
		if path {
			allowKeys(c, v, "include", map[string]struct{}{"path": {}, "vars": {}, "skipTest": {}, "force": {}})
			if p, ok := v.Get("path"); ok && !nonEmptyString(p) {
				c.Add(p.Node, message.IncludePath)
			}
		}
		if steps && !path {
			allowKeys(c, v, "include", map[string]struct{}{
				"steps": {}, "desc": {}, "runners": {}, "vars": {}, "skipTest": {}, "force": {},
			})
		}
		if vars, ok := v.Get("vars"); ok && vars.Kind != runbook.KindMap {
			c.Add(vars.Node, message.IncludeVars)
		}
		if runners, ok := v.Get("runners"); ok && steps && runners.Kind != runbook.KindMap {
			c.Add(runners.Node, message.IncludeRunners)
		}
		if desc, ok := v.Get("desc"); ok && desc.Kind != runbook.KindString {
			c.Add(desc.Node, message.IncludeDesc)
		}
		for _, key := range []string{"skipTest", "force"} {
			if field, ok := v.Get(key); ok && !boolOrTemplate(field) {
				c.Add(field.Node, message.IncludeBool, key)
			}
		}
	default:
		c.Add(v.Node, message.IncludeType)
	}
}
