package runbook

import (
	"strings"
	"testing"
)

// ParsePath splits on dots, keeps a double-quoted segment as one literal key, and reports an unclosed quote or text after a closing quote.
func TestParsePath(t *testing.T) {
	segs, err := ParsePath(`steps.*.req."/v1/{id}.json".get.**`)
	if err != nil {
		t.Fatal(err)
	}
	want := []Seg{{Key: "steps"}, {Key: "*"}, {Key: "req"}, {Key: "/v1/{id}.json", Quoted: true}, {Key: "get"}, {Key: "**"}}
	if len(segs) != len(want) {
		t.Fatalf("%+v", segs)
	}
	for i := range want {
		if segs[i] != want[i] {
			t.Fatalf("seg %d = %+v, want %+v", i, segs[i], want[i])
		}
	}
	if segs, err := ParsePath(""); err != nil || len(segs) != 0 {
		t.Fatalf("%+v %v", segs, err)
	}
	if segs, err := ParsePath(`"*"`); err != nil || len(segs) != 1 || !segs[0].Quoted || segs[0].Key != "*" {
		t.Fatalf("%+v %v", segs, err)
	}
	if _, err := ParsePath(`a."b`); err == nil || !strings.Contains(err.Error(), "引用符") {
		t.Fatal(err)
	}
	if _, err := ParsePath(`a."b"c`); err == nil || !strings.Contains(err.Error(), "引用符") {
		t.Fatal(err)
	}
}

// Lookup follows a quoted key that contains a dot.
func TestLookupQuoted(t *testing.T) {
	doc, err := Parse([]byte("req:\n  /v1/{id}.json:\n    get: null\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Root.Lookup(`req."/v1/{id}.json".get`); !ok {
		t.Fatal("引用符の段を辿れません")
	}
	if _, ok := doc.Root.Lookup(`req./v1/{id}.json.get`); ok {
		t.Fatal("引用符無しの . を含むキーに当たっています")
	}
}
