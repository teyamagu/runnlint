package runbook

import (
	"testing"
)

// StepByName fails for a name that no step has. Walk on a nil book does nothing.
func TestCovStepByNameAndNilWalk(t *testing.T) {
	doc, err := Parse([]byte("steps:\n  login:\n    test: \"true\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Book.StepByName("logout"); ok {
		t.Fatal("found a step that does not exist")
	}
	if s, ok := doc.Book.StepByName("login"); !ok || s.Name != "login" {
		t.Fatalf("%+v %v", s, ok)
	}
	var nilBook *Book
	visited := 0
	nilBook.Walk(func(*Book) { visited++ })
	if visited != 0 {
		t.Fatalf("visited %d", visited)
	}
}

// A list step that is not a mapping, a map step whose value is not a mapping, and steps that are a scalar each become a Problem with the step kept out of Steps.
func TestCovBuildBookProblems(t *testing.T) {
	cases := []struct {
		src   string
		steps int
		msg   string
	}{
		{"steps:\n  - 1\n  - test: \"true\"\n", 1, "steps の各要素は mapping です"},
		{"steps:\n  bad: 1\n  ok:\n    test: \"true\"\n", 1, "step bad の値は mapping です"},
		{"steps: 1\n", 0, "steps は sequence または mapping です"},
	}
	for _, tc := range cases {
		doc, err := Parse([]byte(tc.src))
		if err != nil {
			t.Fatal(err)
		}
		if len(doc.Book.Steps) != tc.steps || len(doc.Book.Problems) != 1 || doc.Book.Problems[0].Message != tc.msg {
			t.Fatalf("%q: steps=%d problems=%+v", tc.src, len(doc.Book.Steps), doc.Book.Problems)
		}
	}
}

// DetectKind: a string with a template is unknown, a bare string is a DB DSN, a string with an unknown scheme is unknown. A mapping with non-empty flags, a cdp:// addr, or a timeout is CDP. An empty mapping and a scalar of another kind are unknown.
func TestCovDetectKindEdges(t *testing.T) {
	str := func(s string) Value { return Value{Kind: KindString, String: s} }
	m := func(entries ...Entry) Value { return Value{Kind: KindMap, Map: entries} }
	e := func(k string, v Value) Entry { return Entry{Key: k, Value: v} }
	cases := []struct {
		v    Value
		want RunnerKind
	}{
		{str("{{ vars.dsn }}"), KindUnknown},
		{str("mydb"), KindDB},
		{str("ftp://host"), KindUnknown},
		{str("pg://host/db"), KindDB},
		{m(e("flags", m(e("headless", Value{Kind: KindBool, Bool: true})))), KindCDP},
		{m(e("addr", str("cdp://127.0.0.1:9222"))), KindCDP},
		{m(e("timeout", str("5sec"))), KindCDP},
		{m(e("flags", m())), KindUnknown},
		{m(), KindUnknown},
		{Value{Kind: KindNumber, Number: "1"}, KindUnknown},
	}
	for _, tc := range cases {
		if got := DetectKind(tc.v); got != tc.want {
			t.Fatalf("%+v: %s want %s", tc.v, got, tc.want)
		}
	}
}

// LoopEnabled is true only when the runbook has a top-level loop. RunnersAt stops collecting runner: definitions at the given step.
func TestCovLoopEnabledAndRunnersAt(t *testing.T) {
	doc, err := Parse([]byte("loop:\n  count: 2\nsteps:\n  - runner:\n      sc: ssh://u@h\n  - sc:\n      command: ls\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Book.LoopEnabled() {
		t.Fatal("loop not detected")
	}
	if _, ok := doc.Book.RunnersAt(0)["sc"]; ok {
		t.Fatal("a runner defined at step 0 is visible at step 0")
	}
	if r, ok := doc.Book.RunnersAt(1)["sc"]; !ok || r.Kind != KindSSH || r.FromStep != 0 {
		t.Fatalf("%+v %v", r, ok)
	}
	plain, err := Parse([]byte("steps:\n  - test: \"true\"\n"))
	if err != nil || plain.Book.LoopEnabled() {
		t.Fatalf("err=%v loop=%v", err, plain.Book.LoopEnabled())
	}
}
