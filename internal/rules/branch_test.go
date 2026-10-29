package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
)

func requireMessage(t *testing.T, src, rule, part string) {
	t.Helper()
	got := lint(t, src)
	for _, f := range got {
		if f.Rule == rule && strings.Contains(f.Message, part) {
			return
		}
	}
	t.Fatalf("%s に %q がありません: %+v", rule, part, got)
}

func rejectMessage(t *testing.T, src, rule, part string) {
	t.Helper()
	got := lint(t, src)
	for _, f := range got {
		if f.Rule == rule && strings.Contains(f.Message, part) {
			t.Fatalf("出てほしくない診断です: %+v", f)
		}
	}
}

// Linting each of the following HTTP steps reports http-request with the expected text, or does not report it.
func TestHTTPRequestBranches(t *testing.T) {
	const head = "desc: x\nrunners:\n  req: https://example.com\nsteps:\n  - req:\n"
	// A step that writes both paths /a and /b reports http-request "an HTTP request is a mapping with exactly one path".
	requireMessage(t, head+"      /a:\n        get:\n          body: null\n      /b:\n        get:\n          body: null\n", "http-request", "path を1つだけ")
	// A step whose path is an empty string reports http-request "HTTP path is empty".
	requireMessage(t, head+"      ? \"\"\n      : \n        get:\n          body: null\n", "http-request", "path が空です")
	// A step that writes both get and post reports http-request "an HTTP request has one method".
	requireMessage(t, head+"      /a:\n        get:\n          body: null\n        post:\n          body:\n            text/plain: hi\n", "http-request", "メソッドは1つ")
	// A step whose method is fetch reports http-request "HTTP method fetch is not allowed".
	requireMessage(t, head+"      /a:\n        fetch:\n          body: null\n", "http-request", "メソッド fetch は使えません")
	// A step whose headers is an array reports http-request "headers is a mapping".
	requireMessage(t, head+"      /a:\n        get:\n          headers: []\n          body: null\n", "http-request", "headers は mapping")
	// A step whose Accept array element is a number reports http-request "an element of header Accept is a string".
	requireMessage(t, head+"      /a:\n        get:\n          headers:\n            Accept:\n              - 1\n          body: null\n", "http-request", "要素は文字列")
	// A step whose Accept value is a number reports http-request "header Accept is a string or an array of strings".
	requireMessage(t, head+"      /a:\n        get:\n          headers:\n            Accept: 1\n          body: null\n", "http-request", "文字列または文字列の配列")
	// A step whose useCookie is a number reports http-request "useCookie is a bool".
	requireMessage(t, head+"      /a:\n        get:\n          useCookie: 1\n          body: null\n", "http-request", "useCookie は bool")
	// A step whose request trace is a number reports http-request "trace is a bool".
	requireMessage(t, head+"      /a:\n        get:\n          trace: 1\n          body: null\n", "http-request", "trace は bool")
	// A POST whose text/plain body is a number reports http-request "a text/plain body is a string".
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            text/plain: 1\n", "http-request", "text/plain の body は文字列")
	// A POST whose form-urlencoded body is a string reports http-request "an application/x-www-form-urlencoded body is a mapping".
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            application/x-www-form-urlencoded: a=b\n", "http-request", "form-urlencoded の body は mapping")
	// A POST whose multipart body is a string reports http-request "a multipart/form-data body is a mapping or an array".
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            multipart/form-data: file\n", "http-request", "multipart/form-data の body は mapping")
	// A POST that writes both json and text/plain reports http-request "body has one media type".
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            application/json:\n              a: 1\n            text/plain: hi\n", "http-request", "media type は1つ")
	// A POST whose octet-stream mapping has no filename reports http-request "a mapping body of media type application/octet-stream needs filename".
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            application/octet-stream:\n              name: file\n", "http-request", "filename が必要")
	// A step with extra directly under post reports http-request "HTTP request has no key extra".
	requireMessage(t, head+"      /a:\n        post:\n          extra: 1\n          body:\n            text/plain: hi\n", "http-request", "キー extra はありません")

	// A step whose GET body is null does not report http-request for a missing body.
	rejectMessage(t, head+"      /a:\n        get:\n          body: null\n", "http-request", "body")
	// A step whose path starts with https:// does not report http-request "HTTP path https://example.com/a must start with /".
	rejectMessage(t, head+"      https://example.com/a:\n        get:\n          body: null\n", "http-request", "から始めます")
	// A POST with a json mapping body does not report http-request for a missing filename.
	rejectMessage(t, head+"      /a:\n        post:\n          body:\n            application/json:\n              a: 1\n", "http-request", "filename")
	// A POST whose octet-stream has filename does not report http-request for a missing filename.
	rejectMessage(t, head+"      /a:\n        post:\n          body:\n            application/octet-stream:\n              filename: a.bin\n", "http-request", "filename")
	// A step whose whole request is {{ vars.req }} does not report http-request.
	rejectMessage(t, "desc: x\nrunners:\n  req: https://example.com\nsteps:\n  - req: \"{{ vars.req }}\"\n", "http-request", "HTTP")
}

