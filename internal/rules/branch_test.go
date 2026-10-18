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

// 次の各 HTTP step を lint したとき、http-request が期待する文言で出る、または出ないことを確認する。
func TestHTTPRequestBranches(t *testing.T) {
	const head = "desc: x\nrunners:\n  req: https://example.com\nsteps:\n  - req:\n"
	// path を /a と /b の2つ書いた step では、http-request が「path を1つだけ」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        get:\n          body: null\n      /b:\n        get:\n          body: null\n", "http-request", "path を1つだけ")
	// path が空文字の step では、http-request が「path が空です」と出ることを確認する。
	requireMessage(t, head+"      ? \"\"\n      : \n        get:\n          body: null\n", "http-request", "path が空です")
	// get と post を同時に書いた step では、http-request が「メソッドは1つ」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        get:\n          body: null\n        post:\n          body:\n            text/plain: hi\n", "http-request", "メソッドは1つ")
	// メソッドが fetch の step では、http-request が「メソッド fetch は使えません」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        fetch:\n          body: null\n", "http-request", "メソッド fetch は使えません")
	// headers が配列の step では、http-request が「headers は mapping」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        get:\n          headers: []\n          body: null\n", "http-request", "headers は mapping")
	// Accept の配列要素が数値の step では、http-request が「要素は文字列」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        get:\n          headers:\n            Accept:\n              - 1\n          body: null\n", "http-request", "要素は文字列")
	// Accept の値が数値の step では、http-request が「文字列または文字列の配列」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        get:\n          headers:\n            Accept: 1\n          body: null\n", "http-request", "文字列または文字列の配列")
	// useCookie が数値の step では、http-request が「useCookie は bool」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        get:\n          useCookie: 1\n          body: null\n", "http-request", "useCookie は bool")
	// リクエストの trace が数値の step では、http-request が「trace は bool」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        get:\n          trace: 1\n          body: null\n", "http-request", "trace は bool")
	// text/plain の body が数値の POST では、http-request が「text/plain の body は文字列」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            text/plain: 1\n", "http-request", "text/plain の body は文字列")
	// form-urlencoded の body が文字列の POST では、http-request が「form-urlencoded の body は mapping」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            application/x-www-form-urlencoded: a=b\n", "http-request", "form-urlencoded の body は mapping")
	// multipart の body が文字列の POST では、http-request が「multipart/form-data の body は mapping」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            multipart/form-data: file\n", "http-request", "multipart/form-data の body は mapping")
	// json と text/plain を同時に書いた POST では、http-request が「media type は1つ」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            application/json:\n              a: 1\n            text/plain: hi\n", "http-request", "media type は1つ")
	// octet-stream の mapping に filename が無い POST では、http-request が「filename が必要」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        post:\n          body:\n            application/octet-stream:\n              name: file\n", "http-request", "filename が必要")
	// post の直下に extra がある step では、http-request が「キー extra はありません」と出ることを確認する。
	requireMessage(t, head+"      /a:\n        post:\n          extra: 1\n          body:\n            text/plain: hi\n", "http-request", "キー extra はありません")

	// GET の body が null の step では、http-request が body 不足とは診断しないことを確認する。
	rejectMessage(t, head+"      /a:\n        get:\n          body: null\n", "http-request", "body")
	// path が https:// で始まる step では、http-request が path の始まりを警告しないことを確認する。
	rejectMessage(t, head+"      https://example.com/a:\n        get:\n          body: null\n", "http-request", "から始めます")
	// json の mapping body を持つ POST では、http-request が filename 不足とは診断しないことを確認する。
	rejectMessage(t, head+"      /a:\n        post:\n          body:\n            application/json:\n              a: 1\n", "http-request", "filename")
	// octet-stream に filename がある POST では、http-request が filename 不足とは診断しないことを確認する。
	rejectMessage(t, head+"      /a:\n        post:\n          body:\n            application/octet-stream:\n              filename: a.bin\n", "http-request", "filename")
	// リクエスト全体が {{ vars.req }} の step では、http-request が出ないことを確認する。
	rejectMessage(t, "desc: x\nrunners:\n  req: https://example.com\nsteps:\n  - req: \"{{ vars.req }}\"\n", "http-request", "HTTP")
}

