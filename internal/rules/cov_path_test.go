package rules

import "testing"

// A gRPC runner whose protos list names a file that does not exist reports local-path for that element.
func TestLocalPathListElement(t *testing.T) {
	got := lint(t, `
desc: d
runners:
  greq:
    addr: localhost:50051
    protos:
      - missing.proto
steps:
  - greq:
      pkg.Svc/Method:
        message:
          name: alice
  - test: steps[0].res.status == 0
`)
	found := findRule(got, "local-path")
	if len(found) != 1 || found[0].Line != 7 {
		t.Fatalf("%+v", got)
	}
}
