package runbook

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kind is the kind of a YAML node.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindNumber
	KindString
	KindSeq
	KindMap
)

// Value is a YAML value that keeps its source position.
type Value struct {
	Kind   Kind
	Node   *yaml.Node
	Bool   bool
	Number string
	String string
	Seq    []Value
	Map    []Entry
}

// Entry is one element of a mapping. Order is the order of appearance in the file.
type Entry struct {
	Key     string
	KeyNode *yaml.Node
	Value   Value
}

// Dup is an explicit duplicate key inside one mapping.
type Dup struct {
	Key  string
	Node *yaml.Node
}

// Get returns the value for key.
func (v Value) Get(key string) (Value, bool) {
	for _, e := range v.Map {
		if e.Key == key {
			return e.Value, true
		}
	}
	return Value{}, false
}

// Lookup follows a relative path that contains no wildcard.
func (v Value) Lookup(path string) (Value, bool) {
	cur := v
	if path == "" {
		return cur, true
	}
	for _, seg := range strings.Split(path, ".") {
		if cur.Kind != KindMap {
			return Value{}, false
		}
		next, ok := cur.Get(seg)
		if !ok {
			return Value{}, false
		}
		cur = next
	}
	return cur, true
}

// Len is the length of a string, sequence, or mapping.
func (v Value) Len() int {
	switch v.Kind {
	case KindString:
		return len([]rune(v.String))
	case KindSeq:
		return len(v.Seq)
	case KindMap:
		return len(v.Map)
	default:
		return 0
	}
}

// Display is a short form for diagnostic messages.
func (v Value) Display() string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindBool:
		return strconv.FormatBool(v.Bool)
	case KindNumber:
		return v.Number
	case KindString:
		s := v.String
		if len(s) > 80 {
			s = s[:77] + "..."
		}
		return strconv.Quote(s)
	case KindSeq:
		return fmt.Sprintf("sequence(%d)", len(v.Seq))
	case KindMap:
		return fmt.Sprintf("mapping(%d)", len(v.Map))
	default:
		return ""
	}
}

// Pos returns the 1-based line and column. A nil node, or a node whose line is 0, is 1,1.
func Pos(n *yaml.Node) (int, int) {
	if n == nil || n.Line == 0 {
		return 1, 1
	}
	return n.Line, n.Column
}
