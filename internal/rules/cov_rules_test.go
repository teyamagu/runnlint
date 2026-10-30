package rules

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
	"gopkg.in/yaml.v3"
)

// CheckConfig rejects a nil config and a config whose Validate fails, such as a severity that is not one of the four.
func TestCheckConfigNilAndInvalid(t *testing.T) {
	if err := CheckConfig(nil); err == nil {
		t.Fatal("nil config is accepted")
	}
	err := CheckConfig(&config.Config{Rules: []config.Rule{{ID: "x", Uses: "duplicate-key", Severity: "fatal"}}})
	if err == nil {
		t.Fatal("invalid severity is accepted")
	}
}

// An empty file reports parse with "空のファイルです" and is not a runbook. A file whose top level is a sequence reports "runbook は mapping です".
// A steps element that is not a mapping reports parse with the problem text from the parser, and the file is still a runbook.
func TestLintWithParseFindings(t *testing.T) {
	cfg := defaultConfig(t)
	got, is := LintWith("empty.yml", []byte(""), cfg, nil)
	if is || len(got) != 1 || got[0].Rule != "parse" || got[0].Message != "空のファイルです" || got[0].Line != 1 {
		t.Fatalf("%v %+v", is, got)
	}
	got, is = LintWith("seq.yml", []byte("- a\n"), cfg, nil)
	if is || len(got) != 1 || got[0].Rule != "parse" || got[0].Message != "runbook は mapping です" {
		t.Fatalf("%v %+v", is, got)
	}
	got, is = LintWith("steps.yml", []byte("desc: d\nsteps:\n  - 1\n"), cfg, nil)
	if !is {
		t.Fatalf("%+v", got)
	}
	parse := findRule(got, "parse")
	if len(parse) != 1 || parse[0].Message != "steps の各要素は mapping です" || parse[0].Line != 3 || parse[0].File != "steps.yml" {
		t.Fatalf("%+v", got)
	}
}

// A Context with severity off records nothing. A Context without a language shows Japanese. Books is empty without a document.
func TestContextAddOffLangAndBooks(t *testing.T) {
	off := &Context{Rule: config.Rule{ID: "x", Severity: finding.Off}}
	off.Add(nil, message.FileEmpty)
	if len(off.out) != 0 {
		t.Fatalf("%+v", off.out)
	}
	ja := &Context{Rule: config.Rule{ID: "x", Severity: finding.Error}}
	ja.Add(&yaml.Node{Line: 2, Column: 3}, message.FileEmpty)
	if len(ja.out) != 1 || ja.out[0].Message != "空のファイルです" || ja.out[0].Match != "file is empty" || ja.out[0].Line != 2 || ja.out[0].File != "" {
		t.Fatalf("%+v", ja.out)
	}
	if books := (&Context{}).Books(); len(books) != 0 {
		t.Fatalf("%+v", books)
	}
}

// optionSet reads a []string value as well as a []any value.
func TestOptionSetStringSlice(t *testing.T) {
	got := optionSet(map[string]any{"ignore": []string{"a", "b"}}, "ignore")
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if _, ok := got["a"]; !ok {
		t.Fatalf("%+v", got)
	}
}

// YAML that does not parse reports parse at line 1 and is not a runbook. With language en, the built-in sentence is shown in English.
func TestLintWithYAMLErrorAndEnglish(t *testing.T) {
	cfg := defaultConfig(t)
	got, is := LintWith("broken.yml", []byte("[\n"), cfg, nil)
	if is || len(got) != 1 || got[0].Rule != "parse" || got[0].Line != 1 || got[0].Column != 1 {
		t.Fatalf("%v %+v", is, got)
	}
	cfg.Language = "en"
	got, is = LintWith("empty.yml", []byte(""), cfg, nil)
	if is || len(got) != 1 || got[0].Message != "file is empty" {
		t.Fatalf("%v %+v", is, got)
	}
}
