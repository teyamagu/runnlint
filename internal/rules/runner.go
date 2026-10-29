package rules

import (
	"net/url"
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
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
				c.Add(r.NameNode, message.RunnerReserved, r.Name)
			}
		}
		for _, step := range b.Steps {
			def, ok := step.Value.Get("runner")
			if !ok || def.Kind != runbook.KindMap {
				continue
			}
			for _, e := range def.Map {
				if reservedRunner(e.Key) {
					c.Add(e.KeyNode, message.RunnerReserved, e.Key)
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
				c.Add(e.KeyNode, message.RunnerMissing, e.Key)
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
				c.Add(r.Value.Node, message.RunnerEndpoint, r.Name)
			}
		}
	case runbook.KindMap:
		if r.Kind == runbook.KindUnknown {
			c.Add(r.Value.Node, message.RunnerKind, r.Name)
			return
		}
		checkRunnerMap(c, r)
	default:
		c.Add(r.Value.Node, message.RunnerType, r.Name)
	}
}

func checkHTTPURL(c *Context, name string, v runbook.Value) {
	if runbook.ContainsTemplate(v.String) {
		return
	}
	u, err := url.Parse(v.String)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		c.Add(v.Node, message.RunnerURL, name)
	}
}

func checkDBString(c *Context, r runbook.Runner) {
	s := strings.TrimSpace(r.Value.String)
	if s == "" {
		c.Add(r.Value.Node, message.RunnerDSNEmpty, r.Name)
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
	c.Add(r.Value.Node, message.RunnerScheme, r.Name)
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
				c.Add(ep.Node, message.RunnerEpString, r.Name)
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
			c.Add(r.Value.Node, message.RunnerHost, r.Name)
		}
		if file, ok := r.Value.Get("identityFile"); ok && nonEmptyString(file) {
			if key, ok := r.Value.Get("identityKey"); ok && nonEmptyString(key) {
				c.Add(key.Node, message.RunnerIdentity, r.Name)
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
			c.Add(flags.Node, message.RunnerFlags, r.Name)
		}
	case runbook.KindInclude:
		if params, ok := r.Value.Get("params"); ok && params.Kind != runbook.KindMap {
			c.Add(params.Node, message.RunnerParams, r.Name)
		}
	case runbook.KindAgent:
		// The schema allows agent to be any string. Accepted values follow the runn implementation.
		if agent, ok := r.Value.Get("agent"); ok && agent.Kind == runbook.KindString && !runbook.ContainsTemplate(agent.String) {
			switch agent.String {
			case "claude", "copilot", "codex":
			default:
				c.Add(agent.Node, message.AgentName)
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
		c.Add(e.KeyNode, message.NoKey, what, e.Key)
	}
}

func checkRequiredString(c *Context, v runbook.Value, key, what string) {
	child, ok := v.Get(key)
	if !ok || !nonEmptyString(child) {
		node := v.Node
		if ok {
			node = child.Node
		}
		c.Add(node, message.NeedKey, what, key)
	}
}

func checkOptionalDuration(c *Context, v runbook.Value, key string) {
	child, ok := v.Get(key)
	if !ok {
		return
	}
	if !validDuration(child) {
		c.Add(child.Node, message.IsDuration, key)
	}
}

func checkBoolish(c *Context, v runbook.Value, keys ...string) {
	for _, key := range keys {
		child, ok := v.Get(key)
		if !ok {
			continue
		}
		if !boolOrString(child) {
			c.Add(child.Node, message.FieldBoolOrStr, key)
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
			c.Add(enable.Node, message.TraceEnable)
		}
		if name, ok := child.Get("headerName"); ok && name.Kind != runbook.KindString {
			c.Add(name.Node, message.TraceHeader)
		}
	default:
		c.Add(child.Node, message.TraceType)
	}
}

func checkStringList(c *Context, v runbook.Value, keys ...string) {
	for _, key := range keys {
		child, ok := v.Get(key)
		if !ok {
			continue
		}
		if child.Kind != runbook.KindSeq {
			c.Add(child.Node, message.FieldStrings, key)
			continue
		}
		for _, el := range child.Seq {
			if el.Kind != runbook.KindString {
				c.Add(el.Node, message.FieldElem, key)
			}
		}
	}
}

func checkPort(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindNumber:
		if strings.HasPrefix(v.Number, "-") {
			c.Add(v.Node, message.PortPositive)
		}
	case runbook.KindString:
		if runbook.ContainsTemplate(v.String) || numOnlyRe.MatchString(strings.TrimSpace(v.String)) {
			return
		}
		c.Add(v.Node, message.PortNumber)
	default:
		c.Add(v.Node, message.PortNumber)
	}
}

func checkKeyboard(c *Context, v runbook.Value) {
	child, ok := v.Get("keyboardInteractive")
	if !ok {
		return
	}
	if child.Kind != runbook.KindSeq {
		c.Add(child.Node, message.KbdArray)
		return
	}
	for _, el := range child.Seq {
		if el.Kind != runbook.KindMap {
			c.Add(el.Node, message.KbdElem)
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
				c.Add(node, message.KbdString, key)
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
