package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
)

// 次の各 runbook を既定ルールで lint したとき、指定したルールが指定した深刻度で出ることを確認する。
func TestRemainingBuiltins(t *testing.T) {
	cases := []struct {
		name string
		src  string
		rule string
		sev  finding.Severity
	}{
		{
			// トップレベルに owner を書いたとき、unknown-key が error で出ることを確認する。
			name: "unknown key",
			src: `
desc: x
owner: team
steps:
  - test: "true"
`,
			rule: "unknown-key",
			sev:  finding.Error,
		},
		{
			// desc が数値のとき、book-shape が error で出ることを確認する。
			name: "book shape",
			src: `
desc: 1
steps:
  - test: "true"
`,
			rule: "book-shape",
			sev:  finding.Error,
		},
		{
			// steps が空配列のとき、require-steps が error で出ることを確認する。
			name: "require steps",
			src: `
desc: x
steps: []
`,
			rule: "require-steps",
			sev:  finding.Error,
		},
		{
			// defer: true の step があるとき、defer-experimental が info で出ることを確認する。
			name: "defer",
			src: `
desc: x
steps:
  - defer: true
    test: "true"
`,
			rule: "defer-experimental",
			sev:  finding.Info,
		},
		{
			// SSH step に extra を書いたとき、ssh-request が error で出ることを確認する。
			name: "ssh",
			src: `
desc: x
runners:
  sc: ssh://user@example.com
steps:
  - sc:
      command: hostname
      extra: 1
`,
			rule: "ssh-request",
			sev:  finding.Error,
		},
		{
			// CDP の actions に nope を書いたとき、cdp-request が error で出ることを確認する。
			name: "cdp",
			src: `
desc: x
runners:
  br: cdp://127.0.0.1:9222
steps:
  - br:
      actions:
        - nope
`,
			rule: "cdp-request",
			sev:  finding.Error,
		},
		{
			// agent の prompt が数値のとき、agent-request が error で出ることを確認する。
			name: "agent",
			src: `
desc: x
runners:
  ai:
    agent: claude
    model: opus
steps:
  - ai:
      prompt: 1
`,
			rule: "agent-request",
			sev:  finding.Error,
		},
		{
			// include に path と steps を両方書いたとき、include-request が error で出ることを確認する。
			name: "include exclusive",
			src: `
desc: x
steps:
  - include:
      path: a.yml
      steps:
        - test: "true"
`,
			rule: "include-request",
			sev:  finding.Error,
		},
		{
			// 実在しない needs のパスを書いたとき、needs-path が warning で出ることを確認する。
			name: "needs path",
			src: `
desc: x
needs:
  auth: missing-need.yml
steps:
  - test: "true"
`,
			rule: "needs-path",
			sev:  finding.Warning,
		},
		{
			// 実在しない openapi3 を書いたとき、local-path が warning で出ることを確認する。
			name: "local path",
			src: `
desc: x
runners:
  req:
    endpoint: https://example.com
    openapi3: missing-openapi.yml
steps:
  - test: "true"
`,
			rule: "local-path",
			sev:  finding.Warning,
		},
		{
			// path が login の HTTP step を lint したとき、http-request が warning で出ることを確認する。
			name: "http path warning",
			src: `
desc: x
runners:
  req: https://example.com
steps:
  - req:
      login:
        get:
          body: null
`,
			rule: "http-request",
			sev:  finding.Warning,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lint(t, tc.src)
			var matched []finding.Finding
			for _, f := range got {
				if f.Rule == tc.rule && f.Severity == tc.sev {
					matched = append(matched, f)
				}
			}
			if len(matched) == 0 {
				t.Fatalf("rule %s severity %s がありません: %+v", tc.rule, tc.sev, got)
			}
		})
	}
}

// x-team と、先頭 step から自分自身の steps[0] を参照する runbook を lint したとき、unknown-key も step-ref も error も出ないことを確認する。
func TestExtensionKeyAndSelfStepRef(t *testing.T) {
	got := lint(t, `
desc: x
x-team: platform
steps:
  - test: steps[0].res.status == 200
`)
	if hasRule(got, "unknown-key") || hasRule(got, "step-ref") {
		t.Fatalf("%+v", got)
	}
	if errs := errorsOf(got); len(errs) > 0 {
		t.Fatalf("%+v", errs)
	}
}

// desc が無い runbook を lint したとき require-desc が出ることを確認する。debug: true の runbook では no-debug が出ることを確認する。
func TestDefaultDeclarativeRules(t *testing.T) {
	missing := lint(t, `
steps:
  - test: "true"
`)
	if !hasRule(missing, "require-desc") {
		t.Fatalf("%+v", missing)
	}
	debug := lint(t, `
desc: x
debug: true
steps:
  - test: "true"
`)
	if !hasRule(debug, "no-debug") {
		t.Fatalf("%+v", debug)
	}
}

// var-ref の ignore に password を入れたとき、未定義の vars.password でも var-ref が出ないことを確認する。severity を off にしたときも出ないことを確認する。
func TestVarRefIgnoreAndOff(t *testing.T) {
	src := `
desc: x
steps:
  - exec:
      command: "echo {{ vars.password }}"
`
	cfg := defaultConfig(t)
	for i := range cfg.Rules {
		if cfg.Rules[i].ID == "var-ref" {
			cfg.Rules[i].Options = map[string]any{"ignore": []any{"password"}}
		}
	}
	got, is := Lint("book.yml", []byte(src), cfg)
	if !is || hasRule(got, "var-ref") {
		t.Fatalf("%+v", got)
	}

	cfg = defaultConfig(t)
	for i := range cfg.Rules {
		if cfg.Rules[i].ID == "var-ref" {
			cfg.Rules[i].Severity = finding.Off
		}
	}
	got, is = Lint("book.yml", []byte(src), cfg)
	if !is || hasRule(got, "var-ref") {
		t.Fatalf("off でも診断があります: %+v", got)
	}
}

// include の外の step で parent.vars.token を参照したとき、var-ref が出ることを確認する。
func TestParentVarsOutsideInclude(t *testing.T) {
	got := lint(t, `
desc: x
steps:
  - test: parent.vars.token == "x"
`)
	if !hasRule(got, "var-ref") {
		t.Fatalf("%+v", got)
	}
}

// 実在する need.yml と openapi.yml を参照する runbook を lint したとき、needs-path も local-path も出ないことを確認する。
func TestExistingLocalAndNeedsPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "need.yml"), []byte("desc: n\nsteps:\n  - test: \"true\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openapi.yml"), []byte("openapi: 3.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	src := `
desc: x
needs:
  auth: need.yml
runners:
  req:
    endpoint: https://example.com
    openapi3: openapi.yml
steps:
  - test: "true"
`
	got, is := Lint("book.yml", []byte(src), cfg)
	if !is {
		t.Fatalf("%+v", got)
	}
	if hasRule(got, "needs-path") || hasRule(got, "local-path") {
		t.Fatalf("%+v", got)
	}
}
