package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/runbook"
)

// customRule builds a rules file with only the rule custom, written from the check body and an optional message.
func customRule(check, msg string) string {
	yml := "version: 1\nextends: none\nyamllint:\n  enabled: false\nrules:\n  - id: custom\n    severity: error\n"
	if msg != "" {
		yml += "    message: " + msg + "\n"
	}
	yml += "    check:\n"
	for _, line := range strings.Split(strings.TrimRight(check, "\n"), "\n") {
		yml += "      " + line + "\n"
	}
	return yml
}

// A broken regular expression in not_matches, key_not_matches, or where.matches reports custom at the runbook root with the field's name.
func TestDeclarativeBadRegexpOtherFields(t *testing.T) {
	cases := []struct {
		check string
		part  string
	}{
		{"select: desc\nnot_matches: '('\n", "not_matches の正規表現が不正です"},
		{"select: steps.*\nkey_not_matches: '('\n", "key_not_matches の正規表現が不正です"},
		{"select: steps.*\nwhere:\n  - path: test\n    matches: '('\nforbid: true\n", "where.matches の正規表現が不正です"},
	}
	for _, tc := range cases {
		got := lintConfig(t, customRule(tc.check, ""), "desc: ab\nsteps:\n  - test: \"true\"\n")
		msgs := messagesOf(got, "custom")
		if len(msgs) != 1 || !strings.Contains(msgs[0], tc.part) {
			t.Fatalf("%s: %+v", tc.part, got)
		}
	}
}

// A declarative rule without message reports the built-in sentence for the condition.
func TestDeclarativeBuiltinSentence(t *testing.T) {
	got := lintConfig(t, customRule("select: desc\nmatches: '^x'\n", ""), "desc: ab\nsteps:\n  - test: \"true\"\n")
	msgs := messagesOf(got, "custom")
	if len(msgs) != 1 || msgs[0] != "値がパターンに一致しません" {
		t.Fatalf("%+v", got)
	}
}

// where keeps only the values that satisfy every condition. exists: false drops a value whose path is present.
// A path that is missing without exists, a condition with exists but no path, an equals mismatch, and a matches mismatch all drop the value.
func TestDeclarativeWhereBranches(t *testing.T) {
	const book = "desc: ab\nsteps:\n  - test: \"true\"\n  - desc: x\n    dump: y\n"
	cases := []struct {
		name  string
		where string
		want  int
	}{
		{"exists false keeps the step without test", "  - path: test\n    exists: false\n", 1},
		{"missing path without exists drops both", "  - path: nope\n", 0},
		{"exists without path drops both", "  - exists: true\n", 0},
		{"equals keeps the matching desc", "  - path: desc\n    equals: x\n", 1},
		{"equals mismatch drops both", "  - path: desc\n    equals: z\n", 0},
		{"matches mismatch drops both", "  - path: desc\n    matches: '^z'\n", 0},
		{"matches keeps the matching desc", "  - path: desc\n    matches: '^x'\n", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintConfig(t, customRule("select: steps.*\nwhere:\n"+tc.where+"forbid: true\n", ""), book)
			msgs := messagesOf(got, "custom")
			if len(msgs) != tc.want {
				t.Fatalf("%+v", got)
			}
			for _, m := range msgs {
				if m != "条件に一致しました" {
					t.Fatalf("%+v", got)
				}
			}
		})
	}
}

