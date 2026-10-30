package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
)

// requestVariation is one shape of a request value. lines are the mapping lines under the step key; scalar is used instead when set.
// extraIndent is the indent of an "extra: 1" line that the extra-key factor adds, or -1 when the shape has no mapping to add it to.
type requestVariation struct {
	name        string
	lines       []string
	scalar      string
	extraIndent int
	want        []message.ID
	wantArgs    [][]any
	extraWhat   string
}

type requestKind struct {
	rule       string
	key        string
	runners    string
	variations []requestVariation
}

func en(id message.ID, args ...any) string { return message.Text(message.EN, id, args...) }

func requestKinds() []requestKind {
	v := func(name string, lines []string, extraIndent int, extraWhat string, want ...message.ID) requestVariation {
		return requestVariation{name: name, lines: lines, extraIndent: extraIndent, extraWhat: extraWhat, want: want}
	}
	s := func(name, scalar string, want ...message.ID) requestVariation {
		return requestVariation{name: name, scalar: scalar, extraIndent: -1, want: want}
	}
	withArgs := func(rv requestVariation, args ...[]any) requestVariation {
		rv.wantArgs = args
		return rv
	}
	grpc := "gRPC リクエスト"
	return []requestKind{
		{rule: "grpc-request", key: "greq", runners: "  greq: grpc://localhost:8080\n", variations: []requestVariation{
			v("ok", []string{"pkg.Svc/Method:", "  message:", "    name: a"}, 2, grpc),
			s("template", "\"{{ vars.req }}\""),
			v("two-calls", []string{"pkg.Svc/A:", "  message: {}", "pkg.Svc/B:", "  message: {}"}, -1, grpc, message.GRPCOne),
			withArgs(v("bad-shape", []string{"Hello:", "  message: {}"}, 2, grpc, message.GRPCShape), []any{"Hello"}),
			v("value-non-map", []string{"pkg.Svc/Method: x"}, -1, grpc, message.GRPCMap),
			v("both-messages", []string{"pkg.Svc/Method:", "  message: {}", "  messages: [receive]"}, 2, grpc, message.GRPCMsgBoth),
			v("message-string", []string{"pkg.Svc/Method:", "  message: x"}, 2, grpc, message.GRPCMessage),
			v("messages-non-seq", []string{"pkg.Svc/Method:", "  messages: x"}, 2, grpc, message.MessagesArray),
			v("messages-bad-word", []string{"pkg.Svc/Method:", "  messages: [hello]"}, 2, grpc, message.MessagesWord),
			v("messages-elem-number", []string{"pkg.Svc/Method:", "  messages: [1]"}, 2, grpc, message.MessagesElem),
			v("timeout-bad", []string{"pkg.Svc/Method:", "  message: {}", "  timeout: soon"}, 2, grpc, message.TimeoutDur),
			v("trace-bad", []string{"pkg.Svc/Method:", "  message: {}", "  trace: 1"}, 2, grpc, message.TraceBool),
			v("headers-scalar", []string{"pkg.Svc/Method:", "  message: {}", "  headers: 1"}, 2, grpc, message.HeadersMap),
		}},
		{rule: "db-request", key: "db", runners: "  db: mysql://root:pass@localhost:3306/app\n", variations: []requestVariation{
			v("ok", []string{"query: SELECT 1"}, 0, "DB リクエスト"),
			s("template", "\"{{ vars.req }}\""),
			s("non-map", "1", message.DBMap),
			v("query-missing", []string{"trace: true"}, 0, "DB リクエスト", message.QueryEmpty),
			v("query-blank", []string{"query: \"  \""}, 0, "DB リクエスト", message.QueryEmpty),
			v("query-number", []string{"query: 1"}, 0, "DB リクエスト", message.QueryEmpty),
			v("trace-bad", []string{"query: SELECT 1", "trace: 1"}, 0, "DB リクエスト", message.TraceBool),
		}},
		{rule: "exec-request", key: "exec", variations: []requestVariation{
			v("ok", []string{"command: echo ok"}, 0, "exec"),
			s("template", "\"{{ vars.req }}\"", message.ExecMap),
			s("non-map", "echo", message.ExecMap),
			v("command-missing", []string{"stdin: x"}, 0, "exec", message.CommandEmpty),
			v("command-blank", []string{"command: \" \""}, 0, "exec", message.CommandEmpty),
			v("stdin-number", []string{"command: echo", "stdin: 1"}, 0, "exec", message.StdinString),
			v("shell-number", []string{"command: echo", "shell: 1"}, 0, "exec", message.ShellString),
			v("shell-unknown", []string{"command: echo", "shell: zsh"}, 0, "exec", message.ShellValue),
			v("shell-bash", []string{"command: echo", "shell: bash"}, 0, "exec"),
			v("shell-placeholder", []string{"command: echo", "shell: \"fish -c {0}\""}, 0, "exec"),
			v("background-string", []string{"command: echo", "background: later"}, 0, "exec", message.BackgroundBool),
			v("liveOutput-string", []string{"command: echo", "liveOutput: yes please"}, 0, "exec", message.LiveBool),
			v("env-scalar", []string{"command: echo", "env: 1"}, 0, "exec", message.EnvMap),
			withArgs(v("env-value-number", []string{"command: echo", "env:", "  K: 1"}, 0, "exec", message.EnvString), []any{"K"}),
		}},
		{rule: "ssh-request", key: "sc", runners: "  sc: ssh://alice@example.com:22\n", variations: []requestVariation{
			v("ok", []string{"command: hostname"}, 0, "SSH"),
			s("template", "\"{{ vars.req }}\""),
			withArgs(s("non-map", "1", message.RequestMap), []any{"SSH"}),
			v("command-missing", []string{"{}"}, -1, "SSH", message.CommandEmpty),
			v("command-blank", []string{"command: \" \""}, 0, "SSH", message.CommandEmpty),
		}},
		{rule: "cdp-request", key: "br", runners: "  br: cdp://127.0.0.1:9222\n", variations: []requestVariation{
			v("ok", []string{"actions: [title]"}, 0, "CDP リクエスト"),
			s("template", "\"{{ vars.req }}\""),
			s("non-map", "1", message.CDPMap),
			v("actions-missing", []string{"{}"}, -1, "CDP リクエスト", message.ActionsEmpty),
			v("actions-empty", []string{"actions: []"}, 0, "CDP リクエスト", message.ActionsEmpty),
			v("actions-string", []string{"actions: title"}, 0, "CDP リクエスト", message.ActionsEmpty),
			withArgs(v("unknown-string", []string{"actions: [nope]"}, 0, "CDP リクエスト", message.CDPUnknown), []any{"nope"}),
			withArgs(v("string-needs-args", []string{"actions: [navigate]"}, 0, "CDP リクエスト", message.CDPArgsNeed), []any{"navigate"}),
			v("map-two-keys", []string{"actions:", "  - navigate: https://example.com", "    title: null"}, 0, "CDP リクエスト", message.CDPOneKey),
			withArgs(v("map-unknown", []string{"actions:", "  - nope: x"}, 0, "CDP リクエスト", message.CDPUnknown), []any{"nope"}),
			withArgs(v("map-string-noargs", []string{"actions:", "  - title: x"}, 0, "CDP リクエスト", message.CDPNoArgs), []any{"title"}),
			withArgs(v("map-string-multiargs", []string{"actions:", "  - sendKeys: x"}, 0, "CDP リクエスト", message.CDPArgMap), []any{"sendKeys", "sel, value"}),
			v("map-string-onearg", []string{"actions:", "  - navigate: https://example.com"}, 0, "CDP リクエスト"),
			withArgs(v("map-map-noargs", []string{"actions:", "  - title:", "      x: 1"}, 0, "CDP リクエスト", message.CDPNoArgs), []any{"title"}),
			withArgs(v("map-map-unknown-key", []string{"actions:", "  - navigate:", "      foo: x"}, 0, "CDP リクエスト", message.NoKey), []any{"CDP アクション navigate", "foo"}),
			v("map-map-ok", []string{"actions:", "  - sendKeys:", "      sel: a", "      value: b"}, 0, "CDP リクエスト"),
			withArgs(v("map-value-number", []string{"actions:", "  - navigate: 1"}, 0, "CDP リクエスト", message.CDPArgType), []any{"navigate"}),
			v("elem-number", []string{"actions: [1]"}, 0, "CDP リクエスト", message.CDPElemType),
		}},
		{rule: "agent-request", key: "ai", runners: "  ai:\n    agent: claude\n", variations: []requestVariation{
			v("ok", []string{"prompt: hello"}, 0, "agent リクエスト"),
			s("template", "\"{{ vars.req }}\""),
			s("non-map", "1", message.AgentMap),
			v("prompt-missing", []string{"clearContext: true"}, 0, "agent リクエスト", message.PromptString),
			v("prompt-number", []string{"prompt: 1"}, 0, "agent リクエスト", message.PromptString),
			v("clearContext-string", []string{"prompt: hello", "clearContext: maybe"}, 0, "agent リクエスト", message.ClearBool),
		}},
		{rule: "include-request", key: "include", variations: []requestVariation{
			s("string-ok", "other.yml"),
			s("string-empty", "\"\"", message.IncludeEmpty),
			v("map-path-ok", []string{"path: other.yml"}, 0, "include"),
			withArgs(v("map-both", []string{"path: other.yml", "steps:", "  - exec:", "      command: echo"}, 0, "include", message.IncludeBoth, message.NoKey), nil, []any{"include", "steps"}),
			v("map-neither", []string{"vars: {}"}, -1, "include", message.IncludeNeed),
			v("path-empty", []string{"path: \"\""}, 0, "include", message.IncludePath),
			v("vars-non-map", []string{"path: other.yml", "vars: x"}, 0, "include", message.IncludeVars),
			v("steps-runners-non-map", []string{"steps:", "  - exec:", "      command: echo", "runners: x"}, 0, "include", message.IncludeRunners),
			v("desc-number", []string{"steps:", "  - exec:", "      command: echo", "desc: 1"}, 0, "include", message.IncludeDesc),
			withArgs(v("skipTest-string", []string{"path: other.yml", "skipTest: maybe"}, 0, "include", message.IncludeBool), []any{"skipTest"}),
			withArgs(v("force-string", []string{"path: other.yml", "force: maybe"}, 0, "include", message.IncludeBool), []any{"force"}),
			s("number", "1", message.IncludeType),
		}},
	}
}

