package runbook

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Style は steps の並び。
type Style int

const (
	StyleMissing Style = iota
	StyleList
	StyleMap
)

// RunnerKind は runner の種別。
type RunnerKind string

const (
	KindHTTP    RunnerKind = "http"
	KindGRPC    RunnerKind = "grpc"
	KindDB      RunnerKind = "db"
	KindSSH     RunnerKind = "ssh"
	KindCDP     RunnerKind = "cdp"
	KindInclude RunnerKind = "include"
	KindAgent   RunnerKind = "agent"
	KindUnknown RunnerKind = "unknown"
)

// Book は1つの runbook。include のインライン steps も Book になる。
type Book struct {
	File          string
	Dir           string
	Root          Value
	Style         Style
	Steps         []Step
	Runners       []Runner
	Parent        *Book
	ParentRunners []Runner
	Problems      []Problem
}

// Problem はモデル化の時点で分かる構造の問題。
type Problem struct {
	Node    *yaml.Node
	Message string
}

// Step は1つの step。
type Step struct {
	Index    int
	Name     string
	NameNode *yaml.Node
	Value    Value
	Defer    bool
	Inline   *Book
}

// Runner は runners または runner: で定義された runner。
type Runner struct {
	Name     string
	NameNode *yaml.Node
	Value    Value
	Kind     RunnerKind
	FromStep int
}

// SetLocation は診断と相対パス解決に使うファイル位置を全書へ書く。
func (b *Book) SetLocation(file, dir string) {
	b.Walk(func(book *Book) {
		book.File = file
		book.Dir = dir
	})
}

// VarNames は vars のキー。
func (b *Book) VarNames() map[string]struct{} {
	names := map[string]struct{}{}
	vars, ok := b.Root.Get("vars")
	if !ok || vars.Kind != KindMap {
		return names
	}
	for _, e := range vars.Map {
		names[e.Key] = struct{}{}
	}
	return names
}

// StepByName は map 形式の step 名を探す。
func (b *Book) StepByName(name string) (Step, bool) {
	for _, s := range b.Steps {
		if s.Name == name {
			return s, true
		}
	}
	return Step{}, false
}

// RunnersAt は stepIndex より前までに使える runner。
func (b *Book) RunnersAt(stepIndex int) map[string]Runner {
	out := map[string]Runner{}
	for _, r := range b.ParentRunners {
		out[r.Name] = r
	}
	for _, r := range b.Runners {
		out[r.Name] = r
	}
	for _, s := range b.Steps {
		if s.Index >= stepIndex {
			break
		}
		def, ok := s.Value.Get("runner")
		if !ok || def.Kind != KindMap {
			continue
		}
		for _, e := range def.Map {
			out[e.Key] = Runner{
				Name:     e.Key,
				NameNode: e.KeyNode,
				Value:    e.Value,
				Kind:     DetectKind(e.Value),
				FromStep: s.Index,
			}
		}
	}
	return out
}

// Walk はインライン include を含む全書を訪れる。
func (b *Book) Walk(fn func(*Book)) {
	if b == nil {
		return
	}
	fn(b)
	for i := range b.Steps {
		if b.Steps[i].Inline != nil {
			b.Steps[i].Inline.Walk(fn)
		}
	}
}

// LoopEnabled は runbook 自身が loop を持つか。
func (b *Book) LoopEnabled() bool {
	_, ok := b.Root.Get("loop")
	return ok
}

