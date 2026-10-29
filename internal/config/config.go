// Package config loads runnlint rules files.
package config

import (
	"bytes"
	"embed"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
	"gopkg.in/yaml.v3"
)

//go:embed default.yaml yamllint.yaml
var embedded embed.FS

// Config is a whole rules file.
type Config struct {
	Version  int      `yaml:"version"`
	Extends  string   `yaml:"extends"`
	Language string   `yaml:"language"`
	Ignore   []string `yaml:"ignore"`
	Yamllint Yamllint `yaml:"yamllint"`
	Rules    []Rule   `yaml:"rules"`

	// SourceDir is the directory that contains the config file. Empty means the current directory.
	SourceDir string `yaml:"-"`
	// Warnings are configuration problems that do not stop loading. They are printed to stderr.
	Warnings []error `yaml:"-"`
}

// UniqueName is the config file name discovery looks for first.
const UniqueName = ".runnlint-runn.yaml"

// configNames are the config file names discovery looks for. .yaml is tried first.
var configNames = []string{UniqueName, ".runnlint-runn.yml"}

// Yamllint is the delegate for generic YAML lint.
type Yamllint struct {
	Enabled *bool  `yaml:"enabled"`
	Bin     string `yaml:"bin"`
	Config  string `yaml:"config"`
	Missing string `yaml:"missing"`
}

// Rule is a built-in rule or a rule written in YAML.
type Rule struct {
	ID       string           `yaml:"id"`
	Uses     string           `yaml:"uses"`
	Severity finding.Severity `yaml:"severity"`
	Message  string           `yaml:"message"`
	Check    *Check           `yaml:"check"`
	Options  map[string]any   `yaml:"options"`
}

// Check is the condition of a declarative rule.
type Check struct {
	Select      string    `yaml:"select"`
	Exists      *bool     `yaml:"exists"`
	Equals      *Any      `yaml:"equals"`
	NotEquals   *Any      `yaml:"not_equals"`
	Matches     string    `yaml:"matches"`
	NotMatch    string    `yaml:"not_matches"`
	KeyMatches  string    `yaml:"key_matches"`
	KeyNotMatch string    `yaml:"key_not_matches"`
	OneOf       []any     `yaml:"one_of"`
	MinLen      *int      `yaml:"min_len"`
	MaxLen      *int      `yaml:"max_len"`
	Forbid      bool      `yaml:"forbid"`
	Where       WhereList `yaml:"where"`
}

// Cond filters select results. path is relative to the selected node.
type Cond struct {
	Path    string `yaml:"path"`
	Exists  *bool  `yaml:"exists"`
	Equals  *Any   `yaml:"equals"`
	Matches string `yaml:"matches"`
}

// WhereList accepts either a single mapping or a sequence.
type WhereList []Cond

// UnmarshalYAML reads where as one mapping or as a sequence.
func (w *WhereList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.SequenceNode {
		var items []Cond
		if err := n.Decode(&items); err != nil {
			return err
		}
		*w = items
		return nil
	}
	var one Cond
	if err := n.Decode(&one); err != nil {
		return err
	}
	*w = []Cond{one}
	return nil
}

// Any is a YAML value from the config. It distinguishes null from unset.
type Any struct {
	Value any
	Set   bool
}

// UnmarshalYAML keeps any scalar, including null.
func (a *Any) UnmarshalYAML(n *yaml.Node) error {
	a.Set = true
	return n.Decode(&a.Value)
}

// EnabledOrDefault reports whether to run yamllint.
func (y Yamllint) EnabledOrDefault() bool {
	if y.Enabled == nil {
		return true
	}
	return *y.Enabled
}

// DefaultYAML is the built-in default rules.
func DefaultYAML() ([]byte, error) {
	return embedded.ReadFile("default.yaml")
}

// YamllintYAML is the yamllint config relaxed for runbooks.
func YamllintYAML() ([]byte, error) {
	return embedded.ReadFile("yamllint.yaml")
}

