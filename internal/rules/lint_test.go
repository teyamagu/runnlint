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

// lintRequireTestStep lints with the default rules and require-test-step set to info, the same overlay a project writes in .runnlint-runn.yaml.
func lintRequireTestStep(t *testing.T, src string) []finding.Finding {
	t.Helper()
	return lintConfig(t, "version: 1\nrules:\n  - id: require-test-step\n    severity: info\n", src)
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

// Linting the following four runbooks with the default rules reports no error.
func TestValidRunbooks(t *testing.T) {
	books := []string{
		// A runbook with array steps that uses HTTP and DB, a template, and a reference to an earlier step reports no error.
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
		// A runbook that refers to steps.login from named steps reports no error.
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
		// A runbook that uses, from the next step, an SSH runner defined by runner: on the previous step reports no error.
		`
desc: dynamic runner
steps:
  - runner:
      sc: ssh://user@example.com:22
  - sc:
      command: hostname
`,
		// A runbook whose inline-include child step uses the parent's HTTP runner reports no error.
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

// Linting each of the following runbooks with the default rules reports the named rule at that severity.
func TestRunnSpecificRules(t *testing.T) {
	cases := []struct {
		name string
		src  string
		rule string
		sev  finding.Severity
	}{
		{
			// Writing nope, which is not in runners, on a step reports runner-defined as error.
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
			// Using exec as a runners name reports runner-reserved as error.
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
			// A POST with headers and no body reports http-request as error.
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
			// A null get value reports http-request as error.
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
			// Referring to steps.login from array steps reports step-ref as error.
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
			// Referring to steps[0] from named steps reports step-ref as error.
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
			// Referring to steps[1] from the first step reports step-ref as error.
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
			// Referring to password, which is not in vars, reports var-ref as warning.
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
			// A label that contains whitespace reports label-format as error.
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
			// A test expression cut off in the middle reports expr-syntax as error.
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
			// Wrapping a whole test in {{ }} reports expr-braces as warning.
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
			// loop.until and test on the same step reports loop-until-with-test as warning.
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
			// login appearing twice in named steps reports duplicate-key as error.
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
			// exec with stdin and no command reports exec-request as error.
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
			// A gRPC key of only Hello reports grpc-request as error.
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
			// A DB step with trace and no query reports db-request as error.
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
			// An include path that does not exist reports include-path as warning.
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
			// Referring to previous on the first step reports previous-step as warning.
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
			// Referring to i on a step without loop reports loop-index as warning.
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
			// A runner of htts:// reports runner-config as error.
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
			// Writing HTTP and DB on one step reports step-shape as error.
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

// Linting a runbook whose password is plaintext with a custom rule reports no-plain-password. ${TEST_PASS} does not.
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

// Linting an HTTP step that expands headers and body with << reports no error and does not treat << as an unknown key.
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
