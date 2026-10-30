package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
)

// Every pair of operator and selected value behaves as documented for declarative rules: exists: true reports a missing path and exists:
// false a present one; equals and not_equals compare by kind and value; matches and not_matches apply to strings and numbers only, and
// matches also reports any other kind; key_matches and key_not_matches look at the key or the list index; one_of compares like equals;
// min_len and max_len count characters, elements, or entries; forbid reports every selected value. The selected value may sit at the top
// level, under vars, as a list element, or as a list or map step.
func TestDeclarativeOperatorsPairwise(t *testing.T) {
	factors := []factor{
		{"op", []string{"exists-true", "exists-false", "equals", "not_equals", "matches", "not_matches", "key_matches", "key_not_matches", "one_of", "min_len", "max_len", "forbid"}},
		{"value", []string{"string", "number", "bool", "null", "seq", "map", "missing"}},
		{"select", []string{"top", "vars", "labels-elem", "steps-list", "steps-map"}},
	}
	runPairwise(t, factors, func(t *testing.T, p pick) {
		literal := map[string]string{"string": "one", "number": "1", "bool": "true", "null": "null", "seq": "[one]", "map": "{k: one}"}[p["value"]]
		var src, sel, key, path string
		kind := p["value"]
		switch p["select"] {
		case "top":
			sel, key, path = "debug", "debug", "debug"
			src = "desc: d\n"
			if kind != "missing" {
				src += "debug: " + literal + "\n"
			}
			src += "steps:\n  - test: \"true\"\n"
		case "vars":
			sel, key, path = "vars.k", "k", "vars.k"
			src = "desc: d\nvars:\n"
			if kind != "missing" {
				src += "  k: " + literal + "\n"
			} else {
				src += "  other: 1\n"
			}
			src += "steps:\n  - test: \"true\"\n"
		case "labels-elem":
			sel, key, path = "labels.*", "0", "labels[0]"
			kind = "string"
			src = "desc: d\nlabels:\n  - one\nsteps:\n  - test: \"true\"\n"
		case "steps-list":
			sel, key, path = "steps.*", "0", "steps[0]"
			kind = "map"
			src = "desc: d\nsteps:\n  - test: \"true\"\n"
		case "steps-map":
			sel, key, path = "steps.*", "a", "steps.a"
			kind = "map"
			src = "desc: d\nsteps:\n  a:\n    test: \"true\"\n"
		}
		cond := map[string]string{
			"exists-true": "exists: true", "exists-false": "exists: false", "equals": "equals: 1", "not_equals": "not_equals: 1",
			"matches": "matches: '^[o1]'", "not_matches": "not_matches: '^[o1]'", "key_matches": "key_matches: '^[a-z]+$'", "key_not_matches": "key_not_matches: '^[a-z]+$'",
			"one_of": "one_of: [one, 1, true]", "min_len": "min_len: 2", "max_len": "max_len: 0", "forbid": "forbid: true",
		}[p["op"]]
		yml := "version: 1\nextends: none\nrules:\n  - id: custom\n    severity: warning\n    check:\n      select: " + sel + "\n      " + cond + "\n"
		got := lintConfig(t, yml, src)
		var gotMatch []string
		for _, f := range findRule(got, "custom") {
			gotMatch = append(gotMatch, f.Match)
		}
		textual := kind == "string" || kind == "number"
		keyAlpha := key == "debug" || key == "k" || key == "a"
		length := map[string]int{"string": 3, "number": 0, "bool": 0, "null": 0, "seq": 1, "map": 1}[kind]
		var want []string
		switch p["op"] {
		case "exists-true":
			if kind == "missing" {
				want = append(want, en(message.PathMissing, sel))
			}
		case "exists-false":
			if kind != "missing" {
				want = append(want, en(message.PathPresent, path))
			}
		case "equals":
			if kind != "missing" && kind != "number" {
				want = append(want, en(message.ValueNotEq, "1"))
			}
		case "not_equals":
			if kind == "number" {
				want = append(want, en(message.ValueIs, "1"))
			}
		case "matches":
			if kind != "missing" && !textual {
				want = append(want, en(message.ValuePattern))
			}
		case "not_matches":
			if textual {
				want = append(want, en(message.ValueForbid))
			}
		case "key_matches":
			if kind != "missing" && !keyAlpha {
				want = append(want, en(message.KeyPattern))
			}
		case "key_not_matches":
			if kind != "missing" && keyAlpha {
				want = append(want, en(message.KeyForbid))
			}
		case "one_of":
			if kind != "missing" && !(kind == "string" || kind == "number" || kind == "bool") {
				want = append(want, en(message.ValueOneOf))
			}
		case "min_len":
			if kind != "missing" && length < 2 {
				want = append(want, en(message.LenUnder, 2))
			}
		case "max_len":
			if kind != "missing" && length > 0 {
				want = append(want, en(message.LenOver, 0))
			}
		case "forbid":
			if kind != "missing" {
				want = append(want, en(message.CondMatch))
			}
		}
		sort.Strings(want)
		sort.Strings(gotMatch)
		if strings.Join(want, "\n") != strings.Join(gotMatch, "\n") {
			t.Fatalf("%s\nwant:\n%s\ngot:\n%s\nconfig:\n%s\n%s", describe(p), strings.Join(want, "\n"), strings.Join(gotMatch, "\n"), yml, src)
		}
	})
}
