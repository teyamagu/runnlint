// Package message holds the built-in diagnostic and error texts.
// Each ID has one Japanese sentence and one English sentence.
// Display uses one language. Baseline matching always uses the English sentence.
package message

import (
	"fmt"
)

// Lang is the language used when a diagnostic is shown.
type Lang string

const (
	JA Lang = "ja"
	EN Lang = "en"
)

// ID identifies one built-in sentence.
type ID string

var catalog = map[ID][2]string{}

func spec(id ID, ja, en string) ID {
	if _, ok := catalog[id]; ok {
		panic("duplicate message " + string(id))
	}
	catalog[id] = [2]string{ja, en}
	return id
}

// Built-in sentences. The format verbs of the two languages are in the same order.
var (
	LangUnknown = spec("lang-unknown", "language %q は ja か en です", "language %q must be ja or en")

	FileEmpty    = spec("file-empty", "空のファイルです", "file is empty")
	BookNotMap   = spec("book-not-map", "runbook は mapping です", "a runbook is a mapping")
	ConfigAbsent = spec("config-absent", "設定がありません", "configuration is missing")
	UsesUnknown  = spec("uses-unknown", "ルール %s の uses %q は未定義です", "rule %s uses %q, which is not defined")

	DefaultRulesRead  = spec("default-rules-read", "デフォルトルールを読めません: %s", "cannot read the default rules: %s")
	DefaultRulesBad   = spec("default-rules-bad", "デフォルトルールが不正です: %s", "default rules are invalid: %s")
	RulesFileRead     = spec("rules-file-read", "ルールファイルを読めません: %s", "cannot read the rules file: %s")
	YAMLRead          = spec("yaml-read", "YAML として読めません: %s", "cannot read YAML: %s")
	TopNotMap         = spec("top-not-map", "トップレベルが mapping ではありません", "the top level is not a mapping")
	UnknownTopKey     = spec("unknown-top-key", "runnlint のルールファイルに無いキー %s があります (行 %d)", "key %s is not part of a runnlint rules file (line %d)")
	RulesShape        = spec("rules-shape", "runnlint のルールファイルの形ではありません: %s", "not a runnlint rules file: %s")
	RulesVersion      = spec("rules-version", "runnlint のルールファイルの version は 1 です (version: %d)", "a runnlint rules file has version 1 (version: %d)")
	UnknownKeyIgnored = spec("unknown-key-ignored", "未知のキーを無視します: %s", "ignoring unknown keys: %s")
	RuleIDEmpty       = spec("rule-id-empty", "id が空のルールがあります", "a rule has an empty id")
	RuleIDDup         = spec("rule-id-dup", "ルール %s が重複しています", "rule %s is duplicated")
	SeverityBad       = spec("severity-bad", "ルール %s の severity %q は error / warning / info / off のいずれかです", "rule %s has severity %q; use error, warning, info, or off")
	UsesAndCheck      = spec("uses-and-check", "ルール %s は uses と check を同時に持てません", "rule %s cannot have both uses and check")
	UsesOrCheck       = spec("uses-or-check", "ルール %s には uses か check が必要です", "rule %s needs uses or check")
	SelectEmpty       = spec("select-empty", "ルール %s の check.select が空です", "rule %s has an empty check.select")
	YamllintMissing   = spec("yamllint-missing-sev", "yamllint.missing %q は error / warning / info / off のいずれかです", "yamllint.missing %q must be error, warning, info, or off")

	BaselineRead    = spec("baseline-read", "baseline を読めません: %s", "cannot read the baseline: %s")
	BaselineJSON    = spec("baseline-json", "%s: baseline の JSON が不正です: %s", "%s: baseline JSON is invalid: %s")
	BaselineVersion = spec("baseline-version", "%s: baseline の version %d は読めません (対応は %d)。--write-baseline で書き直してください", "%s: cannot read baseline version %d (supported version is %d). Rewrite it with --write-baseline")
	BaselineWrite   = spec("baseline-write", "baseline を書けません: %s", "cannot write the baseline: %s")
	BaselineWrote   = spec("baseline-wrote", "baseline %s に %d 件の診断を書きました\n", "baseline %s: wrote %d diagnostics\n")
	BaselineKept    = spec("baseline-kept", "baseline %s: 既存の %d 件を除きました", "baseline %s: suppressed %d existing diagnostics")
	BaselineStale   = spec("baseline-stale", "。%d 件は解消済みです。--write-baseline で更新できます", ". %d are already fixed. Update the file with --write-baseline")

	IncludeRootStat = spec("include-root-stat", "--include-root: %s", "--include-root: %s")
	IncludeRootDir  = spec("include-root-dir", "--include-root %s はディレクトリではありません", "--include-root %s is not a directory")
	FormatUnknown   = spec("format-unknown", "不明な出力形式です: %s", "unknown output format: %s")
	YamllintFailed  = spec("yamllint-failed", "yamllint の実行に失敗しました: %s", "yamllint failed: %s")

	ExprUnclosed = spec("expr-unclosed", "{{ が閉じられていません", "{{ is not closed")
	ExprNotBrace = spec("expr-not-brace", "%s は式なので {{ }} で囲みません", "%s is an expression, so do not wrap it in {{ }}")

	StepsMapRef      = spec("steps-map-ref", "steps は map なので steps.%s の形で参照します", "steps is a map; refer to it as steps.%s")
	StepsIndexMiss   = spec("steps-index-miss", "steps[%d] はこの runbook にありません", "steps[%d] is not in this runbook")
	StepsDuring      = spec("steps-during", "steps は step の実行中に参照します", "refer to steps while a step is running")
	StepsIndexEarly  = spec("steps-index-early", "steps[%d] はまだ実行されていません", "steps[%d] has not run yet")
	StepsArrayRef    = spec("steps-array-ref", "steps は配列なので steps[n] の形で参照します", "steps is an array; refer to it as steps[n]")
	StepsNameMiss    = spec("steps-name-miss", "steps.%s はこの runbook にありません", "steps.%s is not in this runbook")
	StepsNameEarly   = spec("steps-name-early", "steps.%s はまだ実行されていません", "steps.%s has not run yet")
	VarMissing       = spec("var-missing", "vars.%s は vars にありません", "vars.%s is not in vars")
	ParentVarsInside = spec("parent-vars-inside", "parent.vars.%s は include された runbook の中で参照します", "refer to parent.vars.%s inside an included runbook")
	ParentVarsMiss   = spec("parent-vars-miss", "parent.vars.%s は親の vars にありません", "parent.vars.%s is not in the parent vars")
	LoopIndex        = spec("loop-index", "i は loop の中で使える添字です", "i is the loop index")
	NoPrevious       = spec("no-previous", "先頭の step には previous がありません", "the first step has no previous")

	ReMatches    = spec("re-matches", "matches の正規表現が不正です: %s", "matches has an invalid regular expression: %s")
	ReNotMatches = spec("re-not-matches", "not_matches の正規表現が不正です: %s", "not_matches has an invalid regular expression: %s")
	ReKeyMatches = spec("re-key-matches", "key_matches の正規表現が不正です: %s", "key_matches has an invalid regular expression: %s")
	ReKeyNot     = spec("re-key-not", "key_not_matches の正規表現が不正です: %s", "key_not_matches has an invalid regular expression: %s")
	ReWhere      = spec("re-where", "where.matches の正規表現が不正です: %s", "where.matches has an invalid regular expression: %s")
	PathMissing  = spec("path-missing", "パス %s がありません", "path %s is missing")
	PathPresent  = spec("path-present", "パス %s があります", "path %s is present")
	CondMatch    = spec("cond-match", "条件に一致しました", "condition matched")
	ValueNotEq   = spec("value-not-eq", "値が %s ではありません", "value is not %s")
	ValueIs      = spec("value-is", "値が %s です", "value is %s")
	ValuePattern = spec("value-pattern", "値がパターンに一致しません", "value does not match the pattern")
	ValueForbid  = spec("value-forbid", "値が禁止パターンに一致します", "value matches the forbidden pattern")
	KeyPattern   = spec("key-pattern", "キーがパターンに一致しません", "key does not match the pattern")
	KeyForbid    = spec("key-forbid", "キーが禁止パターンに一致します", "key matches the forbidden pattern")
	ValueOneOf   = spec("value-one-of", "値が許可された一覧にありません", "value is not in the allowed list")
	LenUnder     = spec("len-under", "長さが %d 未満です", "length is under %d")
	LenOver      = spec("len-over", "長さが %d を超えています", "length is over %d")

	IfExpr       = spec("if-expr", "if は式または bool です", "if is an expression or a bool")
	DeferType    = spec("defer-type", "defer は bool または文字列です", "defer is a bool or a string")
	ForceType    = spec("force-type", "force は bool または文字列です", "force is a bool or a string")
	TestExpr     = spec("test-expr", "test は式または bool です", "test is an expression or a bool")
	DumpExprMiss = spec("dump-expr-miss", "dump.expr がありません", "dump.expr is missing")
	DumpOutType  = spec("dump-out-type", "dump.out は文字列です", "dump.out is a string")
	DumpType     = spec("dump-type", "dump は式または mapping です", "dump is an expression or a mapping")
	BindType     = spec("bind-type", "bind は mapping です", "bind is a mapping")
	BindEmpty    = spec("bind-empty", "bind が空です", "bind is empty")
	RunnerColon  = spec("runner-colon", "runner: は他の runner と同じ step に書けません", "runner: cannot share a step with another runner")
	RunnerMap    = spec("runner-map", "runner: は runner 名から定義への mapping です", "runner: is a mapping from runner name to definition")
	MainOne      = spec("main-one", "step の main runner は1つです", "a step has one main runner")
	MainMissing  = spec("main-missing", "step に実行する runner がありません", "step has no runner to run")

	HTTPOnePath    = spec("http-one-path", "HTTP リクエストは path を1つだけ持つ mapping です", "an HTTP request is a mapping with exactly one path")
	HTTPPathEmpty  = spec("http-path-empty", "HTTP の path が空です", "HTTP path is empty")
	HTTPPathSlash  = spec("http-path-slash", "HTTP の path %s は / から始めます", "HTTP path %s must start with /")
	HTTPOneMethod  = spec("http-one-method", "HTTP リクエストのメソッドは1つです", "an HTTP request has one method")
	HTTPMethodBad  = spec("http-method-bad", "HTTP メソッド %s は使えません", "HTTP method %s is not allowed")
	HTTPMethodMap  = spec("http-method-map", "メソッド %s の値は mapping です", "method %s must be a mapping")
	UseCookieBool  = spec("use-cookie-bool", "useCookie は bool です", "useCookie is a bool")
	TraceBool      = spec("trace-bool", "trace は bool です", "trace is a bool")
	HeadersMap     = spec("headers-map", "headers は mapping です", "headers is a mapping")
	HeaderElem     = spec("header-elem", "header %s の要素は文字列です", "an element of header %s is a string")
	HeaderValue    = spec("header-value", "header %s の値は文字列または文字列の配列です", "header %s is a string or an array of strings")
	BodyRequired   = spec("body-required", "%s には media type 付きの body が必要です", "%s needs a body with a media type")
	BodyOneType    = spec("body-one-type", "body の media type は1つです", "body has one media type")
	BodyTypeEmpty  = spec("body-type-empty", "body の media type が空です", "body media type is empty")
	BodyNull       = spec("body-null", "%s の body が null です", "body of %s is null")
	BodyText       = spec("body-text", "text/plain の body は文字列です", "a text/plain body is a string")
	BodyForm       = spec("body-form", "application/x-www-form-urlencoded の body は mapping です", "an application/x-www-form-urlencoded body is a mapping")
	BodyMulti      = spec("body-multi", "multipart/form-data の body は mapping または配列です", "a multipart/form-data body is a mapping or an array")
	BodyFilename   = spec("body-filename", "media type %s の mapping body には filename が必要です", "a mapping body of media type %s needs filename")
	GRPCOne        = spec("grpc-one", "gRPC リクエストは service/method を1つだけ持つ mapping です", "a gRPC request is a mapping with exactly one service/method")
	GRPCShape      = spec("grpc-shape", "gRPC のメソッド %s は package.Service/Method の形です", "gRPC method %s has the form package.Service/Method")
	GRPCMap        = spec("grpc-map", "gRPC メソッドの値は mapping です", "a gRPC method value is a mapping")
	GRPCMsgBoth    = spec("grpc-msg-both", "message と messages は同時に指定できません", "message and messages cannot both be set")
	GRPCMessage    = spec("grpc-message", "message は mapping です", "message is a mapping")
	TimeoutDur     = spec("timeout-dur", "timeout は時間です", "timeout is a duration")
	MessagesArray  = spec("messages-array", "messages は配列です", "messages is an array")
	MessagesWord   = spec("messages-word", "messages の文字列は receive または close です", "a string in messages is receive or close")
	MessagesElem   = spec("messages-elem", "messages の要素は mapping、receive、close のいずれかです", "an element of messages is a mapping, receive, or close")
	DBMap          = spec("db-map", "DB リクエストは mapping です", "a DB request is a mapping")
	QueryEmpty     = spec("query-empty", "query は空でない文字列です", "query is a non-empty string")
	ExecMap        = spec("exec-map", "exec は mapping です", "exec is a mapping")
	StdinString    = spec("stdin-string", "stdin は文字列です", "stdin is a string")
	ShellString    = spec("shell-string", "shell は文字列です", "shell is a string")
	ShellValue     = spec("shell-value", "shell は bash、sh、または {0} を含むコマンドです", "shell is bash, sh, or a command that contains {0}")
	BackgroundBool = spec("background-bool", "background は bool です", "background is a bool")
	LiveBool       = spec("live-bool", "liveOutput は bool です", "liveOutput is a bool")
	EnvMap         = spec("env-map", "env は mapping です", "env is a mapping")
	EnvString      = spec("env-string", "env.%s は文字列です", "env.%s is a string")
	RequestMap     = spec("request-map", "%s リクエストは mapping です", "a %s request is a mapping")
	CommandEmpty   = spec("command-empty", "command は空でない文字列です", "command is a non-empty string")
	CDPMap         = spec("cdp-map", "CDP リクエストは mapping です", "a CDP request is a mapping")
	ActionsEmpty   = spec("actions-empty", "actions は空でない配列です", "actions is a non-empty array")
	CDPUnknown     = spec("cdp-unknown", "CDP アクション %s はありません", "CDP action %s does not exist")
	CDPArgsNeed    = spec("cdp-args-need", "CDP アクション %s には引数が必要です", "CDP action %s needs arguments")
	CDPOneKey      = spec("cdp-one-key", "CDP アクションはキーを1つだけ持つ mapping です", "a CDP action is a mapping with exactly one key")
	CDPNoArgs      = spec("cdp-no-args", "CDP アクション %s は引数を取りません", "CDP action %s takes no arguments")
	CDPArgMap      = spec("cdp-arg-map", "CDP アクション %s は %s を mapping で指定します", "CDP action %s takes %s as a mapping")
	CDPArgType     = spec("cdp-arg-type", "CDP アクション %s の引数は文字列または mapping です", "arguments of CDP action %s are a string or a mapping")
	CDPElemType    = spec("cdp-elem-type", "CDP アクションは文字列または mapping です", "a CDP action is a string or a mapping")
	AgentMap       = spec("agent-map", "agent リクエストは mapping です", "an agent request is a mapping")
	PromptString   = spec("prompt-string", "prompt は文字列です", "prompt is a string")
	ClearBool      = spec("clear-bool", "clearContext は bool です", "clearContext is a bool")
	IncludeEmpty   = spec("include-empty", "include の path が空です", "include path is empty")
	IncludeBoth    = spec("include-both", "include では path と steps を同時に指定できません", "include cannot set both path and steps")
	IncludeNeed    = spec("include-need", "include には path または steps が必要です", "include needs path or steps")
	IncludePath    = spec("include-path", "include.path は空でない文字列です", "include.path is a non-empty string")
	IncludeVars    = spec("include-vars", "include.vars は mapping です", "include.vars is a mapping")
	IncludeRunners = spec("include-runners", "include.runners は mapping です", "include.runners is a mapping")
	IncludeDesc    = spec("include-desc", "include.desc は文字列です", "include.desc is a string")
	IncludeBool    = spec("include-bool", "include.%s は bool です", "include.%s is a bool")
	IncludeType    = spec("include-type", "include は path または mapping です", "include is a path or a mapping")

	DupKey         = spec("dup-key", "キー %s が重複しています", "key %s is duplicated")
	UnknownBookKey = spec("unknown-book-key", "トップレベルのキー %s は runbook のフィールドではありません", "top-level key %s is not a runbook field")
	StepsMissing   = spec("steps-missing", "steps がありません", "steps is missing")
	LabelChars     = spec("label-chars", "label に使えない文字があります: %s", "label contains characters that are not allowed: %s")
	DeferExper     = spec("defer-experimental", "defer は実験的機能です", "defer is experimental")
	IntervalDur    = spec("interval-dur", "interval は時間です", "interval is a duration")
	FieldString    = spec("field-string", "%s は文字列です", "%s is a string")
	FieldStrings   = spec("field-strings", "%s は文字列の配列です", "%s is an array of strings")
	FieldElem      = spec("field-elem", "%s の要素は文字列です", "an element of %s is a string")
	FieldMap       = spec("field-map", "%s は mapping です", "%s is a mapping")
	FieldMapString = spec("field-map-string", "%s.%s は文字列です", "%s.%s is a string")
	SecretsArray   = spec("secrets-array", "secrets は文字列の配列です", "secrets is an array of strings")
	SecretsElem    = spec("secrets-elem", "secrets の要素は空でない文字列です", "an element of secrets is a non-empty string")
	FieldBoolOrStr = spec("field-bool-or-str", "%s は bool または文字列です", "%s is a bool or a string")
	ConcEmpty      = spec("conc-empty", "concurrency が空です", "concurrency is empty")
	ConcElem       = spec("conc-elem", "concurrency の要素は空でない文字列です", "an element of concurrency is a non-empty string")
	ConcType       = spec("conc-type", "concurrency は文字列または文字列の配列です", "concurrency is a string or an array of strings")
	LoopCountNeg   = spec("loop-count-neg", "loop の回数は 0 以上です", "loop count is zero or greater")
	LoopCountMiss  = spec("loop-count-miss", "loop.count がありません", "loop.count is missing")
	LoopUnknownKey = spec("loop-unknown-key", "loop にキー %s はありません", "loop has no key %s")
	LoopCountMin   = spec("loop-count-min", "loop.count は 0 以上です", "loop.count is zero or greater")
	LoopCountType  = spec("loop-count-type", "loop.count は数値または式です", "loop.count is a number or an expression")
	LoopDuration   = spec("loop-duration", "loop.%s は時間です", "loop.%s is a duration")
	LoopNumber     = spec("loop-number", "loop.%s は数値です", "loop.%s is a number")
	LoopUntil      = spec("loop-until", "loop.until は式です", "loop.until is an expression")
	LoopType       = spec("loop-type", "%s の loop は数値、式、または mapping です", "loop of %s is a number, an expression, or a mapping")

	RunnerReserved = spec("runner-reserved", "runner 名 %s は runn の予約語です", "runner name %s is reserved by runn")
	RunnerMissing  = spec("runner-missing", "runner %s は runners にも先行する runner: にもありません", "runner %s is not in runners or a preceding runner:")
	RunnerEndpoint = spec("runner-endpoint", "runner %s の値は http(s)、grpc、ssh、cdp、chrome、または DB の DSN ではありません", "runner %s is not an http(s), grpc, ssh, cdp, or chrome value, or a DB DSN")
	RunnerKind     = spec("runner-kind", "runner %s の種別を判定できません", "cannot tell the kind of runner %s")
	RunnerType     = spec("runner-type", "runner %s の値は文字列または mapping です", "runner %s is a string or a mapping")
	RunnerURL      = spec("runner-url", "runner %s の endpoint は http(s) の URL です", "endpoint of runner %s is an http(s) URL")
	RunnerDSNEmpty = spec("runner-dsn-empty", "runner %s の DSN が空です", "DSN of runner %s is empty")
	RunnerScheme   = spec("runner-scheme", "runner %s の値は既知の runner スキームでも DB の DSN でもありません", "runner %s is neither a known runner scheme nor a DB DSN")
	RunnerEpString = spec("runner-ep-string", "runner %s の endpoint は文字列です", "endpoint of runner %s is a string")
	RunnerHost     = spec("runner-host", "runner %s には host または hostname が必要です", "runner %s needs host or hostname")
	RunnerIdentity = spec("runner-identity", "runner %s では identityFile と identityKey を同時に指定できません", "runner %s cannot set both identityFile and identityKey")
	RunnerFlags    = spec("runner-flags", "runner %s の flags は mapping です", "flags of runner %s is a mapping")
	RunnerParams   = spec("runner-params", "runner %s の params は mapping です", "params of runner %s is a mapping")
	AgentName      = spec("agent-name", "agent は claude、copilot、codex のいずれかです", "agent is claude, copilot, or codex")
	NoKey          = spec("no-key", "%s にキー %s はありません", "%s has no key %s")
	NeedKey        = spec("need-key", "%s には %s が必要です", "%s needs %s")
	IsDuration     = spec("is-duration", "%s は時間です", "%s is a duration")
	TraceEnable    = spec("trace-enable", "trace.enable は bool または文字列です", "trace.enable is a bool or a string")
	TraceHeader    = spec("trace-header", "trace.headerName は文字列です", "trace.headerName is a string")
	TraceType      = spec("trace-type", "trace は bool、文字列、または mapping です", "trace is a bool, a string, or a mapping")
	PortPositive   = spec("port-positive", "port は正の数です", "port is a positive number")
	PortNumber     = spec("port-number", "port は数値です", "port is a number")
	KbdArray       = spec("kbd-array", "keyboardInteractive は配列です", "keyboardInteractive is an array")
	KbdElem        = spec("kbd-elem", "keyboardInteractive の要素は mapping です", "an element of keyboardInteractive is a mapping")
	KbdString      = spec("kbd-string", "keyboardInteractive.%s は文字列です", "keyboardInteractive.%s is a string")

	IncludedNone = spec("included-none", "if: %s の runbook ですが、走査した範囲のどの runbook からも include されていません", "runbook has if: %s, but no runbook in the scanned set includes it")
	FileMissing  = spec("file-missing", "ファイル %s が見つかりません", "file %s was not found")

	SuppressInBlock = spec("suppress-in-block", "複数行の値の中のコメントは抑制になりません。診断はキーの行に出るので、キーの行の行末か前の行に書きます", "a comment inside a multiline value is not a suppression. The diagnostic is reported on the key line, so write it at the end of the key line or on the line before it")
	SuppressVerb    = spec("suppress-verb", "runnlint %s は使えません。disable-line を使います", "runnlint %s is not available. Use disable-line")
	SuppressNone    = spec("suppress-none", "runnlint disable-line で抑制する診断がありません", "runnlint disable-line suppresses nothing")
	SuppressYAML    = spec("suppress-yaml", "%s は yamllint の診断です。# yamllint disable-line rule:%s で抑制します", "%s is a yamllint diagnostic. Suppress it with # yamllint disable-line rule:%s")
	SuppressNoRule  = spec("suppress-no-rule", "ルール %s はありません", "rule %s does not exist")
	SuppressNoHit   = spec("suppress-no-hit", "rule:%s で抑制する診断がありません", "rule:%s suppresses nothing")

	OneYAML      = spec("one-yaml", "runbook は1つの YAML document です", "a runbook is one YAML document")
	AliasDeep    = spec("alias-deep", "YAML の参照が深すぎます", "YAML alias nesting is too deep")
	AliasUnknown = spec("alias-unknown", "解決できない alias があります", "an alias cannot be resolved")
	AliasCycle   = spec("alias-cycle", "YAML anchor が循環しています", "a YAML anchor is cyclic")
	YAMLNode     = spec("yaml-node", "未対応の YAML ノードです", "unsupported YAML node")
	KeyScalar    = spec("key-scalar", "mapping のキーはスカラーです (行 %d)", "a mapping key is a scalar (line %d)")
	KeyNull      = spec("key-null", "mapping のキーが null です (行 %d)", "a mapping key is null (line %d)")

	SchemaDefs     = spec("schema-defs", "スキーマに $defs.%s がありません", "schema has no $defs.%s")
	SchemaLabels   = spec("schema-labels", "スキーマに labels の pattern がありません", "schema has no pattern for labels")
	SchemaCDP      = spec("schema-cdp", "CDP のアクションを読めません", "cannot read CDP actions")
	SchemaEmpty    = spec("schema-empty", "スキーマが空です", "schema is empty")
	SchemaProps    = spec("schema-props", "スキーマに properties がありません", "schema has no properties")
	SchemaNoDefs   = spec("schema-no-defs", "スキーマに $defs がありません", "schema has no $defs")
	CDPMapMissing  = spec("cdp-map-missing", "CDPFnMap がありません", "CDPFnMap is missing")
	CDPMapEmpty    = spec("cdp-map-empty", "CDPFnMap が空です", "CDPFnMap is empty")
	CDPMapStart    = spec("cdp-map-start", "CDPFnMap の開始を読めません", "cannot read the start of CDPFnMap")
	CDPMapClose    = spec("cdp-map-close", "CDPFnMap が閉じていません", "CDPFnMap is not closed")
	NotString      = spec("not-string", "文字列ではありません", "not a string")
	BadString      = spec("bad-string", "不正な文字列です", "invalid string")
	StringUnclosed = spec("string-unclosed", "文字列が閉じていません", "string is not closed")

	LicenseEmpty   = spec("license-empty", "ライセンス本文が空です", "license text is empty")
	LicenseForbid  = spec("license-forbid", "MIT ライセンスのプロジェクトからは利用できないライセンスです (%s)", "license cannot be used from an MIT-licensed project (%s)")
	LicenseUnknown = spec("license-unknown", "MIT、BSD、ISC、Apache-2.0 として判定できません", "not recognized as MIT, BSD, ISC, or Apache-2.0")

	GoListParse   = spec("go-list-parse", "Go のリリース一覧を解釈できません: %s", "cannot parse the Go release list: %s")
	GoNoStable    = spec("go-no-stable", "安定版がリリース一覧にありません", "the release list has no stable version")
	GoListFetch   = spec("go-list-fetch", "Go のリリース一覧を取得できません: %s", "cannot fetch the Go release list: %s")
	GoListStatus  = spec("go-list-status", "Go のリリース一覧が HTTP %d を返しました", "the Go release list returned HTTP %d")
	GoVersionRead = spec("go-version-read", ".go-version を読めません: %s", "cannot read .go-version: %s")
	GoModRead     = spec("go-mod-read", "go.mod を読めません: %s", "cannot read go.mod: %s")
	GoModLine     = spec("go-mod-line", "go.mod に go 行がありません", "go.mod has no go line")
	VerNotStable  = spec("ver-not-stable", "バージョン %q は安定版の番号ではありません", "version %q is not a stable version number")
	VerNotTriple  = spec("ver-not-triple", "バージョン %q は major.minor.patch ではありません", "version %q is not major.minor.patch")
	VerNotNumber  = spec("ver-not-number", "バージョン %q を数値として読めません", "version %q is not numeric")

	RunnRelease  = spec("runn-release", "runn のリリース情報を解釈できません: %s", "cannot parse the runn release information: %s")
	RunnTagEmpty = spec("runn-tag-empty", "runn の最新タグが空です", "the latest runn tag is empty")
	HTTPFetch    = spec("http-fetch", "%s を取得できません: %s", "cannot fetch %s: %s")
	HTTPStatus   = spec("http-status", "%s が HTTP %d を返しました", "%s returned HTTP %d")
	HTTPEmpty    = spec("http-empty", "%s が空です", "%s is empty")
	GoModMissing = spec("go-mod-missing", "go.mod が見つかりません", "go.mod was not found")
	TagEmpty     = spec("tag-empty", "タグが空です", "tag is empty")
	SpecEmpty    = spec("spec-empty", "記法定義が空です", "the syntax definition is empty")

	Usage = spec("usage", "使い方: runnlint [flags] [path...]\n\nrunn の runbook を検査します。一般的な YAML の検査は yamllint に委譲します。\n\n行の診断は YAML のコメントで抑制できます。行末ならその行、コメントだけの行なら次の行に効きます。\n  test: current.res.status == 200 # runnlint disable-line rule:expr-syntax -- 理由\n  # runnlint disable-line rule:var-ref,step-ref\n\n",
		"Usage: runnlint [flags] [path...]\n\nLint runn runbooks. General YAML checks are delegated to yamllint.\n\nSuppress a diagnostic on one line with a YAML comment. At the end of a line it applies to that line. On a comment-only line it applies to the next line.\n  test: current.res.status == 200 # runnlint disable-line rule:expr-syntax -- reason\n  # runnlint disable-line rule:var-ref,step-ref\n\n")
	FlagConfig      = spec("flag-config", "ルールファイルのパス", "path to the rules file")
	FlagFormat      = spec("flag-format", "出力形式 (text, json, github)", "output format (text, json, github)")
	FlagNoYamllint  = spec("flag-no-yamllint", "yamllint を実行しない", "do not run yamllint")
	FlagStrict      = spec("flag-strict", "warning でも終了コードを 1 にする。info は対象外", "exit 1 on warnings as well. Info does not count")
	FlagBaseline    = spec("flag-baseline", "既存の診断を記録したファイル。一致する診断を出さない", "file of accepted diagnostics. Matching diagnostics are hidden")
	FlagIncludeRoot = spec("flag-include-root", "include の参照元を探す追加のディレクトリ。繰り返し指定できる", "extra directory to search for include callers. May be repeated")
	FlagWriteBase   = spec("flag-write-baseline", "今回の診断を baseline として書き出して終了する", "write the current diagnostics as a baseline and exit")
	FlagPrintConfig = spec("flag-print-config", "デフォルトのルールファイルを出力して終了する", "print the default rules file and exit")
	FlagPrintYAML   = spec("flag-print-yamllint", "組み込みの yamllint 設定を出力して終了する", "print the built-in yamllint config and exit")
	FlagListRules   = spec("flag-list-rules", "有効なルール一覧を出力して終了する", "print the enabled rules and exit")
	FlagVersion     = spec("flag-version", "バージョンを出力して終了する", "print the version and exit")
	FlagLang        = spec("flag-lang", "診断の表示言語 (ja, en)。設定の language より優先する", "language for diagnostics (ja, en). Overrides language in the config")
)

