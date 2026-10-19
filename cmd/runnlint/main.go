package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/teyamagu/runnlint/internal/app"
	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/rules"
	"github.com/teyamagu/runnlint/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("runnlint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		configPath    string
		format        string
		noYamllint    bool
		baselinePath  string
		writeBaseline string
		printConfig   bool
		printYamllint bool
		listRules     bool
		showVersion   bool
	)
	fs.StringVar(&configPath, "config", "", "ルールファイルのパス")
	fs.StringVar(&configPath, "c", "", "ルールファイルのパス")
	fs.StringVar(&format, "format", "text", "出力形式 (text, json, github)")
	fs.StringVar(&format, "f", "text", "出力形式 (text, json, github)")
	fs.BoolVar(&noYamllint, "no-yamllint", false, "yamllint を実行しない")
	fs.StringVar(&baselinePath, "baseline", "", "既存の診断を記録したファイル。一致する診断を出さない")
	fs.StringVar(&writeBaseline, "write-baseline", "", "今回の診断を baseline として書き出して終了する")
	fs.BoolVar(&printConfig, "print-config", false, "デフォルトのルールファイルを出力して終了する")
	fs.BoolVar(&printYamllint, "print-yamllint-config", false, "組み込みの yamllint 設定を出力して終了する")
	fs.BoolVar(&listRules, "list-rules", false, "有効なルール一覧を出力して終了する")
	fs.BoolVar(&showVersion, "version", false, "バージョンを出力して終了する")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "使い方: runnlint [flags] [path...]\n\nrunn の runbook を検査します。一般的な YAML の検査は yamllint に委譲します。\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if showVersion {
		fmt.Printf("runnlint %s\n", version.Version)
		return 0
	}
	if printConfig {
		b, err := config.DefaultYAML()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		os.Stdout.Write(b)
		return 0
	}
	if printYamllint {
		b, err := config.YamllintYAML()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		os.Stdout.Write(b)
		return 0
	}
	if listRules {
		cfg, err := config.Load(configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if err := rules.CheckConfig(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		for _, rule := range cfg.Rules {
			kind := "check"
			if rule.Uses != "" {
				kind = "builtin:" + rule.Uses
			}
			fmt.Printf("%s\t%s\t%s\n", rule.ID, rule.Severity, kind)
		}
		return 0
	}

	code, err := app.Run(context.Background(), app.Options{
		Paths:         fs.Args(),
		Config:        configPath,
		Format:        format,
		NoYamllint:    noYamllint,
		Baseline:      baselinePath,
		WriteBaseline: writeBaseline,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code == 0 {
			return 2
		}
		return code
	}
	return code
}
