// Package version は runnlint の製品バージョンを持つ。
// ルールファイルの version フィールドとは別である。
package version

import "regexp"

// Version はリリースする版。Git のタグは先頭に v を付ける。
const Version = "0.1.4"

// semver は MAJOR.MINOR.PATCH と、任意のプレリリース部。先頭の v は含めない。
var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// Valid は v が Semantic Versioning の MAJOR.MINOR.PATCH であるときに true を返す。
func Valid(v string) bool {
	return semver.MatchString(v)
}
