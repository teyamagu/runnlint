package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/teyamagu/runnlint/internal/app"
	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/rules"
	"github.com/teyamagu/runnlint/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	helpLang := peekLang(args)
	fs := flag.NewFlagSet("runnlint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		configPath    string
		format        string
		noYamllint    bool
		baselinePath  string
		writeBaseline string
		includeRoots  stringList
		printConfig   bool
		printYamllint bool
		listRules     bool
		showVersion   bool
		langFlag      string
	)
	fs.StringVar(&configPath, "config", "", message.Text(helpLang, message.FlagConfig))
	fs.StringVar(&configPath, "c", "", message.Text(helpLang, message.FlagConfig))
	fs.StringVar(&format, "format", "text", message.Text(helpLang, message.FlagFormat))
	fs.StringVar(&format, "f", "text", message.Text(helpLang, message.FlagFormat))
	fs.BoolVar(&noYamllint, "no-yamllint", false, message.Text(helpLang, message.FlagNoYamllint))
	fs.StringVar(&baselinePath, "baseline", "", message.Text(helpLang, message.FlagBaseline))
	fs.Var(&includeRoots, "include-root", message.Text(helpLang, message.FlagIncludeRoot))
	fs.StringVar(&writeBaseline, "write-baseline", "", message.Text(helpLang, message.FlagWriteBase))
	fs.BoolVar(&printConfig, "print-config", false, message.Text(helpLang, message.FlagPrintConfig))
	fs.BoolVar(&printYamllint, "print-yamllint-config", false, message.Text(helpLang, message.FlagPrintYAML))
	fs.BoolVar(&listRules, "list-rules", false, message.Text(helpLang, message.FlagListRules))
	fs.BoolVar(&showVersion, "version", false, message.Text(helpLang, message.FlagVersion))
	fs.StringVar(&langFlag, "lang", "", message.Text(helpLang, message.FlagLang))
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, message.Text(helpLang, message.Usage))
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if langFlag != "" && langFlag != "ja" && langFlag != "en" {
		fmt.Fprintln(os.Stderr, message.Text(message.JA, message.LangUnknown, langFlag))
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
			show := message.JA
			if langFlag == "en" {
				show = message.EN
			}
			fmt.Fprintln(os.Stderr, message.Localize(show, err))
			return 2
		}
		if langFlag != "" {
			cfg.Language = langFlag
		}
		switch cfg.Language {
		case "", "ja", "en":
		default:
			fmt.Fprintln(os.Stderr, message.Text(message.JA, message.LangUnknown, cfg.Language))
			return 2
		}
		show := message.JA
		if cfg.Language == "en" {
			show = message.EN
		}
		for _, w := range cfg.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", message.Localize(show, w))
		}
		if err := rules.CheckConfig(cfg); err != nil {
			fmt.Fprintln(os.Stderr, message.Localize(show, err))
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
		IncludeRoots:  includeRoots,
		Lang:          langFlag,
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

// stringList is a string flag that can be repeated.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func peekLang(args []string) message.Lang {
	for i, a := range args {
		v := ""
		switch {
		case a == "--lang" && i+1 < len(args):
			v = args[i+1]
		case strings.HasPrefix(a, "--lang="):
			v = strings.TrimPrefix(a, "--lang=")
		default:
			continue
		}
		if v == "en" {
			return message.EN
		}
		return message.JA
	}
	return message.JA
}

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}
