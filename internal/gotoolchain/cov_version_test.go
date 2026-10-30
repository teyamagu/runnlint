package gotoolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
)

func englishOf(err error) string {
	return message.Localize(message.EN, err)
}

// Invalid JSON is reported as a list parse error. A stable entry whose version does not normalize is skipped, so the best remaining one wins.
func TestCovLatestStableParseAndSkip(t *testing.T) {
	_, err := LatestStable([]byte("{"))
	if err == nil || !strings.HasPrefix(englishOf(err), "cannot parse the Go release list: ") {
		t.Fatalf("%v", err)
	}
	got, err := LatestStable([]byte(`[{"version":"go1.27rc1","stable":true},{"version":"godevel","stable":true},{"version":"go1.26.2","stable":true}]`))
	if err != nil || got != "1.26.2" {
		t.Fatalf("got %s err %v", got, err)
	}
}

// Declared reports a missing .go-version, a .go-version that does not normalize, a missing go.mod, and a go.mod without a go line, each with its own sentence.
func TestCovDeclaredErrors(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name   string
		setup  func()
		expect string
	}{
		{"missing .go-version", func() {}, "cannot read .go-version"},
		{"bad .go-version", func() { write(".go-version", "1.x\n") }, ".go-version: version \"1.x\" is not numeric"},
		{"missing go.mod", func() { write(".go-version", "1.27.3\n") }, "cannot read go.mod"},
		{"go.mod without go line", func() { write("go.mod", "module x\n") }, "go.mod has no go line"},
	}
	for _, tc := range cases {
		tc.setup()
		_, _, err := Declared(root)
		if err == nil || !strings.Contains(englishOf(err), tc.expect) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	write("go.mod", "module x\n\ngo 1.27.3\n")
	file, mod, err := Declared(root)
	if err != nil || file != "1.27.3" || mod != "1.27.3" {
		t.Fatalf("%s %s %v", file, mod, err)
	}
}

// Normalize rejects a version with one or four components and a non-numeric component. Compare returns 0 for equal versions and treats a missing component as 0.
func TestCovNormalizeAndCompareEdges(t *testing.T) {
	for _, v := range []string{"1", "1.2.3.4"} {
		if _, err := Normalize(v); err == nil || englishOf(err) != `version "`+v+`" is not major.minor.patch` {
			t.Fatalf("%s: %v", v, err)
		}
	}
	if _, err := Normalize("1.a"); err == nil || englishOf(err) != `version "1.a" is not numeric` {
		t.Fatalf("%v", err)
	}
	if got := Compare("1.27.3", "1.27.3"); got != 0 {
		t.Fatalf("equal = %d", got)
	}
	if got := Compare("1.27", "1.27.0"); got != 0 {
		t.Fatalf("missing patch = %d", got)
	}
}
