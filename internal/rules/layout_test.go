package rules

import "testing"

// runn's parse functions return at the first problem. The linter reads the same keys in the same order and still checks the rest.
// Findings for the later keys of one step remain.
func TestParseWalkKeepsLaterFindings(t *testing.T) {
	cases := []struct {
		name string
		src  string
		rule string
		want []string
	}{
		{
			name: "grpc",
			src: `
desc: d
runners:
  greq: grpc://localhost:8080
steps:
  - greq:
      pkg.Svc/Method:
        timeout: abc
        message: 1
`,
			rule: "grpc-request",
			want: []string{"timeout は時間です", "message は mapping です"},
		},
		{
			name: "db",
			src: `
desc: d
runners:
  db: mysql://u@h/app
steps:
  - db:
      query: 1
      trace: 1
`,
			rule: "db-request",
			want: []string{"query は空でない文字列です", "trace は bool です"},
		},
		{
			name: "exec",
			src: `
desc: d
steps:
  - exec:
      command: ""
      shell: 1
      background: 1
`,
			rule: "exec-request",
			want: []string{"command は空でない文字列です", "shell は文字列です", "background は bool です"},
		},
		{
			name: "include",
			src: `
desc: d
steps:
  - include:
      path: ""
      steps: []
`,
			rule: "include-request",
			want: []string{"path と steps を同時に指定できません", "include にキー steps はありません", "include.path は空でない文字列です"},
		},
		{
			name: "http runner",
			src: `
desc: d
runners:
  req:
    endpoint: https://example.com
    timeout: abc
    skipVerify: 1
steps:
  - req:
      /:
        get:
          body: null
`,
			rule: "runner-config",
			want: []string{"timeout は時間です", "skipVerify は bool または文字列です"},
		},
		{
			name: "step fields",
			src: `
desc: d
steps:
  - desc: 1
    if: 1
    test: 1
    exec:
      command: "true"
`,
			rule: "step-shape",
			want: []string{"desc は文字列です", "if は式または bool です", "test は式または bool です"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, part := range tc.want {
				requireMessage(t, tc.src, tc.rule, part)
			}
		})
	}
}
