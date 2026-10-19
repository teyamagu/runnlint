package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
)

// 抑制コメントの診断は、ルールの評価ではなく診断を集めたあとの共通の層で出す。
// 登録は、デフォルト設定と既定ルールの一覧に載せるためだけにある。
const unusedSuppressionID = "unused-suppression"

func init() {
	register(unusedSuppressionID, func(*Context) {})
}

// directive は1つの runnlint コメント。
type directive struct {
	// line はコメントのある行、column は # の列。どちらも 1 始まり。
	line   int
	column int
	// target は抑制する行。コメントだけの行なら、次の空でもコメントでもない行。
	target int
	// rules は抑制するルール。nil ならすべてのルール。
	rules []string
	// problem があれば抑制として使わず、その内容を unused-suppression で知らせる。
	problem string
}

var (
	directiveRe   = regexp.MustCompile(`^#\s*runnlint\s+(\S+)(.*)$`)
	inBlockRe     = regexp.MustCompile(`(?:^|\s)(#\s*runnlint\s)`)
	blockHeaderRe = regexp.MustCompile(`(?:^|:\s|-\s)\s*[|>][1-9+-]{0,2}\s*$`)
)

// parseDirectives は src の YAML コメントから runnlint の指示を読む。
// yamllint の "# yamllint disable-line rule:..." と同じ形で、"# runnlint disable-line rule:<id> ... -- 理由" と書く。
// 引用符の中と、| や > の複数行スカラーの中の # はコメントではない。複数行スカラーの中の指示は problem 付きで返す。
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
						problem: "複数行の値の中のコメントは抑制になりません。診断はキーの行に出るので、キーの行の行末か前の行に書きます"})
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
			d.problem = fmt.Sprintf("runnlint %s は使えません。disable-line を使います", m[1])
		} else {
			d.rules = parseRuleList(m[2])
		}
		out = append(out, d)
		if standalone && d.problem == "" {
			pending = append(pending, len(out)-1)
		}
	}
	// 後ろに行が無い指示は、どの行も抑制しない。
	for _, k := range pending {
		out[k].target = -1
	}
	return out
}

// parseRuleList は "rule:a rule:b,c -- 理由" からルール名を取り出す。何も無ければ nil。
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

// splitComment は1行をコードとコメントに分ける。quote は前の行から続く引用符。
// col はコメントの # のバイト位置。コメントが無ければ comment は空。
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

// quoteStarts は、ここで始まる引用符がスカラーの始まりか。it's のような語の中の ' は引用符ではない。
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

// blockIndent は、| や > の行のあとで値の中身とみなす行の字下げの下限（これより深ければ中身）。
// "  - test: |" ならキー test の列、"  - |" なら - の列。
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

// applySuppressions は runnlint disable-line で抑制した診断を除き、使われなかった抑制を unused-suppression として足す。
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
	add := func(d directive, format string, args ...any) {
		out = append(out, finding.Finding{
			File: file, Line: d.line, Column: d.column, Severity: unusedSev,
			Rule: unusedSuppressionID, Message: fmt.Sprintf(format, args...),
		})
	}
	for i, d := range dirs {
		if d.problem != "" {
			add(d, "%s", d.problem)
			continue
		}
		if d.rules == nil {
			if !used[i]["*"] {
				add(d, "runnlint disable-line で抑制する診断がありません")
			}
			continue
		}
		for _, id := range d.rules {
			sev, ok := known[id]
			switch {
			case strings.HasPrefix(id, "yamllint/"):
				add(d, "%s は yamllint の診断です。# yamllint disable-line rule:%s で抑制します", id, strings.TrimPrefix(id, "yamllint/"))
			case !ok:
				add(d, "ルール %s はありません", id)
			case sev == finding.Off:
			case !used[i][id]:
				add(d, "rule:%s で抑制する診断がありません", id)
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
