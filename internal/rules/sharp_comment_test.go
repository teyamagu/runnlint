package rules

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// A # comment in test, if, or until reports expr-sharp-comment as info on the field's line. runn warns that # comments are deprecated.
// A comment at the start of a line and a comment after the expression are both reported. The expression itself still parses.
func TestExprSharpComment(t *testing.T) {
	got := lint(t, `
desc: d
runners:
  req: https://example.com
steps:
  - req:
      /a:
        get:
          body: null
    test: |
      # look at the status
      current.res.status == 200
  - dump: x
    if: 'true # always'
  - req:
      /b:
        get:
          body: null
    loop:
      count: 3
      until: 'current.res.status == 200 # ready'
  - test: steps[0].res.status == 200
`)
	lines := map[int]bool{}
	for _, f := range findRule(got, "expr-sharp-comment") {
		if f.Severity != finding.Info {
			t.Fatalf("%+v", f)
		}
		lines[f.Line] = true
	}
	if len(lines) != 3 || !lines[10] || !lines[14] || !lines[21] {
		t.Fatalf("%+v", got)
	}
	if errs := errorsOf(got); len(errs) > 0 {
		t.Fatalf("%+v", errs)
	}
}

// A # inside a string literal, the closure shorthand # inside ( ) or { }, and a // comment are not # comments, so nothing is reported.
func TestExprSharpCommentNotReported(t *testing.T) {
	got := lint(t, `
desc: d
steps:
  - exec:
      command: echo ok
    test: |
      current.stdout == "#tag"
      && all([1, 2], # > 0) // every one positive
      && len(filter([1, 2], {# > 1})) == 1
`)
	if hasRule(got, "expr-sharp-comment") || hasRule(got, "expr-syntax") {
		t.Fatalf("%+v", got)
	}
}
