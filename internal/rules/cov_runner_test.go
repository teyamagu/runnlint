package rules

// Lines that are not covered on purpose:
// runner.go checkDBString's final RunnerScheme: a string runner with :// and an unknown scheme is detected as kind unknown, never as DB, so the DB check never sees it.
// runner.go RunnerEpString: a mapping runner is detected as HTTP only when endpoint is a non-empty string, so endpoint can never be a non-string there.
// runner.go RunnerHost: a mapping runner is detected as SSH only when host or hostname is a non-empty string, so neither can be missing there.

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
)

// runner-config reports each of the following runner definitions with its own sentence.
func TestRunnerConfigRemainingBranches(t *testing.T) {
	cases := []struct {
		name    string
		runners string
		step    string
		part    string
	}{
		{"unknown kind", "  x:\n    foo: 1\n", "  - x: 1\n", "runner x の種別を判定できません"},
		{"url without host", "  req: \"https://\"\n", "  - req:\n      /:\n        get:\n          body: null\n", "runner req の endpoint は http(s) の URL です"},
		{"empty dsn", "  db: \"\"\n", "  - db:\n      query: SELECT 1\n", "runner db の DSN が空です"},
		{"db trace", "  db:\n    dsn: mysql://u@h/app\n    trace: 1\n", "  - db:\n      query: SELECT 1\n", "trace は bool または文字列です"},
		{"cdp timeout", "  br:\n    addr: cdp://127.0.0.1:9222\n    timeout: abc\n", "  - br:\n      actions:\n        - title\n", "timeout は時間です"},
		{"cdp flags", "  br:\n    addr: cdp://127.0.0.1:9222\n    flags: 1\n", "  - br:\n      actions:\n        - title\n", "runner br の flags は mapping です"},
		{"include params", "  inc:\n    path: child.yml\n    params: 1\n", "  - test: \"true\"\n", "runner inc の params は mapping です"},
		{"agent model missing", "  ai:\n    agent: claude\n", "  - ai:\n      prompt: hi\n", "runner ai には model が必要です"},
		{"agent model empty", "  ai:\n    agent: claude\n    model: \"\"\n", "  - ai:\n      prompt: hi\n", "runner ai には model が必要です"},
		{"trace header", "  req:\n    endpoint: https://example.com\n    trace:\n      headerName: 1\n", "  - req:\n      /:\n        get:\n          body: null\n", "trace.headerName は文字列です"},
		{"protos element", "  greq:\n    addr: localhost:50051\n    protos:\n      - 1\n", "  - greq:\n      pkg.Svc/M:\n        message: {}\n", "protos の要素は文字列です"},
		{"port bool", "  sc:\n    host: h\n    port: true\n", "  - sc:\n      command: hostname\n", "port は数値です"},
		{"keyboard not array", "  sc:\n    host: h\n    keyboardInteractive: 1\n", "  - sc:\n      command: hostname\n", "keyboardInteractive は配列です"},
		{"keyboard element", "  sc:\n    host: h\n    keyboardInteractive:\n      - 1\n", "  - sc:\n      command: hostname\n", "keyboardInteractive の要素は mapping です"},
		{"keyboard field", "  sc:\n    host: h\n    keyboardInteractive:\n      - match: 1\n        answer: a\n", "  - sc:\n      command: hostname\n", "keyboardInteractive.match は文字列です"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireMessage(t, "desc: d\nrunners:\n"+tc.runners+"steps:\n"+tc.step, "runner-config", tc.part)
		})
	}
}

// An HTTP endpoint with a template, a DB DSN in the user@tcp(host) form, and a trace given as a bool are accepted.
func TestRunnerConfigAcceptedForms(t *testing.T) {
	rejectMessage(t, "desc: d\nvars:\n  h: example.com\nrunners:\n  req: \"https://{{ vars.h }}\"\nsteps:\n  - req:\n      /:\n        get:\n          body: null\n", "runner-config", "req")
	rejectMessage(t, "desc: d\nrunners:\n  db: \"user:pass@tcp(localhost:3306)/app\"\nsteps:\n  - db:\n      query: SELECT 1\n", "runner-config", "db")
	rejectMessage(t, "desc: d\nrunners:\n  req:\n    endpoint: https://example.com\n    trace: true\nsteps:\n  - req:\n      /:\n        get:\n          body: null\n", "runner-config", "trace")
}

// A reserved name defined with runner: on a step reports runner-reserved on that key. runnerDef has no definition for an unknown kind, and << is a step keyword.
func TestStepRunnerReservedAndHelpers(t *testing.T) {
	got := lint(t, "desc: d\nsteps:\n  - runner:\n      test: ssh://user@example.com:22\n  - exec:\n      command: echo ok\n  - test: \"true\"\n")
	found := findRule(got, "runner-reserved")
	if len(found) != 1 || found[0].Line != 4 || found[0].Message != "runner 名 test は runn の予約語です" {
		t.Fatalf("%+v", got)
	}
	if hasRule(got, "unused-runner") {
		t.Fatalf("%+v", got)
	}
	if runnerDef(runbook.KindUnknown) != "" {
		t.Fatal("unknown kind has a definition")
	}
	if !isStepKeyword("<<") {
		t.Fatal("<< is not a step keyword")
	}
}
