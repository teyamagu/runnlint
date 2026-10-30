package message

import (
	"errors"
	"testing"
)

// Registering an ID twice panics with the duplicate ID in the message.
func TestCovSpecRejectsDuplicateID(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil || r != "duplicate message usage" {
			t.Fatalf("recover = %v", r)
		}
	}()
	spec("usage", "x", "y")
}

// Text renders an unknown ID as the ID itself, regardless of language or arguments.
func TestCovTextUnknownID(t *testing.T) {
	if got := Text(JA, ID("no-such-id"), 1); got != "no-such-id" {
		t.Fatalf("%s", got)
	}
	if got := Text(EN, ID("no-such-id")); got != "no-such-id" {
		t.Fatalf("%s", got)
	}
}

// Catalog returns a copy with the same sentences. Changing the copy does not change what Text renders.
func TestCovCatalogIsCopy(t *testing.T) {
	all := Catalog()
	if len(all) != len(catalog) {
		t.Fatalf("len = %d want %d", len(all), len(catalog))
	}
	pair, ok := all[StepsMissing]
	if !ok || pair[0] != "steps がありません" || pair[1] != "steps is missing" {
		t.Fatalf("%v", pair)
	}
	all[StepsMissing] = [2]string{"changed", "changed"}
	if got := Text(JA, StepsMissing); got != "steps がありません" {
		t.Fatalf("catalog changed through the copy: %s", got)
	}
}

// Unwrap on a wrapped error returns the cause, so errors.Is finds it. A prefixed error unwraps to the error it wraps.
func TestCovUnwrap(t *testing.T) {
	cause := errors.New("boom")
	wrapped := Wrap(RulesFileRead, cause)
	if !errors.Is(wrapped, cause) {
		t.Fatal("wrapped error does not unwrap to its cause")
	}
	if errors.Unwrap(New(RuleIDEmpty)) != nil {
		t.Fatal("an error without a cause unwraps to something")
	}
	inner := New(RuleIDEmpty)
	pre := Prefix("rules.yaml", inner)
	if errors.Unwrap(pre) != inner {
		t.Fatal("prefixed error does not unwrap to the inner error")
	}
}

// A prefixed error renders as prefix, colon, and the inner sentence in the requested language. Error() uses Japanese.
func TestCovPrefixLocalize(t *testing.T) {
	pre := Prefix("rules.yaml", New(RuleIDEmpty))
	if got := Localize(EN, pre); got != "rules.yaml: a rule has an empty id" {
		t.Fatalf("%s", got)
	}
	if got := Localize(JA, pre); got != "rules.yaml: id が空のルールがあります" {
		t.Fatalf("%s", got)
	}
	if got := pre.Error(); got != "rules.yaml: id が空のルールがあります" {
		t.Fatalf("%s", got)
	}
	display, match := Pair(JA, pre)
	if display != "rules.yaml: id が空のルールがあります" || match != "rules.yaml: a rule has an empty id" {
		t.Fatalf("%s / %s", display, match)
	}
}

// Prefix of nil is nil. Localize of nil is the empty string. An error from another package renders as its own Error().
func TestCovNilAndForeignErrors(t *testing.T) {
	if Prefix("x", nil) != nil {
		t.Fatal("Prefix(nil) is not nil")
	}
	if got := Localize(EN, nil); got != "" {
		t.Fatalf("%q", got)
	}
	foreign := errors.New("plain")
	if got := Localize(EN, foreign); got != "plain" {
		t.Fatalf("%s", got)
	}
	if got := Localize(JA, Prefix("p", foreign)); got != "p: plain" {
		t.Fatalf("%s", got)
	}
}
