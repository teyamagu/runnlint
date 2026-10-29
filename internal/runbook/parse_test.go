package runbook

import (
	"strings"
	"testing"
)

// TestParseEmptyAndNonMap checks that parsing nil returns no error, with Empty set and the document not a runbook. YAML whose root is an array parses, has no Book, and is not a runbook.
func TestParseEmptyAndNonMap(t *testing.T) {
	doc, err := Parse(nil)
	if err != nil || doc == nil || !doc.Empty || doc.IsRunbook() {
		t.Fatalf("err=%v doc=%+v", err, doc)
	}
	doc, err = Parse([]byte("- 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Root.Kind != KindSeq || doc.Book != nil || doc.IsRunbook() {
		t.Fatalf("%+v", doc.Root.Kind)
	}
}

// TestParseDuplicateKeyKeepsFirst checks that YAML in which a is written twice, as 1 and as 2, records one duplicate for a and keeps the first value, 1.
func TestParseDuplicateKeyKeepsFirst(t *testing.T) {
	doc, err := Parse([]byte("a: 1\na: 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Dups) != 1 || doc.Dups[0].Key != "a" {
		t.Fatalf("%+v", doc.Dups)
	}
	got, ok := doc.Root.Get("a")
	if !ok || got.Number != "1" {
		t.Fatalf("%+v", got)
	}
}

// TestParseAliasAndMerge checks that expanding x and y with << and overriding x as 9 yields an x of 9 and the expanded y of 2.
func TestParseAliasAndMerge(t *testing.T) {
	doc, err := Parse([]byte("base: &base\n  x: 1\n  y: 2\nout:\n  <<: *base\n  x: 9\n"))
	if err != nil {
		t.Fatal(err)
	}
	out, ok := doc.Root.Get("out")
	if !ok {
		t.Fatal("out がありません")
	}
	x, ok := out.Get("x")
	y, yok := out.Get("y")
	if !ok || !yok || x.Number != "9" || y.Number != "2" {
		t.Fatalf("x=%+v y=%+v", x, y)
	}
}

// TestParseRejectsMultipleDocuments checks that YAML with two documents returns an error saying the runbook is one YAML document.
func TestParseRejectsMultipleDocuments(t *testing.T) {
	_, err := Parse([]byte("a: 1\n---\nb: 2\n"))
	if err == nil || !strings.Contains(err.Error(), "1つの YAML") {
		t.Fatal(err)
	}
}

// TestParseRejectsCyclicAlias checks that a self-referential alias returns an error saying the anchor is cyclic.
func TestParseRejectsCyclicAlias(t *testing.T) {
	_, err := Parse([]byte("a: &a\n  b: *a\n"))
	if err == nil || !strings.Contains(err.Error(), "循環") {
		t.Fatal(err)
	}
}

// TestDetectKind checks that parsing each runner definition below yields the kind named in its comment.
func TestDetectKind(t *testing.T) {
	cases := []struct {
		src  string
		kind RunnerKind
	}{
		{"runners:\n  r: https://example.com\n", KindHTTP},                // An https string is classified as HTTP.
		{"runners:\n  r: grpc://localhost:50051\n", KindGRPC},             // A grpc:// string is classified as gRPC.
		{"runners:\n  r: ssh://user@example.com\n", KindSSH},              // An ssh:// string is classified as SSH.
		{"runners:\n  r: cdp://127.0.0.1:9222\n", KindCDP},                // A cdp:// string is classified as CDP.
		{"runners:\n  r: mysql://root@localhost/app\n", KindDB},           // A mysql:// string is classified as DB.
		{"runners:\n  r: htts://example.com\n", KindUnknown},              // An htts:// string is classified as unknown.
		{"runners:\n  r:\n    endpoint: https://example.com\n", KindHTTP}, // A mapping with endpoint is classified as HTTP.
		{"runners:\n  r:\n    dsn: postgres://localhost/app\n", KindDB},   // A mapping with dsn is classified as DB.
		{"runners:\n  r:\n    agent: claude\n", KindAgent},                // A mapping with agent is classified as agent.
		{"runners:\n  r:\n    path: other.yml\n", KindInclude},            // A mapping with path is classified as include.
	}
	for _, tc := range cases {
		doc, err := Parse([]byte(tc.src))
		if err != nil {
			t.Fatal(err)
		}
		if len(doc.Book.Runners) != 1 || doc.Book.Runners[0].Kind != tc.kind {
			t.Fatalf("%s: %+v", tc.src, doc.Book.Runners)
		}
	}
}
