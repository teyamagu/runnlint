// Package rules は runbook に runn 固有のルールを適用する。
package rules

import (
	"fmt"
	"path/filepath"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/runbook"
	"gopkg.in/yaml.v3"
)

// Builtin は YAML の uses から呼ばれる検査。
type Builtin func(*Context)

var registry = map[string]Builtin{}

func register(name string, fn Builtin) {
	registry[name] = fn
}

// KnownBuiltin は uses に書ける名前か。
func KnownBuiltin(name string) bool {
	_, ok := registry[name]
	return ok
}

// CheckConfig は uses が実装済みか確認する。
func CheckConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("設定がありません")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	for _, rule := range cfg.Rules {
		if rule.Uses == "" || rule.Severity == finding.Off {
			continue
		}
		if !KnownBuiltin(rule.Uses) {
			return fmt.Errorf("ルール %s の uses %q は未定義です", rule.ID, rule.Uses)
		}
	}
	return nil
}

// Lint は src を検査する。2つ目の戻り値は runbook とみなしたか。
func Lint(filename string, src []byte, cfg *config.Config) ([]finding.Finding, bool) {
	doc, err := runbook.Parse(src)
	if err != nil {
		return []finding.Finding{{
			File:     filename,
			Line:     1,
			Column:   1,
			Severity: finding.Error,
			Rule:     "parse",
			Message:  err.Error(),
		}}, false
	}
	if doc.Empty {
		return []finding.Finding{{
			File:     filename,
			Line:     1,
			Column:   1,
			Severity: finding.Error,
			Rule:     "parse",
			Message:  "空のファイルです",
		}}, false
	}
	if doc.Root.Kind != runbook.KindMap || doc.Book == nil {
		return []finding.Finding{{
			File:     filename,
			Line:     1,
			Column:   1,
			Severity: finding.Error,
			Rule:     "parse",
			Message:  "runbook は mapping です",
		}}, false
	}
	doc.Book.SetLocation(filename, filepath.Dir(filename))
	return lintDocument(doc, cfg), doc.IsRunbook()
}

func lintDocument(doc *runbook.Document, cfg *config.Config) []finding.Finding {
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
	ctx := &Context{Doc: doc, Sites: sites}
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

// Context は1ルールの評価中に診断を積む。
type Context struct {
	Doc   *runbook.Document
	Rule  config.Rule
	Sites []site
	out   []finding.Finding
}

func (c *Context) Add(node *yaml.Node, format string, args ...any) {
	c.AddAt(c.Rule.Severity, node, format, args...)
}

func (c *Context) AddAt(sev finding.Severity, node *yaml.Node, format string, args ...any) {
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
		Message:  fmt.Sprintf(format, args...),
	})
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