// 次の各 gRPC step と DB step を lint したとき、grpc-request または db-request が期待する文言で出る、または出ないことを確認する。
func TestGRPCAndDBBranches(t *testing.T) {
	const grpc = "desc: x\nrunners:\n  greq: grpc://localhost:8080\nsteps:\n  - greq:\n"
	// メソッドを2つ書いた gRPC step では、grpc-request が「1つだけ」と出ることを確認する。
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message:\n          name: a\n      other.Svc/M:\n        message:\n          name: b\n", "grpc-request", "1つだけ")
	// message が配列の gRPC step では、grpc-request が「message は mapping」と出ることを確認する。
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message: []\n", "grpc-request", "message は mapping")
	// message と messages を両方書いた gRPC step では、grpc-request が「同時に指定できません」と出ることを確認する。
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message:\n          name: a\n        messages:\n          - receive\n", "grpc-request", "同時に指定できません")
	// messages が文字列の gRPC step では、grpc-request が「messages は配列」と出ることを確認する。
	requireMessage(t, grpc+"      pkg.Svc/M:\n        messages: receive\n", "grpc-request", "messages は配列")
	// messages の要素が wait の gRPC step では、grpc-request が「receive または close」と出ることを確認する。
	requireMessage(t, grpc+"      pkg.Svc/M:\n        messages:\n          - wait\n", "grpc-request", "receive または close")
	// messages の要素が数値の gRPC step では、grpc-request が「mapping、receive、close」と出ることを確認する。
	requireMessage(t, grpc+"      pkg.Svc/M:\n        messages:\n          - 1\n", "grpc-request", "mapping、receive、close")
	// timeout が later の gRPC step では、grpc-request が「timeout は時間」と出ることを確認する。
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message:\n          name: a\n        timeout: later\n", "grpc-request", "timeout は時間")
	// gRPC の trace が数値の step では、grpc-request が「trace は bool」と出ることを確認する。
	requireMessage(t, grpc+"      pkg.Svc/M:\n        message:\n          name: a\n        trace: 1\n", "grpc-request", "trace は bool")
	// 先頭 / 付きのメソッドに receive、close、テンプレート、1s の timeout を書いた gRPC step では、grpc-request が出ないことを確認する。
	rejectMessage(t, grpc+"      /pkg.Svc/M:\n        messages:\n          - receive\n          - close\n          - \"{{ vars.op }}\"\n        timeout: 1s\n", "grpc-request", "gRPC")

	const db = "desc: x\nrunners:\n  db: mysql://root:pass@localhost:3306/app\nsteps:\n  - db:\n"
	// query が空文字の DB step では、db-request が「query は空でない文字列」と出ることを確認する。
	requireMessage(t, db+"      query: \"\"\n", "db-request", "query は空でない文字列")
	// query のほかに rows がある DB step では、db-request が「キー rows はありません」と出ることを確認する。
	requireMessage(t, db+"      query: SELECT 1\n      rows: 1\n", "db-request", "キー rows はありません")
	// DB の trace が数値の step では、db-request が「trace は bool」と出ることを確認する。
	requireMessage(t, db+"      query: SELECT 1\n      trace: 1\n", "db-request", "trace は bool")
	// query と trace: false を両方書いた DB step では、db-request が query 不足とは診断しないことを確認する。
	rejectMessage(t, db+"      query: SELECT 1\n      trace: false\n", "db-request", "query")
}

