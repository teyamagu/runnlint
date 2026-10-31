package rules

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/finding"
)

func findRule(findings []finding.Finding, id string) []finding.Finding {
	var out []finding.Finding
	for _, f := range findings {
		if f.Rule == id {
			out = append(out, f)
		}
	}
	return out
}

// A runner in runners: that no step calls reports unused-runner as warning on the runner's line. A runner that a step calls does not.
func TestUnusedRunnerTopLevel(t *testing.T) {
	got := lint(t, `
desc: x
runners:
  req: https://example.com
  db: mysql://root:pass@localhost:3306/app
steps:
  - req:
      /health:
        get:
          body: null
`)
	found := findRule(got, "unused-runner")
	if len(found) != 1 || found[0].Severity != finding.Warning || found[0].Line != 5 {
		t.Fatalf("%+v", got)
	}
	if errs := errorsOf(got); len(errs) > 0 {
		t.Fatalf("%+v", errs)
	}
}

// A runner defined with runner: on a step that no later step calls reports unused-runner. One that a later step calls does not.
func TestUnusedRunnerStepLevel(t *testing.T) {
	got := lint(t, `
desc: x
steps:
  - runner:
      sc: ssh://user@example.com:22
      other: ssh://user@example.org:22
  - sc:
      command: hostname
`)
	found := findRule(got, "unused-runner")
	if len(found) != 1 || found[0].Line != 6 {
		t.Fatalf("%+v", got)
	}
}

// A runner that only an inline-include child step calls is used. A runner with a reserved name reports runner-reserved only, not unused-runner.
func TestUnusedRunnerNotReported(t *testing.T) {
	inline := lint(t, `
desc: inline include
runners:
  req: https://example.com
steps:
  - include:
      steps:
        - req:
            /health:
              get:
                body: null
`)
	if hasRule(inline, "unused-runner") {
		t.Fatalf("%+v", inline)
	}
	reserved := lint(t, `
desc: reserved
runners:
  test: https://example.com
steps:
  - exec:
      command: echo ok
`)
	if !hasRule(reserved, "runner-reserved") || hasRule(reserved, "unused-runner") {
		t.Fatalf("%+v", reserved)
	}
}

// The default config leaves require-test-step off, so a runbook with no step of only test is not reported.
func TestRequireTestStepDefaultOff(t *testing.T) {
	got := lint(t, `
desc: x
runners:
  req: https://example.com
steps:
  - req:
      /health:
        get:
          body: null
    test: current.res.status == 200
`)
	if hasRule(got, "require-test-step") {
		t.Fatalf("%+v", got)
	}
}

// Setting severity to info, warning, or error in the config reports require-test-step at that severity. severity off reports nothing.
func TestRequireTestStepFollowsConfiguredSeverity(t *testing.T) {
	const src = "desc: x\nsteps:\n  - exec:\n      command: echo ok\n"
	for _, sev := range []finding.Severity{finding.Info, finding.Warning, finding.Error} {
		got := lintConfig(t, "version: 1\nrules:\n  - id: require-test-step\n    severity: "+string(sev)+"\n", src)
		found := findRule(got, "require-test-step")
		if len(found) != 1 || found[0].Severity != sev {
			t.Fatalf("%s: %+v", sev, got)
		}
	}
	off := lintConfig(t, "version: 1\nrules:\n  - id: require-test-step\n    severity: off\n", src)
	if hasRule(off, "require-test-step") {
		t.Fatalf("%+v", off)
	}
}

// With require-test-step set to info, a runbook that can run on its own and has no step with only test reports it as info on the steps line.
// Tests attached to request steps do not count as a verification step.
func TestRequireTestStep(t *testing.T) {
	got := lintRequireTestStep(t, `
desc: x
runners:
  req: https://example.com
steps:
  - req:
      /health:
        get:
          body: null
    test: current.res.status == 200
`)
	found := findRule(got, "require-test-step")
	if len(found) != 1 || found[0].Severity != finding.Info || found[0].Line != 5 {
		t.Fatalf("%+v", got)
	}
	if errs := errorsOf(got); len(errs) > 0 {
		t.Fatalf("%+v", errs)
	}
}

// A step with only test, with or without desc, if, dump, or bind, is a verification step, so the runbook is not reported.
// A verification step inside an inline include also counts.
func TestRequireTestStepSatisfied(t *testing.T) {
	mapped := lintRequireTestStep(t, `
desc: x
runners:
  req: https://example.com
steps:
  health:
    req:
      /health:
        get:
          body: null
  verify:
    desc: the service answers
    if: "true"
    test: steps.health.res.status == 200
`)
	if hasRule(mapped, "require-test-step") {
		t.Fatalf("%+v", mapped)
	}
	inline := lintRequireTestStep(t, `
desc: inline include
steps:
  - include:
      steps:
        - exec:
            command: echo ok
        - test: steps[0].exit_code == 0
`)
	if hasRule(inline, "require-test-step") {
		t.Fatalf("%+v", inline)
	}
}

// A runbook that runs only when included is verified by its caller, so it is not reported. One that can also run on its own is.
// A runbook with no steps is left to require-steps.
func TestRequireTestStepSkipsIncludedAndEmpty(t *testing.T) {
	for _, cond := range []string{"included", "included == true", "included && vars.ready"} {
		got := lintRequireTestStep(t, "desc: part\nif: "+cond+"\nvars:\n  ready: true\nsteps:\n  - exec:\n      command: echo ok\n")
		if hasRule(got, "require-test-step") {
			t.Fatalf("if: %s\n%+v", cond, got)
		}
	}
	either := lintRequireTestStep(t, `
desc: either
if: included || true
steps:
  - exec:
      command: echo ok
`)
	if !hasRule(either, "require-test-step") {
		t.Fatalf("%+v", either)
	}
	empty := lintRequireTestStep(t, `
desc: empty
steps: []
`)
	if hasRule(empty, "require-test-step") || !hasRule(empty, "require-steps") {
		t.Fatalf("%+v", empty)
	}
}
