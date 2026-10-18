package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
)

func defaultConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Chdir(t.TempDir())
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func lint(t *testing.T, src string) []finding.Finding {
	t.Helper()
	got, is := Lint("book.yml", []byte(src), defaultConfig(t))
	if !is {
		t.Fatalf("runbook として解釈できません: %+v", got)
	}
	return got
}

func errorsOf(findings []finding.Finding) []finding.Finding {
	var out []finding.Finding
	for _, f := range findings {
		if f.Severity == finding.Error {
			out = append(out, f)
		}
	}
	return out
}

func hasRule(findings []finding.Finding, id string) bool {
	for _, f := range findings {
		if f.Rule == id {
			return true
		}
	}
	return false
}

// 次の4つの runbook を既定ルールで lint したとき、いずれも error が出ないことを確認する。
func TestValidRunbooks(t *testing.T) {
	books := []string{
		// 配列 steps で HTTP と DB、テンプレート、先行 step の参照を書いた runbook では error が出ないことを確認する。
		`
desc: Login and get projects.
runners:
  req: https://example.com/api/v1
  db: mysql://root:mypass@localhost:3306/testdb
vars:
  username: alice
  password: ${TEST_PASS}
labels:
  - users
  - auth
steps:
  - db:
      query: SELECT id FROM users WHERE name = '{{ vars.username }}'
  - req:
      /login:
        post:
          body:
            application/json:
              name: "{{ vars.username }}"
              password: "{{ vars.password }}"
    test: steps[1].res.status == 200
  - req:
      /projects:
        get:
          headers:
            Authorization: "token {{ steps[1].res.body.session_token }}"
          body: null
    test: len(steps[2].res.body.projects) > 0
`,
		// 名前付き steps から steps.login を参照する runbook では error が出ないことを確認する。
		`
desc: map steps
runners:
  req: https://example.com
steps:
  login:
    req:
      /login:
        post:
          body:
            application/json:
              name: alice
    test: steps.login.res.status == 200
  show:
    req:
      /users/{{ steps.login.res.body.id }}:
        get:
          body: null
`,
		// 前の step の runner: で定義した SSH runner を次の step から使う runbook では error が出ないことを確認する。
		`
desc: dynamic runner
steps:
  - runner:
      sc: ssh://user@example.com:22
  - sc:
      command: hostname
`,
		// インライン include の子 step から親の HTTP runner を使う runbook では error が出ないことを確認する。
		`
desc: inline include
runners:
  req: https://example.com
steps:
  - include:
      steps:
        - req:
            /health:
              get:
                body: null
`,
	}
	for i, src := range books {
		got := lint(t, src)
		if errs := errorsOf(got); len(errs) > 0 {
			t.Errorf("book %d: %+v", i, errs)
		}
	}
}

