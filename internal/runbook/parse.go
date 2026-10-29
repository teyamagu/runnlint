package runbook

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
	"gopkg.in/yaml.v3"
)

// Document is one runbook file.
type Document struct {
	Root  Value
	Dups  []Dup
	Book  *Book
	Empty bool
}

// IsRunbook reports whether this is a mapping that has runners or steps.
func (d *Document) IsRunbook() bool {
	if d == nil || d.Empty || d.Root.Kind != KindMap {
		return false
	}
	_, steps := d.Root.Get("steps")
	_, runners := d.Root.Get("runners")
	return steps || runners
}

// Parse converts YAML into positioned values and a runbook model.
func Parse(src []byte) (*Document, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var first yaml.Node
	if err := dec.Decode(&first); err != nil {
		if err == io.EOF {
			return &Document{Empty: true}, nil
		}
		return nil, err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, message.New(message.OneYAML)
	}

	rootNode := &first
	if first.Kind == yaml.DocumentNode {
		if len(first.Content) == 0 {
			return &Document{Empty: true}, nil
		}
		rootNode = first.Content[0]
	}

	seen := map[*yaml.Node]struct{}{}
	root, dups, err := decode(rootNode, seen, 0)
	if err != nil {
		return nil, err
	}
	doc := &Document{Root: root, Dups: dups}
	if root.Kind != KindMap {
		return doc, nil
	}
	doc.Book = buildBook(root, "", nil, -1)
	return doc, nil
}

func decode(n *yaml.Node, seen map[*yaml.Node]struct{}, depth int) (Value, []Dup, error) {
	if n == nil {
		return Value{Kind: KindNull}, nil, nil
	}
	if depth > 64 {
		return Value{}, nil, message.New(message.AliasDeep)
	}
	if n.Kind == yaml.AliasNode {
		if n.Alias == nil {
			return Value{}, nil, message.New(message.AliasUnknown)
		}
		if _, ok := seen[n.Alias]; ok {
			return Value{}, nil, message.New(message.AliasCycle)
		}
		seen[n.Alias] = struct{}{}
		v, dups, err := decode(n.Alias, seen, depth+1)
		delete(seen, n.Alias)
		if err != nil {
			return Value{}, nil, err
		}
		v.Node = n
		return v, dups, nil
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return Value{Kind: KindNull, Node: n}, nil, nil
		}
		return decode(n.Content[0], seen, depth+1)
	case yaml.ScalarNode:
		return decodeScalar(n), nil, nil
	case yaml.SequenceNode:
		seq := make([]Value, 0, len(n.Content))
		var dups []Dup
		for _, child := range n.Content {
			v, d, err := decode(child, seen, depth+1)
			if err != nil {
				return Value{}, nil, err
			}
			seq = append(seq, v)
			dups = append(dups, d...)
		}
		return Value{Kind: KindSeq, Node: n, Seq: seq}, dups, nil
	case yaml.MappingNode:
		return decodeMap(n, seen, depth)
	default:
		return Value{}, nil, message.New(message.YAMLNode)
	}
}

func decodeScalar(n *yaml.Node) Value {
	switch n.Tag {
	case "!!null", "":
		if n.Tag == "!!null" || n.Value == "" || n.Value == "null" || n.Value == "~" {
			if n.Tag == "!!null" {
				return Value{Kind: KindNull, Node: n}
			}
		}
		if n.Tag == "!!null" {
			return Value{Kind: KindNull, Node: n}
		}
		return Value{Kind: KindString, Node: n, String: n.Value}
	case "!!bool":
		b, _ := strconv.ParseBool(n.Value)
		return Value{Kind: KindBool, Node: n, Bool: b}
	case "!!int", "!!float":
		return Value{Kind: KindNumber, Node: n, Number: n.Value}
	default:
		return Value{Kind: KindString, Node: n, String: n.Value}
	}
}

func decodeMap(n *yaml.Node, seen map[*yaml.Node]struct{}, depth int) (Value, []Dup, error) {
	type pair struct {
		key     string
		keyNode *yaml.Node
		value   Value
		merge   bool
	}
	var pairs []pair
	var dups []Dup
	seenKey := map[string]struct{}{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		kn, vn := n.Content[i], n.Content[i+1]
		key, err := scalarKey(kn)
		if err != nil {
			return Value{}, nil, err
		}
		val, d, err := decode(vn, seen, depth+1)
		if err != nil {
			return Value{}, nil, err
		}
		dups = append(dups, d...)
		if key == "<<" {
			pairs = append(pairs, pair{key: key, keyNode: kn, value: val, merge: true})
			continue
		}
		if _, ok := seenKey[key]; ok {
			dups = append(dups, Dup{Key: key, Node: kn})
			continue
		}
		seenKey[key] = struct{}{}
		pairs = append(pairs, pair{key: key, keyNode: kn, value: val})
	}

	merged := map[string]Entry{}
	var order []string
	for _, p := range pairs {
		if !p.merge {
			continue
		}
		maps := mergeMaps(p.value)
		for _, m := range maps {
			for _, e := range m.Map {
				if _, exists := merged[e.Key]; exists {
					continue
				}
				merged[e.Key] = e
				order = append(order, e.Key)
			}
		}
	}
	explicit := map[string]Entry{}
	var explicitOrder []string
	for _, p := range pairs {
		if p.merge {
			continue
		}
		explicit[p.key] = Entry{Key: p.key, KeyNode: p.keyNode, Value: p.value}
		explicitOrder = append(explicitOrder, p.key)
	}
	var entries []Entry
	for _, key := range order {
		if _, ok := explicit[key]; ok {
			continue
		}
		entries = append(entries, merged[key])
	}
	for _, key := range explicitOrder {
		entries = append(entries, explicit[key])
	}
	return Value{Kind: KindMap, Node: n, Map: entries}, dups, nil
}

func mergeMaps(v Value) []Value {
	switch v.Kind {
	case KindMap:
		return []Value{v}
	case KindSeq:
		var out []Value
		for _, el := range v.Seq {
			out = append(out, mergeMaps(el)...)
		}
		return out
	default:
		return nil
	}
}

func scalarKey(n *yaml.Node) (string, error) {
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		return scalarKey(n.Alias)
	}
	if n.Kind != yaml.ScalarNode {
		return "", message.New(message.KeyScalar, n.Line)
	}
	if n.Tag == "!!null" {
		return "", message.New(message.KeyNull, n.Line)
	}
	return n.Value, nil
}

// ContainsTemplate reports whether s contains a runn {{ }} or ${} template.
func ContainsTemplate(s string) bool {
	return strings.Contains(s, "{{") || strings.Contains(s, "${")
}
