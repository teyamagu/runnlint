package rules

import (
	"strings"
	"testing"
)

// Pin, per rule, forms from production runbooks that were false positives.
// Linting each case's src with the default rules must not report the rule.
type fpCase struct {
	name string
	rule string
	src  string
}

func checkNoFalsePositive(t *testing.T, cases []fpCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range lint(t, tc.src) {
				if f.Rule == tc.rule {
					t.Fatalf("誤検知です: %+v", f)
				}
			}
		})
	}
}

// runn expands ${...} in YAML values to environment variables before evaluating an expression. An unset variable must not report expr-syntax.
func TestEnvPlaceholdersAreExpandedBeforeParse(t *testing.T) {
	t.Setenv("RUNNLINT_TEST_UNSET", "")
	checkNoFalsePositive(t, []fpCase{
		{"test の比較の右辺", "expr-syntax", `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        get:
    test: current.res.status == ${STATUS}
`},
		{"既定値つきの展開", "expr-syntax", `
desc: d
steps:
  - dump: x
    if: ${ENABLED:-true} && ${RUNNLINT_TEST_UNSET:-1} > 0
`},
		{"入れ子の展開", "expr-syntax", `
desc: d
steps:
  - dump: ${A:-${B}}
`},
		{"テンプレートの中", "expr-syntax", `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        get:
          headers:
            X-Id: "{{ ${ID} + 1 }}"
`},
		{"loop.count", "expr-syntax", `
desc: d
steps:
  - dump: x
    loop:
      count: ${COUNT}
`},
		{"bind", "expr-syntax", `
desc: d
steps:
  - dump: x
    bind:
      token: ${TOKEN}
`},
		{"vars のキー", "var-ref", `
desc: d
vars:
  a: 1
steps:
  - dump: vars.${NAME}
`},
		{"steps のキー", "step-ref", `
desc: d
steps:
  first:
    dump: "1"
  second:
    dump: steps.${NAME}
`},
	})
}

// After expanding ${...}, an error in the expression itself is still reported as expr-syntax.
func TestEnvPlaceholderKeepsRealSyntaxErrors(t *testing.T) {
	requireMessage(t, `
desc: d
steps:
  - dump: x
    test: ${STATUS} ==
`, "expr-syntax", "unexpected")
}

