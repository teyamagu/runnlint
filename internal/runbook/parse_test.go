package runbook

import (
	"strings"
	"testing"
)

// nil を Parse したとき、エラーにならず Empty かつ runbook ではないことを確認する。ルートが配列の YAML は解析に成功し、Book が無く runbook ではないことを確認する。
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

// a が 1 と 2 で2回ある YAML を Parse したとき、重複が a の1件として記録され、値は最初の 1 のままであることを確認する。
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

// << で x と y を展開し x を 9 で上書きした YAML を Parse したとき、x は 9、y は展開された 2 になることを確認する。
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

// document が2つの YAML を Parse したとき、エラーになりメッセージに「1つの YAML」が含まれることを確認する。
func TestParseRejectsMultipleDocuments(t *testing.T) {
	_, err := Parse([]byte("a: 1\n---\nb: 2\n"))
	if err == nil || !strings.Contains(err.Error(), "1つの YAML") {
		t.Fatal(err)
	}
}

// 自分自身を参照するエイリアスを Parse したとき、エラーになりメッセージに「循環」が含まれることを確認する。
func TestParseRejectsCyclicAlias(t *testing.T) {
	_, err := Parse([]byte("a: &a\n  b: *a\n"))
	if err == nil || !strings.Contains(err.Error(), "循環") {
		t.Fatal(err)
	}
}

// 次の各 runner 定義を Parse したとき、コメントに書いた種別になることを確認する。
func TestDetectKind(t *testing.T) {
	cases := []struct {
		src  string
		kind RunnerKind
	}{
		{"runners:\n  r: https://example.com\n", KindHTTP},                // https の文字列を HTTP と判定することを確認する。
		{"runners:\n  r: grpc://localhost:50051\n", KindGRPC},             // grpc:// の文字列を gRPC と判定することを確認する。
		{"runners:\n  r: ssh://user@example.com\n", KindSSH},              // ssh:// の文字列を SSH と判定することを確認する。
		{"runners:\n  r: cdp://127.0.0.1:9222\n", KindCDP},                // cdp:// の文字列を CDP と判定することを確認する。
		{"runners:\n  r: mysql://root@localhost/app\n", KindDB},           // mysql:// の文字列を DB と判定することを確認する。
		{"runners:\n  r: htts://example.com\n", KindUnknown},              // htts:// の文字列を unknown と判定することを確認する。
		{"runners:\n  r:\n    endpoint: https://example.com\n", KindHTTP}, // endpoint を持つ mapping を HTTP と判定することを確認する。
		{"runners:\n  r:\n    dsn: postgres://localhost/app\n", KindDB},   // dsn を持つ mapping を DB と判定することを確認する。
		{"runners:\n  r:\n    agent: claude\n", KindAgent},                // agent を持つ mapping を agent と判定することを確認する。
		{"runners:\n  r:\n    path: other.yml\n", KindInclude},            // path を持つ mapping を include と判定することを確認する。
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
