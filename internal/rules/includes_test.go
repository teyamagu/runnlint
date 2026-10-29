package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Read every .yml under dir as a caller and build a Refs whose judgment scope is scopes.
func refsFor(t *testing.T, scopes []string, sources ...string) *Refs {
	t.Helper()
	refs := NewRefs(scopes...)
	for _, dir := range sources {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".yml" {
				return err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			refs.Add(path, src)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return refs
}

// Lint each runbook under dir with refs and return the file names that reported included-unreferenced.
func unreferenced(t *testing.T, dir string, refs *Refs) []string {
	t.Helper()
	cfg := defaultConfig(t)
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yml" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, _ := LintWith(path, src, cfg, refs)
		for _, f := range got {
			if f.Rule == "included-unreferenced" {
				rel, _ := filepath.Rel(dir, path)
				out = append(out, filepath.ToSlash(rel))
				if f.Line != 3 {
					t.Errorf("%s: if の行に出ていません: %+v", rel, f)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func testdataDir(t *testing.T, parts ...string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(append([]string{moduleRoot(t), "internal", "rules", "testdata", "included"}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// Scanning static reports only if: included and included && ... that are not included.
// A string include, include.path (including ./ and ..), the target of an include runner, a runbook without if, included || ..., and !included are not reported.
// A file included from outside is still reported when outside is not in the scan.
func TestIncludedUnreferencedStatic(t *testing.T) {
	dir := testdataDir(t, "static")
	got := unreferenced(t, dir, refsFor(t, []string{dir}, dir))
	want := []string{"compound.yml", "orphan-from-outside.yml", "orphan.yml"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("got %v want %v", got, want)
	}
}

// When a caller outside the scan is also read, a file it includes is not reported.
func TestIncludedUnreferencedExtraRoot(t *testing.T) {
	dir := testdataDir(t, "static")
	outside := testdataDir(t, "outside")
	got := unreferenced(t, dir, refsFor(t, []string{dir, outside}, dir, outside))
	if len(got) != 2 || got[0] != "compound.yml" || got[1] != "orphan.yml" {
		t.Fatalf("%v", got)
	}
}

// A caller whose path contains {{ }} or ${...} cannot be resolved, so files it might match are not reported.
func TestIncludedUnreferencedDynamic(t *testing.T) {
	dirs := map[string]string{"dynamic": testdataDir(t, "dynamic"), "env": testdataDir(t, "env")}
	for name, dir := range dirs {
		if got := unreferenced(t, dir, refsFor(t, []string{dir}, dir)); len(got) != 0 {
			t.Fatalf("%s: %v", name, got)
		}
	}
}

// A file outside the judgment scope, and Lint without Refs, are not reported.
func TestIncludedUnreferencedOutOfScope(t *testing.T) {
	dir := testdataDir(t, "static")
	other := testdataDir(t, "outside")
	if got := unreferenced(t, dir, refsFor(t, []string{other}, dir, other)); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	src, err := os.ReadFile(filepath.Join(dir, "orphan.yml"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Lint(filepath.Join(dir, "orphan.yml"), src, defaultConfig(t))
	if hasRule(got, "included-unreferenced") {
		t.Fatalf("%+v", got)
	}
}

func TestRequiresIncluded(t *testing.T) {
	cases := map[string]bool{
		"included":                   true,
		" included # 説明":             true,
		"included == true":           true,
		"included && vars.x":         true,
		"vars.x and included":        true,
		"(included)":                 true,
		"included || vars.x":         false,
		"!included":                  false,
		"not included":               false,
		"included == false":          false,
		"vars.included":              false,
		"{{ included }}":             false,
		"included &&":                false,
		"${FLAG} && included":        true,
		"len(steps) > 0 && included": true,
	}
	for in, want := range cases {
		if got := requiresIncluded(in); got != want {
			t.Errorf("%q: got %v want %v", in, got, want)
		}
	}
}

func varRefMessages(t *testing.T, path string, refs *Refs) []string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := LintWith(path, src, defaultConfig(t), refs)
	return messagesOf(got, "var-ref")
}

// When a directory is scanned, a variable passed in a caller's include.vars does not report var-ref.
// When there are several callers, a variable passed by any of them is treated as defined. parent.vars is not treated as outside the parent when a caller exists.
// A variable passed by nobody, and an undefined variable in a runbook that is not included, are still reported.
func TestVarRefUsesIncludeVars(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join(moduleRoot(t), "internal", "rules", "testdata", "includevars"))
	if err != nil {
		t.Fatal(err)
	}
	refs := refsFor(t, []string{dir}, dir)
	got := varRefMessages(t, filepath.Join(dir, "child.yml"), refs)
	if len(got) != 1 || !strings.Contains(got[0], "vars.typo") {
		t.Fatalf("%v", got)
	}
	if got := varRefMessages(t, filepath.Join(dir, "solo-included.yml"), refs); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	if got := varRefMessages(t, filepath.Join(dir, "toplevel.yml"), refs); len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

// Linting a file alone, without reading callers, does not report var-ref for a runbook with if: included, and still reports it otherwise.
func TestVarRefWithoutCallers(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join(moduleRoot(t), "internal", "rules", "testdata", "includevars"))
	if err != nil {
		t.Fatal(err)
	}
	if got := varRefMessages(t, filepath.Join(dir, "solo-included.yml"), nil); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	if got := varRefMessages(t, filepath.Join(dir, "child.yml"), nil); len(got) != 4 {
		t.Fatalf("%v", got)
	}
}

// A file that is only a possible target of a dynamic include is not treated as having a caller.
// A runbook with if: included does not have its vars checked, and any other runbook treats as defined only the variables a dynamic include might pass.
func TestVarRefWithDynamicCallers(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	writeFile("parent.yml", "desc: p\nvars:\n  n: a\nsteps:\n  - include:\n      path: \"{{ vars.n }}.yml\"\n      vars:\n        passed: 1\n")
	solo := writeFile("solo.yml", "desc: s\nif: included\nsteps:\n  - dump: vars.anything\n")
	top := writeFile("top.yml", "desc: t\nsteps:\n  - dump: vars.passed + vars.missing\n  - dump: parent.vars.x\n")
	refs := refsFor(t, []string{dir}, dir)
	if got := varRefMessages(t, solo, refs); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	if got := varRefMessages(t, top, refs); len(got) != 1 || !strings.Contains(got[0], "vars.missing") {
		t.Fatalf("%v", got)
	}
}
