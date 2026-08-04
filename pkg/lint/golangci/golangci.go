package golangci

import (
	"context"
	"errors"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrCLINotFound returned when golangci-lint is absent.
var ErrCLINotFound = errors.New("golangci-lint not found in PATH")

// FilterGo returns only .go files.
func FilterGo(files []string) []string {
	var out []string
	for _, f := range files {
		if strings.HasSuffix(f, ".go") {
			out = append(out, f)
		}
	}
	return out
}

// Result holds gofmt and golangci-lint findings.
type Result struct {
	GoFmtIssues       []string
	GolangCIOutput    string
	LintErrorsPresent bool
}

// Issues grouped by module root.
type ModuleGroup struct {
	Root  string
	Files []string
}

// Run checks gofmt and runs golangci-lint grouped by module root.
func Run(files []string) (string, error) {
	goFiles := FilterGo(files)
	if len(goFiles) == 0 {
		return "", nil
	}
	res := &Result{}
	for _, f := range goFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", f, err)
		}
		formatted, err := format.Source(data)
		if err != nil {
			return "", fmt.Errorf("gofmt parse %s: %w", f, err)
		}
		if string(formatted) != string(data) {
			res.GoFmtIssues = append(res.GoFmtIssues, f)
		}
	}
	groups, err := groupByModule(goFiles)
	if err != nil {
		return "", err
	}
	golangciPath, lookErr := exec.LookPath("golangci-lint")
	if lookErr != nil {
		if len(res.GoFmtIssues) > 0 {
			return formatResult(res), nil
		}
		return "", ErrCLINotFound
	}
	var outputs []string
	for _, g := range groups {
		relDirs := dirsForGroup(g)
		for _, d := range relDirs {
			cmd := exec.CommandContext(context.Background(), golangciPath, "run", "./"+d+"/...")
			cmd.Dir = g.Root
			out, err := cmd.CombinedOutput()
			if err != nil {
				res.LintErrorsPresent = true
			}
			if len(out) > 0 {
				outputs = append(outputs, fmt.Sprintf("# %s\n%s", g.Root, strings.TrimSpace(string(out))))
			}
		}
	}
	res.GolangCIOutput = strings.Join(outputs, "\n\n")
	return formatResult(res), nil
}

func formatResult(r *Result) string {
	var b strings.Builder
	if len(r.GoFmtIssues) > 0 {
		b.WriteString("gofmt issues:\n")
		for _, f := range r.GoFmtIssues {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}
	if r.GolangCIOutput != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(r.GolangCIOutput)
	}
	out := b.String()
	if out != "" {
		out += "\nlint violations found"
	}
	return out
}

func groupByModule(files []string) ([]ModuleGroup, error) {
	roots := make(map[string]*ModuleGroup)
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		root, err := findModuleRoot(filepath.Dir(abs))
		if err != nil {
			return nil, err
		}
		if roots[root] == nil {
			roots[root] = &ModuleGroup{Root: root}
		}
		roots[root].Files = append(roots[root].Files, abs)
	}
	groups := make([]ModuleGroup, 0, len(roots))
	for _, g := range roots {
		groups = append(groups, *g)
	}
	return groups, nil
}

func findModuleRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found for %s", dir)
		}
		dir = parent
	}
}

func dirsForGroup(g ModuleGroup) []string {
	dirs := make(map[string]struct{})
	for _, f := range g.Files {
		rel, _ := filepath.Rel(g.Root, filepath.Dir(f))
		if rel == "." {
			rel = ""
		}
		dirs[rel] = struct{}{}
	}
	out := make([]string, 0, len(dirs))
	for d := range dirs {
		out = append(out, d)
	}
	return out
}
