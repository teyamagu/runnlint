package rules

import (
	"regexp"
	"sort"
	"strings"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
)

// Diagnostics for suppression comments are reported in the shared layer after findings are collected, not while rules run.
// Registration exists only so the rule appears in the default config and the built-in rule list.
const unusedSuppressionID = "unused-suppression"

func init() {
	register(unusedSuppressionID, func(*Context) {})
}

// directive is one runnlint comment.
type directive struct {
	// line is the line of the comment and column is the column of #. Both are 1-based.
	line   int
	column int
	// target is the line to suppress. A comment-only line targets the next line that is neither empty nor a comment.
	target int
	// rules are the rules to suppress. nil means every rule.
	rules []string
	// problem is a built-in sentence shown instead of treating the comment as a suppression.
	problem message.ID
	pargs   []any
}

var (
	directiveRe   = regexp.MustCompile(`^#\s*runnlint\s+(\S+)(.*)$`)
	inBlockRe     = regexp.MustCompile(`(?:^|\s)(#\s*runnlint\s)`)
	blockHeaderRe = regexp.MustCompile(`(?:^|:\s|-\s)\s*[|>][1-9+-]{0,2}\s*$`)
)

// parseDirectives reads runnlint directives from YAML comments in src.
// The form matches yamllint's "# yamllint disable-line rule:...", written as "# runnlint disable-line rule:<id> ... -- reason".
// A # inside quotes or inside a | or > multiline scalar is not a comment. A directive inside a multiline scalar is returned with a problem.
func parseDirectives(src []byte) []directive {
	lines := strings.Split(string(src), "\n")
	var out []directive
	var pending []int
	quote := byte(0)
	block := -1
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimRight(raw, "\r")
		indent := len(line) - len(strings.TrimLeft(line, " "))
		blank := strings.TrimSpace(line) == ""
		if block >= 0 {
			if blank || indent > block {
				if m := inBlockRe.FindStringSubmatchIndex(line); m != nil {
					out = append(out, directive{line: lineNo, column: m[2] + 1, target: lineNo,
						problem: message.SuppressInBlock})
				}
				continue
			}
			block = -1
		}
		if blank {
			continue
		}
		code, comment, col, endQuote := splitComment(line, quote)
		quote = endQuote
		if quote == 0 && blockHeaderRe.MatchString(strings.TrimRight(code, " \t")) {
			block = blockIndent(code)
		}
		standalone := strings.TrimSpace(code) == ""
		if !standalone {
			for _, k := range pending {
				out[k].target = lineNo
			}
			pending = nil
		}
		if comment == "" {
			continue
		}
		m := directiveRe.FindStringSubmatch(comment)
		if m == nil {
			continue
		}
		d := directive{line: lineNo, column: col + 1, target: lineNo}
		if m[1] != "disable-line" {
			d.problem = message.SuppressVerb
			d.pargs = []any{m[1]}
		} else {
			d.rules = parseRuleList(m[2])
		}
		out = append(out, d)
		if standalone && d.problem == "" {
			pending = append(pending, len(out)-1)
		}
	}
	// A directive with no following line suppresses nothing.
	for _, k := range pending {
		out[k].target = -1
	}
	return out
}

// parseRuleList extracts rule names from "rule:a rule:b,c -- reason". It returns nil when there are none.
func parseRuleList(s string) []string {
	if i := strings.Index(s, "--"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "#"); i >= 0 {
		s = s[:i]
	}
	var out []string
	for _, tok := range strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '\t' || r == ',' }) {
		tok = strings.TrimPrefix(tok, "rule:")
		if tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// splitComment splits one line into code and comment. quote is a quote that continues from the previous line.
// col is the byte offset of the comment's #. comment is empty when there is no comment.
func splitComment(line string, quote byte) (code, comment string, col int, endQuote byte) {
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			switch {
			case quote == '"' && c == '\\':
				i++
			case quote == '\'' && c == '\'' && i+1 < len(line) && line[i+1] == '\'':
				i++
			case c == quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			if quoteStarts(line[:i]) {
				quote = c
			}
		case '#':
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return line[:i], line[i:], i, 0
			}
		}
	}
	return line, "", -1, quote
}

// quoteStarts reports whether a quote starting here begins a scalar. A ' inside a word such as it's is not a quote.
func quoteStarts(before string) bool {
	t := strings.TrimRight(before, " \t")
	if t == "" {
		return true
	}
	switch t[len(t)-1] {
	case ':', '-', '[', '{', ',', '?':
		return true
	}
	return false
}

// blockIndent is the minimum indent of a line treated as the value after a | or > line (deeper lines are content).
// "  - test: |" uses the column of the key test, and "  - |" uses the column of -.
func blockIndent(code string) int {
	pos := len(code) - len(strings.TrimLeft(code, " "))
	last := pos
	for strings.HasPrefix(code[pos:], "- ") {
		last = pos
		pos += 2
		for pos < len(code) && code[pos] == ' ' {
			pos++
		}
	}
	if pos < len(code) && (code[pos] == '|' || code[pos] == '>') {
		return last
	}
	return pos
}

// applySuppressions drops diagnostics suppressed by runnlint disable-line and adds unused suppressions as unused-suppression.
func applySuppressions(file string, src []byte, findings []finding.Finding, cfg *config.Config) []finding.Finding {
	dirs := parseDirectives(src)
	if len(dirs) == 0 {
		return findings
	}
	known := map[string]finding.Severity{"parse": finding.Error}
	unusedSev := finding.Off
	for _, rule := range cfg.Rules {
		known[rule.ID] = rule.Severity
		if rule.ID == unusedSuppressionID {
			unusedSev = rule.Severity
		}
	}
	used := make([]map[string]bool, len(dirs))
	for i := range used {
		used[i] = map[string]bool{}
	}
	var out []finding.Finding
	for _, f := range findings {
		if suppressedBy(dirs, used, f) {
			continue
		}
		out = append(out, f)
	}
	if unusedSev == finding.Off {
		return out
	}
	lang := displayLang(cfg)
	add := func(d directive, id message.ID, args ...any) {
		out = append(out, finding.Finding{
			File: file, Line: d.line, Column: d.column, Severity: unusedSev,
			Rule:    unusedSuppressionID,
			Message: message.Text(lang, id, args...),
			Match:   message.English(id, args...),
		})
	}
	for i, d := range dirs {
		if d.problem != "" {
			add(d, d.problem, d.pargs...)
			continue
		}
		if d.rules == nil {
			if !used[i]["*"] {
				add(d, message.SuppressNone)
			}
			continue
		}
		for _, id := range d.rules {
			sev, ok := known[id]
			switch {
			case strings.HasPrefix(id, "yamllint/"):
				add(d, message.SuppressYAML, id, strings.TrimPrefix(id, "yamllint/"))
			case !ok:
				add(d, message.SuppressNoRule, id)
			case sev == finding.Off:
			case !used[i][id]:
				add(d, message.SuppressNoHit, id)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Column < out[j].Column
	})
	return out
}

func suppressedBy(dirs []directive, used []map[string]bool, f finding.Finding) bool {
	if f.Rule == unusedSuppressionID {
		return false
	}
	hit := false
	for i, d := range dirs {
		if d.problem != "" || d.target != f.Line {
			continue
		}
		if d.rules == nil {
			used[i]["*"] = true
			hit = true
			continue
		}
		for _, id := range d.rules {
			if id == f.Rule {
				used[i][id] = true
				hit = true
			}
		}
	}
	return hit
}
