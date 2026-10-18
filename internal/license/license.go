// Package license は、依存ライブラリが MIT ライセンスのプロジェクトから利用できるかを判定する。
package license

import (
	"fmt"
	"strings"
)

// Check はライセンス本文が MIT、BSD、ISC、Apache-2.0 のいずれかであるかを確認する。
// 複数の許可ライセンスが併記されていても受け付ける。GPL などのコピーレフトは拒否する。
func Check(text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("ライセンス本文が空です")
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
			return fmt.Errorf("MIT ライセンスのプロジェクトからは利用できないライセンスです (%s)", phrase)
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
		return fmt.Errorf("MIT、BSD、ISC、Apache-2.0 として判定できません")
	}
}
