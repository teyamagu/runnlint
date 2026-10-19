package rules

import (
	"strings"
	"testing"
)

// 実運用の runbook で誤検知だった書き方を、ルールごとに固定する。
// 各ケースの src を既定ルールで lint したとき、rule の診断が出ないことを確認する。
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

// runn は式を評価する前に YAML の値の ${...} を環境変数で展開する。未設定の変数でも expr-syntax を出さないことを確認する。
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

// ${...} を展開したあとも、式そのものの誤りは expr-syntax で検出することを確認する。
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

// runn は式を評価する前に # の行コメントを取り除く。文字列リテラルとクロージャの中の # は残す。
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

// コメントを取り除いたあとに残る式の誤りは expr-syntax で検出することを確認する。
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