func buildBook(root Value, dir string, parent *Book, parentStep int) *Book {
	b := &Book{Dir: dir, Root: root, Parent: parent, Style: StyleMissing}
	if parent != nil && parentStep >= 0 {
		for _, r := range parent.RunnersAt(parentStep) {
			b.ParentRunners = append(b.ParentRunners, r)
		}
	}
	if runners, ok := root.Get("runners"); ok && runners.Kind == KindMap {
		for _, e := range runners.Map {
			b.Runners = append(b.Runners, Runner{
				Name:     e.Key,
				NameNode: e.KeyNode,
				Value:    e.Value,
				Kind:     DetectKind(e.Value),
				FromStep: -1,
			})
		}
	}
	steps, ok := root.Get("steps")
	if !ok {
		return b
	}
	switch steps.Kind {
	case KindSeq:
		b.Style = StyleList
		for i, el := range steps.Seq {
			if el.Kind != KindMap {
				b.Problems = append(b.Problems, Problem{Node: el.Node, Message: "steps の各要素は mapping です"})
				continue
			}
			b.Steps = append(b.Steps, makeStep(b, i, "", nil, el))
		}
	case KindMap:
		b.Style = StyleMap
		for i, e := range steps.Map {
			if e.Value.Kind != KindMap {
				b.Problems = append(b.Problems, Problem{Node: e.Value.Node, Message: "step " + e.Key + " の値は mapping です"})
				continue
			}
			b.Steps = append(b.Steps, makeStep(b, i, e.Key, e.KeyNode, e.Value))
		}
	default:
		b.Problems = append(b.Problems, Problem{Node: steps.Node, Message: "steps は sequence または mapping です"})
	}
	return b
}

func makeStep(parent *Book, index int, name string, nameNode *yaml.Node, value Value) Step {
	s := Step{Index: index, Name: name, NameNode: nameNode, Value: value}
	if d, ok := value.Get("defer"); ok && d.Kind == KindBool && d.Bool {
		s.Defer = true
	}
	if inc, ok := value.Get("include"); ok && inc.Kind == KindMap {
		if _, hasSteps := inc.Get("steps"); hasSteps {
			s.Inline = buildBook(inc, parent.Dir, parent, index)
		}
	}
	return s
}

// DetectKind は runn が runner 定義から種別を決める順に合わせる。
func DetectKind(v Value) RunnerKind {
	switch v.Kind {
	case KindString:
		return detectStringKind(v.String)
	case KindMap:
		return detectMapKind(v)
	default:
		return KindUnknown
	}
}

func detectStringKind(s string) RunnerKind {
	switch {
	case strings.HasPrefix(s, "https://"), strings.HasPrefix(s, "http://"):
		return KindHTTP
	case strings.HasPrefix(s, "grpc://"):
		return KindGRPC
	case isCDPAddr(s):
		return KindCDP
	case strings.HasPrefix(s, "ssh://"):
		return KindSSH
	}
	if ContainsTemplate(s) {
		return KindUnknown
	}
	if i := strings.Index(s, "://"); i > 0 {
		if _, ok := dbSchemes[s[:i]]; ok {
			return KindDB
		}
		return KindUnknown
	}
	return KindDB
}

var dbSchemes = map[string]struct{}{
	"postgres": {}, "postgresql": {}, "pg": {},
	"mysql": {}, "my": {},
	"sqlite": {}, "sqlite3": {}, "sq": {},
	"spanner": {}, "sp": {},
	"file": {},
}

func detectMapKind(v Value) RunnerKind {
	if hasNonEmptyString(v, "endpoint") {
		return KindHTTP
	}
	if addr, ok := stringValue(v, "addr"); ok && addr != "" && !isCDPAddr(addr) {
		return KindGRPC
	}
	if hasNonEmptyString(v, "dsn") {
		return KindDB
	}
	if hasNonEmptyString(v, "host") || hasNonEmptyString(v, "hostname") {
		return KindSSH
	}
	if flags, ok := v.Get("flags"); ok && flags.Kind == KindMap && len(flags.Map) > 0 {
		return KindCDP
	}
	if addr, ok := stringValue(v, "addr"); ok && isCDPAddr(addr) {
		return KindCDP
	}
	if hasNonEmptyString(v, "timeout") {
		return KindCDP
	}
	if hasNonEmptyString(v, "path") {
		return KindInclude
	}
	if hasNonEmptyString(v, "agent") {
		return KindAgent
	}
	return KindUnknown
}

func isCDPAddr(addr string) bool {
	return strings.HasPrefix(addr, "cdp://") || strings.HasPrefix(addr, "chrome://")
}

func hasNonEmptyString(v Value, key string) bool {
	s, ok := stringValue(v, key)
	return ok && strings.TrimSpace(s) != ""
}

func stringValue(v Value, key string) (string, bool) {
	child, ok := v.Get(key)
	if !ok || child.Kind != KindString {
		return "", false
	}
	return child.String, true
}