// 次の exec、SSH、agent の入力を lint したとき、exec-request、ssh-request、agent-request、runner-config が期待する文言で出る、または出ないことを確認する。
func TestExecSSHAndAgentBranches(t *testing.T) {
	const execHead = "desc: x\nsteps:\n  - exec:\n"
	// shell が zsh の exec では、exec-request が「bash、sh」と出ることを確認する。
	requireMessage(t, execHead+"      command: echo\n      shell: zsh\n", "exec-request", "bash、sh")
	// stdin が数値の exec では、exec-request が「stdin は文字列」と出ることを確認する。
	requireMessage(t, execHead+"      command: echo\n      stdin: 1\n", "exec-request", "stdin は文字列")
	// background が数値の exec では、exec-request が「background は bool」と出ることを確認する。
	requireMessage(t, execHead+"      command: echo\n      background: 1\n", "exec-request", "background は bool")
	// liveOutput が数値の exec では、exec-request が「liveOutput は bool」と出ることを確認する。
	requireMessage(t, execHead+"      command: echo\n      liveOutput: 1\n", "exec-request", "liveOutput は bool")
	// env が文字列の exec では、exec-request が「env は mapping」と出ることを確認する。
	requireMessage(t, execHead+"      command: echo\n      env: PATH\n", "exec-request", "env は mapping")
	// env.A が数値の exec では、exec-request が「env.A は文字列」と出ることを確認する。
	requireMessage(t, execHead+"      command: echo\n      env:\n        A: 1\n", "exec-request", "env.A は文字列")
	// shell が run {0} で background がテンプレートの exec では、exec-request が shell を診断しないことを確認する。
	rejectMessage(t, execHead+"      command: echo\n      shell: \"run {0}\"\n      background: \"{{ vars.bg }}\"\n", "exec-request", "shell")

	// command が空白だけの SSH step では、ssh-request が「command は空でない文字列」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  sc: ssh://user@example.com\nsteps:\n  - sc:\n      command: \"  \"\n", "ssh-request", "command は空でない文字列")

	// prompt が無い agent step では、agent-request が「prompt は文字列」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  ai:\n    agent: claude\n    model: opus\nsteps:\n  - ai: {}\n", "agent-request", "prompt は文字列")
	// clearContext が数値の agent step では、agent-request が「clearContext は bool」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  ai:\n    agent: claude\n    model: opus\nsteps:\n  - ai:\n      prompt: hello\n      clearContext: 1\n", "agent-request", "clearContext は bool")
	// agent が gemini の runner では、runner-config が「claude、copilot、codex」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  ai:\n    agent: gemini\n    model: x\nsteps:\n  - test: \"true\"\n", "runner-config", "claude、copilot、codex")
}

// 次の各 CDP step を lint したとき、cdp-request が期待する文言で出る、または出ないことを確認する。
func TestCDPBranches(t *testing.T) {
	const head = "desc: x\nrunners:\n  br: cdp://127.0.0.1:9222\nsteps:\n  - br:\n      actions:\n"
	// click を引数なしの文字列で書いたとき、cdp-request が「引数が必要」と出ることを確認する。
	requireMessage(t, head+"        - click\n", "cdp-request", "引数が必要")
	// title に文字列 extra を渡したとき、cdp-request が「引数を取りません」と出ることを確認する。
	requireMessage(t, head+"        - title: extra\n", "cdp-request", "引数を取りません")
	// sendKeys に文字列 hello を渡したとき、cdp-request が「mapping で指定」と出ることを確認する。
	requireMessage(t, head+"        - sendKeys: hello\n", "cdp-request", "mapping で指定")
	// click の mapping に extra があるとき、cdp-request が「キー extra はありません」と出ることを確認する。
	requireMessage(t, head+"        - click:\n            sel: a\n            extra: b\n", "cdp-request", "キー extra はありません")
	// title に mapping を渡したとき、cdp-request が「引数を取りません」と出ることを確認する。
	requireMessage(t, head+"        - title:\n            nope: 1\n", "cdp-request", "引数を取りません")
	// click の値が数値のとき、cdp-request が「文字列または mapping」と出ることを確認する。
	requireMessage(t, head+"        - click: 1\n", "cdp-request", "文字列または mapping")
	// actions の要素が数値のとき、cdp-request が「文字列または mapping」と出ることを確認する。
	requireMessage(t, head+"        - 1\n", "cdp-request", "文字列または mapping")
	// 1つのアクション mapping に click と wait があるとき、cdp-request が「キーを1つだけ」と出ることを確認する。
	requireMessage(t, head+"        - click:\n            sel: a\n          wait: 1s\n", "cdp-request", "キーを1つだけ")
	// actions が無い CDP step では、cdp-request が「空でない配列」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  br: cdp://127.0.0.1:9222\nsteps:\n  - br: {}\n", "cdp-request", "空でない配列")
	// title、click の sel、sendKeys の sel と value を書いた step では、cdp-request が出ないことを確認する。
	rejectMessage(t, head+"        - title\n        - click:\n            sel: a\n        - sendKeys:\n            sel: input\n            value: hi\n", "cdp-request", "CDP")
}

