package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
)

// Every pair of top-level field and value shape behaves as documented for book-shape and unknown-key: desc is a string; labels and secrets
// are arrays of strings; needs and hostRules are mappings of strings; runners and vars are mappings; debug, skipTest, force, trace, and if
// are a bool or a string; interval is a duration; loop is a number, an expression, or a mapping with count; concurrency is a non-empty
// string or array of non-empty strings; a key runn does not define is reported whatever its value.
func TestBookShapePairwise(t *testing.T) {
	fields := []string{"desc", "labels", "needs", "runners", "vars", "secrets", "hostRules", "debug", "skipTest", "force", "trace", "if", "interval", "loop", "concurrency", "owner"}
	values := map[string]string{
		"string": "one", "number": "1", "neg-number": "-1", "bool": "true", "null": "null", "empty-string": "\"\"", "duration": "1s", "word": "soon",
		"seq-strings": "[one]", "seq-number": "[1]", "map-strings": "{k: one}", "map-number": "{k: 1}",
	}
	var valueNames []string
	for name := range values {
		valueNames = append(valueNames, name)
	}
	sort.Strings(valueNames)
	factors := []factor{{"field", fields}, {"value", valueNames}}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		field, value := p["field"], p["value"]
		src := "desc: d\n"
		if field == "desc" {
			src = ""
		}
		src += field + ": " + values[value] + "\nsteps:\n  - test: \"true\"\n"
		got := lint(t, src)
		var gotMatch []string
		for _, f := range got {
			if f.Rule == "book-shape" || f.Rule == "unknown-key" {
				gotMatch = append(gotMatch, f.Match)
			}
		}
		isString := value == "string" || value == "empty-string" || value == "duration" || value == "word"
		isSeq := strings.HasPrefix(value, "seq-")
		isMap := strings.HasPrefix(value, "map-")
		isNumber := value == "number" || value == "neg-number"
		var want []string
		switch field {
		case "desc":
			if !isString {
				want = append(want, en(message.FieldString, "desc"))
			}
		case "labels":
			if value == "seq-number" {
				want = append(want, en(message.FieldElem, "labels"))
			} else if !isSeq {
				want = append(want, en(message.FieldStrings, "labels"))
			}
		case "secrets":
			if value == "seq-number" {
				want = append(want, en(message.SecretsElem))
			} else if !isSeq {
				want = append(want, en(message.SecretsArray))
			}
		case "needs", "hostRules":
			if value == "map-number" {
				want = append(want, en(message.FieldMapString, field, "k"))
			} else if !isMap {
				want = append(want, en(message.FieldMap, field))
			}
		case "runners", "vars":
			if !isMap {
				want = append(want, en(message.FieldMap, field))
			}
		case "debug", "skipTest", "force", "trace", "if":
			if !isString && value != "bool" {
				want = append(want, en(message.FieldBoolOrStr, field))
			}
		case "interval":
			if !(isNumber || value == "duration" || value == "string" && false) {
				if !(value == "number" || value == "neg-number" || value == "duration") {
					want = append(want, en(message.IntervalDur))
				}
			}
		case "loop":
			switch {
			case value == "neg-number":
				want = append(want, en(message.LoopCountNeg))
			case isNumber || isString:
			case isMap:
				want = append(want, en(message.LoopCountMiss), en(message.LoopUnknownKey, "k"))
			default:
				want = append(want, en(message.LoopType, "runbook"))
			}
		case "concurrency":
			switch {
			case value == "empty-string":
				want = append(want, en(message.ConcEmpty))
			case isString:
			case value == "seq-strings":
			case value == "seq-number":
				want = append(want, en(message.ConcElem))
			default:
				want = append(want, en(message.ConcType))
			}
		case "owner":
			want = append(want, en(message.UnknownBookKey, "owner"))
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
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}

// Every pair of steps value and style behaves as documented for require-steps: steps must exist and hold at least one step, as a list or a
// map; an empty list, an empty map, null, a missing key, and a scalar are reported on the steps line or the runbook when the key is absent.
func TestRequireStepsPairwise(t *testing.T) {
	factors := []factor{
		{"steps", []string{"list-one", "map-one", "list-empty", "map-empty", "null", "absent", "scalar"}},
		{"desc", []string{"present", "absent"}},
	}
	runPairwise(t, factors, func(t *testing.T, p pick) {
		var src string
		if p["desc"] == "present" {
			src = "desc: d\n"
		}
		switch p["steps"] {
		case "list-one":
			src += "steps:\n  - test: \"true\"\n"
		case "map-one":
			src += "steps:\n  a:\n    test: \"true\"\n"
		case "list-empty":
			src += "steps: []\n"
		case "map-empty":
			src += "steps: {}\n"
		case "null":
			src += "steps: null\n"
		case "absent":
			src += "runners:\n  req: https://example.com\n"
		case "scalar":
			src += "steps: 1\n"
		}
		got, is := Lint("book.yml", []byte(src), defaultConfig(t))
		if !is {
			t.Fatalf("%s\nnot a runbook: %+v", describe(p), got)
		}
		found := findRule(got, "require-steps")
		expect := p["steps"] != "list-one" && p["steps"] != "map-one"
		if !expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v", describe(p), found)
			}
			return
		}
		if len(found) != 1 || found[0].Severity != finding.Error {
			t.Fatalf("%s\nwant one error, got %+v\n%s", describe(p), found, src)
		}
		if p["steps"] != "absent" && found[0].Line != lineOf(t, src, "steps:") {
			t.Fatalf("%s\nwant the steps line, got %+v\n%s", describe(p), found, src)
		}
	})
}

