// Package rules applies runn-specific rules to a runbook.
package rules

import (
	"path/filepath"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
	"gopkg.in/yaml.v3"
)

// Builtin is a check invoked from uses in YAML.
type Builtin func(*Context)

var registry = map[string]Builtin{}

func register(name string, fn Builtin) {
	registry[name] = fn
}

// KnownBuiltin reports whether name can be written in uses.
func KnownBuiltin(name string) bool {
	_, ok := registry[name]
	return ok
}

// CheckConfig reports whether every uses value is implemented.
func CheckConfig(cfg *config.Config) error {
	if cfg == nil {
		return message.New(message.ConfigAbsent)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	for _, rule := range cfg.Rules {
		if rule.Uses == "" || rule.Severity == finding.Off {
			continue
		}
		if !KnownBuiltin(rule.Uses) {
			return message.New(message.UsesUnknown, rule.ID, rule.Uses)
		}
	}
	return nil
}

// Lint checks src. The second result reports whether src was treated as a runbook.
func Lint(filename string, src []byte, cfg *config.Config) ([]finding.Finding, bool) {
	return LintWith(filename, src, cfg, nil)
}

// LintWith checks src, also using include references refs from other files. refs may be nil.
func LintWith(filename string, src []byte, cfg *config.Config, refs *Refs) ([]finding.Finding, bool) {
	lang := displayLang(cfg)
	doc, err := runbook.Parse(src)
	if err != nil {
		show, match := message.Pair(lang, err)
		return []finding.Finding{{
			File:     filename,
			Line:     1,
			Column:   1,
			Severity: finding.Error,
			Rule:     "parse",
			Message:  show,
			Match:    match,
		}}, false
	}
	if doc.Empty {
		return []finding.Finding{parseFinding(filename, lang, message.FileEmpty)}, false
	}
	if doc.Root.Kind != runbook.KindMap || doc.Book == nil {
		return []finding.Finding{parseFinding(filename, lang, message.BookNotMap)}, false
	}
	doc.Book.SetLocation(filename, filepath.Dir(filename))
	return applySuppressions(filename, src, lintDocument(doc, cfg, refs), cfg), doc.IsRunbook()
}

func lintDocument(doc *runbook.Document, cfg *config.Config, refs *Refs) []finding.Finding {
	var out []finding.Finding
	doc.Book.Walk(func(b *runbook.Book) {
		for _, p := range b.Problems {
			line, col := runbook.Pos(p.Node)
			out = append(out, finding.Finding{
				File:     b.File,
				Line:     line,
				Column:   col,
				Severity: finding.Error,
				Rule:     "parse",
				Message:  p.Message,
			})
		}
	})
	sites := analyze(doc.Book)
	ctx := &Context{Doc: doc, Sites: sites, Refs: refs, Lang: displayLang(cfg)}
	for _, rule := range cfg.Rules {
		if rule.Severity == finding.Off {
			continue
		}
		ctx.Rule = rule
		switch {
		case rule.Uses != "":
			if fn, ok := registry[rule.Uses]; ok {
				fn(ctx)
			}
		case rule.Check != nil:
			runCheck(ctx)
		}
	}
	out = append(out, ctx.out...)
	return out
}

// Context accumulates diagnostics while one rule runs.
type Context struct {
	Doc   *runbook.Document
	Rule  config.Rule
	Sites []site
	// Refs are includes from other files. Nil when a file is linted alone.
	Refs *Refs
	// Lang is the display language. The zero value shows Japanese.
	Lang message.Lang
	out  []finding.Finding
}

func (c *Context) Add(node *yaml.Node, id message.ID, args ...any) {
	c.AddAt(c.Rule.Severity, node, id, args...)
}

func (c *Context) AddAt(sev finding.Severity, node *yaml.Node, id message.ID, args ...any) {
	c.add(sev, node, message.Text(c.lang(), id, args...), message.English(id, args...))
}

// AddText records a sentence that is not translated, such as a user-written rule message.
func (c *Context) AddText(node *yaml.Node, text string) {
	c.add(c.Rule.Severity, node, text, text)
}

// AddError records err. A built-in error is shown in the display language and matched in English.
func (c *Context) AddError(node *yaml.Node, err error) {
	show, match := message.Pair(c.lang(), err)
	c.add(c.Rule.Severity, node, show, match)
}

func (c *Context) add(sev finding.Severity, node *yaml.Node, show, match string) {
	if sev == finding.Off {
		return
	}
	line, col := runbook.Pos(node)
	file := ""
	if c.Doc != nil && c.Doc.Book != nil {
		file = c.Doc.Book.File
	}
	c.out = append(c.out, finding.Finding{
		File:     file,
		Line:     line,
		Column:   col,
		Severity: sev,
		Rule:     c.Rule.ID,
		Message:  show,
		Match:    match,
	})
}

func (c *Context) lang() message.Lang {
	if c.Lang == "" {
		return message.JA
	}
	return c.Lang
}

func displayLang(cfg *config.Config) message.Lang {
	if cfg != nil && cfg.Language == string(message.EN) {
		return message.EN
	}
	return message.JA
}

func parseFinding(filename string, lang message.Lang, id message.ID) finding.Finding {
	return finding.Finding{
		File:     filename,
		Line:     1,
		Column:   1,
		Severity: finding.Error,
		Rule:     "parse",
		Message:  message.Text(lang, id),
		Match:    message.English(id),
	}
}

func (c *Context) Books() []*runbook.Book {
	var books []*runbook.Book
	if c.Doc == nil || c.Doc.Book == nil {
		return books
	}
	c.Doc.Book.Walk(func(b *runbook.Book) {
		books = append(books, b)
	})
	return books
}

func optionSet(opts map[string]any, key string) map[string]struct{} {
	out := map[string]struct{}{}
	if opts == nil {
		return out
	}
	raw, ok := opts[key]
	if !ok {
		return out
	}
	switch v := raw.(type) {
	case []any:
		for _, el := range v {
			if s, ok := el.(string); ok {
				out[s] = struct{}{}
			}
		}
	case []string:
		for _, s := range v {
			out[s] = struct{}{}
		}
	}
	return out
}

func optionBool(opts map[string]any, key string) bool {
	v, ok := opts[key].(bool)
	return ok && v
}
