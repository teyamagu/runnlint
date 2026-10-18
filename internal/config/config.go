// Package config は runnlint のルールファイルを読み込む。
package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/teyamagu/runnlint/internal/finding"
	"gopkg.in/yaml.v3"
)

//go:embed default.yaml yamllint.yaml
var embedded embed.FS

// Config はルールファイル全体。
type Config struct {
	Version  int      `yaml:"version"`
	Extends  string   `yaml:"extends"`
	Ignore   []string `yaml:"ignore"`
	Yamllint Yamllint `yaml:"yamllint"`
	Rules    []Rule   `yaml:"rules"`

	// SourceDir は設定ファイルがあるディレクトリ。空ならカレントディレクトリ。
	SourceDir string `yaml:"-"`
}

// Yamllint は汎用 YAML lint の委譲先。
type Yamllint struct {
	Enabled *bool  `yaml:"enabled"`
	Bin     string `yaml:"bin"`
	Config  string `yaml:"config"`
	Missing string `yaml:"missing"`
}

// Rule は組み込みルール、または YAML で書いたルール。
type Rule struct {
	ID       string           `yaml:"id"`
	Uses     string           `yaml:"uses"`
	Severity finding.Severity `yaml:"severity"`
	Message  string           `yaml:"message"`
	Check    *Check           `yaml:"check"`
	Options  map[string]any   `yaml:"options"`
}

// Check は宣言的ルールの条件。
type Check struct {
	Select    string    `yaml:"select"`
	Exists    *bool     `yaml:"exists"`
	Equals    *Any      `yaml:"equals"`
	NotEquals *Any      `yaml:"not_equals"`
	Matches   string    `yaml:"matches"`
	NotMatch  string    `yaml:"not_matches"`
	OneOf     []any     `yaml:"one_of"`
	MinLen    *int      `yaml:"min_len"`
	MaxLen    *int      `yaml:"max_len"`
	Forbid    bool      `yaml:"forbid"`
	Where     WhereList `yaml:"where"`
}

// Cond は select 結果を絞り込む条件。path は選択ノードからの相対パス。
type Cond struct {
	Path    string `yaml:"path"`
	Exists  *bool  `yaml:"exists"`
	Equals  *Any   `yaml:"equals"`
	Matches string `yaml:"matches"`
}

// WhereList は mapping 1件でも sequence でも受け付ける。
type WhereList []Cond

// UnmarshalYAML は where の単一 mapping と sequence の両方を読む。
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

// Any は設定された YAML 値。null と未設定を区別する。
type Any struct {
	Value any
	Set   bool
}

// UnmarshalYAML は null を含む任意のスカラーを保持する。
func (a *Any) UnmarshalYAML(n *yaml.Node) error {
	a.Set = true
	return n.Decode(&a.Value)
}

// Enabled は yamllint を起動するか。
func (y Yamllint) EnabledOrDefault() bool {
	if y.Enabled == nil {
		return true
	}
	return *y.Enabled
}

// DefaultYAML は組み込みのデフォルトルール。
func DefaultYAML() ([]byte, error) {
	return embedded.ReadFile("default.yaml")
}

// YamllintYAML は runbook 向けに緩めた yamllint 設定。
func YamllintYAML() ([]byte, error) {
	return embedded.ReadFile("yamllint.yaml")
}

// Load は path のルールファイルを読む。path が空なら探索し、無ければデフォルトを返す。
func Load(path string) (*Config, error) {
	base, err := parseBytes(mustDefault(), "")
	if err != nil {
		return nil, fmt.Errorf("デフォルトルールを読めません: %w", err)
	}
	if err := base.Validate(); err != nil {
		return nil, fmt.Errorf("デフォルトルールが不正です: %w", err)
	}
	if path == "" {
		path = discover()
	}
	if path == "" {
		return base, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ルールファイルを読めません: %w", err)
	}
	user, err := parseBytes(raw, filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var cfg *Config
	if strings.EqualFold(user.Extends, "none") {
		cfg = user
	} else {
		cfg = merge(base, user)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
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

// Validate はルール ID と uses / check の組み合わせを確認する。
func (c *Config) Validate() error {
	seen := map[string]struct{}{}
	for _, rule := range c.Rules {
		if rule.ID == "" {
			return fmt.Errorf("id が空のルールがあります")
		}
		if _, ok := seen[rule.ID]; ok {
			return fmt.Errorf("ルール %s が重複しています", rule.ID)
		}
		seen[rule.ID] = struct{}{}
		switch rule.Severity {
		case finding.Error, finding.Warning, finding.Info, finding.Off:
		default:
			return fmt.Errorf("ルール %s の severity %q は error / warning / info / off のいずれかです", rule.ID, rule.Severity)
		}
		if rule.Severity == finding.Off {
			continue
		}
		switch {
		case rule.Uses != "" && rule.Check != nil:
			return fmt.Errorf("ルール %s は uses と check を同時に持てません", rule.ID)
		case rule.Uses == "" && rule.Check == nil:
			return fmt.Errorf("ルール %s には uses か check が必要です", rule.ID)
		case rule.Check != nil && strings.TrimSpace(rule.Check.Select) == "":
			return fmt.Errorf("ルール %s の check.select が空です", rule.ID)
		}
	}
	if c.Yamllint.Missing != "" {
		switch finding.Severity(c.Yamllint.Missing) {
		case finding.Error, finding.Warning, finding.Info, finding.Off:
		default:
			return fmt.Errorf("yamllint.missing %q は error / warning / info / off のいずれかです", c.Yamllint.Missing)
		}
	}
	return nil
}

// Ignored は path が ignore に含まれるか。
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

func discover() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		for _, name := range []string{".runnlint.yaml", ".runnlint.yml", "runnlint.yaml", "runnlint.yml"} {
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
