// Package report は診断をテキスト、JSON、GitHub Actions 形式で書く。
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Write は format に従って診断を出力する。
func Write(w io.Writer, format string, findings []finding.Finding) error {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		if findings[i].Column != findings[j].Column {
			return findings[i].Column < findings[j].Column
		}
		return findings[i].Rule < findings[j].Rule
	})
	switch format {
	case "", "text":
		for _, f := range findings {
			if _, err := fmt.Fprintf(w, "%s:%d:%d: %s: [%s] %s\n", f.File, f.Line, f.Column, f.Severity, f.Rule, f.Message); err != nil {
				return err
			}
		}
		return nil
	case "github":
		for _, f := range findings {
			level := "notice"
			switch f.Severity {
			case finding.Error:
				level = "error"
			case finding.Warning:
				level = "warning"
			}
			if _, err := fmt.Fprintf(w, "::%s file=%s,line=%d,col=%d::[%s] %s\n", level, f.File, f.Line, f.Column, f.Rule, f.Message); err != nil {
				return err
			}
		}
		return nil
	case "json":
		type item struct {
			File     string `json:"file"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
			Severity string `json:"severity"`
			Rule     string `json:"rule"`
			Message  string `json:"message"`
		}
		items := make([]item, 0, len(findings))
		for _, f := range findings {
			items = append(items, item{
				File: f.File, Line: f.Line, Column: f.Column,
				Severity: string(f.Severity), Rule: f.Rule, Message: f.Message,
			})
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	default:
		return fmt.Errorf("不明な出力形式です: %s", format)
	}
}

// HasError は error の診断が含まれるか。
func HasError(findings []finding.Finding) bool {
	for _, f := range findings {
		if f.Severity == finding.Error {
			return true
		}
	}
	return false
}
