package rules

import "testing"

// A missing key is reported on the parent line. A present value that has the wrong type is reported on that value's line.
// That is the same split parse functions make between a missing key and a value of the wrong type.
func TestFieldFindingLine(t *testing.T) {
	cases := []struct {
		name string
		src  string
		rule string
		msg  string
		line int
	}{
		{
			name: "db query missing",
			src: `
desc: d
runners:
  db: mysql://u@h/app
steps:
  - db: {}
`,
			rule: "db-request",
			msg:  "query は空でない文字列です",
			line: 6,
		},
		{
			name: "db query number",
			src: `
desc: d
runners:
  db: mysql://u@h/app
steps:
  - db:
      query: 1
`,
			rule: "db-request",
			msg:  "query は空でない文字列です",
			line: 7,
		},
		{
			name: "ssh command missing",
			src: `
desc: d
runners:
  sc: ssh://user@example.com
steps:
  - sc: {}
`,
			rule: "ssh-request",
			msg:  "command は空でない文字列です",
			line: 6,
		},
		{
			name: "ssh command blank",
			src: `
desc: d
runners:
  sc: ssh://user@example.com
steps:
  - sc:
      command: ""
`,
			rule: "ssh-request",
			msg:  "command は空でない文字列です",
			line: 7,
		},
		{
			name: "exec command missing",
			src: `
desc: d
steps:
  - exec: {}
`,
			rule: "exec-request",
			msg:  "command は空でない文字列です",
			line: 4,
		},
		{
			name: "agent prompt missing",
			src: `
desc: d
runners:
  ai:
    agent: claude
    model: opus
steps:
  - ai: {}
`,
			rule: "agent-request",
			msg:  "prompt は文字列です",
			line: 8,
		},
		{
			name: "agent prompt number",
			src: `
desc: d
runners:
  ai:
    agent: claude
    model: opus
steps:
  - ai:
      prompt: 1
`,
			rule: "agent-request",
			msg:  "prompt は文字列です",
			line: 9,
		},
		{
			name: "cdp actions missing",
			src: `
desc: d
runners:
  br: cdp://127.0.0.1:9222
steps:
  - br: {}
`,
			rule: "cdp-request",
			msg:  "actions は空でない配列です",
			line: 6,
		},
		{
			name: "http body missing",
			src: `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        post: {}
`,
			rule: "http-request",
			msg:  "POST には media type 付きの body が必要です",
			line: 8,
		},
		{
			name: "steps missing",
			src: `
desc: d
runners:
  req: https://example.com
`,
			rule: "require-steps",
			msg:  "steps がありません",
			line: 2,
		},
		{
			name: "steps empty",
			src: `
desc: d
steps: []
`,
			rule: "require-steps",
			msg:  "steps がありません",
			line: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lint(t, tc.src)
			found := findRule(got, tc.rule)
			if len(found) != 1 || found[0].Message != tc.msg || found[0].Line != tc.line {
				t.Fatalf("%+v", got)
			}
		})
	}
}

// A numeric DB request is not a mapping, and that finding sits on the value's line. Accepting a template is covered in cov_step.
func TestRequestExpandSkipKeepsLine(t *testing.T) {
	got := lint(t, `
desc: d
vars:
  v: x
runners:
  db: mysql://u@h/app
steps:
  - db: 1
`)
	found := findRule(got, "db-request")
	if len(found) != 1 || found[0].Message != "DB リクエストは mapping です" || found[0].Line != 8 {
		t.Fatalf("%+v", got)
	}
}