// Every pair of label position and value behaves as documented for label-format: an element that is not a string, or that contains
// whitespace or one of !+=|.*%^?><, is reported on its own line; other elements are left alone; labels that are not an array are left to
// book-shape.
func TestLabelFormatPairwise(t *testing.T) {
	factors := []factor{
		{"position", []string{"only", "first-of-two", "second-of-two"}},
		{"value", []string{"plain", "with-space", "with-bang", "with-dot", "with-pipe", "number", "empty"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		label := map[string]string{"plain": "users", "with-space": "\"bad label\"", "with-bang": "\"a!b\"", "with-dot": "\"a.b\"", "with-pipe": "\"a|b\"", "number": "1", "empty": "\"\""}[p["value"]]
		var items []string
		switch p["position"] {
		case "only":
			items = []string{label}
		case "first-of-two":
			items = []string{label, "other"}
		case "second-of-two":
			items = []string{"other", label}
		}
		src := "desc: d\nlabels:\n"
		for _, it := range items {
			src += "  - " + it + "\n"
		}
		src += "steps:\n  - test: \"true\"\n"
		got := lint(t, src)
		found := findRule(got, "label-format")
		expect := p["value"] != "plain"
		if expect {
			expected++
		} else {
			unexpected++
		}
		if !expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, src)
			}
			return
		}
		line := lineOf(t, src, "  - "+label)
		if len(found) != 1 || found[0].Severity != finding.Error || found[0].Line != line {
			t.Fatalf("%s\nwant one error at line %d, got %+v\n%s", describe(p), line, found, src)
		}
	})
	if expected < 5 || unexpected < 2 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}

// Every pair of location and style behaves as documented for duplicate-key: the same key written twice in one mapping is reported wherever
// the mapping sits (top level, vars, runners, a step's fields, the step map itself) and whatever the step style.
func TestDuplicateKeyPairwise(t *testing.T) {
	factors := []factor{
		{"where", []string{"top", "vars", "runners", "step-field", "step-map", "body"}},
		{"style", []string{"list", "map"}},
	}
	runPairwise(t, factors, func(t *testing.T, p pick) {
		if p["where"] == "step-map" && p["style"] == "list" {
			t.Skip("a list has no step names to duplicate")
		}
		mapStyle := p["style"] == "map"
		var b strings.Builder
		b.WriteString("desc: d\n")
		if p["where"] == "top" {
			b.WriteString("desc: again\n")
		}
		b.WriteString("vars:\n  v: 1\n")
		if p["where"] == "vars" {
			b.WriteString("  v: 2\n")
		}
		b.WriteString("runners:\n  req: https://example.com\n")
		if p["where"] == "runners" {
			b.WriteString("  req: https://example.org\n")
		}
		b.WriteString("steps:\n")
		head, indent := "  - ", "    "
		if mapStyle {
			b.WriteString("  a:\n")
			head, indent = "    ", "    "
		}
		b.WriteString(head + "req:\n" + indent + "  /x:\n" + indent + "    post:\n" + indent + "      body:\n" + indent + "        application/json:\n" + indent + "          k: 1\n")
		if p["where"] == "body" {
			b.WriteString(indent + "          k: 2\n")
		}
		b.WriteString(indent + "test: \"true\"\n")
		if p["where"] == "step-field" {
			b.WriteString(indent + "test: \"false\"\n")
		}
		if p["where"] == "step-map" {
			b.WriteString("  a:\n    test: \"true\"\n")
		}
		src := b.String()
		got, is := Lint("book.yml", []byte(src), defaultConfig(t))
		if !is {
			t.Fatalf("%s\nnot a runbook: %+v\n%s", describe(p), got, src)
		}
		found := findRule(got, "duplicate-key")
		key := map[string]string{"top": "desc", "vars": "v", "runners": "req", "step-field": "test", "step-map": "a", "body": "k"}[p["where"]]
		if len(found) != 1 || found[0].Severity != finding.Error || !strings.Contains(found[0].Match, "key "+key+" ") {
			t.Fatalf("%s\nwant one error for %s, got %+v\n%s", describe(p), key, found, src)
		}
	})
}

