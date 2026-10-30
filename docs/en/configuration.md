# Rules file

[日本語](../ja/configuration.md)

The settings runnlint uses when it checks a runbook. Write them in YAML. The same content can be passed as JSON. JSON is not discovered, so pass the path with `-c`.

## Discovery

Which file names are read as configuration.

When `-c` / `--config` is absent, runnlint walks from the current directory toward the parent and, in each directory, uses the first file it finds in this order.

1. `.runnlint-runn.yaml`
2. `.runnlint-runn.yml`

A file that is found is not skipped. If its shape is wrong, runnlint fails with exit code `2`. A wrong shape includes an unknown top-level key, a wrong type, a `version` other than `1`, and YAML that cannot be read. An unknown key below the top level, such as inside a `rules` entry, is treated as a typo: runnlint prints `warning:` and still loads the file.

A file passed with `--config` is loaded with a `warning:` even when it has unknown keys or a `version` other than `1`.

If nothing is found, runnlint uses the default embedded in the binary. Print it with:

```console
runnlint --print-config
runnlint --list-rules
```

`--list-rules` prints the loaded `id`, `severity`, and kind (`builtin:name` or `check`).

## Top level

Fields written at the top level.

```yaml
version: 1
extends: default
language: ja

ignore:
  - examples/bad
  - vendor/**

yamllint:
  enabled: true
  bin: yamllint
  missing: warning
  # config: .yamllint.yaml

rules:
  - id: var-ref
    severity: warning
    options:
      ignore:
        - token
```

| Field | Meaning |
| --- | --- |
| `version` | The rules-file version. Only `1` is accepted now. Omitting it leaves `1` after inheritance. A value other than `1` is handled as described under Discovery. |
| `language` | Display language for diagnostics, load errors, and `-h`. `ja` or `en`. Omitting it means Japanese. `--lang` overrides this field. Any other value exits with code `2`. Display prints only the selected language. Built-in diagnostics written to a baseline are English regardless of this setting. |
| `extends` | `default`, or omitting the field, inherits the default. `none` (case-insensitive) does not inherit. Any other string currently inherits as well. |
| `ignore` | Paths to skip during a directory walk. It does not apply when a file is passed as a command argument. Specifying it appends to the default list. Omitting the key keeps the default. |
| `yamllint` | Where general YAML checks are delegated. Only the fields you write are overridden. |
| `rules` | The rule list. The same `id` is merged. An unknown `id` is appended. |

## Inheritance

The usual way to write a config is to keep the default rules and change only some of them.

Entries with the same `id` overlay by these rules.

- Writing `severity` replaces that value.
- Writing `message` replaces that string. A built-in rule produces its own diagnostic text, so `message` applies only to declarative rules.
- `options` is overridden per key. List contents are not concatenated.
- Writing `check` and not writing `uses` removes the built-in implementation and replaces it with a declarative rule.
- Writing `uses` replaces the rule with that built-in implementation. If `check` is not written at the same time, the existing `check` is removed.

Writing the same `id` more than once in one file is not an error. The later entry overlays the earlier one by the same rules. With `extends: none` there is no merge, and two entries with the same `id` fail to load.

An entry that omits `severity` becomes `error` before the merge. To add only `options` to a rule whose default is `warning`, write `severity` as well. Omitting it raises the rule to `error`.

```yaml
rules:
  - id: var-ref
    severity: warning
    options:
      ignore:
        - token
```

To stop a rule, write `severity: off` on that `id`. An `off` rule needs neither `uses` nor `check`.

```yaml
rules:
  - id: var-ref
    severity: off
```

## ignore

Paths, among the files collected by walking a directory, that are not checked. A pattern may be a directory or a file path.

Patterns are consulted only when the argument is a directory. A file written as an argument is checked even when it matches a pattern.

```console
runnlint .
runnlint examples/fixtures/openapi.yml
```

When `ignore` contains `examples/fixtures/openapi.yml` and the current directory contains `examples/`, the first command drops the file whose walked path matches. The second command's argument is a file, so it checks that file even with the same pattern.

A pattern is matched against both the walked path (slash-separated) and its base name. The walked path is joined from the argument directory. When the argument is `.`, it is `examples/fixtures/openapi.yml`. When the argument is `books`, it is `books/a.yml`. It is not rewritten to be relative to the directory that contains the config file. When the argument is an absolute path, the walked path is absolute too. A pattern that contains `**` is matched only against the walked path.

A pattern that contains none of `*`, `?`, or `[` matches when the walked path equals the pattern and when the file is under it. A trailing `/` is stripped only when looking underneath. Both `examples/bad` and `examples/bad/` match `examples/bad/...`. `examples/fixtures/openapi.yml/` does not match the walked path `examples/fixtures/openapi.yml`. When the pattern contains no `/`, it also matches a file whose base name equals the pattern. A prefix such as `./examples/bad` is a different string from the walked `examples/bad/...`, so it does not match.

