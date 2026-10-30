package rules

import (
	"fmt"
	"strings"
	"testing"
)

// factor is one axis of a combinatorial test. Each level is a named variation that the test turns into runbook text.
type factor struct {
	name   string
	levels []string
}

// pick is one row of the matrix: the chosen level of every factor.
type pick map[string]string

// pairwise returns rows of level indexes such that every pair of levels from any two factors appears in at least one row.
// It extends rows factor by factor, choosing for each row the level that covers the most new pairs, then adds rows for the pairs still missing.
// The result is deterministic, so a failing row can be rerun by name.
func pairwise(levels []int) [][]int {
	n := len(levels)
	if n == 0 {
		return nil
	}
	type pair struct{ f1, l1, f2, l2 int }
	covered := map[pair]bool{}
	mark := func(row []int) {
		for i := 0; i < n; i++ {
			if row[i] < 0 {
				continue
			}
			for j := i + 1; j < n; j++ {
				if row[j] >= 0 {
					covered[pair{i, row[i], j, row[j]}] = true
				}
			}
		}
	}
	newRow := func() []int {
		row := make([]int, n)
		for k := range row {
			row[k] = -1
		}
		return row
	}
	var rows [][]int
	if n == 1 {
		for l := 0; l < levels[0]; l++ {
			rows = append(rows, []int{l})
		}
		return rows
	}
	for a := 0; a < levels[0]; a++ {
		for b := 0; b < levels[1]; b++ {
			row := newRow()
			row[0], row[1] = a, b
			rows = append(rows, row)
			mark(row)
		}
	}
	gain := func(row []int, f, l int) int {
		g := 0
		for h := 0; h < n; h++ {
			if h == f || row[h] < 0 {
				continue
			}
			p := pair{h, row[h], f, l}
			if h > f {
				p = pair{f, l, h, row[h]}
			}
			if !covered[p] {
				g++
			}
		}
		return g
	}
	best := func(row []int, f int) int {
		b, bg := 0, -1
		for l := 0; l < levels[f]; l++ {
			if g := gain(row, f, l); g > bg {
				b, bg = l, g
			}
		}
		return b
	}
	for f := 2; f < n; f++ {
		for _, row := range rows {
			if row[f] < 0 {
				row[f] = best(row, f)
				mark(row)
			}
		}
		for g := 0; g < f; g++ {
			for lg := 0; lg < levels[g]; lg++ {
				for lf := 0; lf < levels[f]; lf++ {
					if covered[pair{g, lg, f, lf}] {
						continue
					}
					row := newRow()
					row[g], row[f] = lg, lf
					for h := 0; h < f; h++ {
						if row[h] < 0 {
							row[h] = best(row, h)
						}
					}
					rows = append(rows, row)
					mark(row)
				}
			}
		}
	}
	return rows
}

// assertPairwise fails when a row has an unset factor or when some pair of levels is covered by no row.
func assertPairwise(t *testing.T, rows [][]int, levels []int) {
	t.Helper()
	n := len(levels)
	type pair struct{ f1, l1, f2, l2 int }
	covered := map[pair]bool{}
	for _, row := range rows {
		if len(row) != n {
			t.Fatalf("row %v does not have %d factors", row, n)
		}
		for i := 0; i < n; i++ {
			if row[i] < 0 || row[i] >= levels[i] {
				t.Fatalf("row %v: factor %d has level %d outside 0..%d", row, i, row[i], levels[i]-1)
			}
			for j := i + 1; j < n; j++ {
				covered[pair{i, row[i], j, row[j]}] = true
			}
		}
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for a := 0; a < levels[i]; a++ {
				for b := 0; b < levels[j]; b++ {
					if !covered[pair{i, a, j, b}] {
						t.Fatalf("pair factor%d=%d factor%d=%d is covered by no row", i, a, j, b)
					}
				}
			}
		}
	}
}

// baseChoice returns one row per level of every factor, with every other factor at its first level.
// The first level of each factor is the base case, so these rows vary one factor at a time from it.
func baseChoice(levels []int) [][]int {
	n := len(levels)
	var rows [][]int
	for f := 0; f < n; f++ {
		for l := 0; l < levels[f]; l++ {
			if f > 0 && l == 0 {
				continue
			}
			row := make([]int, n)
			row[f] = l
			rows = append(rows, row)
		}
	}
	return rows
}

// runPairwise generates a pairwise set of rows for factors, adds the base-choice rows, and runs fn once per distinct row as a subtest
// named by the chosen levels. Pairwise rows cover the interactions; base-choice rows keep the outcome that needs every factor at its
// first level, such as "reported", from being rare in the matrix.
func runPairwise(t *testing.T, factors []factor, fn func(t *testing.T, p pick)) {
	t.Helper()
	levels := make([]int, len(factors))
	for i, f := range factors {
		if len(f.levels) == 0 {
			t.Fatalf("factor %s has no levels", f.name)
		}
		levels[i] = len(f.levels)
	}
	rows := pairwise(levels)
	assertPairwise(t, rows, levels)
	seen := map[string]bool{}
	for _, row := range rows {
		seen[fmt.Sprint(row)] = true
	}
	for _, row := range baseChoice(levels) {
		if !seen[fmt.Sprint(row)] {
			rows = append(rows, row)
			seen[fmt.Sprint(row)] = true
		}
	}
	for _, row := range rows {
		p := pick{}
		var parts []string
		for i, f := range factors {
			p[f.name] = f.levels[row[i]]
			parts = append(parts, f.name+"="+f.levels[row[i]])
		}
		t.Run(strings.Join(parts, ","), func(t *testing.T) {
			fn(t, p)
		})
	}
}

// pairwise covers every pair for mixed level counts, returns every single row for one factor, and nothing for no factors.
func TestPairwiseGenerator(t *testing.T) {
	for _, levels := range [][]int{{2, 2}, {3, 2, 4}, {5, 4, 3, 2, 6}, {2, 2, 2, 2, 2, 2, 2}, {11, 12, 6, 5, 2, 2}} {
		rows := pairwise(levels)
		assertPairwise(t, rows, levels)
		full := 1
		for _, l := range levels {
			full *= l
		}
		if len(rows) >= full && len(levels) > 2 {
			t.Fatalf("%v: %d rows is no smaller than the full factorial %d", levels, len(rows), full)
		}
	}
	if rows := pairwise([]int{3}); len(rows) != 3 {
		t.Fatalf("%v", rows)
	}
	if rows := pairwise(nil); rows != nil {
		t.Fatalf("%v", rows)
	}
	if rows := baseChoice([]int{3, 2, 2}); len(rows) != 5 || rows[0][0] != 0 || rows[3][1] != 1 || rows[4][2] != 1 {
		t.Fatalf("%v", rows)
	}
}

// lineOf returns the 1-based line of the first line that contains marker, or fails.
func lineOf(t *testing.T, src, marker string) int {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, marker) {
			return i + 1
		}
	}
	t.Fatalf("marker %q not found in:\n%s", marker, src)
	return 0
}

// linesOf returns the 1-based lines of every line that contains marker.
func linesOf(src, marker string) []int {
	var out []int
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, marker) {
			out = append(out, i+1)
		}
	}
	return out
}

func describe(p pick) string {
	var parts []string
	for k, v := range p {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, " ")
}
