package rules

import (
	"strings"
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
)

// A reference outside {{ }} in a value that runn expands only inside {{ }} reports unexpanded-ref as warning on that value's line.
// include.vars, a request body with ${ENV}, a path key, a header, exec.command, a steps[...] index, dump.out, root vars, and the steps of an inline include are all such values.
// The finding names the reference, cut at the first character that cannot be part of it.
func TestUnexpandedRef(t *testing.T) {
	src := `
desc: d
vars:
  username: alice
  copy: vars.username
runners:
  req: https://example.com
steps:
  create:
    req:
      /items:
        post:
          body:
            application/json:
              owner_id: vars.owners.${ENV}.id
  users:
    req:
      /users/vars.username/items:
        get:
          headers:
            Authorization: Bearer steps.create.res.body.token
          body: null
  child:
    include:
      path: child.yml
      vars:
        username: vars.username
        list:
          - previous.res.body.id
  run:
    exec:
      command: echo current.stdout && echo steps['users'].res.status
  out:
    dump:
      expr: vars.username
      out: parent.vars.file
  inline:
    include:
      steps:
        - exec:
            command: echo vars.username
`
	got := lint(t, src)
	found := findRule(got, "unexpanded-ref")
	want := map[int]string{
		5:  "vars.username",
		15: "vars.owners.${ENV}.id",
		18: "vars.username",
		21: "steps.create.res.body.token",
		27: "vars.username",
		29: "previous.res.body.id",
		32: "current.stdout",
		36: "parent.vars.file",
		41: "vars.username",
	}
	if len(found) != len(want) {
		t.Fatalf("want %d findings, got %+v", len(want), found)
	}
	for _, f := range found {
		ref, ok := want[f.Line]
		if !ok || f.Severity != finding.Warning || f.Match != message.English(message.RefUnexpanded, ref) {
			t.Fatalf("unexpected finding %+v (want %q on line %d)", f, ref, f.Line)
		}
	}
	if f := found[0]; f.Message != "vars.username が {{ }} の外にあります。runn は展開せず、この文字列のまま使います" {
		t.Fatalf("%+v", f)
	}
}

// Expressions (test, if, bind, dump, loop.until, loop.count), desc, labels, and references inside {{ }} are not reported.
// Neither are file names such as vars.yml, hosts and jq paths where the name follows a dot, a shell variable such as $vars.x,
// an e-mail address, a root name without a member, the word followed by a space, or a string whose {{ is not closed.
func TestUnexpandedRefNotReported(t *testing.T) {
	src := `
desc: uses vars.username in the body
labels:
  - vars.label
if: vars.enabled
vars:
  username: alice
  data: json://vars.json
loop:
  count: 2
  until: vars.done
runners:
  req: https://api.vars.example.com
steps:
  - req:
      /users/{{ vars.username }}:
        post:
          body:
            application/json:
              name: '{{ vars.username }}'
              mixed: 'id-{{ steps[0].res.body.id }}-{{ vars.username }}'
              mail: parent.name@example.com
              word: vars and steps. are runn words
              bare: vars
    desc: posts vars.username
    if: vars.username != ""
    test: current.res.status == 200
    bind:
      user: current.res.body.user
    loop:
      count: len(vars.items)
      until: current.res.status == 200
  - exec:
      command: jq '.vars.username' vars.yml && echo $vars.x
  - dump: steps[0].res.body
  - dump:
      expr: steps[0].res.body
      out: steps.json
  - include:
      path: steps.yml
      skipTest: vars.skip
      force: vars.force
      vars:
        username: '{{ vars.username }}'
`
	got := lint(t, src)
	if found := findRule(got, "unexpanded-ref"); len(found) > 0 {
		t.Fatalf("%+v", found)
	}
	unclosed := lint(t, `
desc: d
steps:
  - exec:
      command: echo {{ vars.username
`)
	if hasRule(unclosed, "unexpanded-ref") || !hasRule(unclosed, "expr-syntax") {
		t.Fatalf("%+v", unclosed)
	}
}

// unexpandedRefIn finds the first reference outside {{ }} and cuts it at a path, query, or operator character.
func TestUnexpandedRefIn(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"vars.a", "vars.a"},
		{"/x/vars.a/y", "vars.a"},
		{"?q=vars.a&b=1", "vars.a"},
		{"{vars.a}", "vars.a}"},
		{"${vars.a}", "vars.a}"},
		{"steps[0].res", "steps[0].res"},
		{"steps['a b'].res", "steps['a b'].res"},
		{"'vars.a'", "vars.a"},
		{"{{ vars.a }} needs.b.c.", "needs.b.c"},
		{"vars.a.b.YML", "vars.a.b.YML"},
		{"vars.YML", ""},
		{"steps[x]", ""},
		{"vars.", ""},
		{"vars.1", ""},
		{"xvars.a", ""},
		{"no reference", ""},
	}
	for _, c := range cases {
		got, ok := unexpandedRefIn(c.in)
		if got != c.want || ok != (c.want != "") {
			t.Fatalf("%q: got %q %v, want %q", c.in, got, ok, c.want)
		}
	}
	if strings.Contains(message.English(message.RefUnexpanded, "vars.a"), "%") {
		t.Fatal("message verb not filled")
	}
}
