package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
)

// Extracting templates from a string that contains }} inside quotes yields two parts: the first still contains closed }}, and the second is vars.name. An unclosed {{ is an error.
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

// validDuration is true for the number 1, the string 10sec, and the template {{ vars.wait }}. It is false for later and for whitespace.
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
