package runnspec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// DefKeys of an unknown definition is an empty set, and Required of an unknown definition is nil.
func TestCovUnknownDef(t *testing.T) {
	if got := DefKeys("noSuchDef"); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	if got := Required("noSuchDef"); got != nil {
		t.Fatalf("%+v", got)
	}
	if got := Required(DefExec); len(got) != 1 || got[0] != "command" {
		t.Fatalf("%+v", got)
	}
}

// parseSchema rejects YAML that does not parse, a schema without properties, and a schema without $defs. Empty input has no properties.
func TestCovParseSchemaErrors(t *testing.T) {
	if _, err := parseSchema([]byte("a: [\n")); err == nil || !strings.HasPrefix(err.Error(), "yaml: line 1") {
		t.Fatalf("%v", err)
	}
	cases := []struct {
		raw  string
		want string
	}{
		{"type: object\n", "schema has no properties"},
		{"", "schema has no properties"},
		{"properties:\n  desc: {}\n", "schema has no $defs"},
	}
	for _, tc := range cases {
		_, err := parseSchema([]byte(tc.raw))
		if err == nil || englishOf(err) != tc.want {
			t.Fatalf("%q: %v", tc.raw, err)
		}
	}
}

// A schema with patternProperties, a labels pattern, and $defs with oneOf branches yields prefixes, the label pattern, and the union of branch keys. A branch that is not a mapping adds nothing.
func TestCovParseSchemaShapes(t *testing.T) {
	raw := `
properties:
  desc: {type: string}
  labels:
    items:
      pattern: "^[a-z]+$"
patternProperties:
  "^x-": {}
  "^.*": {}
  "noanchor": {}
  "^": {}
$defs:
  step:
    properties:
      desc: {}
    required: [desc, ""]
    oneOf:
      - properties:
          exec: {}
      - 1
  loopValue: 1
`
	idx, err := parseSchema([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.prefixes) != 1 || idx.prefixes[0] != "x-" {
		t.Fatalf("prefixes = %+v", idx.prefixes)
	}
	if idx.label != "^[a-z]+$" {
		t.Fatalf("label = %s", idx.label)
	}
	step := idx.defs["step"]
	if _, ok := step.keys["exec"]; !ok || len(step.keys) != 2 || len(step.required) != 1 {
		t.Fatalf("%+v", step)
	}
	if loop := idx.defs["loopValue"]; len(loop.keys) != 0 || loop.required != nil {
		t.Fatalf("%+v", loop)
	}
}

// literalPrefix accepts only a pattern that starts with ^ and has a literal before any regex metacharacter.
func TestCovLiteralPrefix(t *testing.T) {
	cases := []struct {
		pattern string
		want    string
		ok      bool
	}{
		{"^x-", "x-", true},
		{"^x-.*", "x-", true},
		{"x-", "", false},
		{"^.*", "", false},
		{"^", "", false},
	}
	for _, tc := range cases {
		got, ok := literalPrefix(tc.pattern)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("%q: %q %v", tc.pattern, got, ok)
		}
	}
}

// mapValue, mapScalar, and mapKeyList return nothing for a nil node, a non-mapping node, and a missing key.
func TestCovMapHelpers(t *testing.T) {
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "x"}
	if mapValue(nil, "k") != nil || mapValue(scalar, "k") != nil {
		t.Fatal("mapValue of a non-mapping is not nil")
	}
	if mapScalar(scalar, "k") != "" {
		t.Fatal("mapScalar of a non-mapping is not empty")
	}
	if mapKeyList(nil) != nil || mapKeyList(scalar) != nil {
		t.Fatal("mapKeyList of a non-mapping is not nil")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("a: 1\nb: two\n"), &doc); err != nil {
		t.Fatal(err)
	}
	root := doc.Content[0]
	if mapScalar(root, "b") != "two" || mapScalar(root, "zzz") != "" {
		t.Fatal("mapScalar")
	}
	if keys := mapKeyList(root); len(keys) != 2 || keys[0] != "a" {
		t.Fatalf("%+v", keys)
	}
}

// WriteNotation fails when the directory does not exist, and rejects an empty tag and empty definitions.
func TestCovWriteNotationErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no", "such", "dir")
	err := WriteNotation(missing, "v1.0.0", []byte("schema"), []byte("cdp"))
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("%v", err)
	}
	if err := WriteNotation(t.TempDir(), " ", []byte("schema"), []byte("cdp")); err == nil || englishOf(err) != "tag is empty" {
		t.Fatalf("%v", err)
	}
	if err := WriteNotation(t.TempDir(), "v1", []byte(" "), []byte("cdp")); err == nil || englishOf(err) != "the syntax definition is empty" {
		t.Fatalf("%v", err)
	}
}
