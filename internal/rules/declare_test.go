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

// equals: null and not_equals: null are real conditions: equals: null reports a value that is not null, and not_equals: null reports a null.
// A where with equals: null keeps only null values. YAML leaves a null node undecoded, so the config must record that the key was written.
func TestEqualsNullIsNotIgnored(t *testing.T) {
	cases := []struct {
		name    string
		check   string
		src     string
		wantHit bool
	}{
		{"equals null on a non-null value", "select: debug\n      equals: null\n", "debug: true\nsteps:\n  - test: \"true\"\n", true},
		{"equals null on a null value", "select: debug\n      equals: null\n", "debug: null\nsteps:\n  - test: \"true\"\n", false},
		{"not_equals null on a null value", "select: debug\n      not_equals: null\n", "debug: null\nsteps:\n  - test: \"true\"\n", true},
		{"not_equals null on a non-null value", "select: debug\n      not_equals: null\n", "debug: true\nsteps:\n  - test: \"true\"\n", false},
		{"where equals null keeps a null", "select: steps.*\n      where:\n        path: dump\n        equals: null\n      forbid: true\n", "steps:\n  - dump: null\n", true},
		{"where equals null drops a non-null", "select: steps.*\n      where:\n        path: dump\n        equals: null\n      forbid: true\n", "steps:\n  - dump: x\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yml := "version: 1\nextends: none\nrules:\n  - id: custom\n    severity: error\n    check:\n      " + tc.check
			got := lintConfig(t, yml, tc.src)
			if hasRule(got, "custom") != tc.wantHit {
				t.Fatalf("%+v", got)
			}
		})
	}
}

// lintConfigAs is lintConfig with a file name, for rules that use files.
func lintConfigAs(t *testing.T, yml, filename, src string) []finding.Finding {
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
	got, is := Lint(filename, []byte(src), cfg)
	if !is {
		t.Fatalf("runbook として解釈できません: %+v", got)
	}
	return got
}

func pathRule(check string) string {
	return "version: 1\nextends: none\nrules:\n  - id: custom\n    severity: warning\n    message: \"path={{ .path }} key={{ .key }} value={{ .value }}\"\n    check:\n      " + check
}

// ** in select stands for zero or more levels. It descends through mapping values and sequence elements at any depth. A trailing ** selects every scalar below. The existing * still means exactly one level.
func TestSelectRecursiveWildcard(t *testing.T) {
	const book = `
desc: x
runners:
  req: https://example.com
  db: mysql://u:p@h/d
vars:
  flat: a
  nested:
    deep:
      - b
      - c
steps:
  - req:
      /graphql:
        post:
          body:
            application/json:
              query: "query { a }"
  - db:
      query: SELECT 1
  - test: "true"
`
	t.Run("trailing ** selects every scalar", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: 'vars.**'\n      forbid: true\n"), book)
		msgs := messagesOf(got, "custom")
		want := []string{"path=vars.flat key=flat value=a", "path=vars.nested.deep[0] key=0 value=b", "path=vars.nested.deep[1] key=1 value=c"}
		if strings.Join(msgs, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%#v", msgs)
		}
	})
	t.Run("** in the middle crosses runner, path, and method", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: 'steps.*.**.query'\n      forbid: true\n"), book)
		msgs := messagesOf(got, "custom")
		want := []string{"path=steps[0].req./graphql.post.body.application/json.query key=query value=query { a }", "path=steps[1].db.query key=query value=SELECT 1"}
		if strings.Join(msgs, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%#v", msgs)
		}
	})
	t.Run("** matches zero levels", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: 'vars.**.flat'\n      forbid: true\n"), book)
		msgs := messagesOf(got, "custom")
		if len(msgs) != 1 || msgs[0] != "path=vars.flat key=flat value=a" {
			t.Fatalf("%#v", msgs)
		}
	})
	t.Run("* still means one level", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: 'steps.*.query'\n      forbid: true\n"), book)
		if msgs := messagesOf(got, "custom"); len(msgs) != 0 {
			t.Fatalf("%#v", msgs)
		}
	})
	t.Run("exists true reports once at the parent before the wildcard", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: 'steps.*.**.timeout'\n      exists: true\n"), book)
		msgs := messagesOf(got, "custom")
		if len(msgs) != 1 || msgs[0] != "path=steps key= value=null" {
			t.Fatalf("%#v", msgs)
		}
	})
	t.Run("trailing ** on a scalar selects that scalar", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: 'desc.**'\n      forbid: true\n"), book)
		msgs := messagesOf(got, "custom")
		if len(msgs) != 1 || msgs[0] != "path=desc key=desc value=x" {
			t.Fatalf("%#v", msgs)
		}
	})
}

