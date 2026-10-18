package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

func sample() []finding.Finding {
	return []finding.Finding{
		{File: "b.yml", Line: 2, Column: 1, Severity: finding.Warning, Rule: "var-ref", Message: "未定義"},
		{File: "a.yml", Line: 4, Column: 3, Severity: finding.Error, Rule: "http-request", Message: "body がありません"},
		{File: "a.yml", Line: 4, Column: 3, Severity: finding.Info, Rule: "defer-experimental", Message: "実験的"},
	}
}

// 位置の違う診断を text で書き出したとき、ファイル・行・列の順に並ぶことを確認する。同じ位置ではルール名の辞書順になり、defer-experimental が http-request より前に出ることを確認する。
func TestWriteTextSortsAndFormats(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "text", sample()); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := strings.Join([]string{
		"a.yml:4:3: info: [defer-experimental] 実験的",
		"a.yml:4:3: error: [http-request] body がありません",
		"b.yml:2:1: warning: [var-ref] 未定義",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

// github 形式で書き出したとき、error が ::error、warning が ::warning、info が ::notice になることを確認する。
func TestWriteGitHubLevels(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "github", sample()); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, line := range []string{
		"::error file=a.yml,line=4,col=3::[http-request] body がありません",
		"::notice file=a.yml,line=4,col=3::[defer-experimental] 実験的",
		"::warning file=b.yml,line=2,col=1::[var-ref] 未定義",
	} {
		if !strings.Contains(got, line) {
			t.Fatalf("missing %s\n%s", line, got)
		}
	}
}

// 形式を空文字にしたとき text として書き出されることを確認する。json にしたとき、診断が配列になり rule、severity、line が残ることを確認する。
func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "", sample()[:1]); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "warning: [var-ref]") {
		t.Fatalf("empty format = %s", buf.String())
	}

	buf.Reset()
	if err := Write(&buf, "json", sample()[:1]); err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["rule"] != "var-ref" || items[0]["severity"] != "warning" || items[0]["line"].(float64) != 2 {
		t.Fatalf("%s", buf.String())
	}
}

// 形式に xml を渡したとき、書き出さずエラーメッセージに xml が含まれることを確認する。
func TestWriteRejectsUnknownFormat(t *testing.T) {
	err := Write(&bytes.Buffer{}, "xml", nil)
	if err == nil || !strings.Contains(err.Error(), "xml") {
		t.Fatalf("%v", err)
	}
}

// 診断が無いときと warning だけのとき HasError が false になり、error を含むときは true になることを確認する。
func TestHasError(t *testing.T) {
	if HasError(nil) || HasError(sample()[:1]) {
		t.Fatal("warning を error と扱っています")
	}
	if !HasError(sample()) {
		t.Fatal("error を見落としています")
	}
}
