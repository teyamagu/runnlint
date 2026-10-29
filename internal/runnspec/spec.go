// Package runnspec reads notation definitions copied from the runn release being followed.
//
// Added keys appear in runbook.schema.yaml. CDP action names are not in the schema;
// they appear in cdpfn.src. Rules the schema does not cover, such as HTTP body checks and
// runner-kind detection, stay as overrides in the rules package.
package runnspec

import (
	"embed"
	"strings"
	"sync"

	"github.com/teyamagu/runnlint/internal/message"
	"gopkg.in/yaml.v3"
)

//go:embed VERSION runbook.schema.yaml cdpfn.src
var files embed.FS

// Schema definition names. They correspond to $defs in runbook.schema.yaml.
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

// Version is the runn tag this copy was taken from.
func Version() string {
	raw, err := files.ReadFile("VERSION")
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(raw))
}

// SchemaYAML is the imported runbook.schema.yaml.
func SchemaYAML() []byte {
	mustLoad()
	return loaded.schemaYAML
}

// CDPSource is the imported cdpfn.go.
func CDPSource() []byte {
	mustLoad()
	return loaded.cdpSource
}

// TopLevelKeys are the keys allowed at the top level of a runbook.
func TopLevelKeys() map[string]struct{} {
	mustLoad()
	return loaded.root
}

// ExtensionPrefixes are the fixed prefixes from patternProperties. Currently x-.
func ExtensionPrefixes() []string {
	mustLoad()
	return loaded.prefixes
}

// LabelPattern is the regular expression used for elements of labels.
func LabelPattern() string {
	mustLoad()
	return loaded.label
}

// DefKeys are the property names of a $defs entry. Each oneOf branch is included in the union.
func DefKeys(name string) map[string]struct{} {
	mustLoad()
	def, ok := loaded.defs[name]
	if !ok {
		return map[string]struct{}{}
	}
	return def.keys
}

// Required is the required list of an object definition. It is not expanded into oneOf branches.
func Required(name string) []string {
	mustLoad()
	def, ok := loaded.defs[name]
	if !ok {
		return nil
	}
	return def.required
}

// CDPArgs maps each CDP action name and alias to its input arguments.
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
			return index{}, message.New(message.SchemaDefs, name)
		}
	}
	if idx.label == "" {
		return index{}, message.New(message.SchemaLabels)
	}
	if len(idx.cdp) == 0 {
		return index{}, message.New(message.SchemaCDP)
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
			return index{}, message.New(message.SchemaEmpty)
		}
		root = doc.Content[0]
	}
	props := mapValue(root, "properties")
	if props == nil {
		return index{}, message.New(message.SchemaProps)
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
		return index{}, message.New(message.SchemaNoDefs)
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
