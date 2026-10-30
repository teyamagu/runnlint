package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
)

// Every pair of levels of the runner-definition factors behaves as documented for runner-reserved, runner-config, and runner-defined:
// a reserved name is reported wherever it is defined; a string definition is checked by the kind its prefix implies (an http(s) URL needs
// a host, a bare DSN without a scheme is reported, a URL with an unknown scheme is of unknown kind, a template is not checked); a mapping
// definition is checked by the kind its keys imply (a mapping whose endpoint is not a string has no recognizable key, so it is of unknown
// kind); a value that is neither a
// string nor a mapping is reported; a step that calls a name no definition provides is reported.
func TestRunnerDefinitionPairwise(t *testing.T) {
	factors := []factor{
		{"site", []string{"runners", "step-runner"}},
		{"name", []string{"normal", "reserved"}},
		{"value", []string{"https", "http-no-host", "bad-scheme", "template", "db-known", "db-unknown-scheme", "db-bare", "db-empty", "ssh", "grpc", "cdp", "map-http", "map-http-ep-number", "map-ssh", "map-unknown", "number", "seq"}},
		{"call", []string{"none", "matching", "unknown-name"}},
	}
	en := func(id message.ID, args ...any) string { return message.Text(message.EN, id, args...) }
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		name := "r1"
		if p["name"] == "reserved" {
			name = "test"
		}
		var def []string
		switch p["value"] {
		case "https":
			def = []string{name + ": https://example.com"}
		case "http-no-host":
			def = []string{name + ": \"http://\""}
		case "bad-scheme":
			def = []string{name + ": htts://example.com"}
		case "template":
			def = []string{name + ": \"{{ vars.endpoint }}\""}
		case "db-known":
			def = []string{name + ": mysql://root:pass@localhost:3306/app"}
		case "db-unknown-scheme":
			def = []string{name + ": oracle://x"}
		case "db-bare":
			def = []string{name + ": localhost"}
		case "db-empty":
			def = []string{name + ": \"\""}
		case "ssh":
			def = []string{name + ": ssh://alice@example.com:22"}
		case "grpc":
			def = []string{name + ": grpc://localhost:8080"}
		case "cdp":
			def = []string{name + ": cdp://127.0.0.1:9222"}
		case "map-http":
			def = []string{name + ":", "  endpoint: https://example.com"}
		case "map-http-ep-number":
			def = []string{name + ":", "  endpoint: 1"}
		case "map-ssh":
			def = []string{name + ":", "  host: example.com", "  user: alice"}
		case "map-unknown":
			def = []string{name + ":", "  foo: 1"}
		case "number":
			def = []string{name + ": 1"}
		case "seq":
			def = []string{name + ": [a]"}
		}
		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  endpoint: x\n")
		if p["site"] == "runners" {
			b.WriteString("runners:\n")
			for _, l := range def {
				b.WriteString("  " + l + "\n")
			}
		}
		b.WriteString("steps:\n")
		if p["site"] == "step-runner" {
			b.WriteString("  - runner:\n")
			for _, l := range def {
				b.WriteString("      " + l + "\n")
			}
		}
		switch p["call"] {
		case "matching":
			if p["name"] == "normal" {
				b.WriteString("  - " + name + ":\n      command: hostname\n")
			} else {
				b.WriteString("  - exec:\n      command: echo ok\n")
			}
		case "unknown-name":
			b.WriteString("  - nope:\n      command: hostname\n")
		default:
			b.WriteString("  - exec:\n      command: echo ok\n")
		}
		b.WriteString("  - test: \"true\"\n")
		src := b.String()
		got := lint(t, src)

		var want []string
		if p["name"] == "reserved" {
			want = append(want, en(message.RunnerReserved, name))
		}
		switch p["value"] {
		case "http-no-host":
			want = append(want, en(message.RunnerURL, name))
		case "bad-scheme", "db-unknown-scheme":
			want = append(want, en(message.RunnerEndpoint, name))
		case "db-bare":
			want = append(want, en(message.RunnerScheme, name))
		case "db-empty":
			want = append(want, en(message.RunnerDSNEmpty, name))
		case "map-http-ep-number", "map-unknown":
			want = append(want, en(message.RunnerKind, name))
		case "number", "seq":
			want = append(want, en(message.RunnerType, name))
		}
		if p["call"] == "unknown-name" {
			want = append(want, en(message.RunnerMissing, "nope"))
		}
		var gotMatch []string
		for _, f := range got {
			switch f.Rule {
			case "runner-reserved", "runner-config", "runner-defined":
				gotMatch = append(gotMatch, f.Match)
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
