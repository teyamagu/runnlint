package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/runbook"
)

// The keys are written in reverse of the check order. Lint sorts by line and column, so this reads the findings a checker appends.
func TestCheckEmitsInRunnFieldOrder(t *testing.T) {
	t.Run("http runner", func(t *testing.T) {
		r := oneRunner(t, `
desc: d
runners:
  req:
    trace: 1
    useCookie: 1
    timeout: abc
    skipVerify: 1
    notFollowRedirect: 1
    skipCircularReferenceCheck: 1
    skipValidateResponse: 1
    skipValidateRequest: 1
    endpoint: "https://"
steps:
  - test: "true"
`)
		c := &Context{}
		checkHTTPRunner(c, r)
		assertEmitOrder(t, c.out, []string{
			"endpoint は http(s) の URL です",
			"skipValidateRequest は bool または文字列です",
			"skipValidateResponse は bool または文字列です",
			"skipCircularReferenceCheck は bool または文字列です",
			"notFollowRedirect は bool または文字列です",
			"skipVerify は bool または文字列です",
			"timeout は時間です",
			"useCookie は bool または文字列です",
			"trace は bool、文字列、または mapping です",
		})
	})

	t.Run("grpc runner", func(t *testing.T) {
		r := oneRunner(t, `
desc: d
runners:
  greq:
    trace: 1
    bufDirs: 1
    protos: 1
    skipVerify: 1
    tls: 1
    addr: localhost:50051
steps:
  - test: "true"
`)
		c := &Context{}
		checkGRPCRunner(c, r)
		assertEmitOrder(t, c.out, []string{
			"tls は bool または文字列です",
			"skipVerify は bool または文字列です",
			"protos は文字列の配列です",
			"bufDirs は文字列の配列です",
			"trace は bool、文字列、または mapping です",
		})
	})

	t.Run("ssh runner", func(t *testing.T) {
		r := oneRunner(t, `
desc: d
runners:
  sc:
    keyboardInteractive: 1
    keepSession: 1
    identityKey: k
    identityFile: f
    port: true
    host: h
steps:
  - test: "true"
`)
		c := &Context{}
		checkSSHRunner(c, r)
		assertEmitOrder(t, c.out, []string{
			"port は数値です",
			"identityFile と identityKey を同時に指定できません",
			"keepSession は bool または文字列です",
			"keyboardInteractive は配列です",
		})
	})

	t.Run("cdp runner", func(t *testing.T) {
		r := oneRunner(t, `
desc: d
runners:
  br:
    timeout: abc
    flags: 1
    addr: cdp://127.0.0.1:9222
steps:
  - test: "true"
`)
		c := &Context{}
		checkCDPRunner(c, r)
		assertEmitOrder(t, c.out, []string{
			"flags は mapping です",
			"timeout は時間です",
		})
	})

	t.Run("agent runner", func(t *testing.T) {
		r := oneRunner(t, `
desc: d
runners:
  ai:
    interactive: 1
    permissions:
      - 1
    tools:
      - 1
    agent: nope
    model: opus
steps:
  - test: "true"
`)
		c := &Context{}
		checkAgentRunner(c, r)
		assertEmitOrder(t, c.out, []string{
			"agent は claude、copilot、codex のいずれかです",
			"tools の要素は文字列です",
			"permissions の要素は文字列です",
			"interactive は bool または文字列です",
		})
	})

	t.Run("http request", func(t *testing.T) {
		v := stepValue(t, `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        post:
          trace: 1
          useCookie: 1
          body: 1
          headers: 1
`)
		c := &Context{}
		checkHTTP(c, v)
		assertEmitOrder(t, c.out, []string{
			"headers は mapping です",
			"body の media type は1つです",
			"useCookie は bool です",
			"trace は bool です",
		})
	})

	t.Run("grpc request", func(t *testing.T) {
		v := stepValue(t, `
desc: d
runners:
  greq: grpc://localhost:8080
steps:
  - greq:
      pkg.Svc/M:
        trace: 1
        messages: 1
        message: 1
        timeout: abc
        headers: 1
`)
		c := &Context{}
		checkGRPC(c, v)
		assertEmitOrder(t, c.out, []string{
			"headers は mapping です",
			"timeout は時間です",
			"message と messages は同時に指定できません",
			"message は mapping です",
			"messages は配列です",
			"trace は bool です",
		})
	})

	t.Run("db request", func(t *testing.T) {
		v := stepValue(t, `
desc: d
runners:
  db: mysql://u@h/app
steps:
  - db:
      trace: 1
      query: 1
`)
		c := &Context{}
		checkDB(c, v)
		assertEmitOrder(t, c.out, []string{
			"query は空でない文字列です",
			"trace は bool です",
		})
	})

	t.Run("exec request", func(t *testing.T) {
		v := stepValue(t, `
desc: d
steps:
  - exec:
      env: 1
      liveOutput: 1
      background: 1
      shell: 1
      stdin: 1
      command: 1
`)
		c := &Context{}
		checkExec(c, v)
		assertEmitOrder(t, c.out, []string{
			"command は空でない文字列です",
			"stdin は文字列です",
			"shell は文字列です",
			"background は bool です",
			"liveOutput は bool です",
			"env は mapping です",
		})
	})

	t.Run("include request", func(t *testing.T) {
		v := stepValue(t, `
desc: d
steps:
  - include:
      steps: []
      path: ""
`)
		c := &Context{}
		checkInclude(c, v)
		assertEmitOrder(t, c.out, []string{
			"path と steps を同時に指定できません",
			"include にキー steps はありません",
			"include.path は空でない文字列です",
		})
	})

	t.Run("book fields", func(t *testing.T) {
		doc, err := runbook.Parse([]byte(`
trace: 1
force: 1
concurrency: 1
loop: true
skipTest: 1
if: 1
interval: []
debug: 1
hostRules: 1
secrets: 1
vars: 1
runners: 1
needs: 1
labels: 1
desc: 1
steps:
  - test: "true"
`))
		if err != nil || doc.Book == nil {
			t.Fatalf("parse: %v", err)
		}
		c := &Context{Doc: doc}
		bookShape(c)
		assertEmitOrder(t, c.out, []string{
			"desc は文字列です",
			"labels は文字列の配列です",
			"needs は mapping です",
			"runners は mapping です",
			"vars は mapping です",
			"secrets は文字列の配列です",
			"hostRules は mapping です",
			"debug は bool または文字列です",
			"interval は時間です",
			"if は bool または文字列です",
			"skipTest は bool または文字列です",
			"runbook の loop は数値、式、または mapping です",
			"concurrency は文字列または文字列の配列です",
			"force は bool または文字列です",
			"trace は bool または文字列です",
		})
	})

	t.Run("step fields", func(t *testing.T) {
		doc := book(t, `
desc: d
steps:
  - bind: x
    dump: 1
    test: 1
    loop: []
    force: 1
    defer: 1
    desc: 1
    if: 1
    exec:
      command: "true"
`)
		c := &Context{}
		checkStepFields(c, doc.Steps[0])
		assertEmitOrder(t, c.out, []string{
			"if は式または bool です",
			"desc は文字列です",
			"defer は bool または文字列です",
			"force は bool または文字列です",
			"step の loop は数値、式、または mapping です",
			"test は式または bool です",
			"dump は式または mapping です",
			"bind は mapping です",
		})
	})
}

