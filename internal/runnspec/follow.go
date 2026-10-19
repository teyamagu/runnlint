package runnspec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Canon は改行の違いを除いて、記法定義のバイト列を比較できる形にする。
func Canon(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.TrimRight(b, "\n")
}

// Changed は、取り込んだ定義と最新の定義で内容が違うファイル名を返す。
// タグだけが進み、中身が同じときは空である。
func Changed(currentSchema, currentCDP, latestSchema, latestCDP []byte) []string {
	var out []string
	if !bytes.Equal(Canon(currentSchema), Canon(latestSchema)) {
		out = append(out, "runbook.schema.yaml")
	}
	if !bytes.Equal(Canon(currentCDP), Canon(latestCDP)) {
		out = append(out, "cdpfn.src")
	}
	return out
}

// WriteNotation は dir に、タグと2つの定義ファイルを書く。
func WriteNotation(dir, tag string, schema, cdp []byte) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("タグが空です")
	}
	if len(bytes.TrimSpace(schema)) == 0 || len(bytes.TrimSpace(cdp)) == 0 {
		return fmt.Errorf("記法定義が空です")
	}
	files := []struct {
		name string
		body []byte
	}{
		{"VERSION", []byte(tag + "\n")},
		{"runbook.schema.yaml", withTrailingNL(Canon(schema))},
		{"cdpfn.src", withTrailingNL(Canon(cdp))},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.name), file.body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func withTrailingNL(b []byte) []byte {
	return append(append([]byte{}, b...), '\n')
}
