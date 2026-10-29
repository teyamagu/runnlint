// Package version holds the runnlint product version.
// It is separate from the version field of a rules file.
package version

import "regexp"

// Version is the version that is released. Git tags add a leading v.
const Version = "0.2.0"

// semver matches MAJOR.MINOR.PATCH plus an optional prerelease. A leading v is not included.
var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// Valid reports whether v is a Semantic Versioning MAJOR.MINOR.PATCH value.
func Valid(v string) bool {
	return semver.MatchString(v)
}
