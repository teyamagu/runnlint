package rules

import "github.com/teyamagu/runnlint/internal/runnspec"

// cdpArgs は runn の cdpfn.go から読んだアクション名と入力引数。
var cdpArgs = map[string][]string{}

func init() {
	for name, args := range runnspec.CDPArgs() {
		cdpArgs[name] = args
	}
}
