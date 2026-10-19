package rules

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// dir 以下の .yml をすべて参照元として読み、scopes を判定範囲にした Refs を作る。
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

// dir 以下の各 runbook を refs 付きで lint し、included-unreferenced が出たファイル名を返す。
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

// static を走査したとき、include されていない if: included と included && ... だけが指摘されることを確認する。
// 文字列の include、include.path（./ と .. を含む）、include runner の参照先、if の無い runbook、included || ...、!included は指摘しない。
// outside から include されるファイルも、outside が走査範囲に入っていなければ指摘される。
func TestIncludedUnreferencedStatic(t *testing.T) {
	dir := testdataDir(t, "static")
	got := unreferenced(t, dir, refsFor(t, []string{dir}, dir))
	want := []string{"compound.yml", "orphan-from-outside.yml", "orphan.yml"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("got %v want %v", got, want)
	}
}

// 走査範囲の外の参照元も読ませると、そこから include されるファイルは指摘されないことを確認する。
func TestIncludedUnreferencedExtraRoot(t *testing.T) {
	dir := testdataDir(t, "static")
	outside := testdataDir(t, "outside")
	got := unreferenced(t, dir, refsFor(t, []string{dir, outside}, dir, outside))
	if len(got) != 2 || got[0] != "compound.yml" || got[1] != "orphan.yml" {
		t.Fatalf("%v", got)
	}
}

// {{ }} や ${...} を含む参照元は解決できないので、照合しうるファイルを指摘しないことを確認する。
func TestIncludedUnreferencedDynamic(t *testing.T) {
	dirs := map[string]string{"dynamic": testdataDir(t, "dynamic"), "env": testdataDir(t, "env")}
	for name, dir := range dirs {
		if got := unreferenced(t, dir, refsFor(t, []string{dir}, dir)); len(got) != 0 {
			t.Fatalf("%s: %v", name, got)
		}
	}
}

// 判定範囲の外のファイルと、Refs を渡さない Lint では指摘しないことを確認する。
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
