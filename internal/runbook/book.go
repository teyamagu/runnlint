package runbook

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Style is how steps are arranged.
type Style int

const (
	StyleMissing Style = iota
	StyleList
	StyleMap
)

// RunnerKind is the kind of a runner.
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

// Book is one runbook. Inline steps of an include are also a Book.
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
	// indexed is set once Steps will not grow. RunnersAt and StepByName then reuse one index.
	indexed bool
	// runnersBefore[i] is the runners visible before step i. The last entry is the set after every step.
	// A map is shared until a step defines runner:. Callers must not modify it.
	runnersBefore []map[string]Runner
	stepsByName   map[string]Step
}

// ProblemKind identifies a structural problem found while building a book.
// The display sentence is chosen later, when the language is known.
type ProblemKind int

const (
	// ProblemStepsElem means a list element of steps is not a mapping.
	ProblemStepsElem ProblemKind = iota + 1
	// ProblemStepValue means a named step's value is not a mapping. Name is the step name.
	ProblemStepValue
	// ProblemStepsKind means steps is neither a sequence nor a mapping.
	ProblemStepsKind
)

// Problem is a structural problem known while building the model.
type Problem struct {
	Node *yaml.Node
	Kind ProblemKind
	Name string
}

// Step is one step.
type Step struct {
	Index    int
	Name     string
	NameNode *yaml.Node
	Value    Value
	Defer    bool
	Inline   *Book
}

// Runner is a runner defined in runners or in runner:.
type Runner struct {
	Name     string
	NameNode *yaml.Node
	Value    Value
	Kind     RunnerKind
	FromStep int
}

// SetLocation writes the file location used for diagnostics and relative paths onto every book.
func (b *Book) SetLocation(file, dir string) {
	b.Walk(func(book *Book) {
		book.File = file
		book.Dir = dir
	})
}

// VarNames returns the keys of vars.
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

// StepByName looks up a step name used by map-style steps. The first step with that name wins.
func (b *Book) StepByName(name string) (Step, bool) {
	if b == nil {
		return Step{}, false
	}
	if !b.indexed {
		for _, s := range b.Steps {
			if s.Name == name {
				return s, true
			}
		}
		return Step{}, false
	}
	b.ensureIndexes()
	s, ok := b.stepsByName[name]
	return s, ok
}

// RunnersAt returns the runners available before stepIndex.
// After the book is built, the result is cached. The returned map must not be modified.
func (b *Book) RunnersAt(stepIndex int) map[string]Runner {
	if b == nil {
		return nil
	}
	if !b.indexed {
		return b.collectRunners(stepIndex)
	}
	b.ensureIndexes()
	if len(b.runnersBefore) == 0 {
		return map[string]Runner{}
	}
	// When the step's index matches its position, that slot holds the runners before it.
	if stepIndex >= 0 && stepIndex < len(b.Steps) && b.Steps[stepIndex].Index == stepIndex {
		return b.runnersBefore[stepIndex]
	}
	for i := range b.Steps {
		if b.Steps[i].Index >= stepIndex {
			return b.runnersBefore[i]
		}
	}
	return b.runnersBefore[len(b.Steps)]
}

// collectRunners walks earlier steps. It is used only while the book is still being built.
func (b *Book) collectRunners(stepIndex int) map[string]Runner {
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
		addStepRunners(out, s)
	}
	return out
}

// ensureIndexes builds the runner and step-name indexes once. A step that does not define runner: keeps the previous map.
func (b *Book) ensureIndexes() {
	if b.runnersBefore != nil {
		return
	}
	current := map[string]Runner{}
	for _, r := range b.ParentRunners {
		current[r.Name] = r
	}
	for _, r := range b.Runners {
		current[r.Name] = r
	}
	b.runnersBefore = make([]map[string]Runner, len(b.Steps)+1)
	for i := range b.Steps {
		b.runnersBefore[i] = current
		def, ok := b.Steps[i].Value.Get("runner")
		if !ok || def.Kind != KindMap {
			continue
		}
		next := make(map[string]Runner, len(current)+len(def.Map))
		for name, r := range current {
			next[name] = r
		}
		addStepRunners(next, b.Steps[i])
		current = next
	}
	b.runnersBefore[len(b.Steps)] = current
	b.stepsByName = make(map[string]Step, len(b.Steps))
	for _, s := range b.Steps {
		if _, ok := b.stepsByName[s.Name]; ok {
			continue
		}
		b.stepsByName[s.Name] = s
	}
}

func addStepRunners(out map[string]Runner, s Step) {
	def, ok := s.Value.Get("runner")
	if !ok || def.Kind != KindMap {
		return
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

// Walk visits every book, including inline includes.
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

// LoopEnabled reports whether this runbook itself has a loop.
func (b *Book) LoopEnabled() bool {
	_, ok := b.Root.Get("loop")
	return ok
}

func buildBook(root Value, dir string, parent *Book, parentStep int) *Book {
	b := &Book{Dir: dir, Root: root, Parent: parent, Style: StyleMissing}
	defer func() { b.indexed = true }()
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
				b.Problems = append(b.Problems, Problem{Node: el.Node, Kind: ProblemStepsElem})
				continue
			}
			b.Steps = append(b.Steps, makeStep(b, i, "", nil, el))
		}
	case KindMap:
		b.Style = StyleMap
		for i, e := range steps.Map {
			if e.Value.Kind != KindMap {
				b.Problems = append(b.Problems, Problem{Node: e.Value.Node, Kind: ProblemStepValue, Name: e.Key})
				continue
			}
			b.Steps = append(b.Steps, makeStep(b, i, e.Key, e.KeyNode, e.Value))
		}
	default:
		b.Problems = append(b.Problems, Problem{Node: steps.Node, Kind: ProblemStepsKind})
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

// DetectKind follows the order runn uses to decide a kind from a runner definition.
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
		if DBScheme(s[:i]) {
			return KindDB
		}
		return KindUnknown
	}
	return KindDB
}

// DBScheme reports whether scheme is a DSN scheme runn treats as a database runner.
func DBScheme(scheme string) bool {
	_, ok := dbSchemes[scheme]
	return ok
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
