package rules

import (
	"strings"
	"testing"
)

// Every pair of levels of the key-template factors behaves as documented: a {{ }} inside a mapping key at any depth (an HTTP path, a body
// field name, a header name, an include.vars key, a runner definition key) is read as an expression, so a bind referenced only there is used,
// an undefined vars.* there reports var-ref, a reference to a missing step reports step-ref, and a broken template reports expr-syntax, all on
// the key's line; a key without {{ }} and a ${...} placeholder report nothing. A broken template also leaves unused-bind unjudged.
func TestKeyTemplatePairwise(t *testing.T) {
	factors := []factor{
		{"depth", []string{"req-path", "body-field", "header-name", "include-vars-key", "runner-key"}},
		{"content", []string{"bind-ref", "vars-missing", "steps-missing", "syntax-error", "no-template", "env-placeholder"}},
		{"steps", []string{"list", "map"}},
	}
	runPairwise(t, factors, func(t *testing.T, p pick) {
		var key string
		switch p["content"] {
		case "bind-ref":
			key = "{{ token }}"
		case "vars-missing":
			key = "{{ vars.missing }}"
		case "steps-missing":
			key = "{{ steps.nope.res.status }}"
		case "syntax-error":
			key = "{{ 1 + }}"
		case "no-template":
			key = "plain"
		case "env-placeholder":
			key = "${RUNN_MATRIX_KEY}"
		}
		mapStyle := p["steps"] == "map"
		var b strings.Builder
		b.WriteString("desc: d\nrunners:\n")
		if p["depth"] == "runner-key" {
			b.WriteString("  \"req-" + key + "\": https://example.com\n")
		} else {
			b.WriteString("  req: https://example.com\n")
		}
		b.WriteString("steps:\n")
		n := 0
		writeStep := func(lines []string) {
			n++
			if mapStyle {
				b.WriteString("  s" + strings.Repeat("x", n) + ":\n")
				for _, l := range lines {
					b.WriteString("    " + l + "\n")
				}
				return
			}
			for i, l := range lines {
				if i == 0 {
					b.WriteString("  - " + l + "\n")
				} else {
					b.WriteString("    " + l + "\n")
				}
			}
		}
		writeStep([]string{"exec:", "  command: echo 11", "bind:", "  token: current.stdout"})
		marker := "KEYLINE"
		switch p["depth"] {
		case "req-path":
			writeStep([]string{"req:", "  \"/documents?filters=" + key + "\": #" + marker, "    get:", "      body: null"})
		case "body-field":
			writeStep([]string{"req:", "  /documents:", "    post:", "      body:", "        application/json:", "          \"" + key + "\": 1 #" + marker})
		case "header-name":
			writeStep([]string{"req:", "  /documents:", "    get:", "      headers:", "        \"X-" + key + "\": v #" + marker, "      body: null"})
		case "include-vars-key":
			writeStep([]string{"include:", "  path: ../valid/http.yml", "  vars:", "    \"" + key + "\": 1 #" + marker})
		case "runner-key":
			if mapStyle {
				writeStep([]string{"dump: steps.sx.stdout"})
			} else {
				writeStep([]string{"dump: steps[0].stdout"})
			}
		}
		if mapStyle {
			writeStep([]string{"test: steps.sx.exit_code == 0"})
		} else {
			writeStep([]string{"test: steps[0].exit_code == 0"})
		}
		src := b.String()
		if p["depth"] == "runner-key" {
			src = strings.Replace(src, "\"req-"+key+"\": https://example.com", "\"req-"+key+"\": https://example.com #"+marker, 1)
		}
		got := lint(t, src)
		line := lineOf(t, src, "#"+marker)
		// The bind is used only when the key references it. A template that does not parse leaves the file unjudged by unused-bind.
		wantUnused := p["content"] != "bind-ref" && p["content"] != "syntax-error"
		if hasRule(got, "unused-bind") != wantUnused {
			t.Fatalf("%s\nunused-bind presence should be %v: %+v\n%s", describe(p), wantUnused, got, src)
		}
		check := func(rule string, want bool) {
			t.Helper()
			found := findRule(got, rule)
			if !want {
				if len(found) != 0 {
					t.Fatalf("%s\n%s unexpected: %+v\n%s", describe(p), rule, found, src)
				}
				return
			}
			if len(found) != 1 || found[0].Line != line {
				t.Fatalf("%s\n%s wanted at line %d, got %+v\n%s", describe(p), rule, line, found, src)
			}
		}
		check("var-ref", p["content"] == "vars-missing")
		check("step-ref", p["content"] == "steps-missing")
		check("expr-syntax", p["content"] == "syntax-error")
	})
}
