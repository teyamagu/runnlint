# Test design

[日本語](../ja/test-design.md)

The checking contracts split into the [rules file](configuration.md), [following runn](runn-follow.md), and the execution flow (command, file collection, rules, yamllint, output, exit code). Tests split on those same boundaries.

From here on, add a failing test before implementing a change. The existing contracts written here are confirmed by tests that fix the current implementation as the specification.

## Binary foundation

The command parses flags and passes them to `app.Run`. The exit code is `1` when there is at least one `error`, `2` when configuration or execution fails, and `0` otherwise. `warning` and `info` alone do not fail the run. With `--strict`, a `warning` also exits `1`.

| ID | Contract | Test |
| --- | --- | --- |
| BIN-01 | `-h` is 0. An unknown flag is 2 | `cmd/runnlint/main_test.go` |
| BIN-02 | `--print-config` and `--print-yamllint-config` print the built-in definitions and exit 0 | same |
| BIN-03 | `--list-rules` prints `id`, severity, and `builtin:` or `check` | same |
| BIN-04 | The command returns the check's exit code as it is. An invalid config is 2 | same |
| BIN-05 | A directory walk skips non-runbooks. Broken YAML that has `steps:` or `runners:` is kept | `internal/app/contract_test.go` |
| BIN-06 | `ignore` drops only files collected by a directory walk. An argument file is checked even when the pattern is a file path. A literal matches the walked path and everything under it. A trailing `/` is stripped only when looking underneath, so `examples/fixtures/openapi.yml/` does not match `examples/fixtures/openapi.yml`. A base name matches only when the whole pattern equals the base name: `*.gen.yml` matches, and `dir/*.yml` is only the files directly under `dir`. `**` matches only the walked path. A pattern with `./`, or a pattern that contains `/`, does not match when the walked-path string differs. `openapi.yml` and `*.gen.yml` also match an absolute path by base name | `internal/app/contract_test.go`, `internal/config/config_test.go` |
| BIN-07 | Warnings alone, or info alone, are 0. An error is 1. An unknown output format and a missing path are 2 | same |
| BIN-08 | The shape of text, github, and json, and sorting by file, line, column, and rule name | `internal/report/report_test.go` |
| BIN-09 | Inheritance, `extends: none`, override of the same id, validation, discovery order, and JSON | `internal/config/contract_test.go` |
| BIN-10 | yamllint parsable output, and resolution of the config path | `internal/yamllint/yamllint_test.go` |
| BIN-11 | YAML duplicate keys, aliases, merge, multiple documents, and runner kinds | `internal/runbook/parse_test.go` |
| BIN-12 | Read keys and CDP actions from the schema and `cdpfn.go`. Fail on a difference from the latest release | `internal/runnspec` |
| BIN-13 | The latest stable Go, and dependency licenses | `internal/gotoolchain`, `internal/license` |
| BIN-14 | The product version is semver. `--version` prints `runnlint X.Y.Z`. `CHANGELOG.md` and `CHANGELOG.en.md` have the same `## [X.Y.Z]` headings, including a section for the current version | `internal/version`, `cmd/runnlint/main_test.go` |
| BIN-15 | `--write-baseline` writes every diagnostic and exits 0. `--baseline` matches by count of file, rule, and message, and does not print an existing diagnostic when lines shift. The exit code is decided only by new diagnostics. Built-in diagnostic messages are recorded in English and matched against that English text even when the display is Japanese. A declarative sentence is stored as written. The format is version 2, and version 1 fails with exit code 2. An unreadable baseline is 2 | `internal/baseline`, `internal/app/contract_test.go`, `cmd/runnlint/main_test.go` |
| BIN-16 | Discovery selects only `.runnlint-runn.yaml`, then `.runnlint-runn.yml`. It does not read `.runnlint.yaml` and similar names, and it does not warn. A found file whose shape is wrong is 2. An unknown key below the top level is warned about and still read. `--config` warns about unknown keys and still reads | `internal/config/contract_test.go`, `internal/app/contract_test.go` |
| BIN-17 | `--strict` exits 1 on warnings alone. Info alone stays 0. Warnings hidden by `--baseline` do not count | `cmd/runnlint/main_test.go`, `internal/app/contract_test.go` |

BIN-12 and BIN-13 fix this repository's update policy, not a product execution path. Importing a notation difference and turning it into a PR is `.github/workflows/runn-follow.yml` and `cmd/runnfollow`. Whether there is a difference is decided by `internal/runnspec.Changed`.

## Built-in rules and custom rules

The default config enables every registered built-in rule. A declarative rule has no `uses`. A rule with `severity: off` emits no diagnostic. `var-ref`'s `options.ignore` and `http-request`'s `options.require-leading-slash` are the extra settings of built-in rules.