// Linting each of the following gRPC steps and DB steps reports grpc-request or db-request with the expected text, or does not report it.
func TestGRPCAndDBBranches(t *testing.T) {
	const grpc = "desc: x\nrunners:\n  greq: grpc://localhost:8080\nsteps:\n  - greq:\n"
	// A gRPC step that writes two methods reports grpc-request "a gRPC request is a mapping with exactly one service/method".
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message:\n          name: a\n      other.Svc/M:\n        message:\n          name: b\n", "grpc-request", "1つだけ")
	// A gRPC step whose message is an array reports grpc-request "message is a mapping".
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message: []\n", "grpc-request", "message は mapping")
	// A gRPC step that writes both message and messages reports grpc-request "message and messages cannot both be set".
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message:\n          name: a\n        messages:\n          - receive\n", "grpc-request", "同時に指定できません")
	// A gRPC step whose messages is a string reports grpc-request "messages is an array".
	requireMessage(t, grpc+"      pkg.Svc/M:\n        messages: receive\n", "grpc-request", "messages は配列")
	// A gRPC step whose messages element is wait reports grpc-request "a string in messages is receive or close".
	requireMessage(t, grpc+"      pkg.Svc/M:\n        messages:\n          - wait\n", "grpc-request", "receive または close")
	// A gRPC step whose messages element is a number reports grpc-request "an element of messages is a mapping, receive, or close".
	requireMessage(t, grpc+"      pkg.Svc/M:\n        messages:\n          - 1\n", "grpc-request", "mapping、receive、close")
	// A gRPC step whose timeout is later reports grpc-request "timeout is a duration".
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message:\n          name: a\n        timeout: later\n", "grpc-request", "timeout は時間")
	// A step whose gRPC trace is a number reports grpc-request "trace is a bool".
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message:\n          name: a\n        trace: 1\n", "grpc-request", "trace は bool")
	// A gRPC step whose method starts with /, and that writes receive, close, a template, and a timeout of 1s, does not report grpc-request.
	rejectMessage(t, grpc+"      /pkg.Svc/M:\n        messages:\n          - receive\n          - close\n          - \"{{ vars.op }}\"\n        timeout: 1s\n", "grpc-request", "gRPC")

	const db = "desc: x\nrunners:\n  db: mysql://root:pass@localhost:3306/app\nsteps:\n  - db:\n"
	// A DB step whose query is an empty string reports db-request "query is a non-empty string".
	requireMessage(t, db+"      query: \"\"\n", "db-request", "query は空でない文字列")
	// A DB step that has rows besides query reports db-request "DB request has no key rows".
	requireMessage(t, db+"      query: SELECT 1\n      rows: 1\n", "db-request", "キー rows はありません")
	// A step whose DB trace is a number reports db-request "trace is a bool".
	requireMessage(t, db+"      query: SELECT 1\n      trace: 1\n", "db-request", "trace は bool")
	// A DB step that writes both query and trace: false does not report db-request for a missing query.
	rejectMessage(t, db+"      query: SELECT 1\n      trace: false\n", "db-request", "query")
}

