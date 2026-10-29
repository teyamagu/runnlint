package message

import (
	"regexp"
	"testing"
)

var verbRe = regexp.MustCompile(`%(?:%|[#+\-\d\.]*[sdqvxX])`)

func TestCatalogVerbsMatch(t *testing.T) {
	if len(catalog) == 0 {
		t.Fatal("catalog is empty")
	}
	for id, pair := range catalog {
		ja, en := verbs(pair[0]), verbs(pair[1])
		if len(ja) != len(en) {
			t.Fatalf("%s verbs ja=%v en=%v\n%s\n%s", id, ja, en, pair[0], pair[1])
		}
		for i := range ja {
			if ja[i] != en[i] && !(ja[i] == "%s" && en[i] == "%s") {
				if ja[i] != en[i] {
					t.Fatalf("%s verb %d ja=%s en=%s", id, i, ja[i], en[i])
				}
			}
		}
		if pair[0] == "" || pair[1] == "" {
			t.Fatalf("%s is empty", id)
		}
	}
}

func verbs(s string) []string {
	all := verbRe.FindAllString(s, -1)
	var out []string
	for _, v := range all {
		if v != "%%" {
			out = append(out, v)
		}
	}
	return out
}

func TestTextUsesLanguage(t *testing.T) {
	if got := Text(JA, StepsMissing); got != "steps がありません" {
		t.Fatalf("%s", got)
	}
	if got := Text(EN, StepsMissing); got != "steps is missing" {
		t.Fatalf("%s", got)
	}
	err := Wrap(RulesFileRead, New(RuleIDEmpty))
	if got := Localize(EN, err); got != "cannot read the rules file: a rule has an empty id" {
		t.Fatalf("%s", got)
	}
	if got := err.Error(); got != "ルールファイルを読めません: id が空のルールがあります" {
		t.Fatalf("%s", got)
	}
}
