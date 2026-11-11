package rules

import (
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
)

func init() {
	register("unexpanded-ref", unexpandedRef)
}

// unexpandedRoots are the store keys that a reference outside {{ }} most likely meant to read.
// included, env, cookies, params, nodes, runn, and i are left out because they also appear as ordinary words or names.
var unexpandedRoots = []string{"vars", "steps", "current", "previous", "parent", "needs"}

// fileExts are extensions that make a token such as vars.yml a file name, not a reference.
var fileExts = map[string]struct{}{
	"yml": {}, "yaml": {}, "json": {}, "jsonl": {}, "csv": {}, "tsv": {}, "txt": {}, "sql": {},
	"proto": {}, "md": {}, "xml": {}, "html": {}, "js": {}, "ts": {}, "sh": {}, "go": {}, "py": {},
	"rb": {}, "png": {}, "jpg": {}, "jpeg": {}, "gif": {}, "pdf": {}, "zip": {}, "env": {},
	"toml": {}, "ini": {}, "log": {}, "pem": {}, "crt": {}, "key": {},
}

// unexpandedRef reports a reference such as vars.x written outside {{ }} in a value that runn expands as a template.
// runn evaluates only test, if, bind, dump, loop.until, and loop.count as expressions. Other values, such as a request body,
// headers, a path key, exec.command, and include.vars, expand only the inside of {{ }}. A reference outside it is passed on as the literal string.
func unexpandedRef(c *Context) {
	for _, b := range c.Books() {
		for _, e := range b.Root.Map {
			switch e.Key {
			case "steps", "desc", "labels", "if", "loop", "skipTest", "force":
				// steps are read per step below. The others are text, labels, expressions, or flags.
				continue
			}
			checkUnexpanded(c, e.Value)
		}
		for i := range b.Steps {
			for _, e := range b.Steps[i].Value.Map {
				switch e.Key {
				case "desc", "if", "test", "bind", "loop":
					// Text or expressions.
				case "dump":
					// A string dump and dump.expr are expressions. dump.out is a template.
					if e.Value.Kind == runbook.KindMap {
						if out, ok := e.Value.Get("out"); ok {
							checkUnexpanded(c, out)
						}
					}
				case "include":
					checkIncludeUnexpanded(c, e.Value)
				default:
					checkUnexpanded(c, e.Value)
				}
			}
		}
	}
}

// checkIncludeUnexpanded reads path and vars of an include. An inline include with steps is read as its own runbook.
func checkIncludeUnexpanded(c *Context, v runbook.Value) {
	if v.Kind != runbook.KindMap {
		checkUnexpanded(c, v)
		return
	}
	if _, inline := v.Get("steps"); inline {
		return
	}
	for _, e := range v.Map {
		if e.Key == "skipTest" || e.Key == "force" {
			continue
		}
		checkUnexpanded(c, e.Value)
	}
}

func checkUnexpanded(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		if ref, ok := unexpandedRefIn(v.String); ok {
			c.Add(v.Node, message.RefUnexpanded, ref)
		}
	case runbook.KindSeq:
		for _, el := range v.Seq {
			checkUnexpanded(c, el)
		}
	case runbook.KindMap:
		for _, e := range v.Map {
			// runn expands {{ }} in mapping keys too, such as an HTTP path or a body field name.
			if e.KeyNode != nil {
				if ref, ok := unexpandedRefIn(e.Key); ok {
					c.Add(e.KeyNode, message.RefUnexpanded, ref)
				}
			}
			checkUnexpanded(c, e.Value)
		}
	}
}

// unexpandedRefIn returns the first reference in s that is outside {{ }}.
// A string whose {{ is not closed is left to expr-syntax.
func unexpandedRefIn(s string) (string, bool) {
	if !strings.Contains(s, ".") && !strings.Contains(s, "[") {
		return "", false
	}
	plain, ok := outsideTemplates(s)
	if !ok {
		return "", false
	}
	for i := 0; i < len(plain); i++ {
		if i > 0 && !refBoundary(plain[i-1]) {
			continue
		}
		root := refRootAt(plain, i)
		if root == "" {
			continue
		}
		raw := refToken(plain[i:])
		if strings.Contains(raw, "@") {
			// An e-mail address such as parent.name@example.com.
			continue
		}
		ref := strings.TrimRight(cutAny(raw, "/:=&?!*+"), ".")
		if isFileName(ref, root) {
			continue
		}
		return ref, true
	}
	return "", false
}

// outsideTemplates replaces each {{ }} in s with a space. The second result is false when a {{ is not closed.
func outsideTemplates(s string) (string, bool) {
	if !strings.Contains(s, "{{") {
		return s, true
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			_, end, ok := scanTemplate(s, i+2)
			if !ok {
				return "", false
			}
			b.WriteByte(' ')
			i = end - 1
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String(), true
}

// refBoundary reports whether c can come right before a reference.
// A letter, digit, _, ., $, or - before it makes the text part of a longer name, a host, a jq path, or a shell variable.
func refBoundary(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return false
	case c == '_', c == '.', c == '$', c == '-':
		return false
	}
	return true
}

// refRootAt returns the root name when a reference such as vars.x, steps[0], or steps['a'] starts at s[i:].
func refRootAt(s string, i int) string {
	for _, root := range unexpandedRoots {
		if !strings.HasPrefix(s[i:], root) {
			continue
		}
		j := i + len(root)
		if j+1 >= len(s) {
			return ""
		}
		next := s[j+1]
		switch s[j] {
		case '.':
			if next == '_' || next >= 'a' && next <= 'z' || next >= 'A' && next <= 'Z' {
				return root
			}
		case '[':
			if next >= '0' && next <= '9' || next == '\'' || next == '"' {
				return root
			}
		}
		return ""
	}
	return ""
}

// refToken returns s up to a space, a quote, or a delimiter that cannot be part of a reference.
// Inside [ ], such as steps['a b'], spaces and quotes belong to the reference.
func refToken(s string) string {
	depth := 0
	for j := 0; j < len(s); j++ {
		switch s[j] {
		case '[':
			depth++
			continue
		case ']':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth > 0 {
			continue
		}
		switch s[j] {
		case ' ', '\t', '\n', '\r', '"', '\'', '`', ',', ';', '(', ')', '<', '>', '|':
			return s[:j]
		}
	}
	return s
}

func cutAny(s, chars string) string {
	if i := strings.IndexAny(s, chars); i >= 0 {
		return s[:i]
	}
	return s
}

// isFileName reports whether ref is a file name such as vars.yml or steps.json.
func isFileName(ref, root string) bool {
	rest := strings.TrimPrefix(ref, root+".")
	if rest == ref || strings.ContainsAny(rest, ".[") {
		return false
	}
	_, ok := fileExts[strings.ToLower(rest)]
	return ok
}