// A where path that starts with $. is resolved from the runbook root. It is a gate: when it does not hold, the rule reports nothing for that runbook. It works with exists, equals, matches, and kind, and also with exists: true on the check.
func TestWhereRootPath(t *testing.T) {
	included := "if: included\nvars:\n  id: 01ARZ3NDEKTSV4RRFFQ69G5FAV\nsteps:\n  - test: \"true\"\n"
	standalone := "vars:\n  id: 01ARZ3NDEKTSV4RRFFQ69G5FAV\nsteps:\n  - test: \"true\"\n"
	cases := []struct {
		name    string
		check   string
		src     string
		wantHit bool
		part    string
	}{
		{
			name:    "matches on root if gates the rule in",
			check:   "select: 'vars.*'\n      where:\n        - path: $.if\n          matches: included\n      not_matches: '^[0-9A-Za-z]{26}$'\n",
			src:     included,
			wantHit: true,
			part:    "path=vars.id",
		},
		{
			name:    "matches on root if gates the rule out",
			check:   "select: 'vars.*'\n      where:\n        - path: $.if\n          matches: included\n      not_matches: '^[0-9A-Za-z]{26}$'\n",
			src:     standalone,
			wantHit: false,
		},
		{
			name:    "exists false on root if with exists true on the check",
			check:   "select: labels\n      where:\n        - path: $.if\n          exists: false\n      exists: true\n",
			src:     standalone,
			wantHit: true,
			part:    "path= key= value=null",
		},
		{
			name:    "exists false on root if is not met when if is present",
			check:   "select: labels\n      where:\n        - path: $.if\n          exists: false\n      exists: true\n",
			src:     included,
			wantHit: false,
		},
		{
			name:    "labels present satisfies exists true",
			check:   "select: labels\n      where:\n        - path: $.if\n          exists: false\n      exists: true\n",
			src:     "labels: [a]\n" + standalone,
			wantHit: false,
		},
		{
			name:    "equals on a root value",
			check:   "select: 'steps.*'\n      where:\n        - path: $.debug\n          equals: true\n      forbid: true\n",
			src:     "debug: true\n" + standalone,
			wantHit: true,
			part:    "path=steps[0]",
		},
		{
			name:    "kind on a root value",
			check:   "select: 'steps.*'\n      where:\n        - path: $.vars\n          kind: mapping\n      forbid: true\n",
			src:     standalone,
			wantHit: true,
		},
		{
			name:    "kind on a root value that differs",
			check:   "select: 'steps.*'\n      where:\n        - path: $.vars\n          kind: sequence\n      forbid: true\n",
			src:     standalone,
			wantHit: false,
		},
		{
			name:    "a relative path still filters each value",
			check:   "select: 'steps.*'\n      where:\n        - path: $.if\n          exists: true\n        - path: test\n          exists: true\n      forbid: true\n",
			src:     included,
			wantHit: true,
			part:    "path=steps[0]",
		},
		{
			name:    "root alone is the whole runbook",
			check:   "select: 'steps.*'\n      where:\n        - path: $\n          kind: mapping\n      forbid: true\n",
			src:     standalone,
			wantHit: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintConfig(t, pathRule(tc.check), tc.src)
			msgs := messagesOf(got, "custom")
			if tc.wantHit && len(msgs) == 0 {
				t.Fatalf("診断がありません: %+v", got)
			}
			if !tc.wantHit && len(msgs) > 0 {
				t.Fatalf("診断があります: %#v", msgs)
			}
			if tc.part != "" && !strings.Contains(msgs[0], tc.part) {
				t.Fatalf("message = %#v", msgs)
			}
		})
	}
}

