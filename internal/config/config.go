// Package config は runnlint のルールファイルを読み込む。
package config

import (
	"bytes"
	"embed"
	"fmt"
	"io"
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
	// Warnings は読み込みを止めるほどではない設定の問題。標準エラーに出す。
	Warnings []string `yaml:"-"`
}

// UniqueName は runnlint だけが使う設定ファイル名。探索では最初に探す。
const UniqueName = ".runnlint-runn.yaml"

// 探索するファイル名。固有の名前を先に、従来の名前を後に探す。
// 従来の名前は同名の別ツールと重なるので、runnlint のルールファイルの形でなければ読み飛ばす。
var (
	uniqueNames = []string{UniqueName, ".runnlint-runn.yml"}
	legacyNames = []string{".runnlint.yaml", ".runnlint.yml", "runnlint.yaml", "runnlint.yml"}
)

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
	var warnings []string
	explicit := path != ""
	if !explicit {
		path, warnings = discover()
	}
	if path == "" {
		base.Warnings = warnings
		return base, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ルールファイルを読めません: %w", err)
	}
	foreign, nested := checkShape(raw)
	if foreign != nil {
		if !explicit {
			return nil, fmt.Errorf("%s: %w", path, foreign)
		}
		// --config で渡したファイルは、未知のキーがあっても従来どおり読む。
		warnings = append(warnings, fmt.Sprintf("%s: %v", path, foreign))
	}
	if nested != nil {
		warnings = append(warnings, fmt.Sprintf("%s: %v", path, nested))
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
	cfg.Warnings = warnings
	return cfg, nil
}

var topLevelKeys = map[string]struct{}{"version": {}, "extends": {}, "ignore": {}, "yamllint": {}, "rules": {}}

// checkShape は raw が runnlint のルールファイルの形か確かめる。
// foreign は別のツールの設定とみなす問題で、YAML として読めない、トップレベルに未知のキーがある、型が違う、version が 1 でない、のどれか。
// nested は rules の中などの未知のキーで、runnlint の設定の書き損じとみなして警告だけにする。
func checkShape(raw []byte) (foreign, nested error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("YAML として読めません: %w", err), nil
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("トップレベルが mapping ではありません"), nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if _, ok := topLevelKeys[root.Content[i].Value]; !ok {
			return fmt.Errorf("runnlint のルールファイルに無いキー %s があります (行 %d)", root.Content[i].Value, root.Content[i].Line), nil
		}
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("runnlint のルールファイルの形ではありません: %w", err), nil
	}
	if cfg.Version != 0 && cfg.Version != 1 {
		return fmt.Errorf("runnlint のルールファイルの version は 1 です (version: %d)", cfg.Version), nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&Config{}); err != nil && err != io.EOF {
		return nil, fmt.Errorf("未知のキーを無視します: %w", err)
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

// Ignored は、走査で得た path（スラッシュ区切り）か、そのベース名が ignore に当たるか。
// ** を含むパターンは path だけを見る。引数のファイルを外すかは、呼び出し側が決める。
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

// discover はカレントディレクトリから親へルールファイルを探す。
// 従来の名前で runnlint の形でないファイルは、警告を残して読み飛ばす。
func discover() (string, []string) {
	dir, err := os.Getwd()
	if err != nil {
		return "", nil
	}
	var warnings []string
	for {
		for _, name := range uniqueNames {
			candidate := filepath.Join(dir, name)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate, warnings
			}
		}
		for _, name := range legacyNames {
			candidate := filepath.Join(dir, name)
			st, err := os.Stat(candidate)
			if err != nil || st.IsDir() {
				continue
			}
			raw, err := os.ReadFile(candidate)
			if err == nil {
				err, _ = checkShape(raw)
			}
			if err == nil {
				return candidate, warnings
			}
			warnings = append(warnings, fmt.Sprintf("%s を読み飛ばします。別のツールの設定ファイルかもしれません (%v)。runnlint の設定は %s に置くか、--config で指定します", candidate, err, UniqueName))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", warnings
		}
		dir = parent
	}
}