// Every pair of levels of the request-shape factors behaves as documented for each runner kind: the kind-specific shape rules, an extra key
// that the kind does not know (reported only where the request is a mapping that is checked), and list or map steps. A template string in
// place of the request skips the shape checks, except for exec, which must always be a mapping.
func TestRequestShapesPairwise(t *testing.T) {
	for _, kind := range requestKinds() {
		kind := kind
		t.Run(kind.rule, func(t *testing.T) {
			var names []string
			for _, rv := range kind.variations {
				names = append(names, rv.name)
			}
			factors := []factor{
				{"variation", names},
				{"extra", []string{"none", "extra-key"}},
				{"style", []string{"list", "map"}},
			}
			expected, unexpected := 0, 0
			runPairwise(t, factors, func(t *testing.T, p pick) {
				var rv requestVariation
				for _, cand := range kind.variations {
					if cand.name == p["variation"] {
						rv = cand
					}
				}
				var b strings.Builder
				b.WriteString("desc: d\nvars:\n  req: x\n")
				if kind.runners != "" {
					b.WriteString("runners:\n" + kind.runners)
				}
				b.WriteString("steps:\n")
				lines := append([]string{}, rv.lines...)
				extra := p["extra"] == "extra-key" && rv.extraIndent >= 0
				if extra {
					lines = append(lines, strings.Repeat(" ", rv.extraIndent)+"extra: 1")
				}
				// The step key sits at column 4 in both styles, so its children are indented by 6.
				head := "  - "
				indent := "      "
				if p["style"] == "map" {
					b.WriteString("  subject:\n")
					head = "    "
				}
				if rv.scalar != "" {
					b.WriteString(head + kind.key + ": " + rv.scalar + "\n")
				} else {
					b.WriteString(head + kind.key + ":\n")
					for _, l := range lines {
						b.WriteString(indent + l + "\n")
					}
				}
				if p["style"] == "map" {
					b.WriteString("  verify:\n    test: \"true\"\n")
				} else {
					b.WriteString("  - test: \"true\"\n")
				}
				src := b.String()
				got := lint(t, src)
				var gotMatch []string
				for _, f := range findRule(got, kind.rule) {
					gotMatch = append(gotMatch, f.Match)
				}
				var want []string
				for i, id := range rv.want {
					var args []any
					if i < len(rv.wantArgs) {
						args = rv.wantArgs[i]
					}
					want = append(want, en(id, args...))
				}
				if extra {
					want = append(want, en(message.NoKey, rv.extraWhat, "extra"))
				}
				if len(want) > 0 {
					expected++
				} else {
					unexpected++
				}
				sort.Strings(want)
				sort.Strings(gotMatch)
				if strings.Join(want, "\n") != strings.Join(gotMatch, "\n") {
					t.Fatalf("%s\nwant:\n%s\ngot:\n%s\n%s", describe(p), strings.Join(want, "\n"), strings.Join(gotMatch, "\n"), src)
				}
			})
			if expected < 3 || unexpected < 3 {
				t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
			}
		})
	}
}
