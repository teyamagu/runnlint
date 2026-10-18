package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Version が v 無しの MAJOR.MINOR.PATCH であることを確認する。先頭 v、桁不足、0 埋めは拒否することを確認する。
func TestVersionIsSemver(t *testing.T) {
	if !Valid(Version) {
		t.Fatalf("Version = %s", Version)
	}
	for _, v := range []string{"v0.1.0", "0.1", "01.2.3", "1.2.3-", "1.2.3-rc."} {
		if Valid(v) {
			t.Fatalf("%s を受けています", v)
		}
	}
	if !Valid("1.2.3-rc.1") {
		t.Fatal("プレリリース")
	}
}

// CHANGELOG.md に、いまの Version の節があることを確認する。
func TestChangelogHasCurrentVersion(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(moduleRoot(t), "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "## ["+Version+"]") {
		t.Fatalf("CHANGELOG に ## [%s] がありません", Version)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod が見つかりません")
		}
		dir = parent
	}
}
