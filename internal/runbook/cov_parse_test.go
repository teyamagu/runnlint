package runbook

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
	"gopkg.in/yaml.v3"
)

func englishOf(err error) string {
	return message.Localize(message.EN, err)
}

// YAML that does not parse fails in the first document and in a second document. A second well-formed document is rejected as more than one document.
func TestCovParseDecodeErrors(t *testing.T) {
	if _, err := Parse([]byte("a: [\n")); err == nil || !strings.Contains(err.Error(), "yaml: line 1") {
		t.Fatalf("%v", err)
	}
	if _, err := Parse([]byte("a: 1\n---\nb: [\n")); err == nil || !strings.Contains(err.Error(), "yaml: line 3") {
		t.Fatalf("%v", err)
	}
	if _, err := Parse([]byte("a: 1\n---\nb: 2\n")); err == nil || englishOf(err) != "a runbook is one YAML document" {
		t.Fatalf("%v", err)
	}
}

// A document marker alone parses as a null root, which is not a runbook. A comment-only file is empty.
func TestCovParseBareDocument(t *testing.T) {
	doc, err := Parse([]byte("---\n"))
	if err != nil || doc.Empty || doc.Root.Kind != KindNull || doc.IsRunbook() {
		t.Fatalf("err=%v %+v", err, doc)
	}
	doc, err = Parse([]byte("# only a comment\n"))
	if err != nil || !doc.Empty {
		t.Fatalf("err=%v %+v", err, doc)
	}
}

// Mapping keys that are not scalars, or that are null, fail with the line number, also inside a sequence and inside a nested mapping. An alias used as a key resolves to the anchored scalar.
func TestCovParseKeys(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"? [a]\n: 1\n", "a mapping key is a scalar (line 1)"},
		{"~: 1\n", "a mapping key is null (line 1)"},
		{"- {~: 1}\n", "a mapping key is null (line 1)"},
		{"a:\n  {~: 1}\n", "a mapping key is null (line 2)"},
	}
	for _, tc := range cases {
		_, err := Parse([]byte(tc.src))
		if err == nil || englishOf(err) != tc.want {
			t.Fatalf("%q: %v", tc.src, err)
		}
	}
	doc, err := Parse([]byte("k: &a name\n*a : 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := doc.Root.Get("name"); !ok || v.Number != "1" {
		t.Fatalf("%+v", doc.Root)
	}
}

// A merge key with a sequence of aliases takes keys from each mapping in order, the first mapping wins for a repeated key, and an explicit key beats merged ones. A merge value that is not a mapping or sequence adds nothing.
func TestCovParseMergeSequence(t *testing.T) {
	src := "x: &a {k: 1, only_a: a}\ny: &b {k: 2, only_b: b}\nz:\n  <<: [*a, *b]\n  own: 3\nw:\n  <<: 1\n  q: 4\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	z, _ := doc.Root.Get("z")
	if k, _ := z.Get("k"); k.Number != "1" {
		t.Fatalf("k = %s", k.Number)
	}
	if _, ok := z.Get("only_b"); !ok {
		t.Fatal("only_b missing")
	}
	if own, _ := z.Get("own"); own.Number != "3" || len(z.Map) != 4 {
		t.Fatalf("%+v", z.Map)
	}
	w, _ := doc.Root.Get("w")
	if len(w.Map) != 1 || w.Map[0].Key != "q" {
		t.Fatalf("%+v", w.Map)
	}
	explicit, err := Parse([]byte("x: &a {k: 1}\nz:\n  k: 9\n  <<: *a\n"))
	if err != nil {
		t.Fatal(err)
	}
	zz, _ := explicit.Root.Get("z")
	if k, _ := zz.Get("k"); k.Number != "9" || len(zz.Map) != 1 {
		t.Fatalf("%+v", zz.Map)
	}
}

// Nesting deeper than 64 levels is rejected as too deep.
func TestCovParseTooDeep(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 70; i++ {
		b.WriteString(strings.Repeat("  ", i) + "a:\n")
	}
	b.WriteString(strings.Repeat("  ", 70) + "b: 1\n")
	_, err := Parse([]byte(b.String()))
	if err == nil || englishOf(err) != "YAML alias nesting is too deep" {
		t.Fatalf("%v", err)
	}
}

// decode handles a nil node as null, an alias whose target is unknown as an error, a document node with and without content, and a node of an unsupported kind as an error.
func TestCovDecodeNodes(t *testing.T) {
	seen := map[*yaml.Node]struct{}{}
	v, _, err := decode(nil, seen, 0)
	if err != nil || v.Kind != KindNull {
		t.Fatalf("nil: %+v %v", v, err)
	}
	if _, _, err := decode(&yaml.Node{Kind: yaml.AliasNode}, seen, 0); err == nil || englishOf(err) != "an alias cannot be resolved" {
		t.Fatalf("alias: %v", err)
	}
	empty := &yaml.Node{Kind: yaml.DocumentNode}
	if v, _, err := decode(empty, seen, 0); err != nil || v.Kind != KindNull || v.Node != empty {
		t.Fatalf("empty document: %+v %v", v, err)
	}
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "5"}
	if v, _, err := decode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{scalar}}, seen, 0); err != nil || v.Kind != KindNumber || v.Number != "5" {
		t.Fatalf("document: %+v %v", v, err)
	}
	if _, _, err := decode(&yaml.Node{}, seen, 0); err == nil || englishOf(err) != "unsupported YAML node" {
		t.Fatalf("zero kind: %v", err)
	}
}

// decodeScalar treats a node without a tag as a string even when its value is empty or null-like, and a !!null tag as null.
func TestCovDecodeScalarWithoutTag(t *testing.T) {
	for _, value := range []string{"", "null", "~", "text"} {
		v := decodeScalar(&yaml.Node{Kind: yaml.ScalarNode, Value: value})
		if v.Kind != KindString || v.String != value {
			t.Fatalf("%q: %+v", value, v)
		}
	}
	if v := decodeScalar(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "~"}); v.Kind != KindNull {
		t.Fatalf("%+v", v)
	}
}

// mergeMaps flattens a sequence of mappings and returns nothing for a scalar.
func TestCovMergeMaps(t *testing.T) {
	m := Value{Kind: KindMap, Map: []Entry{{Key: "a"}}}
	seq := Value{Kind: KindSeq, Seq: []Value{m, {Kind: KindSeq, Seq: []Value{m}}, {Kind: KindString}}}
	if got := mergeMaps(seq); len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got := mergeMaps(Value{Kind: KindNumber}); got != nil {
		t.Fatalf("%+v", got)
	}
}