| ID | Contract | Test |
| --- | --- | --- |
| RULE-01 | The default `uses` values match the registered built-ins, and declarative rules are `check` | `internal/rules/catalog_test.go` |
| RULE-02 | Valid HTTP, DB, map steps, a dynamic runner, an inline include, and an anchor | `lint_test.go` `TestValidRunbooks` `TestAnchorMerge` |
| RULE-03 | Detection and severity of representative built-ins | `lint_test.go` `TestRunnSpecificRules` |
| RULE-04 | Built-ins that were not covered. `unknown-key` and `x-`, `book-shape`, `require-steps`, `defer-experimental`, `ssh-request`, `cdp-request`, `agent-request`, `include-request`, `needs-path`, `local-path`, and a step referencing itself | `internal/rules/builtin_test.go` |
| RULE-05 | `var-ref`'s `options.ignore` and `severity: off`. `parent.vars` warns outside an include | same |
| RULE-06 | The default `require-desc` and `no-debug` | same |
| RULE-07 | Declarative `exists`, `equals`, `not_equals`, `matches`, `not_matches`, `key_matches`, `key_not_matches`, `one_of`, `min_len`, `max_len`, `where`, `forbid`, messages, `steps.*`, and a broken regular expression | `internal/rules/declare_test.go` `lint_test.go` `TestCustomRule` |
| RULE-08 | `{{ }}` does not treat text inside quotes as the end of the expression. Numeric durations and templates | `internal/rules/expr_test.go` |
| RULE-09 | Branches of each check function. HTTP media type, gRPC `message` / `messages`, DB, exec, SSH, CDP arguments, agent, include, runner settings, book types, step-reference visibility, `parent.vars`, `i`, and `previous` | `internal/rules/branch_test.go` |
| RULE-10 | When yamllint is absent, report at the `missing` severity. `off` and disabling report nothing | `internal/yamllint/yamllint_test.go` |
| RULE-11 | `examples/bad/<rule id>.yml` reports that default rule. `examples/valid` reports nothing | `internal/rules/examples_test.go` |
| RULE-12 | Expressions expand `${...}` in the value before parsing, the same way runn does. An unset environment variable does not report `expr-syntax`, `var-ref`, or `step-ref` | `internal/rules/falsepositive_test.go` |
| RULE-13 | Expressions such as `test` and `if` strip a `#` outside parentheses through the end of the line as a comment before parsing. A `#` in a string literal or a closure stays. The inside of `{{ }}` is not stripped | same |
| RULE-14 | `{{ }}` does not treat `}}` inside an expression's `( )`, `[ ]`, or `{ }` as the end. When the parentheses are unbalanced, it cuts at the first `}}` outside quotes | same |
| RULE-15 | A `bind` value may also be a mapping, an array, or a number. Strings inside it are checked as expressions | same |
| RULE-16 | `included-unreferenced` is reported only when an `if: included` runbook (including `included == true` and `included && ...`) inside the walked scope or `--include-root` is not included from anywhere. `||`, `!`, a missing `if`, something a dynamic reference can match, and a file that was only passed directly are not reported. An ignored file still counts as a referrer | `internal/rules/includes_test.go` (`testdata/included`), `internal/app/contract_test.go` |
| RULE-17 | `# runnlint disable-line` removes, in the layer shared by every rule, diagnostics on that line when it is trailing, or on the next line that is neither blank nor a comment when it is a comment line. A multiline-scalar diagnostic can be removed on the key line. A `#` inside quotes or a block is not a suppression. An unknown rule, a yamllint rule, an unknown directive, and a suppression that removes nothing are `unused-suppression`. A rule that is off is excluded. This runs before the baseline | `internal/rules/suppress_test.go`, `internal/app/contract_test.go` |
| RULE-18 | Empty HTTP and gRPC `headers` (`{}` and null) are accepted | `internal/rules/falsepositive_test.go` |
| RULE-19 | An HTTP path that does not start with `/` is not reported by default. It is a warning only when `options.require-leading-slash: true` | same |
| RULE-20 | `var-ref` treats a variable passed in `include.vars` from an including runbook as defined. When an including runbook exists, `parent.vars` is not called undefined. An `if: included` runbook whose including runbook is not found does not have its `vars` checked | `internal/rules/includes_test.go` (`testdata/includevars`) |
| RULE-21 | `{{` inside a `test` or `bind` expression is not a template. When the whole value is one `{{ }}`, the inside is parsed as an expression and `expr-braces` is reported. When the value starts with `{{` but is not exactly one template, only `expr-braces` is reported, and an expression parse error is not | `internal/rules/falsepositive_test.go` |

RULE-03 looks at one main path of each rule. RULE-04 and RULE-07 look, per rule, at contracts that path missed. RULE-09 fixes the branches inside them down to the message.
