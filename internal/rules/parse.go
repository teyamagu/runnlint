package rules

import (
	"strings"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
	"github.com/teyamagu/runnlint/internal/runnspec"
)

func init() {
	register("http-request", httpRequest)
	register("grpc-request", grpcRequest)
	register("db-request", dbRequest)
	register("exec-request", execRequest)
	register("ssh-request", sshRequest)
	register("cdp-request", cdpRequest)
	register("agent-request", agentRequest)
	register("include-request", includeRequest)
}

func httpRequest(c *Context) {
	eachUserRunner(c, runbook.KindHTTP, func(v runbook.Value) {
		checkHTTP(c, v)
	})
}

func grpcRequest(c *Context) {
	eachUserRunner(c, runbook.KindGRPC, func(v runbook.Value) {
		checkGRPC(c, v)
	})
}

func dbRequest(c *Context) {
	eachUserRunner(c, runbook.KindDB, func(v runbook.Value) {
		checkDB(c, v)
	})
}

func sshRequest(c *Context) {
	eachUserRunner(c, runbook.KindSSH, func(v runbook.Value) {
		checkSSH(c, v)
	})
}

func cdpRequest(c *Context) {
	eachUserRunner(c, runbook.KindCDP, func(v runbook.Value) {
		checkCDP(c, v)
	})
}

func agentRequest(c *Context) {
	eachUserRunner(c, runbook.KindAgent, func(v runbook.Value) {
		checkAgent(c, v)
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

// checkHTTP follows parseHTTPRequest. One path, one method, then headers, body, useCookie, and trace.
// A string that still contains {{ }} is expanded before runn parses it, so the shape is not checked.
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
	if body, ok := method.Value.Get("body"); ok || name == "POST" || name == "PATCH" {
		checkHTTPBody(c, name, method.Value, body, ok)
	}
	if cookie, ok := method.Value.Get("useCookie"); ok && !boolOrTemplate(cookie) {
		c.Add(cookie.Node, message.UseCookieBool)
	}
	if trace, ok := method.Value.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, message.TraceBool)
	}
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

// checkHTTPBody follows the body branch of parseHTTPRequest, then the media type runn accepts.
func checkHTTPBody(c *Context, method string, req, body runbook.Value, ok bool) {
	needsBody := method == "POST" || method == "PATCH"
	if !ok || body.Kind == runbook.KindNull {
		if needsBody {
			c.Add(here(req, body, ok), message.BodyRequired, method)
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

// checkGRPC follows parseGrpcRequest: one service/method, then headers, timeout, message, messages, and trace.
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
	if timeout, ok := call.Value.Get("timeout"); ok && !validDuration(timeout) {
		c.Add(timeout.Node, message.TimeoutDur)
	}
	_, hasMessage := call.Value.Get("message")
	_, hasMessages := call.Value.Get("messages")
	if hasMessage && hasMessages {
		c.Add(call.Value.Node, message.GRPCMsgBoth)
	}
	if msg, ok := call.Value.Get("message"); ok && msg.Kind != runbook.KindMap && !(msg.Kind == runbook.KindString && strings.Contains(msg.String, "{{")) {
		c.Add(msg.Node, message.GRPCMessage)
	}
	if msgs, ok := call.Value.Get("messages"); ok {
		checkGRPCMessages(c, msgs)
	}
	if trace, ok := call.Value.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, message.TraceBool)
	}
}

// validGRPCMethod is parseServiceAndMethod without the split result. An empty service or method is also rejected.
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

// checkDB follows parseDBQuery: a mapping with query, then trace.
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
		c.Add(here(v, query, ok), message.QueryEmpty)
	}
	if trace, ok := v.Get("trace"); ok && !boolOrTemplate(trace) {
		c.Add(trace.Node, message.TraceBool)
	}
}

// checkExec follows parseExecCommand: command, stdin, shell, background, liveOutput, then env.
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

// checkSSH follows parseSSHCommand: a mapping whose only payload key is command.
func checkSSH(c *Context, v runbook.Value) {
	if v.Kind == runbook.KindString && strings.Contains(v.String, "{{") {
		return
	}
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.RequestMap, "SSH")
		return
	}
	allowKeys(c, v, "SSH", map[string]struct{}{"command": {}})
	checkCommand(c, v)
}

func checkCommand(c *Context, v runbook.Value) {
	cmd, ok := v.Get("command")
	if !ok || cmd.Kind != runbook.KindString || strings.TrimSpace(cmd.String) == "" {
		c.Add(here(v, cmd, ok), message.CommandEmpty)
	}
}

// checkCDP follows parseCDPActions: a mapping with a non-empty actions array.
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
		c.Add(here(v, actions, ok), message.ActionsEmpty)
		return
	}
	for _, el := range actions.Seq {
		checkCDPAction(c, el)
	}
}

func checkCDPAction(c *Context, el runbook.Value) {
	switch el.Kind {
	case runbook.KindString:
		args, known := cdpArgs[el.String]
		if !known {
			c.Add(el.Node, message.CDPUnknown, el.String)
			return
		}
		if len(args) > 0 {
			c.Add(el.Node, message.CDPArgsNeed, el.String)
		}
	case runbook.KindMap:
		if len(el.Map) != 1 {
			c.Add(el.Node, message.CDPOneKey)
			return
		}
		act := el.Map[0]
		args, known := cdpArgs[act.Key]
		if !known {
			c.Add(act.KeyNode, message.CDPUnknown, act.Key)
			return
		}
		checkCDPArgs(c, act.Key, act.Value, args)
	default:
		c.Add(el.Node, message.CDPElemType)
	}
}

func checkCDPArgs(c *Context, name string, v runbook.Value, args []string) {
	switch v.Kind {
	case runbook.KindString:
		if len(args) == 0 {
			c.Add(v.Node, message.CDPNoArgs, name)
		} else if len(args) > 1 {
			c.Add(v.Node, message.CDPArgMap, name, strings.Join(args, ", "))
		}
	case runbook.KindMap:
		allowed := map[string]struct{}{}
		for _, arg := range args {
			allowed[arg] = struct{}{}
		}
		if len(allowed) == 0 && len(v.Map) > 0 {
			c.Add(v.Node, message.CDPNoArgs, name)
			return
		}
		allowKeys(c, v, "CDP アクション "+name, allowed)
	default:
		c.Add(v.Node, message.CDPArgType, name)
	}
}

// checkAgent follows parseAgentRequest: prompt, then clearContext.
func checkAgent(c *Context, v runbook.Value) {
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
		c.Add(here(v, prompt, ok), message.PromptString)
	}
	if cc, ok := v.Get("clearContext"); ok && !boolOrTemplate(cc) {
		c.Add(cc.Node, message.ClearBool)
	}
}

// checkInclude follows parseIncludeConfig: a string path, or a mapping with path or steps, not both.
func checkInclude(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		if strings.TrimSpace(v.String) == "" {
			c.Add(v.Node, message.IncludeEmpty)
		}
	case runbook.KindMap:
		path, pathOK := v.Get("path")
		_, steps := v.Get("steps")
		if pathOK && steps {
			c.Add(v.Node, message.IncludeBoth)
		}
		if !pathOK && !steps {
			c.Add(v.Node, message.IncludeNeed)
		}
		if pathOK {
			allowKeys(c, v, "include", map[string]struct{}{"path": {}, "vars": {}, "skipTest": {}, "force": {}})
			if !nonEmptyString(path) {
				c.Add(path.Node, message.IncludePath)
			}
		}
		if steps && !pathOK {
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
