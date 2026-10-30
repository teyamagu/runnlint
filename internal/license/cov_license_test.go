package license

import (
	"testing"

	"github.com/teyamagu/runnlint/internal/message"
)

// Empty or whitespace-only license text is rejected as empty. Text that is none of the permissive licenses and none of the denied phrases is rejected as unknown.
func TestCovCheckEmptyAndUnknown(t *testing.T) {
	for _, text := range []string{"", "   \n\t"} {
		err := Check(text)
		if err == nil || message.Localize(message.EN, err) != message.Text(message.EN, message.LicenseEmpty) {
			t.Fatalf("%q: %v", text, err)
		}
	}
	err := Check("Copyright 2024 Someone. All rights reserved. Do whatever.")
	if err == nil || message.Localize(message.EN, err) != message.Text(message.EN, message.LicenseUnknown) {
		t.Fatalf("%v", err)
	}
}
