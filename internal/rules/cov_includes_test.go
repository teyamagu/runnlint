package rules

// Lines that are not covered on purpose:
// includes.go Add, ReferrerNames, InScope, and Callers return early when filepath.Abs fails. Abs fails only when the working directory cannot be read, which a test cannot arrange safely.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
)

// A nil Refs accepts Add, is never in scope, has no callers, and knows no referrer names.
func TestRefsNil(t *testing.T) {
	var r *Refs
	r.Add("x.yml", []byte("desc: d\nsteps:\n  - test: \"true\"\n"))
	if r.InScope("x.yml") {
		t.Fatal("nil refs is in scope")
	}
	if got := r.Callers("x.yml"); got != nil {
		t.Fatalf("%+v", got)
	}
	names, found, unknown := r.ReferrerNames("x.yml")
	if len(names) != 0 || found || unknown {
		t.Fatalf("%+v %v %v", names, found, unknown)
	}
}

// Add skips YAML that does not parse, a document whose top level is not a mapping, a needs value that is not a string, and an include path that is not a string.
func TestRefsAddSkipsUnusableInput(t *testing.T) {
	dir := t.TempDir()
	r := NewRefs(dir)
	r.Add(filepath.Join(dir, "broken.yml"), []byte("[\n"))
	r.Add(filepath.Join(dir, "seq.yml"), []byte("- 1\n"))
	r.Add(filepath.Join(dir, "needs.yml"), []byte("desc: d\nneeds:\n  a: 1\nsteps:\n  - test: \"true\"\n"))
	r.Add(filepath.Join(dir, "inc.yml"), []byte("desc: d\nsteps:\n  - include:\n      path: 1\n"))
	for _, name := range []string{"1", "a", "child.yml"} {
		if got := r.Callers(filepath.Join(dir, name)); len(got) != 0 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	if !r.InScope(filepath.Join(dir, "x.yml")) || r.InScope(filepath.Join(t.TempDir(), "x.yml")) {
		t.Fatal("scope is wrong")
	}
}

// resolveLocal rejects a URL, keeps an absolute path, and resolves against . when the book has no directory. dynamicPattern rejects a URL and a path without a dynamic part.
func TestResolveLocalAndDynamicPattern(t *testing.T) {
	got := lint(t, "desc: d\nsteps:\n  - include: https://example.com/x.yml\n  - test: \"true\"\n")
	if hasRule(got, "include-path") {
		t.Fatalf("%+v", got)
	}
	abs := filepath.Join(t.TempDir(), "missing.yml")
	got = lint(t, "desc: d\nsteps:\n  - include: "+abs+"\n  - test: \"true\"\n")
	if msgs := messagesOf(got, "include-path"); len(msgs) != 1 || !strings.Contains(msgs[0], abs) {
		t.Fatalf("%+v", got)
	}
	if path, ok := resolveLocal(&runbook.Book{}, "x.yml"); !ok || path != "x.yml" {
		t.Fatalf("%q %v", path, ok)
	}
	if dynamicPattern("https://example.com/{{ vars.x }}.yml") != nil {
		t.Fatal("a URL has a pattern")
	}
	if dynamicPattern("plain.yml") != nil {
		t.Fatal("a plain path has a pattern")
	}
	if re := dynamicPattern("{{ vars.d }}/child.yml"); re == nil || !re.MatchString("/a/b/child.yml") {
		t.Fatal("a dynamic path does not match by its suffix")
	}
}

// true == included also means the runbook runs only when included.
func TestRequiresIncludedReversedEquals(t *testing.T) {
	if !requiresIncluded("true == included") {
		t.Fatal("true == included is not recognized")
	}
}

// A caller with an expression that does not parse makes the referrer names unknown, so unused-bind does not judge the included runbook.
func TestUnusedBindSkipsWhenCallerIsBroken(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("parent.yml", "desc: p\nsteps:\n  - include:\n      path: child.yml\n  - test: \"1 +\"\n")
	child := write("child.yml", "desc: c\nif: included\nsteps:\n  - exec:\n      command: echo ok\n    bind:\n      orphan: current.stdout\n")
	refs := refsFor(t, []string{dir}, dir)
	_, found, unknown := refs.ReferrerNames(child)
	if !found || !unknown {
		t.Fatalf("found=%v unknown=%v", found, unknown)
	}
	if got := unusedBindFindings(t, child, refs); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}