// Linting the following exec, SSH, and agent inputs reports exec-request, ssh-request, agent-request, or runner-config with the expected text, or does not report it.
func TestExecSSHAndAgentBranches(t *testing.T) {
	const execHead = "desc: x\nsteps:\n  - exec:\n"
	// An exec whose shell is zsh reports exec-request "shell is bash, sh, or a command that contains {0}".
	requireMessage(t, execHead+"      command: echo\n      shell: zsh\n", "exec-request", "bash、sh")
	// An exec whose stdin is a number reports exec-request "stdin is a string".
	requireMessage(t, execHead+"      command: echo\n      stdin: 1\n", "exec-request", "stdin は文字列")
	// An exec whose background is a number reports exec-request "background is a bool".
	requireMessage(t, execHead+"      command: echo\n      background: 1\n", "exec-request", "background は bool")
	// An exec whose liveOutput is a number reports exec-request "liveOutput is a bool".
	requireMessage(t, execHead+"      command: echo\n      liveOutput: 1\n", "exec-request", "liveOutput は bool")
	// An exec whose env is a string reports exec-request "env is a mapping".
	requireMessage(t, execHead+"      command: echo\n      env: PATH\n", "exec-request", "env は mapping")
	// An exec whose env.A is a number reports exec-request "env.A is a string".
	requireMessage(t, execHead+"      command: echo\n      env:\n        A: 1\n", "exec-request", "env.A は文字列")
	// An exec whose shell is run {0} and whose background is a template does not report exec-request for shell.
	rejectMessage(t, execHead+"      command: echo\n      shell: \"run {0}\"\n      background: \"{{ vars.bg }}\"\n", "exec-request", "shell")

	// An SSH step whose command is only whitespace reports ssh-request "command is a non-empty string".
	requireMessage(t, "desc: x\nrunners:\n  sc: ssh://user@example.com\nsteps:\n  - sc:\n      command: \"  \"\n", "ssh-request", "command は空でない文字列")

	// An agent step with no prompt reports agent-request "prompt is a string".
	requireMessage(t, "desc: x\nrunners:\n  ai:\n    agent: claude\n    model: opus\nsteps:\n  - ai: {}\n", "agent-request", "prompt は文字列")
	// An agent step whose clearContext is a number reports agent-request "clearContext is a bool".
	requireMessage(t, "desc: x\nrunners:\n  ai:\n    agent: claude\n    model: opus\nsteps:\n  - ai:\n      prompt: hello\n      clearContext: 1\n", "agent-request", "clearContext は bool")
	// A runner whose agent is gemini reports runner-config "agent is claude, copilot, or codex".
	requireMessage(t, "desc: x\nrunners:\n  ai:\n    agent: gemini\n    model: x\nsteps:\n  - test: \"true\"\n", "runner-config", "claude、copilot、codex")
}

// Linting each of the following CDP steps reports cdp-request with the expected text, or does not report it.
func TestCDPBranches(t *testing.T) {
	const head = "desc: x\nrunners:\n  br: cdp://127.0.0.1:9222\nsteps:\n  - br:\n      actions:\n"
	// Writing click as a string with no arguments reports cdp-request "CDP action click needs arguments".
	requireMessage(t, head+"        - click\n", "cdp-request", "引数が必要")
	// Passing the string extra to title reports cdp-request "CDP action title takes no arguments".
	requireMessage(t, head+"        - title: extra\n", "cdp-request", "引数を取りません")
	// Passing the string hello to sendKeys reports cdp-request "CDP action sendKeys takes sel, value as a mapping".
	requireMessage(t, head+"        - sendKeys: hello\n", "cdp-request", "mapping で指定")
	// A click mapping that has extra reports cdp-request "CDP action click has no key extra".
	requireMessage(t, head+"        - click:\n            sel: a\n            extra: b\n", "cdp-request", "キー extra はありません")
	// Passing a mapping to title reports cdp-request "CDP action title takes no arguments".
	requireMessage(t, head+"        - title:\n            nope: 1\n", "cdp-request", "引数を取りません")
	// A numeric click value reports cdp-request "arguments of CDP action click are a string or a mapping".
	requireMessage(t, head+"        - click: 1\n", "cdp-request", "文字列または mapping")
	// A numeric actions element reports cdp-request "a CDP action is a string or a mapping".
	requireMessage(t, head+"        - 1\n", "cdp-request", "文字列または mapping")
	// An action mapping that has both click and wait reports cdp-request "a CDP action is a mapping with exactly one key".
	requireMessage(t, head+"        - click:\n            sel: a\n          wait: 1s\n", "cdp-request", "キーを1つだけ")
	// A CDP step with no actions reports cdp-request "actions is a non-empty array".
	requireMessage(t, "desc: x\nrunners:\n  br: cdp://127.0.0.1:9222\nsteps:\n  - br: {}\n", "cdp-request", "空でない配列")
	// A step that writes title, click's sel, and sendKeys' sel and value does not report cdp-request.
	rejectMessage(t, head+"        - title\n        - click:\n            sel: a\n        - sendKeys:\n            sel: input\n            value: hi\n", "cdp-request", "CDP")
}

