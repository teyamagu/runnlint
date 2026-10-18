package rules

import (
	"regexp"
	"strings"

	"github.com/k1LoW/duration"
	"github.com/teyamagu/runnlint/internal/runbook"
)

var numOnlyRe = regexp.MustCompile(`^[0-9.]+$`)

func validDuration(v runbook.Value) bool {
	switch v.Kind {
	case runbook.KindNumber:
		return true
	case runbook.KindString:
		if strings.TrimSpace(v.String) == "" {
			return false
		}
		if runbook.ContainsTemplate(v.String) {
			return true
		}
		return parseDuration(strings.TrimSpace(v.String)) == nil
	default:
		return false
	}
}

func parseDuration(s string) error {
	if numOnlyRe.MatchString(s) {
		s += "sec"
	}
	_, err := duration.Parse(s)
	return err
}

func boolOrString(v runbook.Value) bool {
	return v.Kind == runbook.KindBool || v.Kind == runbook.KindString
}

func boolOrTemplate(v runbook.Value) bool {
	if v.Kind == runbook.KindBool {
		return true
	}
	return v.Kind == runbook.KindString && strings.Contains(v.String, "{{")
}

func nonEmptyString(v runbook.Value) bool {
	return v.Kind == runbook.KindString && strings.TrimSpace(v.String) != ""
}