// exists: false and forbid respect where. exists: true reports when no selected value passes where.
func TestWhereAppliesToExists(t *testing.T) {
	src := "steps:\n  - desc: a\n  - desc: b\n    test: \"true\"\n"
	got := lintConfig(t, pathRule("select: 'steps.*'\n      where:\n        - path: test\n          exists: true\n      exists: false\n"), src)
	msgs := messagesOf(got, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "path=steps[1]") {
		t.Fatalf("%#v", msgs)
	}
	got = lintConfig(t, pathRule("select: 'steps.*'\n      where:\n        - path: test\n          exists: true\n      exists: true\n"), src)
	if msgs := messagesOf(got, "custom"); len(msgs) != 0 {
		t.Fatalf("%#v", msgs)
	}
	got = lintConfig(t, pathRule("select: 'steps.*'\n      where:\n        - path: test\n          exists: true\n      exists: true\n"), "steps:\n  - desc: a\n")
	msgs = messagesOf(got, "custom")
	if len(msgs) != 1 || msgs[0] != "path=steps key= value=null" {
		t.Fatalf("%#v", msgs)
	}
}

// kind reports when the selected value has another type. Each of the six names is accepted. The built-in message names the actual and the expected kind.
func TestKindCondition(t *testing.T) {
	const book = "desc: d\nvars:\n  n: 1\n  f: 1.5\n  s: text\n  b: true\n  z: null\n  m: {a: 1}\n  l: [1]\nsteps:\n  - test: \"true\"\n"
	cases := []struct {
		sel, kind string
		hit       bool
	}{
		{"vars.n", "number", false},
		{"vars.f", "number", false},
		{"vars.n", "string", true},
		{"vars.s", "string", false},
		{"vars.s", "number", true},
		{"vars.b", "bool", false},
		{"vars.b", "string", true},
		{"vars.z", "null", false},
		{"vars.z", "string", true},
		{"vars.m", "mapping", false},
		{"vars.m", "sequence", true},
		{"vars.l", "sequence", false},
		{"vars.l", "mapping", true},
		{"steps", "mapping", true},
	}
	for _, tc := range cases {
		t.Run(tc.sel+" "+tc.kind, func(t *testing.T) {
			got := lintConfig(t, pathRule("select: "+tc.sel+"\n      kind: "+tc.kind+"\n"), book)
			msgs := messagesOf(got, "custom")
			if tc.hit != (len(msgs) == 1) {
				t.Fatalf("%#v", msgs)
			}
		})
	}
	t.Run("list steps with the built-in message", func(t *testing.T) {
		got := lintConfig(t, "version: 1\nextends: none\nrules:\n  - id: custom\n    severity: error\n    check:\n      select: steps\n      kind: mapping\n", book)
		msgs := messagesOf(got, "custom")
		if len(msgs) != 1 || !strings.Contains(msgs[0], "sequence") || !strings.Contains(msgs[0], "mapping") {
			t.Fatalf("%#v", msgs)
		}
		for _, f := range got {
			if f.Rule == "custom" && (f.Line != 11 || f.Match != "value kind is sequence, not mapping") {
				t.Fatalf("%+v", f)
			}
		}
	})
	t.Run("map steps pass", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: steps\n      kind: mapping\n"), "steps:\n  a:\n    test: \"true\"\n")
		if msgs := messagesOf(got, "custom"); len(msgs) != 0 {
			t.Fatalf("%#v", msgs)
		}
	})
	t.Run("kind is checked before the other conditions", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: vars.n\n      kind: string\n      matches: '^1$'\n"), book)
		msgs := messagesOf(got, "custom")
		if len(msgs) != 1 {
			t.Fatalf("%#v", msgs)
		}
	})
	t.Run("kind in where filters values", func(t *testing.T) {
		got := lintConfig(t, pathRule("select: 'vars.*'\n      where:\n        - kind: mapping\n      forbid: true\n"), book)
		msgs := messagesOf(got, "custom")
		if len(msgs) != 1 || !strings.Contains(msgs[0], "path=vars.m") {
			t.Fatalf("%#v", msgs)
		}
	})
}

