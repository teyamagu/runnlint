package rules

import "testing"

// step-shape reports if, defer, force, and test that are numbers, a dump.out that is not a string, a dump that is a number, and a bind that is a string.
func TestStepFieldTypes(t *testing.T) {
	cases := []struct {
		name string
		step string
		part string
	}{
		{"if", "    if: 1\n", "if は式または bool です"},
		{"defer", "    defer: 1\n", "defer は bool または文字列です"},
		{"force", "    force: 1\n", "force は bool または文字列です"},
		{"test", "    test: 1\n", "test は式または bool です"},
		{"dump out", "    dump:\n      expr: x\n      out: 1\n", "dump.out は文字列です"},
		{"dump number", "    dump: 1\n", "dump は式または mapping です"},
		{"bind string", "    bind: x\n", "bind は mapping です"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireMessage(t, "desc: d\nsteps:\n  - exec:\n      command: echo ok\n"+tc.step, "step-shape", tc.part)
		})
	}
}

// When the runner's kind is unknown because its definition is a template, the step value decides the request check.
// A query is a DB request, actions a CDP request, prompt an agent request, command an SSH request, a /path key an HTTP request, and a pkg.Svc/Method key a gRPC request.
func TestGuessStepByValue(t *testing.T) {
	const head = "desc: d\nvars:\n  ep: https://example.com\nrunners:\n  r: \"{{ vars.ep }}\"\nsteps:\n"
	cases := []struct {
		name string
		step string
		rule string
		part string
	}{
		{"db", "  - r:\n      query: \"\"\n", "db-request", "query"},
		{"cdp", "  - r:\n      actions: []\n", "cdp-request", "actions は空でない配列です"},
		{"agent", "  - r:\n      prompt: 1\n", "agent-request", "prompt"},
		{"ssh", "  - r:\n      command: \"\"\n", "ssh-request", "command は空でない文字列です"},
		{"http by path", "  - r:\n      /x:\n        foo: {}\n", "http-request", "HTTP メソッド foo は使えません"},
		{"grpc by method", "  - r:\n      pkg.Svc/M:\n        message: 1\n", "grpc-request", "message"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireMessage(t, head+tc.step, tc.rule, tc.part)
		})
	}
	for _, step := range []string{"  - r: \"{{ vars.req }}\"\n", "  - r: 1\n", "  - r:\n      a:\n        b: 1\n"} {
		got := lint(t, head+step)
		for _, rule := range []string{"http-request", "grpc-request", "db-request", "ssh-request", "cdp-request", "agent-request"} {
			if hasRule(got, rule) {
				t.Fatalf("%s: %+v", step, got)
			}
		}
	}
}

// Each request check accepts a step value that is one template, and reports a value that is not a mapping with its own sentence.
func TestRequestTemplateAndNonMapping(t *testing.T) {
	const runners = "desc: d\nvars:\n  v: x\nrunners:\n  req: https://example.com\n  greq: grpc://localhost:8080\n  db: mysql://u@h/app\n  sc: ssh://user@example.com\n  br: cdp://127.0.0.1:9222\n  ai:\n    agent: claude\n    model: opus\nsteps:\n"
	cases := []struct {
		runner string
		rule   string
		part   string
	}{
		{"req", "http-request", "HTTP"},
		{"greq", "grpc-request", "gRPC"},
		{"db", "db-request", "DB リクエストは mapping です"},
		{"sc", "ssh-request", "SSH リクエストは mapping です"},
		{"br", "cdp-request", "CDP リクエストは mapping です"},
		{"ai", "agent-request", "agent リクエストは mapping です"},
	}
	for _, tc := range cases {
		t.Run(tc.runner, func(t *testing.T) {
			rejectMessage(t, runners+"  - "+tc.runner+": \"{{ vars.v }}\"\n", tc.rule, tc.part)
			requireMessage(t, runners+"  - "+tc.runner+": 1\n", tc.rule, tc.part)
		})
	}
	requireMessage(t, "desc: d\nsteps:\n  - exec: 1\n", "exec-request", "exec は mapping です")
}

