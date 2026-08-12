package shellcheck

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

// gccLineRe matches one gcc-format shellcheck finding line:
//
//	<file>:<line>:<column>: <severity>: <message>
//
// The greedy leading `.*` lets file paths contain colons (e.g. Windows
// drive letters); the trailing numeric line/column fields and the
// severity label are pinned by the anchors, so a colon in the path never
// shifts the field assignment.
var gccLineRe = regexp.MustCompile(`^(.*):([0-9]+):([0-9]+): ([a-z]+): (.*)$`)

// Violation records a shellcheck finding.
type Violation struct {
	File     string
	Line     int
	Severity string
	Message  string
	SC       string
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
	args := append([]string{"--format=gcc", "--enable=all", "--severity=style"}, files...)
	cmd := exec.CommandContext(context.Background(), "shellcheck", args...)
	out, err := cmd.CombinedOutput()
	violations := parseGCC(string(out))
	if err != nil && len(violations) == 0 {
		return nil, string(out), fmt.Errorf("shellcheck failed: %w", err)
	}
	return violations, string(out), nil
}

func parseGCC(output string) []Violation {
	lines := strings.Split(output, "\n")
	violations := make([]Violation, 0, len(lines))
	for _, line := range lines {
		m := gccLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		msg := strings.TrimSpace(m[5])
		sc := ""
		if idx := strings.LastIndex(msg, "["); idx != -1 {
			sc = strings.TrimSuffix(msg[idx:], "]")
		}
		violations = append(violations, Violation{
			File:     m[1],
			Line:     lineNo,
			Severity: m[4],
			Message:  msg,
			SC:       sc,
		})
	}
	return violations
}
