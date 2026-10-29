package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
)

// Linting each of the following runbooks with the default rules reports the named rule at the named severity.
func TestRemainingBuiltins(t *testing.T) {
	cases := []struct {
		name string
		src  string
		rule string
		sev  finding.Severity
	}{
		{
			// Writing owner at the top level reports unknown-key as error.
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
			// A numeric desc reports book-shape as error.
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
			// An empty steps array reports require-steps as error.
			name: "require steps",
			src: `
desc: x
steps: []
`,
			rule: "require-steps",
			sev:  finding.Error,
		},
		{
			// A step with defer: true reports defer-experimental as info.
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
			// Writing extra on an SSH step reports ssh-request as error.
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
			// Writing nope in CDP actions reports cdp-request as error.
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
			// A numeric agent prompt reports agent-request as error.
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
			// Writing both path and steps on include reports include-request as error.
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
			// A needs path that does not exist reports needs-path as warning.
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
			// An openapi3 path that does not exist reports local-path as warning.
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

// Linting a runbook that has x-team and refers to its own steps[0] from the first step reports neither unknown-key nor step-ref, and no error.
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

// Linting a runbook without desc reports require-desc. A runbook with debug: true reports no-debug.
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

// With password in var-ref's ignore, an undefined vars.password does not report var-ref. Setting severity to off also reports nothing.
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

// Referring to parent.vars.token from a step outside include reports var-ref.
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

// Linting a runbook that refers to an existing need.yml and openapi.yml reports neither needs-path nor local-path.
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
