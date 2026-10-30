package rules

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/runbook"
)

// A blank interval reports book-shape as not a duration. A number-only string such as "3" is read as seconds and accepted.
func TestDurationBlankAndNumberOnly(t *testing.T) {
	requireMessage(t, "desc: d\ninterval: \"  \"\nsteps:\n  - test: \"true\"\n", "book-shape", "interval は時間です")
	rejectMessage(t, "desc: d\ninterval: \"3\"\nsteps:\n  - test: \"true\"\n", "book-shape", "interval")
	if validDuration(runbook.Value{Kind: runbook.KindString, String: " "}) {
		t.Fatal("a blank string is not a duration")
	}
	if !validDuration(runbook.Value{Kind: runbook.KindString, String: "3"}) {
		t.Fatal("a number-only string is a duration in seconds")
	}
}

// A value that is neither a number nor a string, such as a bool, is not a duration.
func TestDurationOtherKind(t *testing.T) {
	requireMessage(t, "desc: d\ninterval: true\nsteps:\n  - test: \"true\"\n", "book-shape", "interval は時間です")
	if validDuration(runbook.Value{Kind: runbook.KindBool, Bool: true}) {
		t.Fatal("a bool is a duration")
	}
}