// Linting the following includes and steps reports include-request or step-shape with the expected text.
func TestIncludeAndStepShapeBranches(t *testing.T) {
	// An include that is a whitespace-only string reports include-request "include path is empty".
	requireMessage(t, "desc: x\nsteps:\n  - include: \"  \"\n", "include-request", "path が空です")
	// An include that is an empty mapping reports include-request "include needs path or steps".
	requireMessage(t, "desc: x\nsteps:\n  - include: {}\n", "include-request", "path または steps が必要")
	// An empty include.path reports include-request "include.path is a non-empty string".
	requireMessage(t, "desc: x\nsteps:\n  - include:\n      path: \"\"\n", "include-request", "空でない文字列")
	// An include.vars that is an array reports include-request "include.vars is a mapping".
	requireMessage(t, "desc: x\nsteps:\n  - include:\n      path: child.yml\n      vars: []\n", "include-request", "include.vars は mapping")
	// A numeric skipTest on an inline include reports include-request "include.skipTest is a bool".
	requireMessage(t, "desc: x\nsteps:\n  - include:\n      steps:\n        - test: \"true\"\n      skipTest: 1\n", "include-request", "include.skipTest は bool")
	// A numeric include reports include-request "include is a path or a mapping".
	requireMessage(t, "desc: x\nsteps:\n  - include: 1\n", "include-request", "path または mapping")
	// A step with only desc reports step-shape "step has no runner to run".
	requireMessage(t, "desc: x\nsteps:\n  - desc: only\n", "step-shape", "実行する runner がありません")
	// A step whose runner is an empty mapping reports step-shape "runner: is a mapping from runner name to definition".
	requireMessage(t, "desc: x\nsteps:\n  - runner: {}\n", "step-shape", "mapping です")
	// Writing runner and test on the same step reports step-shape "runner: cannot share a step with another runner".
	requireMessage(t, "desc: x\nsteps:\n  - runner:\n      sc: ssh://user@example.com\n    test: \"true\"\n", "step-shape", "同じ step に書けません")
	// dump with out and no expr reports step-shape "dump.expr is missing".
	requireMessage(t, "desc: x\nsteps:\n  - dump:\n      out: out.txt\n    test: \"true\"\n", "step-shape", "dump.expr がありません")
	// An empty bind mapping reports step-shape "bind is empty".
	requireMessage(t, "desc: x\nsteps:\n  - bind: {}\n    test: \"true\"\n", "step-shape", "bind が空です")
}

// Linting the following runner definitions reports runner-config with the expected text, or does not report it.
func TestRunnerConfigBranches(t *testing.T) {
	// An SSH runner whose port is -1 reports runner-config "port is a positive number".
	requireMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    port: -1\nsteps:\n  - test: \"true\"\n", "runner-config", "port は正の数")
	// An SSH runner whose port is ssh reports runner-config "port is a number".
	requireMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    port: ssh\nsteps:\n  - test: \"true\"\n", "runner-config", "port は数値")
	// keyboardInteractive with match and no answer reports runner-config "keyboardInteractive.answer is a string".
	requireMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    keyboardInteractive:\n      - match: Password\nsteps:\n  - test: \"true\"\n", "runner-config", "keyboardInteractive.answer")
	// Writing both identityFile and identityKey reports runner-config "runner sc cannot set both identityFile and identityKey".
	requireMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    identityFile: id\n    identityKey: secret\nsteps:\n  - test: \"true\"\n", "runner-config", "同時に指定できません")
	// An HTTP runner whose trace.enable is a number and that also has extra reports runner-config "trace.enable is a bool or a string".
	requireMessage(t, "desc: x\nrunners:\n  req:\n    endpoint: https://example.com\n    trace:\n      enable: 1\n      extra: a\nsteps:\n  - test: \"true\"\n", "runner-config", "trace.enable")
	// An HTTP runner whose trace is a number reports runner-config "trace is a bool, a string, or a mapping".
	requireMessage(t, "desc: x\nrunners:\n  req:\n    endpoint: https://example.com\n    trace: 1\nsteps:\n  - test: \"true\"\n", "runner-config", "trace は bool、文字列、または mapping")
	// A gRPC runner whose protos is one string reports runner-config "protos is an array of strings".
	requireMessage(t, "desc: x\nrunners:\n  greq:\n    addr: localhost:50051\n    protos: one.proto\nsteps:\n  - test: \"true\"\n", "runner-config", "protos は文字列の配列")
	// A runner whose value is a number reports runner-config "runner mystery is a string or a mapping".
	requireMessage(t, "desc: x\nrunners:\n  mystery: 1\nsteps:\n  - test: \"true\"\n", "runner-config", "文字列または mapping")
	// An SSH runner whose port is a template and whose match and answer are both set does not report runner-config for port.
	rejectMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    port: \"{{ vars.port }}\"\n    keyboardInteractive:\n      - match: Password\n        answer: secret\nsteps:\n  - test: \"true\"\n", "runner-config", "port")
}