// Every pair of defer value and style behaves as documented for defer-experimental: any defer key reports the info notice, whatever its value
// (a bool, a string, a template, or even a number that step-shape rejects separately), on the step.
func TestDeferExperimentalPairwise(t *testing.T) {
	factors := []factor{
		{"value", []string{"true", "false", "string", "template", "number", "absent"}},
		{"style", []string{"list", "map"}},
	}
	runPairwise(t, factors, func(t *testing.T, p pick) {
		value := map[string]string{"true": "true", "false": "false", "string": "\"true\"", "template": "\"{{ vars.d }}\"", "number": "1"}[p["value"]]
		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  d: true\nsteps:\n")
		head, indent := "  - ", "    "
		if p["style"] == "map" {
			b.WriteString("  a:\n")
			head = "    "
		}
		b.WriteString(head + "exec:\n" + indent + "  command: echo ok\n")
		if p["value"] != "absent" {
			b.WriteString(indent + "defer: " + value + "\n")
		}
		if p["style"] == "map" {
			b.WriteString("  verify:\n    test: \"true\"\n")
		} else {
			b.WriteString("  - test: \"true\"\n")
		}
		src := b.String()
		got := lint(t, src)
		found := findRule(got, "defer-experimental")
		if p["value"] == "absent" {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, src)
			}
			return
		}
		if len(found) != 1 || found[0].Severity != finding.Info {
			t.Fatalf("%s\nwant one info, got %+v\n%s", describe(p), found, src)
		}
		if (p["value"] == "number") != hasRule(got, "step-shape") {
			t.Fatalf("%s\nstep-shape presence is wrong: %+v\n%s", describe(p), got, src)
		}
	})
}

// Every pair of value shapes behaves as documented for the default declarative rules: require-desc reports only a missing desc (an empty or
// null desc is present), no-debug reports only debug: true (the string "true", false, null, and a number are not the bool), and
// loop-until-with-test reports only a step that has both loop.until and test, in either step style.
func TestDefaultDeclarativePairwise(t *testing.T) {
	factors := []factor{
		{"desc", []string{"absent", "string", "empty", "null", "number"}},
		{"debug", []string{"absent", "true", "false", "string-true", "null", "number"}},
		{"step", []string{"until-and-test", "count-and-test", "until-only", "test-only", "top-until-and-test"}},
		{"style", []string{"list", "map"}},
	}
	runPairwise(t, factors, func(t *testing.T, p pick) {
		var b strings.Builder
		switch p["desc"] {
		case "string":
			b.WriteString("desc: d\n")
		case "empty":
			b.WriteString("desc: \"\"\n")
		case "null":
			b.WriteString("desc: null\n")
		case "number":
			b.WriteString("desc: 1\n")
		}
		switch p["debug"] {
		case "true":
			b.WriteString("debug: true\n")
		case "false":
			b.WriteString("debug: false\n")
		case "string-true":
			b.WriteString("debug: \"true\"\n")
		case "null":
			b.WriteString("debug: null\n")
		case "number":
			b.WriteString("debug: 1\n")
		}
		if p["step"] == "top-until-and-test" {
			b.WriteString("loop:\n  count: 2\n  until: steps[0].exit_code == 0\n")
		}
		b.WriteString("steps:\n")
		head, indent := "  - ", "    "
		if p["style"] == "map" {
			b.WriteString("  a:\n")
			head = "    "
		}
		b.WriteString(head + "exec:\n" + indent + "  command: echo ok\n")
		switch p["step"] {
		case "until-and-test":
			b.WriteString(indent + "loop:\n" + indent + "  count: 2\n" + indent + "  until: current.exit_code == 0\n" + indent + "test: current.exit_code == 0\n")
		case "count-and-test":
			b.WriteString(indent + "loop:\n" + indent + "  count: 2\n" + indent + "test: current.exit_code == 0\n")
		case "until-only":
			b.WriteString(indent + "loop:\n" + indent + "  count: 2\n" + indent + "  until: current.exit_code == 0\n")
		case "test-only", "top-until-and-test":
			b.WriteString(indent + "test: current.exit_code == 0\n")
		}
		src := b.String()
		got, is := Lint("book.yml", []byte(src), defaultConfig(t))
		if !is {
			t.Fatalf("%s\nnot a runbook: %+v\n%s", describe(p), got, src)
		}
		if hasRule(got, "require-desc") != (p["desc"] == "absent") {
			t.Fatalf("%s\nrequire-desc presence is wrong: %+v\n%s", describe(p), got, src)
		}
		if hasRule(got, "no-debug") != (p["debug"] == "true") {
			t.Fatalf("%s\nno-debug presence is wrong: %+v\n%s", describe(p), got, src)
		}
		if hasRule(got, "loop-until-with-test") != (p["step"] == "until-and-test") {
			t.Fatalf("%s\nloop-until-with-test presence is wrong: %+v\n%s", describe(p), got, src)
		}
	})
}
