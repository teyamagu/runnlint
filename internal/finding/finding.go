// Package finding represents one lint diagnostic.
package finding

// Severity is the severity of a diagnostic.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
	Off     Severity = "off"
)

// Finding is one diagnostic.
// Message is the sentence in the display language.
// Match is the English sentence used for baseline comparison.
// An empty Match means Message is used, which is how a user-written rule message is stored.
type Finding struct {
	File     string
	Line     int
	Column   int
	Severity Severity
	Rule     string
	Message  string
	Match    string
}

// BaselineMessage is the sentence stored in a baseline file and used when matching.
func (f Finding) BaselineMessage() string {
	if f.Match != "" {
		return f.Match
	}
	return f.Message
}
