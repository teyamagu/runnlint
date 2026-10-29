package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
)

func lintConfig(t *testing.T, yml, src string) []finding.Finding {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".runnlint.yaml")
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got, is := Lint("book.yml", []byte(src), cfg)
	if !is {
		t.Fatalf("runbook として解釈できません: %+v", got)
	}
	return got
}

func messagesOf(findings []finding.Finding, id string) []string {
	var out []string
	for _, f := range findings {
		if f.Rule == id {
			out = append(out, f.Message)
		}
	}
	return out
}

// With only the following checks enabled, a violating input reports custom and a matching input does not. The message also contains the given fragment.
func TestDeclarativeOperators(t *testing.T) {
	const book = "debug: true\nvars:\n  env: staging\ndesc: ab\nlabels:\n  - one\nsteps:\n  - test: \"true\"\n"
	cases := []struct {
		name    string
		check   string
		src     string
		wantHit bool
		part    string
	}{
		{
			// exists: true on a runbook without secret reports custom, and the message contains rule=custom.
			name:    "exists missing",
			check:   "select: secret\n      exists: true\n",
			src:     "steps:\n  - test: \"true\"\n",
			wantHit: true,
			part:    "rule=custom",
		},
		{
			// exists: false on a runbook that has debug reports custom.
			name:    "exists false",
			check:   "select: debug\n      exists: false\n",
			src:     book,
			wantHit: true,
		},
		{
			// equals: false on a runbook whose debug is true reports custom, and the message contains value=true.
			name:    "equals miss",
			check:   "select: debug\n      equals: false\n",
			src:     book,
			wantHit: true,
			part:    "value=true",
		},
		{
			// equals: true on a runbook whose debug is true does not report custom.
			name:    "equals hit",
			check:   "select: debug\n      equals: true\n",
			src:     book,
			wantHit: false,
		},
		{
			// equals: null on a runbook whose debug is null does not report custom.
			name:    "equals null",
			check:   "select: debug\n      equals: null\n",
			src:     "debug: null\nsteps:\n  - test: \"true\"\n",
			wantHit: false,
		},
		{
			// not_equals: true on a runbook whose debug is true reports custom.
			name:    "not equals",
			check:   "select: debug\n      not_equals: true\n",
			src:     book,
			wantHit: true,
		},
		{
			// one_of: [dev, prod] on a runbook whose vars.env is staging reports custom, and the message contains path=vars.env.
			name:    "one of",
			check:   "select: vars.env\n      one_of: [dev, prod]\n",
			src:     book,
			wantHit: true,
			part:    "path=vars.env",
		},
		{
			// one_of: [dev, staging] on a runbook whose vars.env is staging does not report custom.
			name:    "one of pass",
			check:   "select: vars.env\n      one_of: [dev, staging]\n",
			src:     book,
			wantHit: false,
		},
		{
			// min_len: 3 on a runbook whose desc is ab reports custom.
			name:    "min len",
			check:   "select: desc\n      min_len: 3\n",
			src:     book,
			wantHit: true,
		},
		{
			// max_len: 0 on a runbook with one label reports custom.
			name:    "max len sequence",
			check:   "select: labels\n      max_len: 0\n",
			src:     book,
			wantHit: true,
		},
		{
			// not_matches: staging on a runbook whose vars.env is staging reports custom.
			name:    "not matches",
			check:   "select: vars.env\n      not_matches: staging\n",
			src:     book,
			wantHit: true,
		},
		{
			// matches: ^staging$ on a runbook whose vars.env is staging does not report custom.
			name:    "matches pass",
			check:   "select: vars.env\n      matches: '^staging$'\n",
			src:     book,
			wantHit: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yml := "version: 1\nextends: none\nrules:\n  - id: custom\n    severity: error\n    message: \"key={{ .key }} path={{ .path }} value={{ .value }} rule={{ .rule }}\"\n    check:\n      " + tc.check
			got := lintConfig(t, yml, tc.src)
			msgs := messagesOf(got, "custom")
			if tc.wantHit && len(msgs) == 0 {
				t.Fatalf("診断がありません: %+v", got)
			}
			if !tc.wantHit && len(msgs) > 0 {
				t.Fatalf("診断があります: %+v", msgs)
			}
			if tc.part != "" && (len(msgs) == 0 || !strings.Contains(msgs[0], tc.part)) {
				t.Fatalf("message = %#v", msgs)
			}
			for _, f := range got {
				if f.Rule == "custom" && f.Severity != finding.Error {
					t.Fatalf("severity = %s", f.Severity)
				}
			}
		})
	}
}

