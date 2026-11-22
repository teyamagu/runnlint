package rules

import (
	"os"
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
)

func init() {
	register("include-path", includePath)
	register("needs-path", needsPath)
}

func includePath(c *Context) {
	for _, b := range c.Books() {
		for _, step := range b.Steps {
			inc, ok := step.Value.Get("include")
			if !ok {
				continue
			}
			switch inc.Kind {
			case runbook.KindString:
				checkExists(c, b, inc)
			case runbook.KindMap:
				if path, ok := inc.Get("path"); ok {
					checkExists(c, b, path)
				}
			}
		}
		for _, r := range b.Runners {
			if r.Kind != runbook.KindInclude || r.Value.Kind != runbook.KindMap {
				continue
			}
			if path, ok := r.Value.Get("path"); ok {
				checkExists(c, b, path)
			}
		}
	}
}

func needsPath(c *Context) {
	for _, b := range c.Books() {
		needs, ok := b.Root.Get("needs")
		if !ok || needs.Kind != runbook.KindMap {
			continue
		}
		for _, e := range needs.Map {
			checkExists(c, b, e.Value)
		}
	}
}

func checkFileField(c *Context, b *runbook.Book, v runbook.Value, keys ...string) {
	for _, key := range keys {
		child, ok := v.Get(key)
		if !ok {
			continue
		}
		checkExists(c, b, child)
	}
}

func checkFileList(c *Context, b *runbook.Book, v runbook.Value, keys ...string) {
	for _, key := range keys {
		child, ok := v.Get(key)
		if !ok || child.Kind != runbook.KindSeq {
			continue
		}
		for _, el := range child.Seq {
			checkExists(c, b, el)
		}
	}
}

func checkExists(c *Context, b *runbook.Book, v runbook.Value) {
	if v.Kind != runbook.KindString {
		return
	}
	path, ok := resolveLocal(b, v.String)
	if !ok {
		return
	}
	if c != nil && c.Refs != nil {
		if abs, err := c.Refs.Abs(path); err == nil {
			path = abs
		}
	}
	if !fileExists(c, path) {
		c.Add(v.Node, message.FileMissing, strings.TrimSpace(v.String))
	}
}

// fileExists reports whether path is present. The result is remembered for this run.
func fileExists(c *Context, path string) bool {
	if c != nil && c.Refs != nil {
		if exists, ok := c.Refs.exists[path]; ok {
			return exists
		}
	}
	_, err := os.Stat(path)
	exists := err == nil
	if c != nil && c.Refs != nil {
		if c.Refs.exists == nil {
			c.Refs.exists = map[string]bool{}
		}
		c.Refs.exists[path] = exists
	}
	return exists
}
