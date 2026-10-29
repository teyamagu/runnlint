package rules

import "github.com/teyamagu/runnlint/internal/runnspec"

// cdpArgs is action names and input arguments read from runn's cdpfn.go.
var cdpArgs = map[string][]string{}

func init() {
	for name, args := range runnspec.CDPArgs() {
		cdpArgs[name] = args
	}
}
