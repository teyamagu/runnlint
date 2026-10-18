package runbook

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kind は YAML ノードの種別。
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindNumber
	KindString
	KindSeq
	KindMap
)

// Value は位置情報を残した YAML 値。
type Value struct {
	Kind   Kind
	Node   *yaml.Node
	Bool   bool
	Number string
	String string
	Seq    []Value
	Map    []Entry
}

// Entry は mapping の1要素。順序はファイルの出現順。
type Entry struct {
	Key     string
	KeyNode *yaml.Node
	Value   Value
}

// Dup は同一 mapping 内の明示的なキー重複。
type Dup struct {
	Key  string
	Node *yaml.Node
}

// Get はキーに対応する値を返す。
func (v Value) Get(key string) (Value, bool) {
	for _, e := range v.Map {
		if e.Key == key {
			return e.Value, true
		}
	}
	return Value{}, false
}

// Lookup はワイルドカードを含まない相対パスを辿る。
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

// Len は文字列、sequence、mapping の長さ。
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

// Display は診断メッセージ用の短い表現。
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

// Pos は 1 始まりの行と列。ノードが無ければ 1,1。
func Pos(n *yaml.Node) (int, int) {
	if n == nil || n.Line == 0 {
		return 1, 1
	}
	return n.Line, n.Column
}
