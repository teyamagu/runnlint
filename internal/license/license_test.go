package license

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckAllowsCompatibleLicenses checks that Check succeeds for MIT, Apache-2.0, BSD, ISC, and a text that lists both MIT and Apache.
func TestCheckAllowsCompatibleLicenses(t *testing.T) {
	samples := []string{
		"MIT License\n\nPermission is hereby granted, free of charge, to any person obtaining a copy",
		"Licensed under the Apache License, Version 2.0 (the \"License\");",
		"Redistribution and use in source and binary forms, with or without modification, are permitted",
		"ISC License\n\nPermission to use, copy, modify, and/or distribute this software for any purpose",
		"covered by two different licenses: MIT and Apache.\nPermission is hereby granted, free of charge\nApache License, Version 2.0",
	}
	for _, sample := range samples {
		if err := Check(sample); err != nil {
			t.Errorf("%v\n%s", err, sample)
		}
	}
}

// TestCheckRejectsCopyleft checks that Check fails for GPL, MPL, and LGPL texts.
func TestCheckRejectsCopyleft(t *testing.T) {
	samples := []string{
		"GNU GENERAL PUBLIC LICENSE\nVersion 3",
		"Mozilla Public License Version 2.0",
		"GNU LESSER GENERAL PUBLIC LICENSE",
	}
	for _, sample := range samples {
		if err := Check(sample); err == nil {
			t.Errorf("拒否すべきライセンスを受け付けました: %s", sample)
		}
	}
}

// TestImportedModulesUseCompatibleLicenses checks that Check succeeds for the LICENSE file of every external module listed by go list -deps.
func TestImportedModulesUseCompatibleLicenses(t *testing.T) {
	root := moduleRoot(t)
	cmd := exec.Command("go", "list", "-deps", "-f", "{{if and .Module (not .Standard)}}{{.Module.Path}}\t{{.Module.Dir}}{{end}}", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		path, dir, ok := strings.Cut(line, "\t")
		if !ok || path == "" || strings.HasPrefix(path, "github.com/teyamagu/runnlint") {
			continue
		}
		seen[path] = dir
	}
	if len(seen) == 0 {
		t.Fatal("依存モジュールがありません")
	}
	for path, dir := range seen {
		text, err := readLicense(dir)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if err := Check(text); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func readLicense(dir string) (string, error) {
	names := []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.md", "UNLICENSE"}
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			return string(body), nil
		}
	}
	return "", os.ErrNotExist
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod が見つかりません")
		}
		dir = parent
	}
}