// Linting each of the following runbooks reports book-shape or step-shape with the expected text.
func TestBookShapeBranches(t *testing.T) {
	const tail = "\nsteps:\n  - test: \"true\"\n"
	// A string labels reports book-shape "labels is an array of strings".
	requireMessage(t, "desc: x\nlabels: users"+tail, "book-shape", "labels は文字列の配列")
	// A numeric labels element reports book-shape "an element of labels is a string".
	requireMessage(t, "desc: x\nlabels:\n  - 1"+tail, "book-shape", "labels の要素は文字列")
	// A numeric needs.auth reports book-shape "needs.auth is a string".
	requireMessage(t, "desc: x\nneeds:\n  auth: 1"+tail, "book-shape", "needs.auth は文字列")
	// A string secrets reports book-shape "secrets is an array of strings".
	requireMessage(t, "desc: x\nsecrets: token"+tail, "book-shape", "secrets は文字列の配列")
	// An empty secrets element reports book-shape "an element of secrets is a non-empty string".
	requireMessage(t, "desc: x\nsecrets:\n  - \"\""+tail, "book-shape", "空でない文字列")
	// An array vars reports book-shape "vars is a mapping".
	requireMessage(t, "desc: x\nvars: []"+tail, "book-shape", "vars は mapping")
	// A numeric debug reports book-shape "debug is a bool or a string".
	requireMessage(t, "desc: x\ndebug: 1"+tail, "book-shape", "debug は bool または文字列")
	// interval: later reports book-shape "interval is a duration".
	requireMessage(t, "desc: x\ninterval: later"+tail, "book-shape", "interval は時間")
	// A numeric concurrency reports book-shape "concurrency is a string or an array of strings".
	requireMessage(t, "desc: x\nconcurrency: 1"+tail, "book-shape", "文字列または文字列の配列")
	// An empty-string concurrency reports book-shape "concurrency is empty".
	requireMessage(t, "desc: x\nconcurrency: \"\""+tail, "book-shape", "concurrency が空です")
	// An empty-array concurrency reports book-shape "concurrency is empty".
	requireMessage(t, "desc: x\nconcurrency: []"+tail, "book-shape", "concurrency が空です")
	// loop: -1 reports book-shape "loop count is zero or greater".
	requireMessage(t, "desc: x\nloop: -1"+tail, "book-shape", "0 以上")
	// loop with until and no count reports book-shape "loop.count is missing".
	requireMessage(t, "desc: x\nloop:\n  until: 1"+tail, "book-shape", "loop.count がありません")
	// A bool loop.count reports book-shape "loop.count is a number or an expression".
	requireMessage(t, "desc: x\nloop:\n  count: true"+tail, "book-shape", "数値または式")
	// loop.interval: later reports book-shape "loop.interval is a duration".
	requireMessage(t, "desc: x\nloop:\n  count: 1\n  interval: later"+tail, "book-shape", "loop.interval は時間")
	// An array loop on a step reports step-shape "loop of step is a number, an expression, or a mapping".
	requireMessage(t, "desc: x\nsteps:\n  - loop: []\n    test: \"true\"\n", "step-shape", "step の loop は")
}