// Text renders id in lang. A missing ID renders as itself.
func Text(lang Lang, id ID, args ...any) string {
	pair, ok := catalog[id]
	if !ok {
		return string(id)
	}
	format := pair[0]
	if lang == EN {
		format = pair[1]
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// English renders id for baseline matching.
func English(id ID, args ...any) string {
	return Text(EN, id, args...)
}

// Catalog returns a copy of every sentence, Japanese then English.
func Catalog() map[ID][2]string {
	out := make(map[ID][2]string, len(catalog))
	for id, pair := range catalog {
		out[id] = pair
	}
	return out
}

// Error is a built-in error that can be shown in either language.
// Error returns the Japanese sentence so callers that inspect the error keep the previous text.
type Error struct {
	id    ID
	args  []any
	cause error
}

// New returns an error for id.
func New(id ID, args ...any) error {
	return &Error{id: id, args: args}
}

// Wrap returns an error whose format string ends with a verb for cause.
func Wrap(id ID, cause error, args ...any) error {
	return &Error{id: id, args: args, cause: cause}
}

func (e *Error) Error() string { return e.Localize(JA) }

func (e *Error) Unwrap() error { return e.cause }

// Localize renders e in lang. A wrapped cause is rendered in the same language and passed as the last argument.
func (e *Error) Localize(lang Lang) string {
	args := e.args
	if e.cause != nil {
		args = append(append([]any{}, e.args...), Localize(lang, e.cause))
	}
	return Text(lang, e.id, args...)
}

type prefixed struct {
	prefix string
	err    error
}

func (p *prefixed) Error() string { return p.prefix + ": " + p.err.Error() }

func (p *prefixed) Unwrap() error { return p.err }

func (p *prefixed) Localize(lang Lang) string {
	return p.prefix + ": " + Localize(lang, p.err)
}

// Prefix attaches a path in front of err.
func Prefix(prefix string, err error) error {
	if err == nil {
		return nil
	}
	return &prefixed{prefix: prefix, err: err}
}

// Localize renders err in lang. Errors from other packages are returned unchanged.
func Localize(lang Lang, err error) string {
	if err == nil {
		return ""
	}
	switch e := err.(type) {
	case *Error:
		return e.Localize(lang)
	case *prefixed:
		return e.Localize(lang)
	default:
		return err.Error()
	}
}

// Pair returns the display sentence and the English baseline sentence for err.
func Pair(lang Lang, err error) (display, match string) {
	return Localize(lang, err), Localize(EN, err)
}