// 次の include と step を lint したとき、include-request または step-shape が期待する文言で出ることを確認する。
func TestIncludeAndStepShapeBranches(t *testing.T) {
	// include が空白だけの文字列のとき、include-request が「path が空です」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - include: \"  \"\n", "include-request", "path が空です")
	// include が空の mapping のとき、include-request が「path または steps が必要」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - include: {}\n", "include-request", "path または steps が必要")
	// include.path が空文字のとき、include-request が「空でない文字列」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - include:\n      path: \"\"\n", "include-request", "空でない文字列")
	// include.vars が配列のとき、include-request が「include.vars は mapping」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - include:\n      path: child.yml\n      vars: []\n", "include-request", "include.vars は mapping")
	// インライン include の skipTest が数値のとき、include-request が「include.skipTest は bool」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - include:\n      steps:\n        - test: \"true\"\n      skipTest: 1\n", "include-request", "include.skipTest は bool")
	// include が数値のとき、include-request が「path または mapping」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - include: 1\n", "include-request", "path または mapping")
	// desc だけの step では、step-shape が「実行する runner がありません」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - desc: only\n", "step-shape", "実行する runner がありません")
	// runner が空の mapping の step では、step-shape が「mapping です」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - runner: {}\n", "step-shape", "mapping です")
	// runner と test を同じ step に書いたとき、step-shape が「同じ step に書けません」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - runner:\n      sc: ssh://user@example.com\n    test: \"true\"\n", "step-shape", "同じ step に書けません")
	// dump に out だけで expr が無いとき、step-shape が「dump.expr がありません」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - dump:\n      out: out.txt\n    test: \"true\"\n", "step-shape", "dump.expr がありません")
	// bind が空の mapping のとき、step-shape が「bind が空です」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - bind: {}\n    test: \"true\"\n", "step-shape", "bind が空です")
}

// 次の runner 定義を lint したとき、runner-config が期待する文言で出る、または出ないことを確認する。
func TestRunnerConfigBranches(t *testing.T) {
	// port が -1 の SSH runner では、runner-config が「port は正の数」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    port: -1\nsteps:\n  - test: \"true\"\n", "runner-config", "port は正の数")
	// port が ssh の SSH runner では、runner-config が「port は数値」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    port: ssh\nsteps:\n  - test: \"true\"\n", "runner-config", "port は数値")
	// keyboardInteractive に match だけで answer が無いとき、runner-config が「keyboardInteractive.answer」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    keyboardInteractive:\n      - match: Password\nsteps:\n  - test: \"true\"\n", "runner-config", "keyboardInteractive.answer")
	// identityFile と identityKey を両方書いたとき、runner-config が「同時に指定できません」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    identityFile: id\n    identityKey: secret\nsteps:\n  - test: \"true\"\n", "runner-config", "同時に指定できません")
	// trace.enable が数値で extra もある HTTP runner では、runner-config が「trace.enable」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  req:\n    endpoint: https://example.com\n    trace:\n      enable: 1\n      extra: a\nsteps:\n  - test: \"true\"\n", "runner-config", "trace.enable")
	// HTTP runner の trace が数値のとき、runner-config が「trace は bool、文字列、または mapping」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  req:\n    endpoint: https://example.com\n    trace: 1\nsteps:\n  - test: \"true\"\n", "runner-config", "trace は bool、文字列、または mapping")
	// protos が文字列1つの gRPC runner では、runner-config が「protos は文字列の配列」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  greq:\n    addr: localhost:50051\n    protos: one.proto\nsteps:\n  - test: \"true\"\n", "runner-config", "protos は文字列の配列")
	// runner の値が数値のとき、runner-config が「文字列または mapping」と出ることを確認する。
	requireMessage(t, "desc: x\nrunners:\n  mystery: 1\nsteps:\n  - test: \"true\"\n", "runner-config", "文字列または mapping")
	// port がテンプレートで、match と answer が揃った SSH runner では、runner-config が port を診断しないことを確認する。
	rejectMessage(t, "desc: x\nrunners:\n  sc:\n    host: example.com\n    port: \"{{ vars.port }}\"\n    keyboardInteractive:\n      - match: Password\n        answer: secret\nsteps:\n  - test: \"true\"\n", "runner-config", "port")
}