func TestExpandEnvPlaceholders(t *testing.T) {
	cases := map[string]string{
		"a == ${X}":         "a == " + envPlaceholder,
		"${A:-${B}}":        envPlaceholder,
		"'${X}' + \"${Y}\"": "'" + envPlaceholder + "' + \"" + envPlaceholder + "\"",
		"$X + $":            "$X + $",
		"${unclosed":        "${unclosed",
		"no env":            "no env",
	}
	for in, want := range cases {
		if got := expandEnvPlaceholders(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
	if !strings.HasPrefix(envPlaceholder, "__") {
		t.Fatal("placeholder は識別子です")
	}
}

// runn strips # line comments before evaluating an expression. A # inside a string literal or a closure stays.
func TestSharpCommentsInExpressions(t *testing.T) {
	checkNoFalsePositive(t, []fpCase{
		{"行頭と行末のコメント", "expr-syntax", `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        get:
    test: |
      # ステータスを見る
      current.res.status == 200 # ok
      && current.res.body.tag == "#tag" # 文字列の中の # は残す
`},
		{"if のコメント", "expr-syntax", `
desc: d
steps:
  - dump: x
    if: true # 常に実行
`},
		{"クロージャの # と併用", "expr-syntax", `
desc: d
steps:
  - dump: x
    test: |
      all([1, 2], # > 0) # すべて正
`},
	})
}

// An error that remains in the expression after comments are stripped is reported as expr-syntax.
func TestSharpCommentKeepsRealSyntaxErrors(t *testing.T) {
	requireMessage(t, `
desc: d
steps:
  - dump: x
    test: |
      # コメント
      1 == # 右辺が無い
`, "expr-syntax", "unexpected")
}

func TestTrimSharpComment(t *testing.T) {
	cases := map[string]string{
		"a == 1 # c":                 "a == 1",
		"# c\na == 1":                "a == 1",
		"  # c\na == 1\n":            "a == 1",
		`a == "#x"`:                  `a == "#x"`,
		"map(xs, # * 2)":             "map(xs, # * 2)",
		"map(xs, # * 2) # c":         "map(xs, # * 2)",
		"\"日本\" == a # コメント":         "\"日本\" == a",
		"a == 1 // 標準のコメント":          "a == 1 // 標準のコメント",
		"filter(xs, {# > 1}) # c\n1": "filter(xs, {# > 1})\n1",
	}
	for in, want := range cases {
		if got := trimSharpComment(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

// Nested parentheses and braces inside {{ }} must not be misread as the end of the expression.
func TestNestedBracketsInTemplates(t *testing.T) {
	checkNoFalsePositive(t, []fpCase{
		{"入れ子の関数呼び出し", "expr-syntax", `
desc: d
steps:
  - dump: x
    bind:
      y: string(int("1"))
  - exec:
      command: |
        echo "{{ string(int(len(toJSON({"a": {"b": 1}})))) }}"
`},
		{"map リテラルの入れ子", "expr-syntax", `
desc: d
steps:
  - exec:
      command: |
        echo '{{ {"a": {"b": 1}}.a.b }}' '{{ {"a":1}}}'
`},
		{"配列とクロージャ", "expr-syntax", `
desc: d
steps:
  - exec:
      command: |
        echo "{{ len(filter([{"n": 1}], {.n > 0})) }}"
`},
	})
}

func TestExtractTemplatesNested(t *testing.T) {
	cases := map[string][]string{
		`{{ {"a": {"b": 1}} }}`:    {` {"a": {"b": 1}} `},
		`{{ {"a":1}}}`:             {` {"a":1}`},
		`{{ f(g(x)) }}-{{ y }}`:    {` f(g(x)) `, ` y `},
		`{{ "}}" }}`:               {` "}}" `},
		`{{ a) }} {{ b }}`:         {` a) `, ` b `},
		`{{ {"a": 1 }} {{ b }}`:    {` {"a": 1 `, ` b `},
		`{{ toJSON({"a": [1]}) }}`: {` toJSON({"a": [1]}) `},
	}
	for in, want := range cases {
		got, err := extractTemplates(in)
		if err != nil || strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%q: got %q (%v) want %q", in, got, err, want)
		}
	}
}

// runn evaluates bind values recursively and reads strings inside mappings and arrays as expressions. Numbers and similar values are bound as-is. step-shape must not be reported.
func TestBindAcceptsNestedValues(t *testing.T) {
	checkNoFalsePositive(t, []fpCase{
		{"mapping と配列と数値", "step-shape", `
desc: d
steps:
  - dump: "1"
    bind:
      user:
        id: current
        tags:
          - '"a"'
      retry: 3
`},
	})
}

// A string inside a bind mapping is also checked as an expression and reported as expr-syntax.
func TestBindNestedStringsAreExpressions(t *testing.T) {
	requireMessage(t, `
desc: d
steps:
  - dump: "1"
    bind:
      user:
        id: current ==
`, "expr-syntax", "unexpected")
}

// Empty headers ({} and null) are accepted by runn, so http-request and grpc-request must not be reported.
func TestEmptyHeaders(t *testing.T) {
	checkNoFalsePositive(t, []fpCase{
		{"HTTP の {}", "http-request", `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        get:
          headers: {}
          body: null
`},
		{"HTTP の null", "http-request", `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        get:
          headers:
          body: null
`},
		{"gRPC の null", "grpc-request", `
desc: d
runners:
  greq: grpc://example.com:443
steps:
  - greq:
      pkg.Service/Method:
        headers:
        message: {}
`},
	})
	requireMessage(t, `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        get:
          headers: [a]
          body: null
`, "http-request", "headers は mapping です")
}

// A {{ inside a test or bind expression is not a template. Inside a string, neither expr-syntax nor expr-braces is reported.
func TestBracesInsideTestAndBindAreNotTemplates(t *testing.T) {
	checkNoFalsePositive(t, []fpCase{
		{"test の文字列", "expr-syntax", `
desc: d
steps:
  - test: 'current.res.body == "hello {{ name }}"'
`},
		{"test の文字列", "expr-braces", `
desc: d
steps:
  - test: 'current.res.body == "hello {{ name }}"'
`},
		{"閉じていない {{", "expr-syntax", `
desc: d
steps:
  - test: 'current.res.body == "{{"'
`},
		{"閉じていない {{", "expr-braces", `
desc: d
steps:
  - test: 'current.res.body == "{{"'
`},
		{"bind の文字列", "expr-syntax", `
desc: d
steps:
  - dump: "1"
    bind:
      note: '"{{title}}"'
      body:
        query: 'sprintf("{{%s}}", "a")'
`},
		{"bind の文字列", "expr-braces", `
desc: d
steps:
  - dump: "1"
    bind:
      note: '"{{title}}"'
      body:
        query: 'sprintf("{{%s}}", "a")'
`},
	})
}

// A test or bind whose whole value is wrapped in {{ }} reports expr-braces, and the inside is parsed as an expression.
// When the value starts with {{ but is not a single template, only expr-braces is reported, not an expression parse error.
func TestWrappedTestAndBindStillWarn(t *testing.T) {
	requireMessage(t, `
desc: d
steps:
  - test: "{{ current.res.status == 200 }}"
`, "expr-braces", "test")
	requireMessage(t, `
desc: d
steps:
  - dump: "1"
    bind:
      id: "{{ current }}"
`, "expr-braces", "bind")
	requireMessage(t, `
desc: d
steps:
  - test: "{{ 1 + }}"
`, "expr-syntax", "unexpected")
	src := `
desc: d
steps:
  - test: "{{ a }} {{ b }}"
`
	requireMessage(t, src, "expr-braces", "test")
	rejectMessage(t, src, "expr-syntax", "")
}

// An HTTP path that does not start with / still works because runn joins it to the endpoint, so http-request is not reported by default.
// A warning is reported only when options.require-leading-slash is true.
func TestHTTPPathWithoutLeadingSlash(t *testing.T) {
	src := `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      graphql:
        post:
          body:
            application/json:
              query: "{ viewer { login } }"
`
	checkNoFalsePositive(t, []fpCase{{"graphql", "http-request", src}})
	got := lintConfig(t, `
rules:
  - id: http-request
    severity: error
    options:
      require-leading-slash: true
`, src)
	msgs := messagesOf(got, "http-request")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "graphql は / から始めます") || got[0].Severity != "warning" {
		t.Fatalf("%+v", got)
	}
}
