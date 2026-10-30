package runbook

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
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

// Lookup follows a relative path that contains no wildcard. A path that cannot be parsed finds nothing.
func (v Value) Lookup(path string) (Value, bool) {
	segs, err := ParsePath(path)
	if err != nil {
		return Value{}, false
	}
	return v.LookupSegs(segs)
}

// LookupSegs follows segs as plain keys. * and ** are not wildcards here.
func (v Value) LookupSegs(segs []Seg) (Value, bool) {
	cur := v
	for _, seg := range segs {
		if cur.Kind != KindMap {
			return Value{}, false
		}
		next, ok := cur.Get(seg.Key)
		if !ok {
			return Value{}, false
		}
		cur = next
	}
	return cur, true
}

// Seg is one dot-separated segment of a select or where path.
// Quoted means the segment was written in double quotes, so it is a literal key even when it is *, **, or $.
type Seg struct {
	Key    string
	Quoted bool
}

// ParsePath splits a dot-separated path. A segment in double quotes is one key and may contain dots.
// A quote that is not closed, or text after a closing quote, is an error.
func ParsePath(path string) ([]Seg, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	var out []Seg
	var cur strings.Builder
	quoted := false
	flush := func() {
		out = append(out, Seg{Key: cur.String(), Quoted: quoted})
		cur.Reset()
		quoted = false
	}
	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch {
		case ch == '"' && cur.Len() == 0 && !quoted:
			end := strings.IndexByte(path[i+1:], '"')
			if end < 0 {
				return nil, message.New(message.QuoteOpen)
			}
			cur.WriteString(path[i+1 : i+1+end])
			quoted = true
			i += end + 1
			if i+1 < len(path) && path[i+1] != '.' {
				return nil, message.New(message.QuoteTail)
			}
		case ch == '.':
			flush()
		default:
			cur.WriteByte(ch)
		}
	}
	flush()
	return out, nil
}

// KindName is the name of k used in kind conditions: mapping, sequence, string, number, bool, or null.
func KindName(k Kind) string {
	switch k {
	case KindMap:
		return "mapping"
	case KindSeq:
		return "sequence"
	case KindString:
		return "string"
	case KindNumber:
		return "number"
	case KindBool:
		return "bool"
	default:
		return "null"
	}
}

// KindNames lists the names KindName returns, in the order written in the documentation.
var KindNames = []string{"mapping", "sequence", "string", "number", "bool", "null"}

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
