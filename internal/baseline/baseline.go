// Package baseline records existing diagnostics and keeps only those that are new.
//
// A diagnostic is matched by file, rule ID, and message. Line and column are not used, so
// adding or removing lines before a violation leaves an existing diagnostic still matched.
// When the same triple occurs more than once, matching is by count, and any excess over the record is a new diagnostic.
// Built-in diagnostics are stored as the English sentence. A declarative message is stored as written.
package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
)

// Version is the baseline file format version.
const Version = 2

type file struct {
	Version  int     `json:"version"`
	Findings []entry `json:"findings"`
}

type entry struct {
	File    string `json:"file"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type key struct {
	file    string
	rule    string
	message string
}

// Baseline is the loaded set of existing diagnostics.
type Baseline struct {
	dir    string
	counts map[key]int
}

// Result is the result of Filter.
type Result struct {
	// Findings are the diagnostics that are not in the baseline.
	Findings []finding.Finding
	// Suppressed is how many diagnostics were removed because they matched the baseline.
	Suppressed int
	// Stale is how many baseline entries did not appear this run.
	Stale int
}

// Load reads the baseline at path.
func Load(path string) (*Baseline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, message.Wrap(message.BaselineRead, err)
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, message.Wrap(message.BaselineJSON, err, path)
	}
	if f.Version != Version {
		return nil, message.New(message.BaselineVersion, path, f.Version, Version)
	}
	dir, err := absDir(path)
	if err != nil {
		return nil, err
	}
	b := &Baseline{dir: dir, counts: map[key]int{}}
	for _, e := range f.Findings {
		n := e.Count
		if n <= 0 {
			n = 1
		}
		b.counts[key{file: filepath.ToSlash(e.File), rule: e.Rule, message: e.Message}] += n
	}
	return b, nil
}

// Filter removes diagnostics recorded in the baseline.
func (b *Baseline) Filter(findings []finding.Finding) Result {
	left := make(map[key]int, len(b.counts))
	for k, n := range b.counts {
		left[k] = n
	}
	var res Result
	for _, f := range findings {
		k := keyOf(b.dir, f)
		if left[k] > 0 {
			left[k]--
			res.Suppressed++
			continue
		}
		res.Findings = append(res.Findings, f)
	}
	for _, n := range left {
		res.Stale += n
	}
	return res
}

// Write writes findings to path as a baseline.
func Write(path string, findings []finding.Finding) error {
	dir, err := absDir(path)
	if err != nil {
		return err
	}
	counts := map[key]int{}
	for _, f := range findings {
		counts[keyOf(dir, f)]++
	}
	out := file{Version: Version, Findings: make([]entry, 0, len(counts))}
	for k, n := range counts {
		out.Findings = append(out.Findings, entry{File: k.file, Rule: k.rule, Message: k.message, Count: n})
	}
	sort.Slice(out.Findings, func(i, j int) bool {
		a, b := out.Findings[i], out.Findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Message < b.Message
	})
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return message.Wrap(message.BaselineWrite, err)
	}
	return nil
}

func absDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Dir(abs), nil
}

// keyOf makes a diagnostic's file path relative to the directory that contains the baseline.
// Matching then does not change with the working directory or a leading ./ on an argument.
func keyOf(dir string, f finding.Finding) key {
	name := f.File
	if abs, err := filepath.Abs(name); err == nil {
		if rel, err := filepath.Rel(dir, abs); err == nil {
			name = rel
		}
	}
	return key{file: filepath.ToSlash(name), rule: f.Rule, message: f.BaselineMessage()}
}
