package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
)

// Linting each file in examples/bad with the default rules reports the rule named by the file. A default rule with no corresponding file fails the test.
func TestBadExamplesCoverDefaultRules(t *testing.T) {
	root := moduleRoot(t)
	t.Chdir(root)
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	badDir := filepath.Join(root, "examples", "bad")
	refs := refsFor(t, []string{filepath.Join(root, "examples")}, filepath.Join(root, "examples"))
	entries, err := os.ReadDir(badDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".yml") {
			t.Fatalf("examples/bad にはルールごとの .yml だけを置きます: %s", name)
		}
		id := strings.TrimSuffix(name, ".yml")
		rel := filepath.Join("examples", "bad", name)
		src, err := os.ReadFile(filepath.Join(badDir, name))
		if err != nil {
			t.Fatal(err)
		}
		got, is := LintWith(rel, src, cfg, refs)
		if !is {
			t.Fatalf("%s は runbook ではありません: %+v", rel, got)
		}
		if !hasRule(got, id) {
			t.Fatalf("%s に %s がありません: %+v", rel, id, got)
		}
		seen[id] = true
	}
	for _, rule := range cfg.Rules {
		if rule.Severity == finding.Off {
			continue
		}
		if !seen[rule.ID] {
			t.Fatalf("examples/bad に %s.yml がありません", rule.ID)
		}
	}
}

// Linting examples/valid with the default rules reports no diagnostics.
func TestValidExamplesAreClean(t *testing.T) {
	root := moduleRoot(t)
	t.Chdir(root)
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	validDir := filepath.Join(root, "examples", "valid")
	refs := refsFor(t, []string{filepath.Join(root, "examples")}, filepath.Join(root, "examples"))
	entries, err := os.ReadDir(validDir)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			t.Fatalf("examples/valid には通る runbook の .yml だけを置きます: %s", entry.Name())
		}
		paths = append(paths, filepath.Join(validDir, entry.Name()))
	}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, is := LintWith(rel, src, cfg, refs)
		if !is || len(got) > 0 {
			t.Fatalf("%s: runbook=%v findings=%+v", rel, is, got)
		}
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