// Load reads the rules file at path. An empty path searches, and returns the defaults when none is found.
func Load(path string) (*Config, error) {
	base, err := parseBytes(mustDefault(), "")
	if err != nil {
		return nil, message.Wrap(message.DefaultRulesRead, err)
	}
	if err := base.Validate(); err != nil {
		return nil, message.Wrap(message.DefaultRulesBad, err)
	}
	var warnings []error
	explicit := path != ""
	if !explicit {
		path = discover()
	}
	if path == "" {
		base.Warnings = warnings
		return base, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, message.Wrap(message.RulesFileRead, err)
	}
	foreign, nested := checkShape(raw)
	if foreign != nil {
		if !explicit {
			return nil, message.Prefix(path, foreign)
		}
		// A file passed with --config is still read when it has unknown keys.
		warnings = append(warnings, message.Prefix(path, foreign))
	}
	if nested != nil {
		warnings = append(warnings, message.Prefix(path, nested))
	}
	user, err := parseBytes(raw, filepath.Dir(path))
	if err != nil {
		return nil, message.Prefix(path, err)
	}
	var cfg *Config
	if strings.EqualFold(user.Extends, "none") {
		cfg = user
	} else {
		cfg = merge(base, user)
	}
	if err := cfg.Validate(); err != nil {
		return nil, message.Prefix(path, err)
	}
	cfg.Warnings = warnings
	return cfg, nil
}

var topLevelKeys = map[string]struct{}{"version": {}, "extends": {}, "language": {}, "ignore": {}, "yamllint": {}, "rules": {}}

// checkShape reports whether raw has the shape of a runnlint rules file.
// foreign means the file looks like another tool's config: unreadable YAML, an unknown top-level key, a wrong type, or a version other than 1.
// nested is an unknown key inside rules or similar. It is treated as a typo in a runnlint config and only warned about.
func checkShape(raw []byte) (foreign, nested error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return message.Wrap(message.YAMLRead, err), nil
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return message.New(message.TopNotMap), nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if _, ok := topLevelKeys[root.Content[i].Value]; !ok {
			return message.New(message.UnknownTopKey, root.Content[i].Value, root.Content[i].Line), nil
		}
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return message.Wrap(message.RulesShape, err), nil
	}
	if cfg.Version != 0 && cfg.Version != 1 {
		return message.New(message.RulesVersion, cfg.Version), nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&Config{}); err != nil && err != io.EOF {
		return nil, message.Wrap(message.UnknownKeyIgnored, err)
	}
	return nil, nil
}

func mustDefault() []byte {
	b, err := DefaultYAML()
	if err != nil {
		panic(err)
	}
	return b
}

func parseBytes(raw []byte, dir string) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	cfg.SourceDir = dir
	if cfg.Yamllint.Bin == "" {
		cfg.Yamllint.Bin = "yamllint"
	}
	if cfg.Yamllint.Missing == "" {
		cfg.Yamllint.Missing = string(finding.Warning)
	}
	for i := range cfg.Rules {
		if cfg.Rules[i].Severity == "" {
			cfg.Rules[i].Severity = finding.Error
		}
	}
	return &cfg, nil
}

func merge(base, user *Config) *Config {
	out := *base
	out.SourceDir = user.SourceDir
	out.Version = user.Version
	if user.Version == 0 {
		out.Version = base.Version
	}
	if user.Language != "" {
		out.Language = user.Language
	}
	if user.Ignore != nil {
		out.Ignore = append(append([]string{}, base.Ignore...), user.Ignore...)
	}
	if userHasYamllint(user) {
		y := base.Yamllint
		if user.Yamllint.Enabled != nil {
			y.Enabled = user.Yamllint.Enabled
		}
		if user.Yamllint.Bin != "" {
			y.Bin = user.Yamllint.Bin
		}
		if user.Yamllint.Config != "" {
			y.Config = user.Yamllint.Config
		}
		if user.Yamllint.Missing != "" {
			y.Missing = user.Yamllint.Missing
		}
		out.Yamllint = y
	}
	index := map[string]int{}
	for i, rule := range out.Rules {
		index[rule.ID] = i
	}
	for _, rule := range user.Rules {
		if i, ok := index[rule.ID]; ok {
			out.Rules[i] = overlay(out.Rules[i], rule)
			continue
		}
		out.Rules = append(out.Rules, rule)
		index[rule.ID] = len(out.Rules) - 1
	}
	return &out
}

