# runnlint

[日本語](README.md)

[![build](https://github.com/teyamagu/runnlint/actions/workflows/ci.yml/badge.svg)](https://github.com/teyamagu/runnlint/actions/workflows/ci.yml)
[![coverage](docs/coverage.svg)](docs/coverage.svg)
[![code to test ratio](docs/ratio.svg)](docs/ratio.svg)
[![test execution time](docs/time.svg)](docs/time.svg)

A linter for [runn](https://github.com/k1LoW/runn) runbooks.

General YAML checks, such as indentation, duplicate keys, and line length, are delegated to [yamllint](https://yamllint.readthedocs.io/). This tool checks runn-specific runners, steps, `{{ }}`, and `steps` references. Enable, set the severity of, and add rules in YAML.

## Requirements

Use the latest stable Go. The adopted version is written in `.go-version` and the `go` line of `go.mod`, and it matches the [latest stable Go](https://go.dev/dl/). GitHub Actions runs `go test ./...` on that latest stable release. CI fails when the declaration is older, or when the tests fail on that Go. Even in a week with no code changes, check whether a new stable release has appeared.

Install yamllint for general YAML linting.

```console
pip install yamllint
```

Without it, the runn-specific checks still run. runnlint emits one warning and skips yamllint. yamllint is a GPL-3.0 external command and is not part of this repository's Go module.

## Usage

```console
go install ./cmd/runnlint
runnlint path/to/runbook.yml
runnlint path/to/runbooks/
```

A directory argument checks only YAML whose root has `steps` or `runners`.

```console
runnlint -f json .
runnlint -f github .
runnlint --no-yamllint .
runnlint --baseline runnlint-baseline.json .
runnlint --strict .
runnlint --include-root . books/payments
runnlint --list-rules
runnlint --print-config > .runnlint-runn.yaml
runnlint --version
```

The exit code is `1` when there is at least one error, `2` when configuration or execution fails, and `0` otherwise. Warnings and info alone do not fail the run. With `--strict`, a warning also exits `1`. Info never fails, even with `--strict`. Diagnostics hidden by `--baseline` do not count, so existing warnings can stay in the baseline while new warnings stop the run.

The display language defaults to Japanese. Set `language: en` in the config, or pass `--lang en`, to show English. `--lang` overrides the config. Any other value exits with code `2`. text, github, the JSON `message`, and `-h` usage print only the selected language.

`included-unreferenced` looks at includes from other files, so the scope is fixed. It judges only files found by walking a directory argument and files under `--include-root`, and it reads every YAML file in that scope as a possible referrer. A single file argument is not judged. When the including file is outside the walk, add it with `--include-root`. See [Scope of included-unreferenced](docs/en/configuration.md#scope-of-included-unreferenced).

### Inline suppression

Per-line diagnostics can be suppressed with a YAML comment. The form matches yamllint's `# yamllint disable-line rule:...`.

```yaml
steps:
  - req:
      /legacy:
        get:
          body: null
    # A comment-only line applies to the next line, skipping blank lines and comment lines.
    # runnlint disable-line rule:var-ref -- passed in from the including runbook
    test: current.res.status == vars.expected
  # A trailing comment applies to that line.
  - dump: vars.legacy # runnlint disable-line rule:var-ref -- reason
  - dump: vars.legacy # runnlint disable-line -- omitting rules suppresses every rule on that line
```

- List rules as `rule:<id>` or `<id>`, separated by spaces or commas. Text after `--` is a reason and is ignored. Omitting the rules suppresses every rule on that line. The scope is wide, so write a reason.
- This applies to built-in rules, rules written only in YAML, `included-unreferenced`, and `parse`. Diagnostics that have no line, for the file as a whole, are out of scope. Suppress yamllint diagnostics with `# yamllint disable-line`.
- A diagnostic for an expression inside a multiline scalar (`|` or `>`) is reported on the key line. Suppress it at the end of the key line (`test: | # runnlint disable-line rule:expr-syntax`) or on the line before it. A comment inside the block is part of the value, so it is not a suppression. A `# ...` inside an expression such as `test` is removed before checking, the same way runn does.
- A suppression that names a missing rule, suppresses nothing, sits inside a block, or uses a directive other than `disable-line` is reported as `unused-suppression` (warning). Naming a rule whose `severity` is `off` is not reported.

Inline suppression runs before the baseline. A suppressed diagnostic is not written to the baseline and is removed before `--baseline` matches. `unused-suppression` is an ordinary diagnostic, so it can be recorded in a baseline or stopped with `severity: off`.

### baseline

To adopt runnlint while accepting current violations, write the current diagnostics as a baseline and pass that file from then on. Diagnostics in the baseline are not printed. Only new ones are. The exit code is decided by the diagnostics that remain.

```console
runnlint --write-baseline runnlint-baseline.json .
runnlint --baseline runnlint-baseline.json .
```

`--write-baseline` writes every error, warning, and info, then exits `0`. The match key is the path relative to the baseline file's directory, the rule ID, and the message. Line and column are not used, so inserting lines before a violation leaves it matched. Diagnostics with the same key are matched by count, and any excess over the record is reported as new. When a fixed violation is still in the baseline, the count is printed to stderr. Rewrite the file with the same command.

Built-in diagnostic messages are recorded and matched in English even when the display language is Japanese. A `message` written on a declarative rule is stored as written. The baseline file format is version 2. Version 1 cannot be loaded. Rewrite it with `--write-baseline`.

The product version is Semantic Versioning. It is separate from `version: 1` in a rules file. How to bump the number, and how a tag becomes a release, is in [docs/en/versioning.md](docs/en/versioning.md).

## Rules file

Field meanings, inheritance, and how to write declarative rules are in [docs/en/configuration.md](docs/en/configuration.md). Following runn updates is in [docs/en/runn-follow.md](docs/en/runn-follow.md).

From the current directory upward, runnlint looks for `.runnlint-runn.yaml`, then `.runnlint-runn.yml`. If neither exists, it uses the built-in default. `--config` passes a path directly.

Omitting `extends` inherits the default and overrides only rules with the same `id`. Repeating the same `id` makes the later entry override the earlier one.

```yaml
version: 1
extends: default

yamllint:
  enabled: true
  bin: yamllint
  missing: warning
  # config: .yamllint.yaml

ignore:
  - examples/bad

rules:
  # To disable a built-in rule, write only severity: off on the same id.
  # Do not warn that a variable passed from the CLI is undefined. Omitting severity makes it error.
  - id: var-ref
    severity: warning
    options:
      ignore:
        - token

  # Add a rule in YAML alone.
  - id: no-plain-password
    severity: error
    message: "vars.password must reference ${VAR}"
    check:
      select: vars.password
      matches: '^\$\{.+\}$'
```

`extends: none` does not inherit. `severity` is `error`, `warning`, `info`, or `off`. `ignore` applies only to files collected by a directory walk. Writing a file path in a pattern does not skip that file when it is passed as an argument.

### Declarative rules

`check.select` is dot-separated. `*` is each value of a mapping, or each element of a sequence. `steps.*` means each step whether `steps` is a list or a map.

The conditions are `exists`, `equals`, `not_equals`, `matches`, `not_matches`, `key_matches`, `key_not_matches`, `one_of`, `min_len`, `max_len`, and `forbid`. `key_matches` looks at the key name. It can express a convention for map step names. `where` narrows the values before the check. `message` can use `{{ .path }}`, `{{ .key }}`, `{{ .value }}`, and `{{ .rule }}`.

```yaml
rules:
  - id: loop-until-with-test
    severity: warning
    message: "step {{ .key }} has both loop.until and test"
    check:
      select: steps.*
      where:
        - path: loop.until
          exists: true
        - path: test
          exists: true
      forbid: true
```

### Built-in rules

This table says which failure each id reported in a diagnostic means. The initial severity, `uses`, options, and the range of expression checks are in [Built-in rules](docs/en/configuration.md#built-in-rules).

| id | What it reports |
| --- | --- |
| `duplicate-key` | The same key is written twice in one mapping |
| `unknown-key` | A top-level key is not a runbook field, does not start with `x-`, and is not `<<` |
| `book-shape` | A field directly under the runbook does not have the shape runn accepts |
| `require-steps` | There is no step to run |
| `label-format` | A label is empty, or contains a character runn does not accept |
| `runner-reserved` | A runner name uses a reserved word such as `test` or `exec` |
| `runner-config` | The runner kind is unknown, or that kind's settings do not have the shape runn accepts |
| `runner-defined` | A step calls a runner that is not defined |
| `step-shape` | One step does not decide exactly one action to run |
| `http-request` | An HTTP call does not have the shape runn accepts |
| `grpc-request` | A gRPC call does not have the shape runn accepts |
| `db-request` | A DB step has no SQL to run |
| `exec-request` | exec has no command to run, or an accompanying field has the wrong shape |
| `ssh-request` | An SSH step contains something other than `command`, or has no `command` |
| `cdp-request` | Browser `actions` are not a valid runn operation |
| `agent-request` | An agent prompt is missing, or has the wrong shape |
| `include-request` | include does not decide exactly one runbook to load |
| `include-path` | The file referenced by include does not exist |
| `needs-path` | The file referenced by `needs` does not exist |
| `local-path` | A file a runner references, such as a certificate or proto, does not exist |
| `expr-syntax` | An expression in `{{ }}`, `test`, or a similar field cannot be parsed as an expression |
| `expr-braces` | A field that is read as an expression is wrapped in `{{ }}` |
| `expr-sharp-comment` | An expression such as `test` or `if` has a `#` comment. runn strips it but prints a deprecation warning |
| `step-ref` | An array step is referenced by name, a named step is referenced by index, or a step that has not run yet is referenced |
| `var-ref` | A variable that is not in `vars` is referenced |
| `loop-index` | The index `i` is used outside `loop` |
| `previous-step` | `previous` is used where there is no previous step |
| `defer-experimental` | Notice that `defer` is experimental |
| `unused-runner` | A runner defined in `runners` or `runner:` is not called by any step |
| `unused-bind` | A name given by `bind` is read neither by this file's expressions nor by any file that includes or needs this runbook |
| `require-test-step` | A runbook that can run on its own has no step with only `test`, a verification step that names what the scenario checks |
| `included-unreferenced` | A runbook that runs only when included is not included from anywhere |
| `unused-suppression` | A suppression comment does not suppress a diagnostic |

The default config also contains declarative rules. `require-desc` reports a missing `desc`, `no-debug` reports that `debug` is enabled, and `loop-until-with-test` reports a step that has both `loop.until` and `test`. `test` is evaluated before the repeat, so the step fails before the retry starts.

## Development

Add a failing test before implementing a change. The map from contracts to tests is in [docs/en/test-design.md](docs/en/test-design.md).

```console
go test ./...
go run ./cmd/runnlint --no-yamllint examples/valid
go run ./cmd/runnlint --no-yamllint examples/bad/http-request.yml
go run ./cmd/runnlint --no-yamllint --include-root examples examples/bad/included-unreferenced.yml
```

`go test ./...` also checks the following.

- `.go-version`, `go.mod`, and the running Go are the same version
- That version is the latest stable Go
- Every dependency license is MIT, BSD, ISC, or Apache-2.0

When a new stable release appears, this test fails until `.go-version` and `go.mod` are raised. CI runs the full test suite on that Go first, so compatibility can be checked before raising the declaration.

`examples/valid` contains runbooks that pass. HTTP and DB are in `examples/valid/http.yml`. `examples/bad` has one detection example per default rule, and the file name is the rule id. A directory walk ignores `examples/bad`. Passing the file as an argument checks it. `included-unreferenced` is the exception: passing the file alone does not judge it. The command above, with `--include-root examples`, reports it.

## License

[MIT License](LICENSE).

The Go libraries this module depends on are only those that an MIT project can use. The current direct dependencies are:

| Module | License |
| --- | --- |
| `github.com/expr-lang/expr` | MIT |
| `github.com/k1LoW/duration` | MIT |
| `gopkg.in/yaml.v3` | MIT and Apache-2.0 |
