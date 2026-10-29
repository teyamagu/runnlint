// Package license reports whether a dependency can be used from an MIT-licensed project.
package license

import (
	"strings"

	"github.com/teyamagu/runnlint/internal/message"
)

// Check reports whether the license text is MIT, BSD, ISC, or Apache-2.0.
// A text that lists more than one of those permissive licenses is accepted. Copyleft such as GPL is rejected.
func Check(text string) error {
	if strings.TrimSpace(text) == "" {
		return message.New(message.LicenseEmpty)
	}
	upper := strings.ToUpper(text)
	denied := []string{
		"GNU GENERAL PUBLIC LICENSE",
		"GNU AFFERO GENERAL PUBLIC LICENSE",
		"GNU LESSER GENERAL PUBLIC LICENSE",
		"MOZILLA PUBLIC LICENSE",
		"BUSINESS SOURCE LICENSE",
		"COMMONS CLAUSE",
	}
	for _, phrase := range denied {
		if strings.Contains(upper, phrase) {
			return message.New(message.LicenseForbid, phrase)
		}
	}
	switch {
	case strings.Contains(text, "Permission is hereby granted, free of charge"):
		return nil
	case strings.Contains(text, "Apache License, Version 2.0"), strings.Contains(text, "Licensed under the Apache License, Version 2.0"):
		return nil
	case strings.Contains(text, "Redistribution and use in source and binary forms"):
		return nil
	case strings.Contains(text, "ISC License"), strings.Contains(text, "Permission to use, copy, modify, and/or distribute this software"):
		return nil
	default:
		return message.New(message.LicenseUnknown)
	}
}
