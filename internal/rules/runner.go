package rules

import (
	"net/url"
	"strings"

	"github.com/teyamagu/runnlint/internal/runbook"
	"github.com/teyamagu/runnlint/internal/runnspec"
)

func init() {
	register("runner-reserved", runnerReserved)
	register("runner-config", runnerConfig)
	register("runner-defined", runnerDefined)
	register("local-path", localPath)
}

func reservedRunner(name string) bool {
	_, ok := runnspec.DefKeys(runnspec.DefStep)[name]
	return ok
}

var dbSchemes = map[string]struct{}{
	"postgres": {}, "postgresql": {}, "pg": {},
	"mysql": {}, "my": {},
	"sqlite": {}, "sqlite3": {}, "sq": {},
	"spanner": {}, "sp": {},
	"file": {},
}

func runnerReserved(c *Context) {
	for _, b := range c.Books() {
		for _, r := range b.Runners {
			if reservedRunner(r.Name) {
				c.Add(r.NameNode, "runner 名 %s は runn の予約語です", r.Name)
			}
		}
		for _, step := range b.Steps {
			def, ok := step.Value.Get("runner")
			if !ok || def.Kind != runbook.KindMap {
				continue
			}
			for _, e := range def.Map {
				if reservedRunner(e.Key) {
					c.Add(e.KeyNode, "runner 名 %s は runn の予約語です", e.Key)
				}
			}
		}
	}
}

func runnerDefined(c *Context) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			known := b.RunnersAt(step.Index)
			for _, e := range step.Value.Map {
				if isStepKeyword(e.Key) {
					continue
				}
				if _, ok := known[e.Key]; ok {
					continue
				}
				c.Add(e.KeyNode, "runner %s は runners にも先行する runner: にもありません", e.Key)
			}
		}
	}
}

func isStepKeyword(key string) bool {
	if key == "<<" {
		return true
	}
	_, ok := runnspec.DefKeys(runnspec.DefStep)[key]
	return ok
}

func runnerConfig(c *Context) {
	for _, b := range c.Books() {
		for _, r := range b.Runners {
			checkRunner(c, r)
		}
		for _, step := range b.Steps {
			def, ok := step.Value.Get("runner")
			if !ok || def.Kind != runbook.KindMap {
				continue
			}
			for _, e := range def.Map {
				r := runbook.Runner{
					Name:     e.Key,
					NameNode: e.KeyNode,
					Value:    e.Value,
					Kind:     runbook.DetectKind(e.Value),
					FromStep: step.Index,
				}
				checkRunner(c, r)
			}
		}
	}
}

func checkRunner(c *Context, r runbook.Runner) {
	switch r.Value.Kind {
	case runbook.KindString:
		switch r.Kind {
		case runbook.KindDB:
			checkDBString(c, r)
		case runbook.KindHTTP:
			checkHTTPURL(c, r.Name, r.Value)
		case runbook.KindUnknown:
			if !runbook.ContainsTemplate(r.Value.String) {
				c.Add(r.Value.Node, "runner %s の値は http(s)、grpc、ssh、cdp、chrome、または DB の DSN ではありません", r.Name)
			}
		}
	case runbook.KindMap:
		if r.Kind == runbook.KindUnknown {
			c.Add(r.Value.Node, "runner %s の種別を判定できません", r.Name)
			return
		}
		checkRunnerMap(c, r)
	default:
		c.Add(r.Value.Node, "runner %s の値は文字列または mapping です", r.Name)
	}
}

func checkHTTPURL(c *Context, name string, v runbook.Value) {
	if runbook.ContainsTemplate(v.String) {
		return
	}
	u, err := url.Parse(v.String)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		c.Add(v.Node, "runner %s の endpoint は http(s) の URL です", name)
	}
}

func checkDBString(c *Context, r runbook.Runner) {
	s := strings.TrimSpace(r.Value.String)
	if s == "" {
		c.Add(r.Value.Node, "runner %s の DSN が空です", r.Name)
		return
	}
	if runbook.ContainsTemplate(s) || strings.Contains(s, "@tcp(") || strings.Contains(s, "@") && !strings.Contains(s, "://") {
		return
	}
	if i := strings.Index(s, "://"); i > 0 {
		scheme := s[:i]
		if _, ok := dbSchemes[scheme]; ok {
			return
		}
	}
	c.Add(r.Value.Node, "runner %s の値は既知の runner スキームでも DB の DSN でもありません", r.Name)
}