// 次の各 runbook を既定ルールで lint したとき、指定したルールがその深刻度で出ることを確認する。
func TestRunnSpecificRules(t *testing.T) {
	cases := []struct {
		name string
		src  string
		rule string
		sev  finding.Severity
	}{
		{
			// runners に無い nope を step に書いたとき、runner-defined が error で出ることを確認する。
			name: "unknown runner",
			src: `
desc: x
steps:
  - nope:
      query: SELECT 1
`,
			rule: "runner-defined",
			sev:  finding.Error,
		},
		{
			// runners の名前に exec を使ったとき、runner-reserved が error で出ることを確認する。
			name: "reserved runner",
			src: `
desc: x
runners:
  exec: https://example.com
steps:
  - exec:
      /:
        get:
          body: null
`,
			rule: "runner-reserved",
			sev:  finding.Error,
		},
		{
			// POST に headers だけで body が無いとき、http-request が error で出ることを確認する。
			name: "post without body",
			src: `
desc: x
runners:
  req: https://example.com
steps:
  - req:
      /login:
        post:
          headers:
            Authorization: token
`,
			rule: "http-request",
			sev:  finding.Error,
		},
		{
			// get の値が null のとき、http-request が error で出ることを確認する。
			name: "get null",
			src: `
desc: x
runners:
  req: https://example.com
steps:
  - req:
      /users:
        get: null
`,
			rule: "http-request",
			sev:  finding.Error,
		},
		{
			// 配列の steps から steps.login を参照したとき、step-ref が error で出ることを確認する。
			name: "list step name ref",
			src: `
desc: x
steps:
  - test: steps.login.res.status == 200
`,
			rule: "step-ref",
			sev:  finding.Error,
		},
		{
			// 名前付き steps から steps[0] を参照したとき、step-ref が error で出ることを確認する。
			name: "map step index ref",
			src: `
desc: x
runners:
  req: https://example.com
steps:
  login:
    req:
      /:
        get:
          body: null
    test: steps[0].res.status == 200
`,
			rule: "step-ref",
			sev:  finding.Error,
		},
		{
			// 先頭 step から steps[1] を参照したとき、step-ref が error で出ることを確認する。
			name: "forward ref",
			src: `
desc: x
steps:
  - test: steps[1].res.status == 200
  - exec:
      command: "echo ok"
`,
			rule: "step-ref",
			sev:  finding.Error,
		},
		{
			// vars に無い password を参照したとき、var-ref が warning で出ることを確認する。
			name: "undefined var",
			src: `
desc: x
vars:
  username: alice
steps:
  - exec:
      command: "echo {{ vars.password }}"
`,
			rule: "var-ref",
			sev:  finding.Warning,
		},
		{
			// 空白を含む label を書いたとき、label-format が error で出ることを確認する。
			name: "bad label",
			src: `
desc: x
labels:
  - bad label
steps:
  - test: "true"
`,
			rule: "label-format",
			sev:  finding.Error,
		},
		{
			// 途中で切れた test 式を書いたとき、expr-syntax が error で出ることを確認する。
			name: "expr syntax",
			src: `
desc: x
steps:
  - test: "current.res.status =="
`,
			rule: "expr-syntax",
			sev:  finding.Error,
		},
		{
			// test 全体を {{ }} で包んだとき、expr-braces が warning で出ることを確認する。
			name: "expr braces",
			src: `
desc: x
steps:
  - test: "{{ current.res.status == 200 }}"
`,
			rule: "expr-braces",
			sev:  finding.Warning,
		},
		{
			// 同じ step に loop.until と test があるとき、loop-until-with-test が warning で出ることを確認する。
			name: "loop until with test",
			src: `
desc: x
runners:
  req: https://example.com
steps:
  - req:
      /status:
        get:
          body: null
    loop:
      count: 3
      until: current.res.status == 200
    test: current.res.status == 200
`,
			rule: "loop-until-with-test",
			sev:  finding.Warning,
		},
		{
			// 名前付き steps に login が2回あるとき、duplicate-key が error で出ることを確認する。
			name: "duplicate key",
			src: `
desc: x
steps:
  login:
    test: "true"
  login:
    test: "false"
`,
			rule: "duplicate-key",
			sev:  finding.Error,
		},
		{
			// exec に stdin だけで command が無いとき、exec-request が error で出ることを確認する。
			name: "exec without command",
			src: `
desc: x
steps:
  - exec:
      stdin: hello
`,
			rule: "exec-request",
			sev:  finding.Error,
		},
		{
			// gRPC のキーが Hello だけのとき、grpc-request が error で出ることを確認する。
			name: "grpc method",
			src: `
desc: x
runners:
  greq: grpc://localhost:8080
steps:
  - greq:
      Hello:
        message:
          name: alice
`,
			rule: "grpc-request",
			sev:  finding.Error,
		},
		{
			// DB step に trace だけで query が無いとき、db-request が error で出ることを確認する。
			name: "db query",
			src: `
desc: x
runners:
  db: mysql://root:pass@localhost:3306/app
steps:
  - db:
      trace: false
`,
			rule: "db-request",
			sev:  finding.Error,
		},
		{
			// 実在しない include path を書いたとき、include-path が warning で出ることを確認する。
			name: "include path",
			src: `
desc: x
steps:
  - include: missing-runbook.yml
`,
			rule: "include-path",
			sev:  finding.Warning,
		},
		{
			// 先頭 step で previous を参照したとき、previous-step が warning で出ることを確認する。
			name: "previous",
			src: `
desc: x
steps:
  - test: previous.res.status == 200
`,
			rule: "previous-step",
			sev:  finding.Warning,
		},
		{
			// loop の無い step で i を参照したとき、loop-index が warning で出ることを確認する。
			name: "loop index",
			src: `
desc: x
steps:
  - test: i == 0
`,
			rule: "loop-index",
			sev:  finding.Warning,
		},
		{
			// runner が htts:// のとき、runner-config が error で出ることを確認する。
			name: "bad dsn scheme",
			src: `
desc: x
runners:
  req: htts://example.com
steps:
  - test: "true"
`,
			rule: "runner-config",
			sev:  finding.Error,
		},
		{
			// 1つの step に HTTP と DB を書いたとき、step-shape が error で出ることを確認する。
			name: "two main runners",
			src: `
desc: x
runners:
  req: https://example.com
  db: mysql://root:pass@localhost:3306/app
steps:
  - req:
      /:
        get:
          body: null
    db:
      query: SELECT 1
`,
			rule: "step-shape",
			sev:  finding.Error,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lint(t, tc.src)
			var matched []finding.Finding
			for _, f := range got {
				if f.Rule == tc.rule {
					matched = append(matched, f)
				}
			}
			if len(matched) == 0 {
				t.Fatalf("rule %s がありません: %+v", tc.rule, got)
			}
			if matched[0].Severity != tc.sev {
				t.Fatalf("severity = %s, findings = %+v", matched[0].Severity, matched)
			}
		})
	}
}

