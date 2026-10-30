package runbook

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Lookup with an empty path returns the value itself. A path through a non-mapping or a missing key fails.
func TestCovLookup(t *testing.T) {
	doc, err := Parse([]byte("a:\n  b: 1\n  c: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := doc.Root.Lookup(""); !ok || got.Kind != KindMap {
		t.Fatalf("empty path: %+v %v", got.Kind, ok)
	}
	if got, ok := doc.Root.Lookup("a.b"); !ok || got.Number != "1" {
		t.Fatalf("a.b: %+v %v", got, ok)
	}
	if _, ok := doc.Root.Lookup("a.c.d"); ok {
		t.Fatal("a.c.d resolved through a scalar")
	}
	if _, ok := doc.Root.Lookup("a.zzz"); ok {
		t.Fatal("a.zzz resolved")
	}
}

// Len counts runes of a string, elements of a sequence, entries of a mapping, and is 0 for other kinds.
func TestCovLen(t *testing.T) {
	cases := []struct {
		v    Value
		want int
	}{
		{Value{Kind: KindString, String: "あい"}, 2},
		{Value{Kind: KindSeq, Seq: []Value{{}, {}, {}}}, 3},
		{Value{Kind: KindMap, Map: []Entry{{Key: "a"}}}, 1},
		{Value{Kind: KindNull}, 0},
		{Value{Kind: KindBool, Bool: true}, 0},
		{Value{Kind: KindNumber, Number: "12"}, 0},
	}
	for _, tc := range cases {
		if got := tc.v.Len(); got != tc.want {
			t.Fatalf("%+v: len = %d want %d", tc.v, got, tc.want)
		}
	}
}

// Display renders null, a bool, a number, a quoted string cut at 80 characters, a sequence, a mapping, and an unknown kind.
func TestCovDisplay(t *testing.T) {
	long := strings.Repeat("x", 100)
	cases := []struct {
		v    Value
		want string
	}{
		{Value{Kind: KindNull}, "null"},
		{Value{Kind: KindBool, Bool: true}, "true"},
		{Value{Kind: KindNumber, Number: "3.5"}, "3.5"},
		{Value{Kind: KindString, String: "ab"}, `"ab"`},
		{Value{Kind: KindString, String: long}, `"` + strings.Repeat("x", 77) + `..."`},
		{Value{Kind: KindSeq, Seq: []Value{{}, {}}}, "sequence(2)"},
		{Value{Kind: KindMap, Map: []Entry{{Key: "a"}}}, "mapping(1)"},
		{Value{Kind: Kind(99)}, ""},
	}
	for _, tc := range cases {
		if got := tc.v.Display(); got != tc.want {
			t.Fatalf("%+v: %q want %q", tc.v.Kind, got, tc.want)
		}
	}
}

// Pos is 1,1 for a nil node and for a node without a line, and the node's line and column otherwise.
func TestCovPos(t *testing.T) {
	if l, c := Pos(nil); l != 1 || c != 1 {
		t.Fatalf("nil: %d,%d", l, c)
	}
	if l, c := Pos(&yaml.Node{}); l != 1 || c != 1 {
		t.Fatalf("zero: %d,%d", l, c)
	}
	if l, c := Pos(&yaml.Node{Line: 4, Column: 7}); l != 4 || c != 7 {
		t.Fatalf("set: %d,%d", l, c)
	}
}