func checkRunnerMap(c *Context, r runbook.Runner) {
	if def := runnerDef(r.Kind); def != "" {
		allowKeys(c, r.Value, "runner "+r.Name, runnspec.DefKeys(def))
		checkSchemaRequired(c, r.Value, def, "runner "+r.Name)
	}
	switch r.Kind {
	case runbook.KindHTTP:
		if ep, ok := r.Value.Get("endpoint"); ok {
			if ep.Kind == runbook.KindString {
				checkHTTPURL(c, r.Name, ep)
			} else {
				c.Add(ep.Node, "runner %s の endpoint は文字列です", r.Name)
			}
		}
		checkOptionalDuration(c, r.Value, "timeout")
		checkTrace(c, r.Value)
		checkBoolish(c, r.Value, "skipValidateRequest", "skipValidateResponse", "skipCircularReferenceCheck", "notFollowRedirect", "skipVerify", "useCookie")
	case runbook.KindGRPC:
		checkStringList(c, r.Value, "importPaths", "protos", "bufDirs", "bufLocks", "bufConfigs", "bufModules")
		checkBoolish(c, r.Value, "tls", "skipVerify")
		checkTrace(c, r.Value)
	case runbook.KindDB:
		checkBoolish(c, r.Value, "trace")
	case runbook.KindSSH:
		_, host := r.Value.Get("host")
		_, hostname := r.Value.Get("hostname")
		if !host && !hostname {
			c.Add(r.Value.Node, "runner %s には host または hostname が必要です", r.Name)
		}
		if file, ok := r.Value.Get("identityFile"); ok && nonEmptyString(file) {
			if key, ok := r.Value.Get("identityKey"); ok && nonEmptyString(key) {
				c.Add(key.Node, "runner %s では identityFile と identityKey を同時に指定できません", r.Name)
			}
		}
		if port, ok := r.Value.Get("port"); ok {
			checkPort(c, port)
		}
		checkBoolish(c, r.Value, "keepSession")
		checkKeyboard(c, r.Value)
	case runbook.KindCDP:
		checkOptionalDuration(c, r.Value, "timeout")
		if flags, ok := r.Value.Get("flags"); ok && flags.Kind != runbook.KindMap {
			c.Add(flags.Node, "runner %s の flags は mapping です", r.Name)
		}
	case runbook.KindInclude:
		if params, ok := r.Value.Get("params"); ok && params.Kind != runbook.KindMap {
			c.Add(params.Node, "runner %s の params は mapping です", r.Name)
		}
	case runbook.KindAgent:
		// スキーマは agent を任意の文字列にしている。受け付ける値は runn の実装に合わせる。
		if agent, ok := r.Value.Get("agent"); ok && agent.Kind == runbook.KindString && !runbook.ContainsTemplate(agent.String) {
			switch agent.String {
			case "claude", "copilot", "codex":
			default:
				c.Add(agent.Node, "agent は claude、copilot、codex のいずれかです")
			}
		}
		checkStringList(c, r.Value, "tools", "permissions")
		checkBoolish(c, r.Value, "interactive")
	}
}

func runnerDef(kind runbook.RunnerKind) string {
	switch kind {
	case runbook.KindHTTP:
		return runnspec.DefHTTPRunner
	case runbook.KindGRPC:
		return runnspec.DefGRPCRunner
	case runbook.KindDB:
		return runnspec.DefDBRunner
	case runbook.KindCDP:
		return runnspec.DefCDPRunner
	case runbook.KindSSH:
		return runnspec.DefSSHRunner
	case runbook.KindInclude:
		return runnspec.DefIncludeRun
	case runbook.KindAgent:
		return runnspec.DefAgentRunner
	default:
		return ""
	}
}

func checkSchemaRequired(c *Context, v runbook.Value, def, what string) {
	for _, key := range runnspec.Required(def) {
		checkRequiredString(c, v, key, what)
	}
}