| Form | Example |
| --- | --- |
| File path | `examples/fixtures/openapi.yml` is a file whose walked path equals this. `examples/valid/openapi.yml` does not match this pattern |
| Directory | `examples/bad` or `vendor` is that path and everything under it. `vendor` contains no `/`, so it also matches a file whose base name is `vendor` |
| Base name | `openapi.yml` matches a file whose base name is `openapi.yml`, in any directory |
| `*` and `?` | `*.gen.yml` also matches the base name. `dir/*.yml` is only the files directly under `dir`. `*` and `?` do not cross a directory separator |
| `**` | `vendor/**` is a file whose walked path is under `vendor/`. It crosses intermediate directories. It does not match `other/vendor/a.yml`. `**/` stands for zero or more whole directories, so `*` and `?` after it stay inside one segment: `a/**/?.yml` matches `a/c.yml` and `a/x/c.yml` but not `a/x/cc.yml` |

`.git`, `vendor`, and `node_modules` are not descended into even when `ignore` is absent. They are not read as include referrers either.

A file excluded from checking by `ignore` is still read as an include referrer. The scope is under "Scope of included-unreferenced".

## yamllint

Settings for passing general YAML checks to the external yamllint command.

| Field | Meaning |
| --- | --- |
| `enabled` | `true` or `false`. Enabled when omitted. |
| `bin` | Executable name. `yamllint` when omitted. |
| `config` | yamllint's own config file. A relative path is resolved from the directory that contains the rules file. |
| `missing` | Severity when the command is absent. `error`, `warning`, `info`, or `off`. `warning` when omitted. |

When `config` is not written, runnlint passes a built-in config loosened for runbooks. Print it with `runnlint --print-yamllint-config`. Even with `extends: none`, leaving `bin` and `missing` empty makes them `yamllint` and `warning`. To stop yamllint, write `enabled: false`.

A diagnostic's rule ID is the yamllint rule name with a prefix, such as `yamllint/trailing-spaces`. yamllint is a GPL-3.0 external command and is not included in this module. Pass `--no-yamllint` to skip it for one run.

## Fields common to every rule

Fields for one rule. Write them both when changing a built-in severity and when adding a rule in YAML alone.

```yaml
rules:
  - id: no-plain-password
    severity: error
    message: "vars.password must reference ${VAR}"
    check:
      select: vars.password
      matches: '^\$\{.+\}$'
```

| Field | Meaning |
| --- | --- |
| `id` | The name printed in a diagnostic. Unique within the config. |
| `uses` | The built-in implementation name. It may differ from `id`. |
| `severity` | `error`, `warning`, `info`, or `off`. `error` when omitted. |
| `message` | The diagnostic text of a declarative rule. |
| `check` | The condition of a declarative rule. It cannot be written together with `uses`. |
| `options` | Extra settings passed to a built-in implementation. |

Other than `off`, either `uses` or `check` is required. `check.select` cannot be empty. An unknown `uses` fails before the run, and the exit code is `2`.

The exit code is `1` when there is at least one `error`, `2` when configuration or execution fails, and `0` otherwise. `warning` and `info` alone do not fail the run.

## Built-in rules

Names that can be written in `uses`, their initial severity, and the failure that name reports. The usual change is to keep `id` equal to that name and change only `severity`.

