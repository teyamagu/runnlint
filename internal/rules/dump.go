package rules

import (
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
	"github.com/teyamagu/runnlint/internal/runnspec"
)

// checkDump follows the dump branch of appendStep.
// A string is the expression. A mapping needs expr, and may set out, disableTrailingNewline, and disableMaskingSecrets.
func checkDump(c *Context, v runbook.Value) {
	switch v.Kind {
	case runbook.KindString:
		return
	case runbook.KindMap:
		allowKeys(c, v, "dump", runnspec.DefKeys(runnspec.DefDump))
		if _, ok := v.Get("expr"); !ok {
			c.Add(v.Node, message.DumpExprMiss)
		}
		if out, ok := v.Get("out"); ok && out.Kind != runbook.KindString {
			c.Add(out.Node, message.DumpOutType)
		}
		checkBoolish(c, v, "disableTrailingNewline", "disableMaskingSecrets")
	default:
		c.Add(v.Node, message.DumpType)
	}
}