// password が平文の runbook を独自ルールで lint したとき no-plain-password が出ることを確認する。${TEST_PASS} のときは出ないことを確認する。
func TestCustomRule(t *testing.T) {
	cfg := &config.Config{
		Rules: []config.Rule{
			{
				ID:       "require-steps",
				Uses:     "require-steps",
				Severity: finding.Error,
			},
			{
				ID:       "no-plain-password",
				Severity: finding.Error,
				Message:  "password は環境変数を参照してください",
				Check: &config.Check{
					Select:  "vars.password",
					Matches: `^\$\{.+\}$`,
				},
			},
		},
	}
	if err := CheckConfig(cfg); err != nil {
		t.Fatal(err)
	}
	src := []byte("desc: x\nvars:\n  password: plain\nsteps:\n  - test: 'true'\n")
	got, _ := Lint("book.yml", src, cfg)
	if !hasRule(got, "no-plain-password") {
		t.Fatalf("%+v", got)
	}
	ok := []byte("desc: x\nvars:\n  password: ${TEST_PASS}\nsteps:\n  - test: 'true'\n")
	got, _ = Lint("book.yml", ok, cfg)
	if hasRule(got, "no-plain-password") {
		t.Fatalf("一致すべき値が違反になりました: %+v", got)
	}
}

// << でヘッダと body を展開した HTTP step を lint したとき、error が出ず << を未知キーとして診断しないことを確認する。
func TestAnchorMerge(t *testing.T) {
	src := `
desc: anchored headers
anchors:
  base: &base
    headers:
      Authorization: Bearer token
    body: null
runners:
  req: https://example.com
steps:
  - req:
      /users:
        get:
          <<: *base
`
	got := lint(t, src)
	if errs := errorsOf(got); len(errs) > 0 {
		t.Fatalf("%+v", errs)
	}
	if strings.Contains(strings.Join(messages(got), "\n"), "<<") {
		t.Fatalf("merge key を未知のキーとして扱っています: %+v", got)
	}
}

func messages(findings []finding.Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = f.Message
	}
	return out
}