| uses | Default | Failure it reports |
| --- | --- | --- |
| `duplicate-key` | error | The same key is written twice in one mapping |
| `unknown-key` | error | A top-level key is not a runbook field, does not start with `x-`, and is not `<<` |
| `book-shape` | error | A field directly under the runbook does not have the shape runn accepts |
| `require-steps` | error | There is no step to run |
| `label-format` | error | A label is empty, or contains whitespace or one of `!+=|.*%^?><`. A non-string also uses this id |
| `runner-reserved` | error | A runner name uses a reserved word such as `test` or `exec` |
| `runner-config` | error | The runner kind is unknown, or that kind's settings do not have the shape runn accepts |
| `runner-defined` | error | A step calls a runner that is not defined |
| `step-shape` | error | One step does not decide exactly one action to run. A wrong shape on a value attached to a step, such as `if` or `dump`, also uses this id |
| `http-request` | error | An HTTP call does not have the shape runn accepts |
| `grpc-request` | error | A gRPC call does not have the shape runn accepts |
| `db-request` | error | A DB step has no SQL to run. A wrong shape for `trace` also uses this id |
| `exec-request` | error | exec has no command to run, or an accompanying field has the wrong shape |
| `ssh-request` | error | An SSH step contains something other than `command`, or has no `command` |
| `cdp-request` | error | Browser `actions` are not a valid runn operation |
| `agent-request` | error | An agent prompt is missing, or has the wrong shape |
| `include-request` | error | include does not decide exactly one runbook to load |
| `include-path` | warning | The file referenced by include does not exist |
| `needs-path` | warning | The file referenced by `needs` does not exist |
| `local-path` | warning | A file a runner references, such as a certificate or proto, does not exist |
| `expr-syntax` | error | An expression in `{{ }}`, `test`, or a similar field cannot be parsed as an expression |
| `expr-braces` | warning | A field that is read as an expression is wrapped in `{{ }}` |
| `expr-sharp-comment` | info | An expression such as `test`, `if`, `until`, or `bind` has a `#` comment outside brackets. runn strips it but warns "`#` comment is deprecated. Use `//` instead." A `#` inside a string literal or the closure `#` inside `( )` or `{ }` is not reported. The inside of `{{ }}` is not inspected |
| `step-ref` | error | An array step is referenced by name, a named step is referenced by index, or a step that has not run yet is referenced |
| `var-ref` | warning | A variable that is not in `vars` is referenced |
| `loop-index` | warning | The index `i` is used outside `loop` |
| `previous-step` | warning | `previous` is used where there is no previous step |
| `defer-experimental` | info | Notice that `defer` is experimental |
| `unused-runner` | warning | A runner defined in `runners` or `runner:` is not called by any step. A runner called by a step inside an inline include counts as used. Reserved names are left to `runner-reserved` |
| `unused-bind` | warning | A name given by `bind` is read neither by this file's expressions nor by any file that includes or needs this runbook. See "Scope of unused-bind" |
| `require-test-step` | info | A runbook that can run on its own has no step with only `test`. A `test` on a request checks one response; a step with only `test` names what the scenario verifies. Runbooks whose top-level `if` requires `included` are verified by their caller and are not reported. Runbooks without steps are left to `require-steps` |
| `included-unreferenced` | warning | A runbook that runs only when included is not included from anywhere |
| `unused-suppression` | warning | A suppression comment does not suppress a diagnostic |

The built-in rules that read `options` are `var-ref` and `http-request`.

```yaml
rules:
  - id: var-ref
    severity: warning
    options:
      ignore:
        - token
```

`ignore` is an array of strings. A name written here is not warned about when it is absent from both `vars` and `parent.vars`. Use it for variables passed from the CLI.

A variable passed from an including runbook counts even when it is not in `ignore`. Before linting, runnlint collects includes from the walked directory, `--include-root`, and argument files (the same scope as `included-unreferenced`).

- When an including runbook is found, a variable that any including runbook passes in `include.vars` is treated as defined. When `include.vars` is not a mapping and its contents are unknown, that file's `vars` are not checked. `parent.vars` is not checked either, because it differs per including runbook.
- An include whose path contains `{{ }}` or `${...}` only adds `include.vars` as variables that might be passed to a file that might be the target. It is not treated as a found including runbook.
- An `if: included` runbook at the top level whose including runbook is not found does not have its `vars` checked, even when the file is passed alone. It is assumed to run by being included.
- Every other runbook still looks only at that runbook's `vars`.

`http-request` reads `options.require-leading-slash`. Setting it to `true` makes a path that starts with neither `/` nor `://` (such as `graphql`) a warning under this id. runn joins the endpoint and the path, so this path is not a failure by default.

```yaml
rules:
  - id: http-request
    severity: error
    options:
      require-leading-slash: true
```

### Expressions and step references

`expr-syntax` looks inside `{{ }}` and at `test`, `if`, `until`, and `bind`. `{{ }}` is inspected not only in mapping values but also in keys, such as an HTTP path or a body field name, because runn expands keys too. Before checking, it expands `${...}` in the value and removes a `#` outside parentheses through the end of the line as a comment. `{{` inside a `test` or `bind` expression is not a template. `bind` accepts a string, a mapping, an array, or a number, and checks strings inside it as expressions.

For `test` and `bind`, `expr-braces` is reported only when the whole value is `{{ }}`. `{{` inside a string is not looked at.

In `step-ref`, a step may reference itself.

### Scope of unused-bind

A name given by `bind` can be read by name in the same runbook, as `steps.<n>.<name>` or `previous.<name>` by a file that includes it, and as `needs.<name>.<bind>` by a file that lists it in `needs`. This rule treats a bind as read in the following cases and reports the rest.

- Some expression in this file (`test`, `if`, `until`, `dump`, `bind` values, `{{ }}`) mentions the name as an identifier or a member name
- Some expression in a file that includes this file by a fixed path, that may include it by a path containing `{{ }}` or `${...}`, or that lists it in `needs` mentions the name. runn evaluates an `include.vars` value as an expression in the caller, so a value that parses as one counts even without `{{ }}`. Referrers are read in the same scope as "Scope of included-unreferenced"