// 次の各 runbook を lint したとき、book-shape または step-shape が期待する文言で出ることを確認する。
func TestBookShapeBranches(t *testing.T) {
	const tail = "\nsteps:\n  - test: \"true\"\n"
	// labels が文字列のとき、book-shape が「labels は文字列の配列」と出ることを確認する。
	requireMessage(t, "desc: x\nlabels: users"+tail, "book-shape", "labels は文字列の配列")
	// labels の要素が数値のとき、book-shape が「labels の要素は文字列」と出ることを確認する。
	requireMessage(t, "desc: x\nlabels:\n  - 1"+tail, "book-shape", "labels の要素は文字列")
	// needs.auth が数値のとき、book-shape が「needs.auth は文字列」と出ることを確認する。
	requireMessage(t, "desc: x\nneeds:\n  auth: 1"+tail, "book-shape", "needs.auth は文字列")
	// secrets が文字列のとき、book-shape が「secrets は文字列の配列」と出ることを確認する。
	requireMessage(t, "desc: x\nsecrets: token"+tail, "book-shape", "secrets は文字列の配列")
	// secrets の要素が空文字のとき、book-shape が「空でない文字列」と出ることを確認する。
	requireMessage(t, "desc: x\nsecrets:\n  - \"\""+tail, "book-shape", "空でない文字列")
	// vars が配列のとき、book-shape が「vars は mapping」と出ることを確認する。
	requireMessage(t, "desc: x\nvars: []"+tail, "book-shape", "vars は mapping")
	// debug が数値のとき、book-shape が「debug は bool または文字列」と出ることを確認する。
	requireMessage(t, "desc: x\ndebug: 1"+tail, "book-shape", "debug は bool または文字列")
	// interval が later のとき、book-shape が「interval は時間」と出ることを確認する。
	requireMessage(t, "desc: x\ninterval: later"+tail, "book-shape", "interval は時間")
	// concurrency が数値のとき、book-shape が「文字列または文字列の配列」と出ることを確認する。
	requireMessage(t, "desc: x\nconcurrency: 1"+tail, "book-shape", "文字列または文字列の配列")
	// concurrency が空文字のとき、book-shape が「concurrency が空です」と出ることを確認する。
	requireMessage(t, "desc: x\nconcurrency: \"\""+tail, "book-shape", "concurrency が空です")
	// concurrency が空配列のとき、book-shape が「concurrency が空です」と出ることを確認する。
	requireMessage(t, "desc: x\nconcurrency: []"+tail, "book-shape", "concurrency が空です")
	// loop が -1 のとき、book-shape が「0 以上」と出ることを確認する。
	requireMessage(t, "desc: x\nloop: -1"+tail, "book-shape", "0 以上")
	// loop に until だけで count が無いとき、book-shape が「loop.count がありません」と出ることを確認する。
	requireMessage(t, "desc: x\nloop:\n  until: 1"+tail, "book-shape", "loop.count がありません")
	// loop.count が bool のとき、book-shape が「数値または式」と出ることを確認する。
	requireMessage(t, "desc: x\nloop:\n  count: true"+tail, "book-shape", "数値または式")
	// loop.interval が later のとき、book-shape が「loop.interval は時間」と出ることを確認する。
	requireMessage(t, "desc: x\nloop:\n  count: 1\n  interval: later"+tail, "book-shape", "loop.interval は時間")
	// step の loop が配列のとき、step-shape が「step の loop は」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - loop: []\n    test: \"true\"\n", "step-shape", "step の loop は")
}