// Linting each of the following expressions reports step-ref, loop-index, previous-step, or var-ref with the expected text, or does not report it.
func TestReferenceBranches(t *testing.T) {
	// Referring to steps[3] from a runbook with one step reports step-ref "steps[3] is not in this runbook".
	requireMessage(t, "desc: x\nsteps:\n  - test: steps[3].res.status == 200\n", "step-ref", "ありません")
	// Referring to steps[0] from a top-level if reports step-ref "refer to steps while a step is running".
	requireMessage(t, "desc: x\nif: steps[0].res.status == 200\nsteps:\n  - test: \"true\"\n", "step-ref", "実行中に参照")
	// Referring to an earlier defer from a later defer reports step-ref "steps[0] has not run yet".
	requireMessage(t, `
desc: x
steps:
  - defer: true
    test: steps[1].res.status == 200
  - defer: true
    test: steps[0].res.status == 200
`, "step-ref", "まだ実行されていません")
	// Referring to a preceding ordinary step from a defer does not report step-ref "steps[0] has not run yet".
	rejectMessage(t, `
desc: x
steps:
  - test: "true"
  - defer: true
    test: steps[0].res.status == 200
`, "step-ref", "まだ実行されていません")
	// Referring to steps[0] from a top-level loop.until does not report step-ref.
	rejectMessage(t, `
desc: x
loop:
  count: 1
  until: steps[0].res.status == 200
steps:
  - test: "true"
`, "step-ref", "steps")
	// Referring to i on a step that has loop does not report loop-index.
	rejectMessage(t, `
desc: x
steps:
  - loop:
      count: 2
    test: i == 0
`, "loop-index", "添字")
	// Referring to previous on the second step does not report previous-step.
	rejectMessage(t, `
desc: x
steps:
  - test: "true"
  - test: previous.res.status == 200
`, "previous-step", "previous")

	// Referring to parent.vars.token from inside an include whose parent has vars.token does not report var-ref.
	rejectMessage(t, `
desc: parent
vars:
  token: a
steps:
  - include:
      steps:
        - test: parent.vars.token == "a"
`, "var-ref", "parent.vars")
	// Referring to parent.vars.token from inside an include whose parent has no token reports var-ref "parent.vars.token is not in the parent vars".
	requireMessage(t, `
desc: parent
steps:
  - include:
      steps:
        - test: parent.vars.token == "a"
`, "var-ref", "親の vars にありません")
}

// validGRPCMethod is false for Hello, /Only/, and pkg.Svc/. It is true for pkg.Svc/Method and for a method with a leading /.
func TestValidGRPCMethod(t *testing.T) {
	if validGRPCMethod("Hello") || validGRPCMethod("/Only/") || validGRPCMethod("pkg.Svc/") {
		t.Fatal("不正なメソッドを受けています")
	}
	if !validGRPCMethod("pkg.Svc/Method") || !validGRPCMethod("/pkg.Svc/Method") {
		t.Fatal("正当なメソッドを拒否しています")
	}
}

// stepVisible is false when an ordinary step looks at a later defer, and true when a defer looks at a preceding ordinary step. Between defers, a later defer is visible from an earlier one, an earlier defer is not visible from a later one, and a step is visible from itself.
func TestStepVisible(t *testing.T) {
	normal := &runbook.Book{Steps: []runbook.Step{{Index: 0}, {Index: 1, Defer: true}}}
	if stepVisible(normal, &normal.Steps[0], 1) {
		t.Fatal("通常 step から後続の defer を参照できてしまいます")
	}
	if !stepVisible(normal, &normal.Steps[1], 0) {
		t.Fatal("defer から先行する通常 step を参照できません")
	}
	both := &runbook.Book{Steps: []runbook.Step{{Index: 0, Defer: true}, {Index: 1, Defer: true}}}
	if !stepVisible(both, &both.Steps[0], 1) {
		t.Fatal("先に実行される後ろの defer を参照できません")
	}
	if stepVisible(both, &both.Steps[1], 0) {
		t.Fatal("まだ実行されていない前の defer を参照できてしまいます")
	}
	if !stepVisible(both, &both.Steps[0], 0) {
		t.Fatal("自分自身を参照できません")
	}
}

// Passing token, a number, and other in ignore yields a set of only token and other. A nil or empty options value yields an empty set.
func TestOptionSetSkipsNonStrings(t *testing.T) {
	got := optionSet(map[string]any{"ignore": []any{"token", 1, "other"}}, "ignore")
	if _, ok := got["token"]; !ok {
		t.Fatal("token")
	}
	if _, ok := got["other"]; !ok {
		t.Fatal("other")
	}
	if len(got) != 2 {
		t.Fatalf("%#v", got)
	}
	if len(optionSet(nil, "ignore")) != 0 || len(optionSet(map[string]any{}, "ignore")) != 0 {
		t.Fatal("空の options")
	}
}