func book(t *testing.T, src string) *runbook.Book {
	t.Helper()
	doc, err := runbook.Parse([]byte(src))
	if err != nil || doc.Book == nil {
		t.Fatalf("parse: %v", err)
	}
	return doc.Book
}

func oneRunner(t *testing.T, src string) runbook.Runner {
	t.Helper()
	b := book(t, src)
	if len(b.Runners) != 1 {
		t.Fatalf("%d runners", len(b.Runners))
	}
	return b.Runners[0]
}

func stepValue(t *testing.T, src string) runbook.Value {
	t.Helper()
	b := book(t, src)
	if len(b.Steps) != 1 || len(b.Steps[0].Value.Map) != 1 {
		t.Fatalf("%+v", b.Steps)
	}
	return b.Steps[0].Value.Map[0].Value
}

func assertEmitOrder(t *testing.T, got []finding.Finding, want []string) {
	t.Helper()
	var msgs []string
	for _, f := range got {
		msgs = append(msgs, f.Message)
	}
	if len(msgs) != len(want) {
		t.Fatalf("got %v\nwant %v", msgs, want)
	}
	for i, part := range want {
		if !strings.Contains(msgs[i], part) {
			t.Fatalf("%d: got %q, want %q\nall %v", i, msgs[i], part, msgs)
		}
	}
}
