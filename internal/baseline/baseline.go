// Package baseline は既存の診断を記録し、新しく増えた診断だけを残す。
//
// 診断はファイル、ルール ID、メッセージの組で照合する。行と列は使わないので、
// 違反の前に行を足したり消したりしても既存の診断は既存のままになる。
// 同じ組が複数あるときは件数で照合し、記録より増えた分を新しい診断とする。
package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/teyamagu/runnlint/internal/finding"
)

// Version は baseline ファイルの形式の版。
const Version = 1

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

// Baseline は読み込んだ既存の診断。
type Baseline struct {
	dir    string
	counts map[key]int
}

// Result は Filter の結果。
type Result struct {
	// Findings は baseline に無い診断。
	Findings []finding.Finding
	// Suppressed は baseline と一致して除いた件数。
	Suppressed int
	// Stale は baseline にあるが今回は出なかった件数。
	Stale int
}

// Load は path の baseline を読む。
func Load(path string) (*Baseline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baseline を読めません: %w", err)
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("%s: baseline の JSON が不正です: %w", path, err)
	}
	if f.Version != Version {
		return nil, fmt.Errorf("%s: baseline の version %d は読めません (対応は %d)", path, f.Version, Version)
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

// Filter は baseline に記録された診断を除く。
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

// Write は findings を path に baseline として書く。
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
		return fmt.Errorf("baseline を書けません: %w", err)
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

// keyOf は診断のファイルを baseline のあるディレクトリからの相対パスにする。
// 実行するディレクトリや、引数に ./ を付けるかどうかで照合が変わらないようにする。
func keyOf(dir string, f finding.Finding) key {
	name := f.File
	if abs, err := filepath.Abs(name); err == nil {
		if rel, err := filepath.Rel(dir, abs); err == nil {
			name = rel
		}
	}
	return key{file: filepath.ToSlash(name), rule: f.Rule, message: f.Message}
}