It does not judge a runbook whose top-level `if` requires `included` when no file includes or needs it by a fixed path, because the caller may be outside the scanned files. It also does not judge when this file or any referrer has a value that does not parse as an expression. A bind key such as `a.b` or `a[0]` assigns into an existing `a`, so it counts as a reference to `a`, not as a new name.

### Scope of included-unreferenced

`if: included` means "run only when included". A runbook that is not included from anywhere does not run, and has been left behind. This rule looks at references from other files. The scope is:

- It judges only files found by walking a directory argument and files under `--include-root`. A file passed directly is not judged unless it is under `--include-root`.
- As referrers, it reads every YAML file in the walked directory and under `--include-root`. A file excluded from checking by `ignore` is still read as a referrer. A file passed directly is also a referrer.
- An include path is resolved, as in `include-path`, from the directory that contains the including file. `include: path`, `include.path`, and an include runner's `path` count as references.
- A path that contains `{{ }}` or `${...}` has an unknown value, so every file whose name ends with the text after the last dynamic part is counted as referenced. For `{{ vars.name }}.yml`, every `.yml` qualifies.
- An `if` expression that can be true on its own, such as `included || ...` or `!included`, is out of scope. `included && ...` is in scope, because it runs only when included.

When the including file is outside the walk, add the parent directory to `--include-root`.

```console
runnlint --include-root . books/payments
```

The default config also contains declarative rules. `require-desc` reports a missing `desc`, `no-debug` reports that `debug` is enabled, and `loop-until-with-test` reports a step that has both `loop.until` and `test`. `test` is evaluated before the repeat, so the step fails before the retry starts. These rules have no `uses`.

## Declarative rules

`check` selects runbook values and looks at conditions. No code is required.

```yaml
rules:
  - id: no-plain-password
    severity: error
    message: "{{ .path }} must reference ${VAR}"
    check:
      select: vars.password
      matches: '^\$\{.+\}$'
```

### select

Which values the conditions apply to. It is dot-separated. `*` is each value of a mapping, or each element of a sequence. `steps.*` means each step whether `steps` is a list or a map.

| select | What it means |
| --- | --- |
| `desc` | The top-level `desc` |
| `vars.password` | `password` under `vars` |
| `steps.*` | Each step |
| `steps.*.test` | `test` of each step |

When a list element is selected, `key` is an index starting at `0`. When a map value is selected, `key` is the key name. `path` is a location such as `steps[0]` or `steps.login`.

### Conditions

When a selected value becomes a diagnostic. Every written condition is applied, and a diagnostic is reported when any one of them fails.

| Condition | When it reports |
| --- | --- |
| `exists: true` | The path is absent. Other conditions are not looked at |
| `exists: false` | The path is present at least once. Other conditions are not looked at |
| `equals` | The value differs. `null` can be compared with YAML null |
| `not_equals` | The value is the same |
| `matches` | A string or number does not match the regular expression |
| `not_matches` | A string or number matches the regular expression |
| `key_matches` | The key does not match the regular expression |
| `key_not_matches` | The key matches the regular expression |
| `one_of` | The value equals none of the list |
| `min_len` | The string length, sequence length, or mapping size is smaller |
| `max_len` | The same, and it is larger |
| `forbid: true` | A value matches `where`. Other conditions are not looked at |

Regular expressions are Go RE2. A broken expression reports a diagnostic at the runbook root.

`key_matches` and `key_not_matches` look at the key of the selected value. For a map's `steps.*` that is the step name. For a list's `steps.*` it is an index starting at `0`. The diagnostic is placed on the key line. A list has no key node, so it is placed on the value line. The default rules have no step-name convention. A project that needs one writes it. Existing step names can be accepted with a baseline.

```yaml
rules:
  - id: step-name
    severity: warning
    message: "step name {{ .key }} does not match the convention"
    check:
      select: steps.*
      key_matches: '^[a-z][a-z0-9-]*$'
```

`where` narrows values before the check. It can be a list or a single mapping. When there is more than one entry, only values that match every entry remain. `where` may contain `path`, `exists`, `equals`, and `matches`. `path` is relative to the selected value.

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

One `where` entry can also be written as follows.

```yaml
where:
  path: test
  exists: true
```

### message

The text printed in the diagnostic. When it is empty, a sentence that depends on the condition is printed. When it is written, it is a [Go text/template](https://pkg.go.dev/text/template). The four available values are below. An unknown name becomes an empty string.

| Name | Contents |
| --- | --- |
| `{{ .path }}` | `vars.password` or `steps[0]` |
| `{{ .key }}` | The last key, or a list index |
| `{{ .value }}` | The scalar as a string. A mapping or sequence is empty |
| `{{ .rule }}` | The rule `id` |

When `exists: true` and the path is absent, `path` is the deepest parent that exists. When the top level is absent, it is empty.