// A segment wrapped in double quotes is one key even when it contains a dot. It also turns off the meaning of *, **, and $. A key with a dot is quoted in path.
func TestQuotedSegment(t *testing.T) {
	const book = `
desc: d
runners:
  req: https://example.com
vars:
  "*": star
  "$": dollar
  "a.b": dotted
steps:
  - req:
      /v1/{id}.json:
        get:
          headers:
            X-A: "1"
`
	got := lintConfig(t, pathRule("select: 'steps.*.req.\"/v1/{id}.json\".get'\n      forbid: true\n"), book)
	msgs := messagesOf(got, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], `path=steps[0].req."/v1/{id}.json".get key=get`) {
		t.Fatalf("%#v", msgs)
	}
	got = lintConfig(t, pathRule("select: 'steps.*.req.\"/v1/{id}.json\"'\n      where:\n        - path: 'get.headers.\"X-A\"'\n          equals: \"1\"\n      forbid: true\n"), book)
	if msgs := messagesOf(got, "custom"); len(msgs) != 1 {
		t.Fatalf("%#v", msgs)
	}
	got = lintConfig(t, pathRule("select: 'vars.\"*\"'\n      forbid: true\n"), book)
	msgs = messagesOf(got, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], "value=star") {
		t.Fatalf("%#v", msgs)
	}
	got = lintConfig(t, pathRule("select: 'vars.\"a.b\"'\n      forbid: true\n"), book)
	msgs = messagesOf(got, "custom")
	if len(msgs) != 1 || !strings.Contains(msgs[0], `path=vars."a.b" key=a.b value=dotted`) {
		t.Fatalf("%#v", msgs)
	}
	got = lintConfig(t, pathRule("select: 'steps.*'\n      where:\n        - path: '$.vars.\"$\"'\n          equals: dollar\n      forbid: true\n"), book)
	if msgs := messagesOf(got, "custom"); len(msgs) != 1 {
		t.Fatalf("%#v", msgs)
	}
}

// files limits a rule to files whose walked path or base name matches, with the same rules as ignore. It applies to uses and check alike, and to a file passed directly.
func TestRuleFiles(t *testing.T) {
	yml := `
version: 1
extends: none
rules:
  - id: only-include
    severity: error
    files: ['integrations/**']
    message: "{{ .path }}"
    check:
      select: 'steps.*'
      where:
        - path: include
          exists: false
      forbid: true
  - id: require-steps
    uses: require-steps
    files: ['setup/**', '*.gen.yml']
`
	src := "desc: d\nsteps:\n  - test: \"true\"\n"
	nosteps := "desc: d\n"
	cases := []struct {
		file string
		src  string
		rule string
		hit  bool
	}{
		{"integrations/a.yml", src, "only-include", true},
		{"integrations/sub/a.yml", src, "only-include", true},
		{"books/a.yml", src, "only-include", false},
		{"integrations.yml", src, "only-include", false},
		{"/abs/integrations/a.yml", src, "only-include", false},
		{"setup/a.yml", nosteps, "require-steps", true},
		{"books/a.yml", nosteps, "require-steps", false},
		{"books/a.gen.yml", nosteps, "require-steps", true},
	}
	for _, tc := range cases {
		t.Run(tc.file+" "+tc.rule, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".runnlint.yaml")
			if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := Lint(tc.file, []byte(tc.src), cfg)
			if hasRule(got, tc.rule) != tc.hit {
				t.Fatalf("%+v", got)
			}
		})
	}
}

