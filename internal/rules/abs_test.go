package rules

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAbsMatchesFilepathAbs checks that one captured working directory resolves paths the same way as filepath.Abs.
func TestAbsMatchesFilepathAbs(t *testing.T) {
	refs := NewRefs(".")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"", ".", "a", "a/b", "../x", "/tmp", "/tmp/../tmp/a", wd, filepath.Join(wd, "a")}
	for _, path := range paths {
		got, gotErr := refs.Abs(path)
		want, wantErr := filepath.Abs(path)
		if (gotErr == nil) != (wantErr == nil) || got != want {
			t.Fatalf("Abs %q got %q %v want %q %v", path, got, gotErr, want, wantErr)
		}
	}
}
