package shellcheck

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrCLINotFound returned when shellcheck is absent.
var ErrCLINotFound = errors.New("shellcheck not found in PATH")

// ShellScriptRe matches POSIX shell script shebangs.
var ShellScriptRe = regexp.MustCompile(`(?m)^#!\s*/bin/(?:ba)?sh\b`)

// FilterShellScripts returns shell scripts via extension or shebang.
func FilterShellScripts(files []string) []string {
	var out []string
	for _, f := range files {
		if isShellScript(f) {
			out = append(out, f)
		}
	}
	return out
}

func isShellScript(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".sh" || ext == ".bash" {
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return ShellScriptRe.Match(data)
}

// Violation records a shellcheck finding.
type Violation struct {
	File      string
	Line      int
	Severity  string
	Message   string
	SC        string
}

// Run executes shellcheck on the given script files.
func Run(files []string) ([]Violation, string, error) {
	files = FilterShellScripts(files)
	if len(files) == 0 {
		return nil, "", nil
	}
	if _, err := exec.LookPath("shellcheck"); err != nil {
		return nil, "", ErrCLINotFound
	}
	args := append([]string{"--format=gcc", "--severity=warning"}, files...)
	cmd := exec.CommandContext(context.Background(), "shellcheck", args...)
	out, err := cmd.CombinedOutput()
	violations := parseGCC(string(out))
	if err != nil && len(violations) == 0 {
		return nil, string(out), fmt.Errorf("shellcheck failed: %w", err)
	}
	return violations, string(out), nil
}

func parseGCC(output string) []Violation {
	var violations []Violation
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ":", 5)
		if len(parts) < 5 {
			continue
		}
		file := parts[0]
		lineNo := 0
		fmt.Sscanf(parts[1], "%d", &lineNo)
		msg := strings.TrimSpace(parts[4])
		sc := ""
		if idx := strings.LastIndex(msg, "["); idx != -1 {
			sc = strings.TrimSuffix(msg[idx:], "]")
		}
		violations = append(violations, Violation{
			File:     file,
			Line:     lineNo,
			Severity: "warning",
			Message:  msg,
			SC:       sc,
		})
	}
	return violations
}
