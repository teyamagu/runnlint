# Following runn updates

[日本語](../runn-follow.md)

The runn that runnlint follows is [v1.11.1](https://github.com/k1LoW/runn/releases/tag/v1.11.1). The tag is in `internal/runnspec/VERSION`.

Reading the [CHANGELOG](https://github.com/k1LoW/runn/blob/main/CHANGELOG.md) from v0.101 (2024-03) through v1.11.1 (2026-09), most releases are dependency updates. Few releases change runbook notation, and the changes fall into the three kinds below. The follow mechanism matches that split.

## How the notation changes

| Kind | Example | Does it appear in the schema? |
| --- | --- | --- |
| Adding a key or a runner | `exec.env` in v1.2.0, CDP `flags` in v0.135.0, CDP `timeout` in v0.138.0, inline include in v1.7.0, the agent runner in v1.8.0 | Yes. The schema itself was added in v1.4.0, and top-level keys were closed in v1.6.2 |
| A shape that exists only in the parser | In v0.132.0, CDP `latestTab` became `tabTo`. In v0.134.0, the HTTP content-type restriction was removed | No. The HTTP, gRPC, and DB contents of a step are outside [runbook.schema.yaml](https://github.com/k1LoW/runn/blob/main/runbook.schema.yaml) |
| Adding an expression function | `file()` in v0.130.0, JWT in v0.137.0, `faker.Regex()` in v0.140.1, `hash()` in v1.3.0 | No. The YAML shape does not change |

Many breaking changes are runtime behavior, such as the environment-variable snapshot (v1.11.0) or cookies across includes (v1.9.0). They are separate from the YAML shape, so the linter does not follow them.

## Three layers

1. **Schema.** Read `internal/runnspec/runbook.schema.yaml` and use it to check top-level keys, `x-`, the label regular expression, keys of a runner or of `exec` / `dump` / `loop` / `trace`, and `required`. An addition such as `exec.env` is allowed by replacing this file.
2. **CDP actions.** The schema has no action names, so names, aliases, and input arguments are read from `internal/runnspec/cdpfn.src` (runn's `cdpfn.go`). The change from `latestTab` to `tabTo` is detected here.
3. **Parser overrides.** Places where the schema and the implementation differ, and places the schema does not write, stay in Go. Raising runn does not change them automatically.
   - A `loop` mapping needs `count`. The schema does not require it
   - `interval` and duration fields also accept a number. The book's `interval` in the schema is a string
   - A step's `if` also accepts a bool. The schema says string
   - `agent` is `claude`, `copilot`, or `codex`. The schema says any string
   - SSH `identityFile` and `identityKey` cannot be written together
   - `shell` is `bash`, `sh`, or a string that contains `{0}`
   - include's `path` and `steps` are exclusive. The schema is `oneOf`, so the union of the key sets is wider than either mode
   - HTTP path and body, the gRPC method, DB `query`, and the order in which runner kinds are decided. These come from runn's `parse.go` and `book.go`

### Why the official schema is not used as a validator

runn's official JSON Schema (`runbook.schema.yaml`) is used, as in layer 1, as the source of keys and `required`. Passing the whole schema through a JSON Schema validator to judge the shape of a runbook is not done now. The reasons are:

- Where the schema and the parser disagree (the list in layer 3), a runbook that runn accepts would be judged invalid. That works against the policy of reducing false positives
- The HTTP, gRPC, and DB contents of a step are outside the schema, so checks such as `http-request` stay in Go
- A validator's errors come out as JSON Pointers, so a separate mechanism is needed to map them back to a YAML line and column. Anchors and merges (`<<`) shift that mapping

The policy from here is:

1. Keep reading the key set and `required` from the schema. When runn is raised, follow it by replacing the schema
2. Move types the schema can express (`desc` is a string, `labels` is an array of strings, and so on) to schema-derived checks, starting with items that do not disagree with the parser. When moving one, leave the layer 3 overrides outside the schema
3. Add a whole-document validator only after disagreements with the parser can be absorbed by the overrides and lines can be mapped. If it is added, add it first as a rule that is disabled by default, measure false positives on real runbooks, and only then make it a default

Expression functions are not an allowlist of names. When `file()` or `hash()` is added, an expression that can be parsed still passes.

## When the test fails

`go test ./...`, including the weekly CI, checks the following.

- The imported schema and `cdpfn.src` match the files at the `VERSION` tag
- When the same two files in the latest release differ from the import, the test fails

If only the tag moves and these two files are the same, the test does not fail. That keeps CI green for a dependency-only release.

The weekly [runn follow](../../.github/workflows/runn-follow.yml) does the following only when that difference exists.

1. Replace `internal/runnspec/runbook.schema.yaml` and `internal/runnspec/cdpfn.src` with `runbook.schema.yaml` and `cdpfn.go` from the new tag. Set `internal/runnspec/VERSION` to that tag.
2. Run `go test ./...` to see whether the rules and examples still pass.
3. Open a PR whose body contains the result. A PR whose tests failed is a draft. Rules that are not in the schema are not changed automatically, so a failure means fixing a parser override or the examples by hand.

When a PR or branch for the same tag already exists, a new one is not created. Following by hand uses the same order, from 1 to 3.

The two imported files are MIT-licensed works of [k1LoW/runn](https://github.com/k1LoW/runn). Their contents are copied without added comments so they can be compared.
