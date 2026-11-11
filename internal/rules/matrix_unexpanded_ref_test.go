package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
)

// unexpandedValue writes a reference to root in the chosen form. ref is the reference unexpanded-ref names, or "" when none is expected.
func unexpandedValue(root, form string) (value, ref string) {
	switch form {
	case "braced":
		return "{{ " + root + ".a }}", ""
	case "bare":
		return root + ".a", root + ".a"
	case "after-template":
		return "{{ vars.x }}/" + root + ".a", root + ".a"
	case "env":
		return root + ".a.${ENV}.b", root + ".a.${ENV}.b"
	case "index":
		if root == "steps" {
			return "steps[0].a", "steps[0].a"
		}
		return root + `["a"].b`, root + `["a"].b`
	case "file-name":
		return root + ".yml", ""
	default: // dotted: a host or jq path, where the root follows a dot
		return "api." + root + ".a", ""
	}
}

// Every pair of levels of the unexpanded-ref factors behaves as documented: a reference outside {{ }} in include.vars, a request
// body, a path key, a header, exec.command, dump.out, or root vars reports warning on that line, naming the reference; the same text in
// test, if, bind, a dump expression, loop.until, or desc is not reported; a reference inside {{ }}, a file name, and a dotted host are not reported.
func TestUnexpandedRefPairwise(t *testing.T) {
	factors := []factor{
		{"field", []string{"include-vars", "body", "path-key", "header", "exec-command", "dump-out", "root-vars", "test", "if", "bind", "dump-expr", "loop-until", "desc"}},
		{"form", []string{"bare", "braced", "after-template", "env", "index", "file-name", "dotted"}},
		{"root", []string{"vars", "steps", "current", "previous", "parent", "needs"}},
	}
	template := map[string]bool{"include-vars": true, "body": true, "path-key": true, "header": true, "exec-command": true, "dump-out": true, "root-vars": true}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		value, ref := unexpandedValue(p["root"], p["form"])
		v := "'" + value + "' # MARK"
		var b strings.Builder
		if p["field"] == "desc" {
			b.WriteString("desc: " + v + "\n")
		} else {
			b.WriteString("desc: d\n")
		}
		b.WriteString("vars:\n  x: 1\n")
		if p["field"] == "root-vars" {
			b.WriteString("  y: " + v + "\n")
		}
		b.WriteString("runners:\n  req: https://example.com\nsteps:\n  - req:\n      /a:\n        get:\n          body: null\n")
		switch p["field"] {
		case "include-vars":
			b.WriteString("  - include:\n      path: child.yml\n      vars:\n        y: " + v + "\n")
		case "body":
			b.WriteString("  - req:\n      /b:\n        post:\n          body:\n            application/json:\n              y: " + v + "\n")
		case "path-key":
			b.WriteString("  - req:\n      '/x/" + value + "': # MARK\n        get:\n          body: null\n")
		case "header":
			b.WriteString("  - req:\n      /c:\n        get:\n          headers:\n            X-Y: " + v + "\n          body: null\n")
		case "exec-command":
			b.WriteString("  - exec:\n      command: " + v + "\n")
		case "dump-out":
			b.WriteString("  - dump:\n      expr: steps[0].res\n      out: " + v + "\n")
		case "test":
			b.WriteString("  - test: " + v + "\n")
		case "if":
			b.WriteString("  - dump: steps[0].res\n    if: " + v + "\n")
		case "bind":
			b.WriteString("  - dump: steps[0].res\n    bind:\n      y: " + v + "\n")
		case "dump-expr":
			b.WriteString("  - dump:\n      expr: " + v + "\n")
		case "loop-until":
			b.WriteString("  - dump: steps[0].res\n    loop:\n      count: 2\n      until: " + v + "\n")
		default: // root-vars and desc are outside steps
			b.WriteString("  - dump: steps[0].res\n")
		}
		b.WriteString("  - test: 'true'\n")
		src := b.String()
		got := lint(t, src)
		found := findRule(got, "unexpanded-ref")
		if !template[p["field"]] {
			ref = ""
		}
		if ref == "" {
			unexpected++
			if len(found) != 0 {
				t.Fatalf("%s\nwant no finding, got %+v\n%s", describe(p), found, src)
			}
			return
		}
		expected++
		if len(found) != 1 {
			t.Fatalf("%s\nwant 1 finding, got %+v\n%s", describe(p), found, src)
		}
		f := found[0]
		if f.Severity != finding.Warning || f.Line != lineOf(t, src, "MARK") || f.Match != message.English(message.RefUnexpanded, ref) {
			t.Fatalf("%s\nwant %q on line %d, got %+v\n%s", describe(p), ref, lineOf(t, src, "MARK"), f, src)
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
