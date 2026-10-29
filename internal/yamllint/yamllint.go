// Package yamllint delegates generic YAML lint to yamllint.
package yamllint

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/teyamagu/runnlint/internal/config"
	"github.com/teyamagu/runnlint/internal/finding"
	"github.com/teyamagu/runnlint/internal/message"
)

var parsableRe = regexp.MustCompile(`^(.*?):(\d+):(\d+): \[(error|warning)\] (.+) \(([^)]+)\)$`)

// ParseOutput converts yamllint -f parsable output into diagnostics.
func ParseOutput(out string) []finding.Finding {
	var findings []finding.Finding
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := parsableRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		sev := finding.Warning
		if m[4] == "error" {
			sev = finding.Error
		}
		findings = append(findings, finding.Finding{
			File:     m[1],
			Line:     lineNo,
			Column:   col,
			Severity: sev,
			Rule:     "yamllint/" + m[6],
			Message:  m[5],
		})
	}
	return findings
}

// Run runs yamllint on a set of files.
func Run(ctx context.Context, cfg config.Yamllint, configDir string, files []string) ([]finding.Finding, error) {
	if !cfg.EnabledOrDefault() || len(files) == 0 {
		return nil, nil
	}
	bin := cfg.Bin
	if bin == "" {
		bin = "yamllint"
	}
	if _, err := exec.LookPath(bin); err != nil {
		sev := finding.Severity(cfg.Missing)
		if sev == "" {
			sev = finding.Warning
		}
		if sev == finding.Off {
			return nil, nil
		}
		return []finding.Finding{{
			File:     files[0],
			Line:     1,
			Column:   1,
			Severity: sev,
			Rule:     "yamllint",
			Message:  "yamllint が見つかりません。一般的な YAML lint をスキップしました。pip install yamllint で導入できます",
		}}, nil
	}

	args := []string{"-f", "parsable"}
	if cfg.Config != "" {
		args = append(args, "-c", resolveConfig(configDir, cfg.Config))
	} else {
		data, err := config.YamllintYAML()
		if err != nil {
			return nil, err
		}
		args = append(args, "-d", string(data))
	}
	args = append(args, files...)

	cmd := exec.CommandContext(ctx, bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	findings := ParseOutput(buf.String())
	if err == nil || isLintFailure(err) {
		return findings, nil
	}
	if len(findings) > 0 {
		return findings, nil
	}
	msg := strings.TrimSpace(buf.String())
	if msg == "" {
		msg = err.Error()
	}
	return nil, message.New(message.YamllintFailed, msg)
}

func isLintFailure(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

func resolveConfig(configDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if configDir != "" {
		candidate := filepath.Join(configDir, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}