// HTTP body checks: a POST whose body is null reports the body node, a template body is accepted, an empty media type is reported, a null media payload on POST is reported, and a media type with parameters is read by its base type.
func TestHTTPBodyRemainingBranches(t *testing.T) {
	const head = "desc: d\nvars:\n  b: x\nrunners:\n  req: https://example.com\nsteps:\n  - req:\n      /a:\n        post:\n"
	got := lint(t, head+"          body: null\n")
	found := findRule(got, "http-request")
	if len(found) != 1 || found[0].Message != "POST には media type 付きの body が必要です" || found[0].Line != 10 {
		t.Fatalf("%+v", got)
	}
	rejectMessage(t, head+"          body: \"{{ vars.b }}\"\n", "http-request", "body")
	requireMessage(t, head+"          body:\n            \"\": {}\n", "http-request", "body の media type が空です")
	requireMessage(t, head+"          body:\n            application/json: null\n", "http-request", "POST の body が null です")
	rejectMessage(t, head+"          body:\n            \"text/plain; charset=utf-8\": hello\n", "http-request", "body")
}

// gRPC checks: a method value that is not a mapping is reported, and a template message, a template messages, and a messages element that is a mapping are accepted.
func TestGRPCRemainingBranches(t *testing.T) {
	const head = "desc: d\nvars:\n  m: x\nrunners:\n  greq: grpc://localhost:8080\nsteps:\n  - greq:\n"
	requireMessage(t, head+"      pkg.Svc/M: 1\n", "grpc-request", "gRPC メソッドの値は mapping です")
	rejectMessage(t, head+"      pkg.Svc/M:\n        message: \"{{ vars.m }}\"\n", "grpc-request", "message")
	rejectMessage(t, head+"      pkg.Svc/M:\n        messages: \"{{ vars.m }}\"\n", "grpc-request", "messages")
	rejectMessage(t, head+"      pkg.Svc/M:\n        messages:\n          - a: 1\n", "grpc-request", "messages")
}

// exec checks: a shell that is not a string and a shell that is neither bash, sh, nor a {0} command are reported.
func TestExecShellBranches(t *testing.T) {
	requireMessage(t, "desc: d\nsteps:\n  - exec:\n      command: echo ok\n      shell: 1\n", "exec-request", "shell は文字列です")
	requireMessage(t, "desc: d\nsteps:\n  - exec:\n      command: echo ok\n      shell: zsh\n", "exec-request", "shell は bash、sh、または {0} を含むコマンドです")
}

// CDP checks: actions that is not an array is reported on the actions node, and an unknown action written as a mapping is reported by its key.
func TestCDPRemainingBranches(t *testing.T) {
	const head = "desc: d\nrunners:\n  br: cdp://127.0.0.1:9222\nsteps:\n  - br:\n"
	got := lint(t, head+"      actions: 1\n")
	found := findRule(got, "cdp-request")
	if len(found) != 1 || found[0].Message != "actions は空でない配列です" || found[0].Line != 6 {
		t.Fatalf("%+v", got)
	}
	requireMessage(t, head+"      actions:\n        - nope: 1\n", "cdp-request", "CDP アクション nope はありません")
}

// include checks: runners that is not a mapping in an inline include, and desc that is not a string, are reported.
func TestIncludeRunnersAndDesc(t *testing.T) {
	requireMessage(t, "desc: d\nsteps:\n  - include:\n      runners: 1\n      steps:\n        - test: \"true\"\n", "include-request", "include.runners は mapping です")
	requireMessage(t, "desc: d\nsteps:\n  - include:\n      path: child.yml\n      desc: 1\n", "include-request", "include.desc は文字列です")
}

// headers given as one template are accepted.
func TestHeadersTemplate(t *testing.T) {
	rejectMessage(t, "desc: d\nvars:\n  h: x\nrunners:\n  req: https://example.com\nsteps:\n  - req:\n      /:\n        get:\n          headers: \"{{ vars.h }}\"\n          body: null\n", "http-request", "headers")
}
