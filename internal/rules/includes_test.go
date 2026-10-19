package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func varRefMessages(t *testing.T, path string, refs *Refs) []string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := LintWith(path, src, defaultConfig(t), refs)
	return messagesOf(got, "var-ref")
}

// ディレクトリを走査したとき、include 元の include.vars で渡す変数は var-ref を出さないことを確認する。
// 呼び出し元が複数あれば、どれかで渡す変数を定義済みとする。parent.vars も include 元があれば親の外とは言わない。
// どこからも渡されない変数と、include されない runbook の未定義の変数は、これまでどおり指摘する。
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

// include 元を読まずにファイルだけを lint したとき、if: included の runbook では var-ref を出さず、それ以外はこれまでどおり出すことを確認する。
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

// 動的な include の参照先になりうるだけのファイルは、include 元が見つかったとは扱わないことを確認する。
// if: included の runbook は vars を調べず、そうでない runbook は動的な include が渡しうる変数だけを定義済みとする。
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
