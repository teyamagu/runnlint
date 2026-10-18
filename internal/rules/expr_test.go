package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
)

// 引用符の中に }} がある文字列からテンプレートを取り出したとき、2件に分かれ、1件目に closed }} が残り 2件目が vars.name になることを確認する。閉じていない {{ ではエラーになることを確認する。
func TestExtractTemplatesRespectsQuotes(t *testing.T) {
	got, err := extractTemplates("{{ \"closed }}\" }} {{ vars.name }}")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !strings.Contains(got[0], "closed }}") || strings.TrimSpace(got[1]) != "vars.name" {
		t.Fatalf("%#v", got)
	}
	if _, err := extractTemplates(`{{ vars.name`); err == nil {
		t.Fatal("閉じていない {{ を受けています")
	}
}

// 数値 1、文字列 10sec、テンプレート {{ vars.wait }} では validDuration が true になることを確認する。later と空白では false になることを確認する。
func TestValidDuration(t *testing.T) {
	if !validDuration(runbook.Value{Kind: runbook.KindNumber, Number: "1"}) {
		t.Fatal("数値")
	}
	if !validDuration(runbook.Value{Kind: runbook.KindString, String: "10sec"}) {
		t.Fatal("10sec")
	}
	if !validDuration(runbook.Value{Kind: runbook.KindString, String: "{{ vars.wait }}"}) {
		t.Fatal("template")
	}
	if validDuration(runbook.Value{Kind: runbook.KindString, String: "later"}) {
		t.Fatal("later")
	}
	if validDuration(runbook.Value{Kind: runbook.KindString, String: "  "}) {
		t.Fatal("blank")
	}
}
