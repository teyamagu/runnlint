package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionIsSemver checks that Version is MAJOR.MINOR.PATCH with no leading v. A leading v, too few components, and zero-padding are rejected.
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

// TestChangelogHasCurrentVersion checks that both changelogs have the same version headings, including the current Version.
func TestChangelogHasCurrentVersion(t *testing.T) {
	root := moduleRoot(t)
	ja := changelogHeadings(t, filepath.Join(root, "CHANGELOG.md"))
	en := changelogHeadings(t, filepath.Join(root, "CHANGELOG.en.md"))
	if strings.Join(ja, "\n") != strings.Join(en, "\n") {
		t.Fatalf("見出しが違います\nja:\n%s\nen:\n%s", strings.Join(ja, "\n"), strings.Join(en, "\n"))
	}
	want := "## [" + Version + "]"
	for _, heading := range ja {
		if heading == want {
			return
		}
	}
	t.Fatalf("CHANGELOG に %s がありません", want)
}

func changelogHeadings(t *testing.T, path string) []string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var headings []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "## [") {
			headings = append(headings, line)
		}
	}
	return headings
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
