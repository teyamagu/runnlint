# Changelog

[日本語](CHANGELOG.md)

Versions follow [Semantic Versioning 2.0.0](https://semver.org/). How to bump them is in [docs/en/versioning.md](docs/en/versioning.md). The GitHub Release body is taken from `CHANGELOG.md`. This file is the English copy, and its `## [X.Y.Z]` headings match that file.

## [Unreleased]

### Added

- `unused-runner` (warning). Reports a runner defined in `runners` or in a step's `runner:` that no step calls. A runner called by a step inside an inline include counts as used.
- `require-test-step` (info). Reports a runbook that can run on its own but has no step with only `test`. A `test` on a request checks one response; a step with only `test` names what the scenario verifies. Runbooks with `if: included` are not reported.
- `expr-sharp-comment` (info). Reports an expression such as `test` or `if` that has a `#` comment outside brackets. runn strips it but prints a deprecation warning asking for `//`.
- `unused-bind` (warning). Reports a name given by `bind` that neither this file's expressions nor any file that includes or needs this runbook reads. A runbook with `if: included` and no fixed-path referrer is not judged.

### Changed

- Japanese documents live in `docs/ja/`.
- The `examples` runbooks gained a step with only `test` that names what each one verifies.

## [0.2.0]

### Added

- A display language. Config `language` is `ja` or `en`, and omitting it means Japanese. `--lang` overrides the config. text, github, the JSON `message`, and usage print only the selected language.
- English copies of the README, the rules file, versioning, test design, following runn, and the changelog.

### Changed

- Baseline matching for built-in diagnostics uses the English sentence regardless of the display language. A declarative rule's `message` is stored as written. The baseline file format is now version 2. Version 1 cannot be loaded, so rewrite it with `--write-baseline`.
- Code comments are in English.

## [0.1.4]

### Added

- Show build, coverage, code-to-test ratio, and test-duration badges in the README.
- When runn's schema or CDP action definitions change, update the import, check the rules and examples, and open a fix PR.

### Changed

- Config discovery looks only for `.runnlint-runn.yaml` and `.runnlint-runn.yml`. It does not read `.runnlint.yaml`, `.runnlint.yml`, `runnlint.yaml`, or `runnlint.yml`.
- Describe built-in rules by the failure the diagnostic reports.
- Build the test-duration badge from the duration of the CI `テスト` step.
- Document config `version`, `{{ .rule }}`, `trace` on `db-request`, `${...}` and line comments in expressions, and the shape of `bind` values. Document the command that reproduces the `included-unreferenced` example.

## [0.1.3]

### Added

- `key_matches` and `key_not_matches` on declarative rules. They check the key name of a selected value with a regular expression. A project that needs a step-name convention writes it this way. It is not a default rule.

### Changed

- `{{` inside a `test` or `bind` expression is not read as a template. `{{` inside a string does not report `expr-syntax` or `expr-braces`. Wrapping the whole expression in `{{ }}` still reports `expr-braces`.

## [0.1.2]

### Added

- `--write-baseline FILE` and `--baseline FILE`. Record existing diagnostics and print only those that are new. Matching is by count of file, rule ID, and message, and is not affected by shifted lines.
- Inline suppression `# runnlint disable-line [rule:<id>...] [-- reason]`. A trailing comment suppresses that line. A comment line suppresses the next line. An unused suppression is reported as `unused-suppression` (warning).
- `included-unreferenced` (warning). It reports an `if: included` runbook that is not included from anywhere in the walked scope. `--include-root DIR` adds a directory to search for referrers.
- The dedicated config names `.runnlint-runn.yaml` and `.runnlint-runn.yml`. They are searched before the older names.
- `http-request`'s `options.require-leading-slash`.

### Changed

- Expressions expand `${...}` and strip `#` line comments before parsing, the same way runn does. An unset environment variable or a comment does not report `expr-syntax`.
- Nested parentheses and braces inside `{{ }}` are not mistaken for the end of the expression.
- A `bind` value may be a mapping, an array, or a number. Strings inside it are checked as expressions.
- Accept an empty `headers` (`{}` and null).
- An HTTP path that does not start with `/` is not a warning by default.
- `var-ref` treats a variable passed in `include.vars` from an including runbook as defined. An `if: included` runbook whose including runbook is not found does not check `vars`.
- When a discovered `.runnlint.yaml` or similar file is not shaped as a runnlint rules file, warn and skip it. Do not stop with exit code 2.

## [0.1.1]

- Place one detection example per default rule in `examples/bad`, and passing runbooks in `examples/valid`. Comments in the detection examples describe the broken part in Japanese.

## [0.1.0]

The first release. It checks runn-specific notation in runn runbooks.
