package rules

import (
	"strconv"
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runbook"
	"github.com/teyamagu/runnlint/internal/runnspec"
)

// checkLoop follows newLoop. A number or a string is the short form of count.
// A mapping uses the same keys as Loop: count, interval, minInterval, maxInterval, jitter, multiplier, until.
func checkLoop(c *Context, v runbook.Value, step bool) {
	switch v.Kind {
	case runbook.KindNumber:
		if strings.HasPrefix(v.Number, "-") {
			c.Add(v.Node, message.LoopCountNeg)
		}
	case runbook.KindString:
		return
	case runbook.KindMap:
		allowed := runnspec.DefKeys(runnspec.DefLoop)
		if _, ok := v.Get("count"); !ok {
			c.Add(v.Node, message.LoopCountMiss)
		}
		for _, e := range v.Map {
			if _, ok := allowed[e.Key]; !ok {
				c.Add(e.KeyNode, message.LoopUnknownKey, e.Key)
				continue
			}
			switch e.Key {
			case "count":
				if e.Value.Kind == runbook.KindNumber && strings.HasPrefix(e.Value.Number, "-") {
					c.Add(e.Value.Node, message.LoopCountMin)
				}
				if e.Value.Kind != runbook.KindNumber && e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, message.LoopCountType)
				}
			case "interval", "minInterval", "maxInterval":
				if !validDuration(e.Value) {
					c.Add(e.Value.Node, message.LoopDuration, e.Key)
				}
			case "jitter", "multiplier":
				if !loopNumber(e.Value) {
					c.Add(e.Value.Node, message.LoopNumber, e.Key)
				}
			case "until":
				if e.Value.Kind != runbook.KindString {
					c.Add(e.Value.Node, message.LoopUntil)
				}
			}
		}
	default:
		where := "runbook"
		if step {
			where = "step"
		}
		c.Add(v.Node, message.LoopType, where)
	}
}

// loopNumber follows Loop.jitter and Loop.multiplier, which are floats. A template is expanded before runn reads it.
func loopNumber(v runbook.Value) bool {
	switch v.Kind {
	case runbook.KindNumber:
		return true
	case runbook.KindString:
		if runbook.ContainsTemplate(v.String) {
			return true
		}
		// goccy accepts a numeric string, including underscores, and rejects surrounding space.
		_, err := strconv.ParseFloat(strings.ReplaceAll(v.String, "_", ""), 64)
		return err == nil
	default:
		return false
	}
}
