package runnspec

import "testing"

// 取り込んだスキーマを読んだとき、版が v1.11.1 で、トップレベル、x-、ラベル、HTTP と exec と loop と step のキー、各 required が期待どおりであることを確認する。loop の required に count が無いことも確認する。
func TestSchemaSuppliesNotationKeys(t *testing.T) {
	if Version() != "v1.11.1" {
		t.Fatalf("version = %s", Version())
	}
	for _, key := range []string{"desc", "steps", "runners", "hostRules", "anchors"} {
		if _, ok := TopLevelKeys()[key]; !ok {
			t.Fatalf("トップレベル %s がスキーマにありません", key)
		}
	}
	if _, ok := TopLevelKeys()["owner"]; ok {
		t.Fatal("スキーマに無いキーを許可しています")
	}
	if got := ExtensionPrefixes(); len(got) != 1 || got[0] != "x-" {
		t.Fatalf("prefixes = %#v", got)
	}
	if LabelPattern() != `^[^ \n\r!+=|.*%^?><]+$` {
		t.Fatalf("label = %s", LabelPattern())
	}
	for _, key := range []string{"endpoint", "openapi3", "timeout", "trace"} {
		if _, ok := DefKeys(DefHTTPRunner)[key]; !ok {
			t.Fatalf("http runner に %s がありません", key)
		}
	}
	if _, ok := DefKeys(DefExec)["env"]; !ok {
		t.Fatal("exec.env がスキーマから読めていません")
	}
	for _, key := range []string{"count", "until", "minInterval"} {
		if _, ok := DefKeys(DefLoop)[key]; !ok {
			t.Fatalf("loop に %s がありません", key)
		}
	}
	for _, key := range Required(DefLoop) {
		if key == "count" {
			t.Fatal("スキーマが loop.count を必須にしました。checkLoop の上書きを見直してください")
		}
	}
	for _, key := range []string{"defer", "runner", "include"} {
		if _, ok := DefKeys(DefStep)[key]; !ok {
			t.Fatalf("step に %s がありません", key)
		}
	}
	wantRequired := map[string]string{
		DefHTTPRunner:  "endpoint",
		DefGRPCRunner:  "addr",
		DefDBRunner:    "dsn",
		DefIncludeRun:  "path",
		DefAgentRunner: "model",
		DefExec:        "command",
	}
	for def, key := range wantRequired {
		if !contains(Required(def), key) {
			t.Fatalf("%s の required に %s がありません: %#v", def, key, Required(def))
		}
	}
}

// cdpfn を解析したとき、navigate と scrollIntoView と sendKeys の引数、html という別名、tabTo などの名前が期待どおりであることを確認する。latestTab は含まれないことを確認する。
func TestCDPArgsFollowCdpfn(t *testing.T) {
	args := CDPArgs()
	if got := args["navigate"]; len(got) != 1 || got[0] != "url" {
		t.Fatalf("navigate = %#v", got)
	}
	if got := args["scrollIntoView"]; len(got) != 1 || got[0] != "sel" {
		t.Fatalf("scrollIntoView = %#v", got)
	}
	if got := args["sendKeys"]; len(got) != 2 || got[0] != "sel" || got[1] != "value" {
		t.Fatalf("sendKeys = %#v", got)
	}
	if _, ok := args["html"]; !ok {
		t.Fatal("fullHTML の別名 html がありません")
	}
	if len(args["html"]) != 0 {
		t.Fatalf("html args = %#v", args["html"])
	}
	if _, ok := args["latestTab"]; ok {
		t.Fatal("latestTab は v0.132.0 で tabTo に置き換わっています")
	}
	for _, name := range []string{"tabTo", "click", "evaluate", "eval", "setUploadFile"} {
		if _, ok := args[name]; !ok {
			t.Fatalf("%s がありません", name)
		}
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
