package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Every pair of levels of the file-path factors behaves as documented for include-path, needs-path, and local-path: a path written as a
// plain relative string, with ./, or in a subdirectory is resolved from the runbook's directory and reported when the file is absent; a path
// with {{ }} or ${...}, a URL, and an empty string are not resolved; the fields are include (string, map path, include runner path), needs,
// an HTTP runner's openapi3/cacert/cert/key, a gRPC runner's cert files and proto lists, and an SSH runner's sshConfig/identityFile.
func TestFilePathPairwise(t *testing.T) {
	factors := []factor{
		{"field", []string{"include-string", "include-map", "include-runner", "needs", "http-openapi3", "http-cert", "grpc-cacert", "grpc-protos", "ssh-identityFile", "ssh-sshConfig"}},
		{"form", []string{"plain", "dot-slash", "subdir", "template", "env", "url", "empty"}},
		{"exists", []string{"present", "absent"}},
	}
	expected, unexpected := 0, 0
	runPairwise(t, factors, func(t *testing.T, p pick) {
		dir := t.TempDir()
		rel := map[string]string{"plain": "target.yml", "dot-slash": "./target.yml", "subdir": "sub/target.yml", "template": "{{ vars.name }}.yml", "env": "${RUNN_MATRIX_PATH}", "url": "https://example.com/target.yml", "empty": ""}[p["form"]]
		if p["exists"] == "present" {
			for _, name := range []string{"target.yml", "sub/target.yml"} {
				full := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte("desc: t\nsteps:\n  - test: \"true\"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		quoted := "\"" + rel + "\""
		var b strings.Builder
		b.WriteString("desc: d\nvars:\n  name: target\n")
		rule := ""
		switch p["field"] {
		case "include-string":
			rule = "include-path"
			b.WriteString("steps:\n  - include: " + quoted + " #PATH\n")
		case "include-map":
			rule = "include-path"
			b.WriteString("steps:\n  - include:\n      path: " + quoted + " #PATH\n")
		case "include-runner":
			rule = "include-path"
			b.WriteString("runners:\n  inc:\n    path: " + quoted + " #PATH\nsteps:\n  - inc:\n      vars: {}\n")
		case "needs":
			rule = "needs-path"
			b.WriteString("needs:\n  t: " + quoted + " #PATH\nsteps:\n  - exec:\n      command: echo\n")
		case "http-openapi3":
			rule = "local-path"
			b.WriteString("runners:\n  req:\n    endpoint: https://example.com\n    openapi3: " + quoted + " #PATH\nsteps:\n  - req:\n      /x:\n        get:\n          body: null\n")
		case "http-cert":
			rule = "local-path"
			b.WriteString("runners:\n  req:\n    endpoint: https://example.com\n    cert: " + quoted + " #PATH\nsteps:\n  - req:\n      /x:\n        get:\n          body: null\n")
		case "grpc-cacert":
			rule = "local-path"
			b.WriteString("runners:\n  greq:\n    addr: localhost:8080\n    cacert: " + quoted + " #PATH\nsteps:\n  - greq:\n      pkg.Svc/M:\n        message: {}\n")
		case "grpc-protos":
			rule = "local-path"
			b.WriteString("runners:\n  greq:\n    addr: localhost:8080\n    protos:\n      - " + quoted + " #PATH\nsteps:\n  - greq:\n      pkg.Svc/M:\n        message: {}\n")
		case "ssh-identityFile":
			rule = "local-path"
			b.WriteString("runners:\n  sc:\n    host: example.com\n    identityFile: " + quoted + " #PATH\nsteps:\n  - sc:\n      command: hostname\n")
		case "ssh-sshConfig":
			rule = "local-path"
			b.WriteString("runners:\n  sc:\n    host: example.com\n    sshConfig: " + quoted + " #PATH\nsteps:\n  - sc:\n      command: hostname\n")
		}
		b.WriteString("  - test: \"true\"\n")
		src := b.String()
		path := filepath.Join(dir, "book.yml")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		got, is := LintWith(path, []byte(src), defaultConfig(t), nil)
		if !is {
			t.Fatalf("%s\nnot a runbook: %+v\n%s", describe(p), got, src)
		}
		found := findRule(got, rule)
		resolvable := p["form"] == "plain" || p["form"] == "dot-slash" || p["form"] == "subdir"
		expect := resolvable && p["exists"] == "absent"
		if expect {
			expected++
		} else {
			unexpected++
		}
		if !expect {
			if len(found) != 0 {
				t.Fatalf("%s\nunexpected: %+v\n%s", describe(p), found, src)
			}
			return
		}
		line := lineOf(t, src, "#PATH")
		if len(found) != 1 || found[0].Severity != finding.Warning || found[0].Line != line || !strings.Contains(found[0].Message, strings.TrimSpace(rel)) {
			t.Fatalf("%s\nwant one warning at line %d, got %+v\n%s", describe(p), line, found, src)
		}
	})
	if expected < 10 || unexpected < 10 {
		t.Fatalf("the matrix is one-sided: %d rows expect a report, %d do not", expected, unexpected)
	}
}
