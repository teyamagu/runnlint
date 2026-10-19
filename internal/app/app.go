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
	Stdout        io.Writer
	Stderr        io.Writer
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

	var findings []finding.Finding
	var yamlFiles []string
	for _, file := range files {
		src, err := os.ReadFile(file.path)
		if err != nil {
			return 2, err
		}
		got, isRunbook := rules.Lint(file.path, src, cfg)
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
				base := d.Name()
				if base == ".git" || base == "vendor" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(name)
			if ext != ".yml" && ext != ".yaml" {
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
