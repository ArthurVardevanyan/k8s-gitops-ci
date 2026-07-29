package markdownlint

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrCLINotFound returned when markdownlint is not available.
var ErrCLINotFound = errors.New("markdownlint not found in PATH")

// Filter returns markdown files.
func Filter(files []string) []string {
	var out []string
	for _, f := range files {
		l := strings.ToLower(f)
		if strings.HasSuffix(l, ".md") || strings.HasSuffix(l, ".markdown") {
			out = append(out, f)
		}
	}
	return out
}

// Run executes markdownlint on files.
func Run(files []string) (string, error) {
	files = Filter(files)
	if len(files) == 0 {
		return "", nil
	}
	cmdName := "markdownlint"
	if _, err := exec.LookPath(cmdName); err != nil {
		if _, err := exec.LookPath("markdownlint-cli2"); err == nil {
			cmdName = "markdownlint-cli2"
		} else {
			return "", ErrCLINotFound
		}
	}
	args := append([]string{"--dot"}, files...)
	cmd := exec.CommandContext(context.Background(), cmdName, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("markdownlint failed: %w", err)
	}
	return string(out), nil
}
