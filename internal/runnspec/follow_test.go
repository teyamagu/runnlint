package runnspec

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFollowsRunnNotation checks that runbook.schema.yaml and cdpfn.go fetched at the pinned tag match the imported bytes. It fails when those two files differ at the latest tag.
func TestFollowsRunnNotation(t *testing.T) {
	pin := Version()
	schemaPin := fetchText(t, rawURL(pin, "runbook.schema.yaml"))
	cdpPin := fetchText(t, rawURL(pin, "cdpfn.go"))
	if !bytes.Equal(Canon(schemaPin), Canon(SchemaYAML())) {
		t.Fatalf("取り込んだスキーマが runn %s の runbook.schema.yaml と一致しません", pin)
	}
	if !bytes.Equal(Canon(cdpPin), Canon(CDPSource())) {
		t.Fatalf("取り込んだ cdpfn.src が runn %s の cdpfn.go と一致しません", pin)
	}

	latest := fetchLatestTag(t)
	if latest == pin {
		return
	}
	var changed []string
	if !bytes.Equal(Canon(fetchText(t, rawURL(latest, "runbook.schema.yaml"))), Canon(schemaPin)) {
		changed = append(changed, "runbook.schema.yaml")
	}
	if !bytes.Equal(Canon(fetchText(t, rawURL(latest, "cdpfn.go"))), Canon(cdpPin)) {
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

// TestChangedIgnoresNewlineAndSameBytes checks that identical definitions produce no difference, and that a schema or CDP source that differs returns only that file name. A newline difference is not a difference.
func TestChangedIgnoresNewlineAndSameBytes(t *testing.T) {
	schema := []byte("a: 1\n")
	cdp := []byte("var CDPFnMap\n")
	if files := Changed(schema, cdp, []byte("a: 1\r\n"), cdp); len(files) != 0 {
		t.Fatalf("改行を差にしています: %v", files)
	}
	files := Changed(schema, cdp, []byte("a: 2\n"), []byte("var CDPFnMap\r\n"))
	if len(files) != 1 || files[0] != "runbook.schema.yaml" {
		t.Fatalf("%v", files)
	}
	files = Changed(schema, cdp, schema, []byte("other\n"))
	if len(files) != 1 || files[0] != "cdpfn.src" {
		t.Fatalf("%v", files)
	}
}

// TestWriteNotation checks that WriteNotation writes the tag and both definitions in the form used for comparison. An empty tag or an empty definition fails.
func TestWriteNotation(t *testing.T) {
	dir := t.TempDir()
	if err := WriteNotation(dir, " v9.9.9\n", []byte("a: 1\r\n"), []byte("cdp\n")); err != nil {
		t.Fatal(err)
	}
	tag, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if string(tag) != "v9.9.9\n" {
		t.Fatalf("VERSION = %q", tag)
	}
	schema, err := os.ReadFile(filepath.Join(dir, "runbook.schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(schema) != "a: 1\n" {
		t.Fatalf("schema = %q", schema)
	}
	if err := WriteNotation(dir, " ", []byte("a: 1\n"), []byte("cdp\n")); err == nil {
		t.Fatal("空のタグを受けています")
	}
	if err := WriteNotation(dir, "v1.0.0", []byte(" \n"), []byte("cdp\n")); err == nil {
		t.Fatal("空のスキーマを受けています")
	}
}