// Forbidding a step that has test reports only one finding, key=0 and path=steps[0], for array steps. For named steps it reports only one finding, key=login and path=steps.login.
func TestDeclarativeWhereForbidAndSteps(t *testing.T) {
	yml := `
version: 1
extends: none
rules:
  - id: custom
    severity: warning
    message: "key={{ .key }} path={{ .path }}"
    check:
      select: steps.*
      where:
        - path: test
          exists: true
      forbid: true
`
	list := lintConfig(t, yml, `
steps:
  - desc: a
    test: "true"
  - desc: b
`)
	msgs := messagesOf(list, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "key=0") || !strings.Contains(msgs[0], "path=steps[0]") {
		t.Fatalf("%#v", msgs)
	}

	mapped := lintConfig(t, yml, `
steps:
  login:
    desc: a
    test: "true"
  show:
    desc: b
`)
	msgs = messagesOf(mapped, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "key=login") || !strings.Contains(msgs[0], "path=steps.login") {
		t.Fatalf("%#v", msgs)
	}
}

// key_matches looks at map key names. A matching step name is not reported, and a non-matching step name is reported on the key line.
// For list steps the key is an index starting at 0. key_not_matches reports keys that match.
func TestKeyMatches(t *testing.T) {
	yml := `
version: 1
extends: none
rules:
  - id: custom
    severity: warning
    message: "key={{ .key }} path={{ .path }}"
    check:
      select: steps.*
      key_matches: '^[a-z][a-z0-9-]*$'
`
	mapped := lintConfig(t, yml, "steps:\n  ok:\n    test: \"true\"\n  Bad:\n    test: \"true\"\n")
	msgs := messagesOf(mapped, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "key=Bad") || !strings.Contains(msgs[0], "path=steps.Bad") {
		t.Fatalf("%#v", msgs)
	}
	var line int
	for _, f := range mapped {
		if f.Rule == "custom" {
			line = f.Line
		}
	}
	if line != 4 {
		t.Fatalf("診断の行 = %d。キーの行である 4 を期待します: %+v", line, mapped)
	}

	list := lintConfig(t, yml, "steps:\n  - test: \"true\"\n")
	msgs = messagesOf(list, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "key=0") {
		t.Fatalf("%#v", msgs)
	}

	not := lintConfig(t, `
version: 1
extends: none
rules:
  - id: custom
    severity: error
    message: "key={{ .key }}"
    check:
      select: steps.*
      key_not_matches: '^_'
`, "steps:\n  ok:\n    test: \"true\"\n  _hidden:\n    test: \"true\"\n")
	msgs = messagesOf(not, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "key=_hidden") {
		t.Fatalf("%#v", msgs)
	}

	bad := lintConfig(t, `
version: 1
extends: none
rules:
  - id: custom
    check:
      select: steps.*
      key_matches: "("
`, "steps:\n  ok:\n    test: \"true\"\n")
	msgs = messagesOf(bad, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "key_matches") {
		t.Fatalf("%#v", msgs)
	}
}

// Linting with a rule whose matches contains '(' reports one custom finding, and the message contains "matches has an invalid regular expression".
func TestDeclarativeBadRegexp(t *testing.T) {
	got := lintConfig(t, `
version: 1
extends: none
rules:
  - id: custom
    check:
      select: desc
      matches: "("
`, "desc: x\nsteps:\n  - test: \"true\"\n")
	msgs := messagesOf(got, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "正規表現") {
		t.Fatalf("%#v", msgs)
	}
}

// An exists rule with severity off does not report custom even when desc is missing.
func TestDeclarativeOffDoesNotRun(t *testing.T) {
	got := lintConfig(t, `
version: 1
extends: none
rules:
  - id: custom
    severity: off
    check:
      select: desc
      exists: true
`, "steps:\n  - test: \"true\"\n")
	if hasRule(got, "custom") {
		t.Fatalf("%+v", got)
	}
}
