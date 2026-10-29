// Package app is the procedure for running runnlint.
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
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/report"
	"github.com/teyamagu/runnlint/internal/rules"
	"github.com/teyamagu/runnlint/internal/yamllint"
)

// Options are the run settings passed in from the CLI.
type Options struct {
	Paths      []string
	Config     string
	Format     string
	NoYamllint bool
	// Baseline is a file of existing diagnostics. Matching diagnostics are omitted.
	Baseline string
	// WriteBaseline is the file that receives every diagnostic from this run. After it is written, the exit code is 0.
	WriteBaseline string
	// IncludeRoots are extra directories searched for include referrers. Files in them are also subject to included-unreferenced.
	IncludeRoots []string
	// Lang overrides the config language. Empty keeps the config value, which defaults to ja.
	Lang   string
	Stdout io.Writer
	Stderr io.Writer
}

var rootKeyRe = regexp.MustCompile(`(?m)^(?:steps|runners):`)

// Run lints runbooks and returns an exit code.
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
		return 2, present(opt.Lang, err)
	}
	if opt.Lang != "" {
		cfg.Language = opt.Lang
	}
	switch cfg.Language {
	case "", "ja", "en":
	default:
		return 2, present(opt.Lang, message.New(message.LangUnknown, cfg.Language))
	}
	lang := message.JA
	if cfg.Language == "en" {
		lang = message.EN
	}
	for _, w := range cfg.Warnings {
		fmt.Fprintf(opt.Stderr, "warning: %s\n", message.Localize(lang, w))
	}
	if err := rules.CheckConfig(cfg); err != nil {
		return 2, present(cfg.Language, err)
	}
	var base *baseline.Baseline
	if opt.Baseline != "" && opt.WriteBaseline == "" {
		base, err = baseline.Load(opt.Baseline)
		if err != nil {
			return 2, present(cfg.Language, err)
		}
	}
	paths := opt.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := collect(paths, cfg)
	if err != nil {
		return 2, present(cfg.Language, err)
	}

	refs, err := buildRefs(paths, opt.IncludeRoots, files)
	if err != nil {
		return 2, present(cfg.Language, err)
	}

	var findings []finding.Finding
	var yamlFiles []string
	for _, file := range files {
		src, err := os.ReadFile(file.path)
		if err != nil {
			return 2, present(cfg.Language, err)
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
			return 2, present(cfg.Language, err)
		}
		findings = append(findings, extra...)
	}
	if opt.WriteBaseline != "" {
		if err := baseline.Write(opt.WriteBaseline, findings); err != nil {
			return 2, present(cfg.Language, err)
		}
		fmt.Fprint(opt.Stderr, message.Text(lang, message.BaselineWrote, opt.WriteBaseline, len(findings)))
		return 0, nil
	}
	if base != nil {
		res := base.Filter(findings)
		findings = res.Findings
		if res.Suppressed > 0 || res.Stale > 0 {
			fmt.Fprint(opt.Stderr, message.Text(lang, message.BaselineKept, opt.Baseline, res.Suppressed))
			if res.Stale > 0 {
				fmt.Fprint(opt.Stderr, message.Text(lang, message.BaselineStale, res.Stale))
			}
			fmt.Fprintln(opt.Stderr)
		}
	}
	if err := report.Write(opt.Stdout, opt.Format, findings); err != nil {
		return 2, present(cfg.Language, err)
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

// buildRefs collects include references. Every YAML file under walked directories and roots is read as a referrer, whether or not ignore applies.
// Only files in that range are treated as having had every referrer read. A file passed directly as an argument is read only as a referrer.
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
			return nil, message.Wrap(message.IncludeRootStat, err)
		}
		if !info.IsDir() {
			return nil, message.New(message.IncludeRootDir, root)
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

func present(lang string, err error) error {
	if err == nil || lang != string(message.EN) {
		return err
	}
	return fmt.Errorf("%s", message.Localize(message.EN, err))
}

func skipDir(base string) bool {
	return base == ".git" || base == "vendor" || base == "node_modules"
}

func isYAML(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yml" || ext == ".yaml"
}
