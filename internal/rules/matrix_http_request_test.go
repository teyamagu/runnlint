package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
)

// Every pair of levels of the http-request factors behaves as documented: a request is one path with one method whose value is a mapping;
// a template string stands in for the whole request; POST and PATCH need a body with a non-null media type; a body has exactly one media type
// whose payload fits the type (text/plain is a string, form is a mapping, multipart is a mapping or an array, other types need filename
// on a mapping); headers are strings or string arrays; useCookie and trace are bools; a later check is skipped once the shape is unusable.
func TestHTTPRequestPairwise(t *testing.T) {
	factors := []factor{
		{"path", []string{"one", "two-paths", "string-template", "empty-path"}},
		{"method", []string{"get", "post", "patch", "put", "delete", "bogus", "two-methods", "non-map"}},
		{"body", []string{"absent", "null", "json-map", "json-null", "two-types", "string-template", "empty-type", "text-string", "text-map", "form-map", "form-string", "multipart-seq", "multipart-string", "custom-filename", "custom-no-filename"}},
		{"headers", []string{"absent", "map-strings", "map-seq-strings", "map-seq-number", "map-number", "string-template", "scalar"}},
		{"flags", []string{"none", "useCookie-bool", "useCookie-string", "trace-template", "trace-number"}},
	}
	en := func(id message.ID, args ...any) string { return message.Text(message.EN, id, args...) }
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		var body []string
		switch p["body"] {
		case "null":
			body = []string{"body: null"}
		case "json-map":
			body = []string{"body:", "  application/json:", "    a: 1"}
		case "json-null":
			body = []string{"body:", "  application/json: null"}
		case "two-types":
			body = []string{"body:", "  application/json:", "    a: 1", "  text/plain: x"}
		case "string-template":
			body = []string{"body: \"{{ vars.body }}\""}
		case "empty-type":
			body = []string{"body:", "  \"\": x"}
		case "text-string":
			body = []string{"body:", "  text/plain: hello"}
		case "text-map":
			body = []string{"body:", "  text/plain:", "    a: 1"}
		case "form-map":
			body = []string{"body:", "  application/x-www-form-urlencoded:", "    a: 1"}
		case "form-string":
			body = []string{"body:", "  application/x-www-form-urlencoded: a=1"}
		case "multipart-seq":
			body = []string{"body:", "  multipart/form-data:", "    - file: a.txt"}
		case "multipart-string":
			body = []string{"body:", "  multipart/form-data: x"}
		case "custom-filename":
			body = []string{"body:", "  image/png:", "    filename: a.png"}
		case "custom-no-filename":
			body = []string{"body:", "  image/png:", "    data: x"}
		}
		var headers []string
		switch p["headers"] {
		case "map-strings":
			headers = []string{"headers:", "  Authorization: token"}
		case "map-seq-strings":
			headers = []string{"headers:", "  Accept:", "    - a", "    - b"}
		case "map-seq-number":
			headers = []string{"headers:", "  Accept:", "    - 1"}
		case "map-number":
			headers = []string{"headers:", "  Accept: 1"}
		case "string-template":
			headers = []string{"headers: \"{{ vars.headers }}\""}
		case "scalar":
			headers = []string{"headers: 1"}
		}
		var flags []string
		switch p["flags"] {
		case "useCookie-bool":
			flags = []string{"useCookie: true"}
		case "useCookie-string":
			flags = []string{"useCookie: yes please"}
		case "trace-template":
			flags = []string{"trace: \"{{ vars.trace }}\""}
		case "trace-number":
			flags = []string{"trace: 1"}
		}
		methodName := p["method"]
		if methodName == "two-methods" || methodName == "non-map" {
			methodName = "get"
		}
		var inner []string
		inner = append(inner, headers...)
		inner = append(inner, body...)
		inner = append(inner, flags...)

		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  body: x\n  headers: x\n  trace: x\nrunners:\n  req: https://example.com\nsteps:\n  - req:")
		switch p["path"] {
		case "string-template":
			b.WriteString(" \"{{ vars.body }}\"\n")
		default:
			b.WriteString("\n")
			pathKey := "/items"
			if p["path"] == "empty-path" {
				pathKey = "\"\""
			}
			writeMethod := func(name string) {
				if p["method"] == "non-map" {
					b.WriteString("        " + name + ": x\n")
					return
				}
				b.WriteString("        " + name + ":\n")
				if len(inner) == 0 {
					b.WriteString("          {}\n")
				}
				for _, l := range inner {
					b.WriteString("          " + l + "\n")
				}
			}
			b.WriteString("      " + pathKey + ":\n")
			writeMethod(methodName)
			if p["method"] == "two-methods" {
				writeMethod("post")
			}
			if p["path"] == "two-paths" {
				b.WriteString("      /other:\n")
				writeMethod(methodName)
			}
		}
		b.WriteString("  - test: steps[0].res.status == 200\n")
		src := b.String()
		got := lint(t, src)
		var gotMatch []string
		for _, f := range findRule(got, "http-request") {
			gotMatch = append(gotMatch, f.Match)
		}

		var want []string
		upper := strings.ToUpper(methodName)
		needsBody := upper == "POST" || upper == "PATCH"
		switch {
		case p["path"] == "string-template":
		case p["path"] == "two-paths":
			want = append(want, en(message.HTTPOnePath))
		case p["method"] == "two-methods":
			if p["path"] == "empty-path" {
				want = append(want, en(message.HTTPPathEmpty))
			}
			want = append(want, en(message.HTTPOneMethod))
		case p["method"] == "non-map":
			if p["path"] == "empty-path" {
				want = append(want, en(message.HTTPPathEmpty))
			}
			want = append(want, en(message.HTTPMethodMap, methodName))
		default:
			if p["path"] == "empty-path" {
				want = append(want, en(message.HTTPPathEmpty))
			}
			if p["method"] == "bogus" {
				want = append(want, en(message.HTTPMethodBad, "bogus"))
			}
			switch p["headers"] {
			case "map-seq-number":
				want = append(want, en(message.HeaderElem, "Accept"))
			case "map-number":
				want = append(want, en(message.HeaderValue, "Accept"))
			case "scalar":
				want = append(want, en(message.HeadersMap))
			}
			switch p["flags"] {
			case "useCookie-string":
				want = append(want, en(message.UseCookieBool))
			case "trace-number":
				want = append(want, en(message.TraceBool))
			}
			switch p["body"] {
			case "absent", "null":
				if needsBody {
					want = append(want, en(message.BodyRequired, upper))
				}
			case "two-types":
				want = append(want, en(message.BodyOneType))
			case "empty-type":
				want = append(want, en(message.BodyTypeEmpty))
			case "json-null":
				if needsBody {
					want = append(want, en(message.BodyNull, upper))
				}
			case "text-map":
				want = append(want, en(message.BodyText))
			case "form-string":
				want = append(want, en(message.BodyForm))
			case "multipart-string":
				want = append(want, en(message.BodyMulti))
			case "custom-no-filename":
				want = append(want, en(message.BodyFilename, "image/png"))
			}
		}
		if len(want) > 0 {
			expected++
		} else {
			unexpected++
		}
		sort.Strings(want)
		sort.Strings(gotMatch)
		if strings.Join(want, "\n") != strings.Join(gotMatch, "\n") {
			t.Fatalf("%s\nwant:\n%s\ngot:\n%s\n%s", describe(p), strings.Join(want, "\n"), strings.Join(gotMatch, "\n"), src)
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
