package runnspec

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ピンしたタグの runbook.schema.yaml と cdpfn.go を取得したとき、取り込んだファイルとバイト列が一致することを確認する。最新タグでこの2ファイルが変わっていれば失敗することを確認する。
func TestFollowsRunnNotation(t *testing.T) {
	pin := Version()
	schemaPin := fetchText(t, rawURL(pin, "runbook.schema.yaml"))
	cdpPin := fetchText(t, rawURL(pin, "cdpfn.go"))
	if !bytes.Equal(canon(schemaPin), canon(SchemaYAML())) {
		t.Fatalf("取り込んだスキーマが runn %s の runbook.schema.yaml と一致しません", pin)
	}
	if !bytes.Equal(canon(cdpPin), canon(CDPSource())) {
		t.Fatalf("取り込んだ cdpfn.src が runn %s の cdpfn.go と一致しません", pin)
	}

	latest := fetchLatestTag(t)
	if latest == pin {
		return
	}
	var changed []string
	if !bytes.Equal(canon(fetchText(t, rawURL(latest, "runbook.schema.yaml"))), canon(schemaPin)) {
		changed = append(changed, "runbook.schema.yaml")
	}
	if !bytes.Equal(canon(fetchText(t, rawURL(latest, "cdpfn.go"))), canon(cdpPin)) {
		changed = append(changed, "cdpfn.go")
	}
	if len(changed) == 0 {
		return
	}
	t.Fatalf("runn %s で記法の定義が変わっています（%s）。いまの追従対象は %s です。internal/runnspec の VERSION、runbook.schema.yaml、cdpfn.src を %s の内容に替え、docs/runn-follow.md のパーサ上書きを見直してから go test ./... を再実行してください", latest, strings.Join(changed, ", "), pin, latest)
}

func rawURL(tag, name string) string {
	return "https://raw.githubusercontent.com/k1LoW/runn/" + tag + "/" + name
}

func fetchLatestTag(t *testing.T) string {
	t.Helper()
	body := fetchText(t, "https://api.github.com/repos/k1LoW/runn/releases/latest")
	var meta struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		t.Fatalf("runn のリリース情報を解釈できません: %v", err)
	}
	if meta.TagName == "" {
		t.Fatal("runn の最新タグが空です")
	}
	return meta.TagName
}

func fetchText(t *testing.T, url string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "runnlint-runnspec")
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s を取得できません: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s が HTTP %d を返しました", url, resp.StatusCode)
	}
	if len(body) == 0 {
		t.Fatalf("%s が空です", url)
	}
	return body
}

func canon(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.TrimRight(b, "\n")
}