func userHasYamllint(user *Config) bool {
	return user.Yamllint.Enabled != nil || user.Yamllint.Bin != "" || user.Yamllint.Config != "" || user.Yamllint.Missing != ""
}

func overlay(base, over Rule) Rule {
	out := base
	if over.Severity != "" {
		out.Severity = over.Severity
	}
	if over.Message != "" {
		out.Message = over.Message
	}
	if over.Options != nil {
		merged := map[string]any{}
		for k, v := range base.Options {
			merged[k] = v
		}
		for k, v := range over.Options {
			merged[k] = v
		}
		out.Options = merged
	}
	if over.Check != nil && over.Uses == "" {
		out.Uses = ""
		out.Check = over.Check
	}
	if over.Uses != "" {
		out.Uses = over.Uses
		if over.Check == nil {
			out.Check = nil
		}
	}
	return out
}

// Validate checks rule IDs and the combination of uses and check.
func (c *Config) Validate() error {
	seen := map[string]struct{}{}
	for _, rule := range c.Rules {
		if rule.ID == "" {
			return message.New(message.RuleIDEmpty)
		}
		if _, ok := seen[rule.ID]; ok {
			return message.New(message.RuleIDDup, rule.ID)
		}
		seen[rule.ID] = struct{}{}
		switch rule.Severity {
		case finding.Error, finding.Warning, finding.Info, finding.Off:
		default:
			return message.New(message.SeverityBad, rule.ID, rule.Severity)
		}
		if rule.Severity == finding.Off {
			continue
		}
		switch {
		case rule.Uses != "" && rule.Check != nil:
			return message.New(message.UsesAndCheck, rule.ID)
		case rule.Uses == "" && rule.Check == nil:
			return message.New(message.UsesOrCheck, rule.ID)
		case rule.Check != nil && strings.TrimSpace(rule.Check.Select) == "":
			return message.New(message.SelectEmpty, rule.ID)
		}
	}
	if c.Yamllint.Missing != "" {
		switch finding.Severity(c.Yamllint.Missing) {
		case finding.Error, finding.Warning, finding.Info, finding.Off:
		default:
			return message.New(message.YamllintMissing, c.Yamllint.Missing)
		}
	}
	return nil
}

// Ignored reports whether a walked path (slash-separated), or its base name, matches ignore.
// A pattern containing ** is matched only against the path. The caller decides whether to exclude a file passed as an argument.
func (c *Config) Ignored(path string) bool {
	slash := filepath.ToSlash(path)
	base := filepath.Base(path)
	for _, pattern := range c.Ignore {
		if matchIgnore(pattern, slash, base) {
			return true
		}
	}
	return false
}

func matchIgnore(pattern, slash, base string) bool {
	pattern = filepath.ToSlash(pattern)
	if strings.Contains(pattern, "**") {
		return globMatch(pattern, slash)
	}
	if ok, _ := filepath.Match(pattern, slash); ok {
		return true
	}
	if ok, _ := filepath.Match(pattern, base); ok {
		return true
	}
	if !strings.ContainsAny(pattern, "*?[") && (slash == pattern || strings.HasPrefix(slash, strings.TrimSuffix(pattern, "/")+"/")) {
		return true
	}
	return false
}

func globMatch(pattern, path string) bool {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
				}
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '[', ']', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(pattern[i])
		default:
			b.WriteByte(pattern[i])
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

// discover searches upward from the current directory for a rules file.
func discover() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		for _, name := range configNames {
			candidate := filepath.Join(dir, name)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
