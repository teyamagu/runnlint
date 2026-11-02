package runbook

import "testing"

// TestDBScheme checks the DSN schemes runn treats as a database runner in parseRunner.
// http and grpc are runner schemes of their own, and an empty scheme is not a database.
func TestDBScheme(t *testing.T) {
	for _, scheme := range []string{"postgres", "postgresql", "pg", "mysql", "my", "sqlite", "sqlite3", "sq", "spanner", "sp", "file"} {
		if !DBScheme(scheme) {
			t.Fatalf("%s is a database scheme", scheme)
		}
	}
	for _, scheme := range []string{"http", "https", "grpc", "ssh", "cdp", "chrome", ""} {
		if DBScheme(scheme) {
			t.Fatalf("%s is not a database scheme", scheme)
		}
	}
}
