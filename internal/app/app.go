// Package app は runnlint の実行手順。
package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/teyamagu/runnlint/internal/baseline"
	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/report"
	"github.com/teyamagu/runnlint/internal/rules"
	"github.com/teyamagu/runnlint/internal/yamllint"
)

// Options は CLI から渡す実行条件。
type Options struct {
	Paths      []string
	Config     string
	Format     string
	NoYamllint bool
	// Baseline は既存の診断を記録したファイル。一致する診断は出さない。
	Baseline string
	// WriteBaseline は今回の診断をすべて書き出すファイル。書いたら終了コード 0 で終える。
	WriteBaseline string
	// IncludeRoots は include の参照元を探す追加のディレクトリ。中のファイルは included-unreferenced の判定対象にもなる。
	IncludeRoots []string
	Stdout       io.Writer
	Stderr       io.Writer
}

var rootKeyRe = regexp.MustCompile(`(?m)^(?:steps|runners):`)

// Run は runbook を検査し、終了コードを返す。
func Run(ctx context.Context, opt Options) (int, error) {
	if opt.Stdout == nil {
		opt.Stdout = os.Stdout
	}
	if opt.Stderr == nil {
		opt.Stderr = os.Stderr
	}
	if opt.Format == "" {
		opt.Format = "text"
	}
	cfg, err := config.Load(opt.Config)
	if err != nil {
		return 2, err
	}
	for _, w := range cfg.Warnings {
		fmt.Fprintf(opt.Stderr, "warning: %s\n", w)
	}
	if err := rules.CheckConfig(cfg); err != nil {
		return 2, err
	}
	var base *baseline.Baseline
	if opt.Baseline != "" && opt.WriteBaseline == "" {
		base, err = baseline.Load(opt.Baseline)
		if err != nil {
			return 2, err
		}
	}
	paths := opt.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := collect(paths, cfg)
	if err != nil {
		return 2, err
	}

	refs, err := buildRefs(paths, opt.IncludeRoots, files)
	if err != nil {
		return 2, err
	}

	var findings []finding.Finding
	var yamlFiles []string
	for _, file := range files {
		src, err := os.ReadFile(file.path)
		if err != nil {
			return 2, err
		}
		got, isRunbook := rules.LintWith(file.path, src, cfg, refs)
		if !isRunbook && !file.explicit && !rootKeyRe.Match(src) {
			continue
		}
		findings = append(findings, got...)
		yamlFiles = append(yamlFiles, file.path)
	}
	if !opt.NoYamllint {
		extra, err := yamllint.Run(ctx, cfg.Yamllint, cfg.SourceDir, yamlFiles)
		if err != nil {
			return 2, err
		}
		findings = append(findings, extra...)
	}
	if opt.WriteBaseline != "" {
		if err := baseline.Write(opt.WriteBaseline, findings); err != nil {
			return 2, err
		}
		fmt.Fprintf(opt.Stderr, "baseline %s に %d 件の診断を書きました\n", opt.WriteBaseline, len(findings))
		return 0, nil
	}
	if base != nil {
		res := base.Filter(findings)
		findings = res.Findings
		if res.Suppressed > 0 || res.Stale > 0 {
			fmt.Fprintf(opt.Stderr, "baseline %s: 既存の %d 件を除きました", opt.Baseline, res.Suppressed)
			if res.Stale > 0 {
				fmt.Fprintf(opt.Stderr, "。%d 件は解消済みです。--write-baseline で更新できます", res.Stale)
			}
			fmt.Fprintln(opt.Stderr)
		}
	}
	if err := report.Write(opt.Stdout, opt.Format, findings); err != nil {
		return 2, err
	}
	if report.HasError(findings) {
		return 1, nil
	}
	return 0, nil
}

type input struct {
	path     string
	explicit bool
}

func collect(paths []string, cfg *config.Config) ([]input, error) {
	var out []input
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, input{path: path, explicit: true})
			continue
		}
		err = filepath.WalkDir(path, func(name string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !isYAML(name) {
				return nil
			}
			if cfg.Ignored(name) {
				return nil
			}
			out = append(out, input{path: name, explicit: false})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// buildRefs は include の参照を集める。走査したディレクトリと roots の中は、ignore に関係なくすべての YAML を参照元として読む。
// その範囲の中のファイルだけを、参照元を残らず読んだものとして判定に使う。引数で直接渡したファイルは、参照元としてだけ読む。
func buildRefs(paths, roots []string, files []input) (*rules.Refs, error) {
	var scopes []string
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			scopes = append(scopes, path)
		}
	}
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, fmt.Errorf("--include-root: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("--include-root %s はディレクトリではありません", root)
		}
		scopes = append(scopes, root)
	}
	refs := rules.NewRefs(scopes...)
	seen := map[string]struct{}{}
	add := func(name string) error {
		abs, err := filepath.Abs(name)
		if err != nil {
			return err
		}
		if _, ok := seen[abs]; ok {
			return nil
		}
		seen[abs] = struct{}{}
		src, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		refs.Add(name, src)
		return nil
	}
	for _, scope := range scopes {
		err := filepath.WalkDir(scope, func(name string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !isYAML(name) {
				return nil
			}
			return add(name)
		})
		if err != nil {
			return nil, err
		}
	}
	for _, file := range files {
		if err := add(file.path); err != nil {
			return nil, err
		}
	}
	return refs, nil
}

func skipDir(base string) bool {
	return base == ".git" || base == "vendor" || base == "node_modules"
}

func isYAML(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yml" || ext == ".yaml"
}