func allowKeys(c *Context, v runbook.Value, what string, allowed map[string]struct{}) {
	for _, e := range v.Map {
		if _, ok := allowed[e.Key]; ok || e.Key == "<<" {
			continue
		}
		c.Add(e.KeyNode, "%s にキー %s はありません", what, e.Key)
	}
}

func checkRequiredString(c *Context, v runbook.Value, key, what string) {
	child, ok := v.Get(key)
	if !ok || !nonEmptyString(child) {
		node := v.Node
		if ok {
			node = child.Node
		}
		c.Add(node, "%s には %s が必要です", what, key)
	}
}

func checkOptionalDuration(c *Context, v runbook.Value, key string) {
	child, ok := v.Get(key)
	if !ok {
		return
	}
	if !validDuration(child) {
		c.Add(child.Node, "%s は時間です", key)
	}
}

func checkBoolish(c *Context, v runbook.Value, keys ...string) {
	for _, key := range keys {
		child, ok := v.Get(key)
		if !ok {
			continue
		}
		if !boolOrString(child) {
			c.Add(child.Node, "%s は bool または文字列です", key)
		}
	}
}

func checkTrace(c *Context, v runbook.Value) {
	child, ok := v.Get("trace")
	if !ok {
		return
	}
	switch child.Kind {
	case runbook.KindBool, runbook.KindString:
		return
	case runbook.KindMap:
		allowKeys(c, child, "trace", runnspec.DefKeys(runnspec.DefTrace))
		if enable, ok := child.Get("enable"); ok && !boolOrString(enable) {
			c.Add(enable.Node, "trace.enable は bool または文字列です")
		}
		if name, ok := child.Get("headerName"); ok && name.Kind != runbook.KindString {
			c.Add(name.Node, "trace.headerName は文字列です")
		}
	default:
		c.Add(child.Node, "trace は bool、文字列、または mapping です")
	}
}

func checkStringList(c *Context, v runbook.Value, keys ...string) {
	for _, key := range keys {
		child, ok := v.Get(key)
		if !ok {
			continue
		}
		if child.Kind != runbook.KindSeq {
			c.Add(child.Node, "%s は文字列の配列です", key)
			continue
		}
		for _, el := range child.Seq {
			if el.Kind != runbook.KindString {
				c.Add(el.Node, "%s の要素は文字列です", key)
			}
		}
	}
}

func checkPort(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindNumber:
		if strings.HasPrefix(v.Number, "-") {
			c.Add(v.Node, "port は正の数です")
		}
	case runbook.KindString:
		if runbook.ContainsTemplate(v.String) || numOnlyRe.MatchString(strings.TrimSpace(v.String)) {
			return
		}
		c.Add(v.Node, "port は数値です")
	default:
		c.Add(v.Node, "port は数値です")
	}
}

func checkKeyboard(c *Context, v runbook.Value) {
	child, ok := v.Get("keyboardInteractive")
	if !ok {
		return
	}
	if child.Kind != runbook.KindSeq {
		c.Add(child.Node, "keyboardInteractive は配列です")
		return
	}
	for _, el := range child.Seq {
		if el.Kind != runbook.KindMap {
			c.Add(el.Node, "keyboardInteractive の要素は mapping です")
			continue
		}
		allowKeys(c, el, "keyboardInteractive", map[string]struct{}{"match": {}, "answer": {}})
		for _, key := range []string{"match", "answer"} {
			field, ok := el.Get(key)
			if !ok || field.Kind != runbook.KindString {
				node := el.Node
				if ok {
					node = field.Node
				}
				c.Add(node, "keyboardInteractive.%s は文字列です", key)
			}
		}
	}
}

func localPath(c *Context) {
	for _, b := range c.Books() {
		for _, r := range b.Runners {
			if r.Value.Kind != runbook.KindMap {
				continue
			}
			switch r.Kind {
			case runbook.KindHTTP:
				checkFileField(c, b, r.Value, "openapi3", "cacert", "cert", "key")
			case runbook.KindGRPC:
				checkFileField(c, b, r.Value, "cacert", "cert", "key")
				checkFileList(c, b, r.Value, "importPaths", "protos", "bufDirs", "bufLocks", "bufConfigs", "bufModules")
			case runbook.KindSSH:
				checkFileField(c, b, r.Value, "sshConfig", "identityFile")
			}
		}
	}
}
