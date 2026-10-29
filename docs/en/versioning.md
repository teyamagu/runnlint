# Versions and releases

[日本語](../ja/versioning.md)

The runnlint product version is [Semantic Versioning 2.0.0](https://semver.org/). The value lives in `Version` in `internal/version/version.go`, and the Git tag adds a leading `v`. The tag for `0.1.0` is `v0.1.0`.

`version: 1` in a rules file is the number of the configuration shape. It is separate from the product version.

The baseline file `version` is also separate. The current format is `2`. Built-in diagnostics are recorded in English. A version `1` file cannot be loaded, so rewrite it with `--write-baseline`. Before `1.0.0`, changing diagnostic wording does not bump the product version. Record the change in the changelog, and rewrite existing baselines.

## How to bump

During `0.y.z`, compatibility after publication is not yet promised. A breaking change may be a MINOR. Write what breaks in the changelog.

From `1.0.0` on, split changes as follows.

| Bump | Change |
| --- | --- |
| MAJOR | Change a flag, the meaning of an exit code, how a config file is read, or an existing output field. Add a rule that is error by default |
| MINOR | Add a rule whose default is warning or info. Add an item that works with an existing config |
| PATCH | Fix a false positive, change diagnostic wording, or update a dependency |

Adding a rule that is error by default after `1.0.0` changes the exit code of a runbook that used to pass from `0` to `1`. That is a MAJOR.

## Release steps

1. Move the items under `[Unreleased]` in `CHANGELOG.md` into a new `## [X.Y.Z]`. Give `CHANGELOG.en.md` the same heading and the corresponding English items. The GitHub Release body is taken from `CHANGELOG.md`.
2. Set `Version` in `internal/version/version.go` to `X.Y.Z`. Write a prerelease as `1.2.3-rc.1`.
3. Confirm that `go test ./...` passes. This test checks that `Version` is semver and that `CHANGELOG.md` and `CHANGELOG.en.md` have the same `## [X.Y.Z]` headings.
4. Tag that commit `vX.Y.Z` and push it.
5. The Release workflow on GitHub Actions checks that the tag matches `runnlint --version`, then publishes binaries for each OS and `SHA256SUMS` to the GitHub Release. A tag that contains `-` is a prerelease.

The binary name is `runnlint_X.Y.Z_GOOS_GOARCH`. Only Windows has an `.exe` suffix.
