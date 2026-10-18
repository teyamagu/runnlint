// Package finding は lint 結果の1件を表す。
package finding

// Severity は診断の深刻度。
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
	Off     Severity = "off"
)

// Finding は1件の診断。
type Finding struct {
	File     string
	Line     int
	Column   int
	Severity Severity
	Rule     string
	Message  string
}