// The six guardrail rules from the runbook review are written in YAML and behave as expected on positive and negative inputs.
func TestGuardrailRules(t *testing.T) {
	yml := `
version: 1
extends: none
rules:
  - id: no-literal-uuid
    severity: warning
    message: '{{ .path }} に環境キー無しの UUID があります'
    check:
      select: 'vars.**'
      not_matches: '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-'
  - id: included-vars-have-no-id-default
    severity: warning
    check:
      select: 'vars.*'
      where:
        - path: $.if
          matches: included
      not_matches: '^[0-9A-Za-z]{26}$|^[0-9a-fA-F-]{36}$'
  - id: standalone-needs-labels
    severity: warning
    check:
      select: labels
      where:
        - path: $.if
          exists: false
      exists: true
  - id: integrations-only-include
    severity: error
    files: ['integrations/**']
    check:
      select: 'steps.*'
      where:
        - path: include
          exists: false
      forbid: true
  - id: steps-must-be-map
    severity: error
    check:
      select: steps
      kind: mapping
  - id: no-typename
    severity: warning
    check:
      select: 'steps.*.**.body.*.query'
      not_matches: __typename
`
	const uuid = "0f8fad5a-1234-4b7c-9c3d-123456789abc"
	cases := []struct {
		name string
		file string
		src  string
		rule string
		want int
	}{
		{"uuid in vars at any depth", "books/a.yml", "labels: [a]\nvars:\n  tenant: " + uuid + "\n  nested:\n    ids:\n      - " + uuid + "\n  ok: ${TENANT:-" + uuid + "}\nsteps:\n  s:\n    test: \"true\"\n", "no-literal-uuid", 2},
		{"included runbook with an id default", "books/a.yml", "if: included\nvars:\n  user: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n  tenant: " + uuid + "\n  name: alice\nsteps:\n  s:\n    test: \"true\"\n", "included-vars-have-no-id-default", 2},
		{"standalone runbook with an id default", "books/a.yml", "labels: [a]\nvars:\n  user: 01ARZ3NDEKTSV4RRFFQ69G5FAV\nsteps:\n  s:\n    test: \"true\"\n", "included-vars-have-no-id-default", 0},
		{"standalone without labels", "books/a.yml", "steps:\n  s:\n    test: \"true\"\n", "standalone-needs-labels", 1},
		{"standalone with labels", "books/a.yml", "labels: [a]\nsteps:\n  s:\n    test: \"true\"\n", "standalone-needs-labels", 0},
		{"included without labels", "books/a.yml", "if: included\nsteps:\n  s:\n    test: \"true\"\n", "standalone-needs-labels", 0},
		{"integration step that is not include", "integrations/a.yml", "labels: [a]\nsteps:\n  a:\n    include:\n      path: ./x.yml\n  b:\n    test: \"true\"\n", "integrations-only-include", 1},
		{"integration with include only", "integrations/a.yml", "labels: [a]\nsteps:\n  a:\n    include:\n      path: ./x.yml\n", "integrations-only-include", 0},
		{"same steps outside integrations", "books/a.yml", "labels: [a]\nsteps:\n  b:\n    test: \"true\"\n", "integrations-only-include", 0},
		{"list steps", "books/a.yml", "labels: [a]\nsteps:\n  - test: \"true\"\n", "steps-must-be-map", 1},
		{"map steps", "books/a.yml", "labels: [a]\nsteps:\n  s:\n    test: \"true\"\n", "steps-must-be-map", 0},
		{"typename in a graphql query", "books/a.yml", "labels: [a]\nrunners:\n  req: https://example.com\nsteps:\n  q:\n    req:\n      /graphql:\n        post:\n          body:\n            application/json:\n              query: \"query { user { __typename id } }\"\n", "no-typename", 1},
		{"graphql query without typename", "books/a.yml", "labels: [a]\nrunners:\n  req: https://example.com\nsteps:\n  q:\n    req:\n      /graphql:\n        post:\n          body:\n            application/json:\n              query: \"query { user { id } }\"\n", "no-typename", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintConfigAs(t, yml, tc.file, tc.src)
			if msgs := messagesOf(got, tc.rule); len(msgs) != tc.want {
				t.Fatalf("%s = %#v (all: %+v)", tc.rule, msgs, got)
			}
		})
	}
}
