package runnspec

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
)

func englishOf(err error) string {
	return message.Localize(message.EN, err)
}

// ParseCDPSource fails for source without the map, with an unclosed map, and with an empty map. A map whose entries are not objects is also empty.
func TestCovParseCDPSourceErrors(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"package runn\n", "CDPFnMap is missing"},
		{"var CDPFnMap = map[string]CDPFn{\n\"a\": {", "CDPFnMap is not closed"},
		{"var CDPFnMap = map[string]CDPFn{}", "CDPFnMap is empty"},
		{"var CDPFnMap = map[string]CDPFn{\n\"a\": nope,\n}", "CDPFnMap is empty"},
		{"var CDPFnMap = map[string]CDPFn{\n\"unclosed: {},\n}", "string is not closed"},
	}
	for _, tc := range cases {
		_, err := ParseCDPSource(tc.src)
		if err == nil || englishOf(err) != tc.want {
			t.Fatalf("%q: %v", tc.src, err)
		}
	}
}

// Comments, raw strings, escaped quotes, and nested braces inside the map are skipped while matching. Aliases are mapped to the same arguments as the action.
func TestCovParseCDPSourceSkipsCommentsAndStrings(t *testing.T) {
	src := "var CDPFnMap = map[string]CDPFn{\n" +
		"\t// line comment with } brace\n" +
		"\t/* block comment with } brace */\n" +
		"\t\"click\": {\n" +
		"\t\tDesc: `raw } string`,\n" +
		"\t\tArgs: CDPFnArgs{{CDPArgTypeArg, \"sel\"}, {CDPArgTypeRes, \"out\"}},\n" +
		"\t\tAliases: []string{\"tap\", \"press\"},\n" +
		"\t},\n" +
		"\t\"text\": {Desc: \"escaped \\\" quote }\", Args: CDPFnArgs{{CDPArgTypeArg, \"sel\"}, {CDPArgTypeArg, \"value\"}}},\n" +
		"}\n"
	got, err := ParseCDPSource(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("%+v", got)
	}
	for _, name := range []string{"click", "tap", "press"} {
		if args := got[name]; len(args) != 1 || args[0] != "sel" {
			t.Fatalf("%s: %+v", name, args)
		}
	}
	if args := got["text"]; len(args) != 2 || args[1] != "value" {
		t.Fatalf("text: %+v", args)
	}
}

// matchingBrace fails when the index is not at an open brace. topLevelObjects skips a name not followed by a colon, a colon not followed by a brace, and fails on an unclosed string or an unclosed nested object.
func TestCovBraceScanning(t *testing.T) {
	if _, err := matchingBrace("abc", 0); err == nil || englishOf(err) != "cannot read the start of CDPFnMap" {
		t.Fatalf("%v", err)
	}
	if _, err := matchingBrace("{ \"unterminated }", 0); err == nil || englishOf(err) != "string is not closed" {
		t.Fatalf("%v", err)
	}
	got, err := topLevelObjects("\"a\" \"b\": {x}, \"c\": 1, \"d\": {y}")
	if err != nil || len(got) != 2 || got["b"] != "{x}" || got["d"] != "{y}" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := topLevelObjects("\"open"); err == nil || englishOf(err) != "string is not closed" {
		t.Fatalf("%v", err)
	}
	if _, err := topLevelObjects("\"a\": {"); err == nil || englishOf(err) != "CDPFnMap is not closed" {
		t.Fatalf("%v", err)
	}
	if _, err := topLevelObjects("\"a\": {\"\xff\"}"); err == nil || englishOf(err) != "invalid string" {
		t.Fatalf("%v", err)
	}
}

// The scanner reads 0 at the end, reports no skip at the end, stops a block comment at the end of input, and readString fails when not at a quote.
func TestCovScannerEdges(t *testing.T) {
	s := scanner{src: "x"}
	s.next()
	if s.peek() != 0 || s.skipSpaceComments() {
		t.Fatalf("end: %d", s.i)
	}
	s.next()
	if s.i != 1 {
		t.Fatalf("next past end moved to %d", s.i)
	}
	block := scanner{src: "/* never closed"}
	if !block.skipSpaceComments() || block.i != len(block.src)-1 {
		t.Fatalf("block: %d", block.i)
	}
	closed := scanner{src: "/* c */x"}
	if !closed.skipSpaceComments() || closed.peek() != 'x' {
		t.Fatalf("closed: %d", closed.i)
	}
	slash := scanner{src: "/x"}
	if slash.skipSpaceComments() {
		t.Fatal("a lone slash was skipped")
	}
	not := scanner{src: "abc"}
	if _, err := not.readString(); err == nil || englishOf(err) != "not a string" {
		t.Fatalf("%v", err)
	}
}
