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
	if _, err := os.Stat(path); err != nil {
		c.Add(v.Node, message.FileMissing, strings.TrimSpace(v.String))
	}
}