// 次の各式を lint したとき、step-ref、loop-index、previous-step、var-ref が期待する文言で出る、または出ないことを確認する。
func TestReferenceBranches(t *testing.T) {
	// steps が1件の runbook から steps[3] を参照したとき、step-ref が「ありません」と出ることを確認する。
	requireMessage(t, "desc: x\nsteps:\n  - test: steps[3].res.status == 200\n", "step-ref", "ありません")
	// runbook 直下の if から steps[0] を参照したとき、step-ref が「実行中に参照」と出ることを確認する。
	requireMessage(t, "desc: x\nif: steps[0].res.status == 200\nsteps:\n  - test: \"true\"\n", "step-ref", "実行中に参照")
	// 後ろの defer から前の defer を参照したとき、step-ref が「まだ実行されていません」と出ることを確認する。
	requireMessage(t, `
desc: x
steps:
  - defer: true
    test: steps[1].res.status == 200
  - defer: true
    test: steps[0].res.status == 200
`, "step-ref", "まだ実行されていません")
	// defer から先行する通常 step を参照したとき、step-ref が「まだ実行されていません」と出ないことを確認する。
	rejectMessage(t, `
desc: x
steps:
  - test: "true"
  - defer: true
    test: steps[0].res.status == 200
`, "step-ref", "まだ実行されていません")
	// runbook 直下の loop.until から steps[0] を参照したとき、step-ref が出ないことを確認する。
	rejectMessage(t, `
desc: x
loop:
  count: 1
  until: steps[0].res.status == 200
steps:
  - test: "true"
`, "step-ref", "steps")
	// loop のある step で i を参照したとき、loop-index が出ないことを確認する。
	rejectMessage(t, `
desc: x
steps:
  - loop:
      count: 2
    test: i == 0
`, "loop-index", "添字")
	// 2件目の step で previous を参照したとき、previous-step が出ないことを確認する。
	rejectMessage(t, `
desc: x
steps:
  - test: "true"
  - test: previous.res.status == 200
`, "previous-step", "previous")

	// 親の vars.token がある include 内から parent.vars.token を参照したとき、var-ref が出ないことを確認する。
	rejectMessage(t, `
desc: parent
vars:
  token: a
steps:
  - include:
      steps:
        - test: parent.vars.token == "a"
`, "var-ref", "parent.vars")
	// 親に token が無い include 内から parent.vars.token を参照したとき、var-ref が「親の vars にありません」と出ることを確認する。
	requireMessage(t, `
desc: parent
steps:
  - include:
      steps:
        - test: parent.vars.token == "a"
`, "var-ref", "親の vars にありません")
}

// Hello、/Only/、pkg.Svc/ では validGRPCMethod が false になることを確認する。pkg.Svc/Method と先頭 / 付きでは true になることを確認する。
func TestValidGRPCMethod(t *testing.T) {
	if validGRPCMethod("Hello") || validGRPCMethod("/Only/") || validGRPCMethod("pkg.Svc/") {
		t.Fatal("不正なメソッドを受けています")
	}
	if !validGRPCMethod("pkg.Svc/Method") || !validGRPCMethod("/pkg.Svc/Method") {
		t.Fatal("正当なメソッドを拒否しています")
	}
}

// 通常 step から後続の defer を見たとき stepVisible が false、defer から先行する通常 step を見たとき true になることを確認する。defer 同士では、後ろの defer は前から見え、前の defer は後ろから見えず、自分自身は見えることを確認する。
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

// ignore に token、数値、other を渡したとき、集合が token と other だけになることを確認する。options が nil または空のとき、集合が空になることを確認する。
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
