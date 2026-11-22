package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
)

// TestBindScansMatchStepScan checks that the shared scans report the same reference as the original per-step walk.
func TestBindScansMatchStepScan(t *testing.T) {
	names := []string{"token", "user", "id", "a", "ab", "later", "mid"}
	patterns := [][]string{
		{"o", "o", "o"},
		{"d", "d"},
		{"d", "d", "d"},
		{"o", "d"},
		{"d", "o"},
		{"o", "d", "o", "d"},
		{"d", "o", "d", "o"},
		{"d", "d", "o"},
		{"o", "o", "d", "d"},
		{"d", "o", "o", "d"},
	}
	var books int
	for _, pattern := range patterns {
		for mask := 0; mask < 1<<uint(len(pattern)); mask++ {
			src := bookWithBinds(pattern, names, mask)
			doc, err := runbook.Parse([]byte(src))
			if err != nil || doc.Book == nil {
				t.Fatalf("parse: %v\n%s", err, src)
			}
			books++
			if diff := bindScanDiff(doc.Book); diff != "" {
				t.Fatalf("%s\n%s", diff, src)
			}
		}
	}
	gapped := "steps:\n  - 1\n  - exec:\n      command: echo token.value\n    bind:\n      token: x\n  - exec:\n      command: echo token.value\n    defer: true\n    bind:\n      token: x\n"
	doc, err := runbook.Parse([]byte(gapped))
	if err != nil {
		t.Fatal(err)
	}
	if diff := bindScanDiff(doc.Book); diff != "" {
		t.Fatalf("%s\n%s", diff, gapped)
	}
	t.Logf("books %d", books+1)
}

func bookWithBinds(pattern, names []string, mask int) string {
	var b strings.Builder
	b.WriteString("desc: d\nsteps:\n")
	for i, kind := range pattern {
		name := names[i%len(names)]
		if mask&(1<<uint(i)) != 0 {
			name = names[(i+3)%len(names)]
		}
		fmt.Fprintf(&b, "  - exec:\n      command: echo %s.value\n", name)
		if kind == "d" {
			b.WriteString("    defer: true\n")
		}
		fmt.Fprintf(&b, "    bind:\n      %s: current.stdout\n", name)
		if i%4 == 0 {
			fmt.Fprintf(&b, "      %s.name: current.stdout\n", names[(i+1)%len(names)])
		}
	}
	return b.String()
}

func bindScanDiff(b *runbook.Book) string {
	bound := boundNames(b)
	scans := bindScans(b, bound)
	probes := probeStrings(bound)
	for i := range b.Steps {
		old := stepScan(b, &b.Steps[i], bound)
		cur := scans[i]
		if got, want := bindSet(cur), bindSet(old); !sameSet(got, want) {
			return fmt.Sprintf("step %d defer=%v set got %v want %v", i, b.Steps[i].Defer, got, want)
		}
		for _, probe := range probes {
			gr, gok := cur.find(probe)
			wr, wok := old.find(probe)
			if gr != wr || gok != wok {
				return fmt.Sprintf("step %d find %q got %q %v want %q %v", i, probe, gr, gok, wr, wok)
			}
			gr, gok = cur.findValue(probe)
			wr, wok = old.findValue(probe)
			if gr != wr || gok != wok {
				return fmt.Sprintf("step %d findValue %q got %q %v want %q %v", i, probe, gr, gok, wr, wok)
			}
		}
	}
	return ""
}

func probeStrings(bound map[int][]string) []string {
	var names []string
	seen := map[string]struct{}{}
	for _, list := range bound {
		for _, name := range list {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	out := []string{"token", "token.value", "user.id", "plain"}
	for _, name := range names {
		out = append(out, name, name+".id", name+"[0]", "echo "+name+".value && "+name)
	}
	if len(names) >= 2 {
		out = append(out, names[0]+"."+names[1], names[1]+".value "+names[0]+".id")
	}
	return out
}

func bindSet(s refScan) map[string]struct{} {
	out := map[string]struct{}{}
	for name := range s.binds {
		out[name] = struct{}{}
	}
	return out
}

func sameSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for name := range a {
		if _, ok := b[name]; !ok {
			return false
		}
	}
	return true
}
