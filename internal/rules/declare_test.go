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

// 次の各 check だけを有効にして lint したとき、違反する入力では custom が出て、一致する入力では出ないことを確認する。メッセージに指定した断片が含まれることも確認する。
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
			// secret が無い runbook に exists: true をかけたとき、custom が出てメッセージに rule=custom が含まれることを確認する。
			name:    "exists missing",
			check:   "select: secret\n      exists: true\n",
			src:     "steps:\n  - test: \"true\"\n",
			wantHit: true,
			part:    "rule=custom",
		},
		{
			// debug がある runbook に exists: false をかけたとき、custom が出ることを確認する。
			name:    "exists false",
			check:   "select: debug\n      exists: false\n",
			src:     book,
			wantHit: true,
		},
		{
			// debug が true の runbook に equals: false をかけたとき、custom が出てメッセージに value=true が含まれることを確認する。
			name:    "equals miss",
			check:   "select: debug\n      equals: false\n",
			src:     book,
			wantHit: true,
			part:    "value=true",
		},
		{
			// debug が true の runbook に equals: true をかけたとき、custom が出ないことを確認する。
			name:    "equals hit",
			check:   "select: debug\n      equals: true\n",
			src:     book,
			wantHit: false,
		},
		{
			// debug が null の runbook に equals: null をかけたとき、custom が出ないことを確認する。
			name:    "equals null",
			check:   "select: debug\n      equals: null\n",
			src:     "debug: null\nsteps:\n  - test: \"true\"\n",
			wantHit: false,
		},
		{
			// debug が true の runbook に not_equals: true をかけたとき、custom が出ることを確認する。
			name:    "not equals",
			check:   "select: debug\n      not_equals: true\n",
			src:     book,
			wantHit: true,
		},
		{
			// vars.env が staging の runbook に one_of: [dev, prod] をかけたとき、custom が出てメッセージに path=vars.env が含まれることを確認する。
			name:    "one of",
			check:   "select: vars.env\n      one_of: [dev, prod]\n",
			src:     book,
			wantHit: true,
			part:    "path=vars.env",
		},
		{
			// vars.env が staging の runbook に one_of: [dev, staging] をかけたとき、custom が出ないことを確認する。
			name:    "one of pass",
			check:   "select: vars.env\n      one_of: [dev, staging]\n",
			src:     book,
			wantHit: false,
		},
		{
			// desc が ab の runbook に min_len: 3 をかけたとき、custom が出ることを確認する。
			name:    "min len",
			check:   "select: desc\n      min_len: 3\n",
			src:     book,
			wantHit: true,
		},
		{
			// labels が1件の runbook に max_len: 0 をかけたとき、custom が出ることを確認する。
			name:    "max len sequence",
			check:   "select: labels\n      max_len: 0\n",
			src:     book,
			wantHit: true,
		},
		{
			// vars.env が staging の runbook に not_matches: staging をかけたとき、custom が出ることを確認する。
			name:    "not matches",
			check:   "select: vars.env\n      not_matches: staging\n",
			src:     book,
			wantHit: true,
		},
		{
			// vars.env が staging の runbook に matches: ^staging$ をかけたとき、custom が出ないことを確認する。
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

// test がある step を forbid したとき、配列 steps では key=0 と path=steps[0] の1件だけが出ることを確認する。名前付き steps では key=login と path=steps.login の1件だけが出ることを確認する。
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

// matches に '(' を書いたルールで lint したとき、custom が1件出てメッセージに「正規表現」が含まれることを確認する。
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

// desc が無くても、severity が off の exists ルールでは custom が出ないことを確認する。
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
