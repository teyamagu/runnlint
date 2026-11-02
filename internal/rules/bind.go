package rules

import (
	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
)

// checkBind follows the bind branch of appendStep.
// The value is a mapping. runn evaluates nested mappings and arrays and binds anything else as-is.
func checkBind(c *Context, v runbook.Value) {
	if v.Kind != runbook.KindMap {
		c.Add(v.Node, message.BindType)
		return
	}
	if len(v.Map) == 0 {
		c.Add(v.Node, message.BindEmpty)
	}
}