// equals compares integers, floats, and other values against the selected scalar. A nil want matches only a YAML null, and an int64 want compares by its decimal text.
// equals: null written in the rules file is not exercised here: yaml.v3 leaves the *Any nil for a null node, so the condition never reaches equalValue (see the report).
func TestDeclarativeEqualsKinds(t *testing.T) {
	cases := []struct {
		name  string
		eq    string
		value string
		want  int
	}{
		{"int mismatch", "1", "2", 1},
		{"int match", "1", "1", 0},
		{"float match", "1.5", "1.5", 0},
		{"float against a string", "1.5", "a", 1},
		{"list against its text", "[1]", "\"[1]\"", 0},
		{"list against another text", "[1]", "a", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintConfig(t, customRule("select: vars.x\nequals: "+tc.eq+"\n", ""), "desc: d\nvars:\n  x: "+tc.value+"\nsteps:\n  - test: \"true\"\n")
			if len(messagesOf(got, "custom")) != tc.want {
				t.Fatalf("%+v", got)
			}
		})
	}
	if !equalValue(runbook.Value{Kind: runbook.KindNull}, nil) || equalValue(runbook.Value{Kind: runbook.KindNumber, Number: "1"}, nil) {
		t.Fatal("nil does not match only null")
	}
	if !equalValue(runbook.Value{Kind: runbook.KindNumber, Number: "7"}, int64(7)) {
		t.Fatal("int64 does not compare by text")
	}
	if equalValue(runbook.Value{Kind: runbook.KindString, String: "7"}, int64(7)) {
		t.Fatal("a string is not an int64")
	}
}

// exists: true reports the deepest existing parent: the mapping before a missing key, the value before a *, and a scalar before a further key.
func TestDeclarativeExistsDeepest(t *testing.T) {
	const book = "desc: ab\nvars:\n  a:\n    b: 1\nsteps:\n  - test: \"true\"\n"
	cases := []struct {
		sel  string
		line int
	}{
		{"vars.password", 3},
		{"vars.a.b.c", 4},
		{"steps.*.nope", 6},
		{"desc.x", 1},
		{"nope", 1},
	}
	for _, tc := range cases {
		t.Run(tc.sel, func(t *testing.T) {
			got := lintConfig(t, customRule("select: "+tc.sel+"\nexists: true\n", ""), book)
			found := findRule(got, "custom")
			if len(found) != 1 || found[0].Message != "パス "+tc.sel+" がありません" || found[0].Line != tc.line {
				t.Fatalf("%+v", got)
			}
		})
	}
}

// A message that is not a valid template, or whose template fails to run, is shown as written.
func TestDeclarativeMessageTemplateErrors(t *testing.T) {
	for _, msg := range []string{"'{{ .path'", "'{{ .path.x }}'"} {
		got := lintConfig(t, customRule("select: desc\nmatches: '^x'\n", msg), "desc: ab\nsteps:\n  - test: \"true\"\n")
		msgs := messagesOf(got, "custom")
		if len(msgs) != 1 || msgs[0] != strings.Trim(msg, "'") {
			t.Fatalf("%s: %+v", msg, got)
		}
	}
}

// selectPath with an empty select returns the root as $. A * or a key under a scalar selects nothing. runCheck does nothing without a check or a document.
func TestSelectPathEdgesAndRunCheckNoop(t *testing.T) {
	doc, err := runbook.Parse([]byte("desc: a\nsteps:\n  - test: \"true\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	root := selectPath(doc.Root, "")
	if len(root) != 1 || root[0].path != "$" {
		t.Fatalf("%+v", root)
	}
	if got := selectPath(doc.Root, "desc.*"); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	if got := selectPath(doc.Root, "desc.x"); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	noCheck := &Context{Rule: config.Rule{ID: "custom"}, Doc: doc}
	runCheck(noCheck)
	noDoc := &Context{Rule: config.Rule{ID: "custom", Check: &config.Check{Select: "desc"}}}
	runCheck(noDoc)
	if len(noCheck.out) != 0 || len(noDoc.out) != 0 {
		t.Fatalf("%+v %+v", noCheck.out, noDoc.out)
	}
}

// deepest with no segments returns the root and an empty path.
func TestDeepestNoSegments(t *testing.T) {
	doc, err := runbook.Parse([]byte("desc: a\nsteps:\n  - test: \"true\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	cur, acc := deepest(doc.Root, nil)
	if cur.Kind != runbook.KindMap || acc != "" {
		t.Fatalf("%v %q", cur.Kind, acc)
	}
}
