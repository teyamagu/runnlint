// Package runnspec は、追従対象の runn から写した記法の定義を読む。
//
// キーの追加は runbook.schema.yaml に現れる。CDP のアクション名はスキーマに無く、
// cdpfn.src に現れる。HTTP の body や runner 種別の判定など、スキーマに無い規則は
// rules パッケージ側の上書きとして残す。
package runnspec

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed VERSION runbook.schema.yaml cdpfn.src
var files embed.FS

// スキーマ上の定義名。runbook.schema.yaml の $defs と対応する。
const (
	DefStep        = "step"
	DefLoop        = "loopValue"
	DefDump        = "dumpValue"
	DefExec        = "execStepValue"
	DefTrace       = "traceConfig"
	DefHTTPRunner  = "httpRunnerConfig"
	DefGRPCRunner  = "grpcRunnerConfig"
	DefDBRunner    = "dbRunnerConfig"
	DefCDPRunner   = "cdpRunnerConfig"
	DefSSHRunner   = "sshRunnerConfig"
	DefIncludeRun  = "includeRunnerConfig"
	DefAgentRunner = "agentRunnerConfig"
)

type index struct {
	root       map[string]struct{}
	prefixes   []string
	label      string
	defs       map[string]definition
	cdp        map[string][]string
	schemaYAML []byte
	cdpSource  []byte
}

type definition struct {
	keys     map[string]struct{}
	required []string
}

var (
	loadOnce sync.Once
	loaded   index
	loadErr  error
)

// Version は写しの元になった runn のタグ。
func Version() string {
	raw, err := files.ReadFile("VERSION")
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(raw))
}

// SchemaYAML は取り込んだ runbook.schema.yaml。
func SchemaYAML() []byte {
	mustLoad()
	return loaded.schemaYAML
}

// CDPSource は取り込んだ cdpfn.go。
func CDPSource() []byte {
	mustLoad()
	return loaded.cdpSource
}

// TopLevelKeys は runbook のトップレベルに書いてよいキー。
func TopLevelKeys() map[string]struct{} {
	mustLoad()
	return loaded.root
}

// ExtensionPrefixes は patternProperties の固定前置。いまは x-。
func ExtensionPrefixes() []string {
	mustLoad()
	return loaded.prefixes
}

// LabelPattern は labels の要素に使う正規表現。
func LabelPattern() string {
	mustLoad()
	return loaded.label
}

// DefKeys は $defs のプロパティ名。oneOf の各枝は和集合にする。
func DefKeys(name string) map[string]struct{} {
	mustLoad()
	def, ok := loaded.defs[name]
	if !ok {
		return map[string]struct{}{}
	}
	return def.keys
}

// Required はオブジェクト定義の required。oneOf の枝には広げない。
func Required(name string) []string {
	mustLoad()
	def, ok := loaded.defs[name]
	if !ok {
		return nil
	}
	return def.required
}

// CDPArgs は CDP アクション名と、その別名、入力引数。
func CDPArgs() map[string][]string {
	mustLoad()
	return loaded.cdp
}

func mustLoad() {
	loadOnce.Do(func() {
		loaded, loadErr = load()
	})
	if loadErr != nil {
		panic(loadErr)
	}
}

func load() (index, error) {
	schema, err := files.ReadFile("runbook.schema.yaml")
	if err != nil {
		return index{}, err
	}
	cdp, err := files.ReadFile("cdpfn.src")
	if err != nil {
		return index{}, err
	}
	idx, err := parseSchema(schema)
	if err != nil {
		return index{}, err
	}
	args, err := ParseCDPSource(string(cdp))
	if err != nil {
		return index{}, err
	}
	idx.cdp = args
	idx.schemaYAML = schema
	idx.cdpSource = cdp
	for _, name := range []string{
		DefStep, DefLoop, DefDump, DefExec, DefTrace,
		DefHTTPRunner, DefGRPCRunner, DefDBRunner, DefCDPRunner,
		DefSSHRunner, DefIncludeRun, DefAgentRunner,
	} {
		if _, ok := idx.defs[name]; !ok {
			return index{}, fmt.Errorf("スキーマに $defs.%s がありません", name)
		}
	}
	if idx.label == "" {
		return index{}, fmt.Errorf("スキーマに labels の pattern がありません")
	}
	if len(idx.cdp) == 0 {
		return index{}, fmt.Errorf("CDP のアクションを読めません")
	}
	return idx, nil
}

func parseSchema(raw []byte) (index, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return index{}, err
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return index{}, fmt.Errorf("スキーマが空です")
		}
		root = doc.Content[0]
	}
	props := mapValue(root, "properties")
	if props == nil {
		return index{}, fmt.Errorf("スキーマに properties がありません")
	}
	idx := index{
		root: mapKeys(props),
		defs: map[string]definition{},
	}
	if labels := mapValue(props, "labels"); labels != nil {
		if items := mapValue(labels, "items"); items != nil {
			idx.label = mapScalar(items, "pattern")
		}
	}
	if patterns := mapValue(root, "patternProperties"); patterns != nil {
		for _, pattern := range mapKeyList(patterns) {
			if prefix, ok := literalPrefix(pattern); ok {
				idx.prefixes = append(idx.prefixes, prefix)
			}
		}
	}
	defs := mapValue(root, "$defs")
	if defs == nil {
		return index{}, fmt.Errorf("スキーマに $defs がありません")
	}
	for _, name := range mapKeyList(defs) {
		node := mapValue(defs, name)
		idx.defs[name] = parseDef(node)
	}
	return idx, nil
}

func parseDef(n *yaml.Node) definition {
	def := definition{keys: map[string]struct{}{}}
	if n == nil || n.Kind != yaml.MappingNode {
		return def
	}
	if props := mapValue(n, "properties"); props != nil {
		def.keys = mapKeys(props)
	}
	if req := mapValue(n, "required"); req != nil && req.Kind == yaml.SequenceNode {
		for _, item := range req.Content {
			if item.Value != "" {
				def.required = append(def.required, item.Value)
			}
		}
	}
	if one := mapValue(n, "oneOf"); one != nil && one.Kind == yaml.SequenceNode {
		for _, branch := range one.Content {
			sub := parseDef(branch)
			for key := range sub.keys {
				def.keys[key] = struct{}{}
			}
		}
	}
	return def
}

func literalPrefix(pattern string) (string, bool) {
	if !strings.HasPrefix(pattern, "^") {
		return "", false
	}
	rest := strings.TrimPrefix(pattern, "^")
	end := strings.IndexAny(rest, `.+*?()[]{}|\`)
	if end == 0 {
		return "", false
	}
	if end > 0 {
		rest = rest[:end]
	}
	if rest == "" {
		return "", false
	}
	return rest, true
}

func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func mapScalar(n *yaml.Node, key string) string {
	v := mapValue(n, key)
	if v == nil {
		return ""
	}
	return v.Value
}

func mapKeyList(n *yaml.Node) []string {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var keys []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		keys = append(keys, n.Content[i].Value)
	}
	return keys
}

func mapKeys(n *yaml.Node) map[string]struct{} {
	out := map[string]struct{}{}
	for _, key := range mapKeyList(n) {
		out[key] = struct{}{}
	}
	return out
}
