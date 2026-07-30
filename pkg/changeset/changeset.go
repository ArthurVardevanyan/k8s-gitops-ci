package changeset

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Options configures changed-file resolution.
type Options struct {
	RepoURL          string
	PR               string
	BaseRef          string
	IncludeDeletions bool
}

// GetChangedFiles returns changed files based on the configured source.
func GetChangedFiles(opts Options) ([]string, error) {
	if opts.PR != "" && isNumericString(opts.PR) {
		return fetchPRFiles(opts)
	}
	return gitDiff(opts.BaseRef, opts.IncludeDeletions)
}

// GetAddedFiles returns only added files from the changeset.
func GetAddedFiles(opts Options) ([]string, error) {
	if opts.PR != "" && isNumericString(opts.PR) {
		return fetchPRAddedFiles(opts)
	}
	return gitDiffAdded(opts.BaseRef)
}

// GetAllFiles walks the repository and returns all tracked-ish files.
func GetAllFiles() ([]string, error) {
	return walkDir(".")
}

// GetFilesUnderDirs walks each of the given directories and returns all files
// found within them, deduplicated. Use this instead of GetAllFiles when only
// specific subdirectories need to be processed.
func GetFilesUnderDirs(dirs []string) ([]string, error) {
	seen := make(map[string]bool)
	var all []string
	for _, dir := range dirs {
		files, err := walkDir(dir)
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", dir, err)
		}
		for _, f := range files {
			if !seen[f] {
				seen[f] = true
				all = append(all, f)
			}
		}
	}
	return all, nil
}

// FilterByExtension keeps files ending with any of the given extensions.
func FilterByExtension(files []string, exts ...string) []string {
	var out []string
	for _, f := range files {
		for _, ext := range exts {
			if strings.HasSuffix(strings.ToLower(f), strings.ToLower(ext)) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// ExcludeByExtension removes files ending with any of the given extensions.
func ExcludeByExtension(files []string, exts ...string) []string {
	var out []string
	set := make(map[string]bool)
	for _, e := range exts {
		set[strings.ToLower(e)] = true
	}
	for _, f := range files {
		if !set[strings.ToLower(filepath.Ext(f))] {
			out = append(out, f)
		}
	}
	return out
}

// FilterByPrefix keeps files starting with prefix.
func FilterByPrefix(files []string, prefix string) []string {
	var out []string
	for _, f := range files {
		if strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	return out
}

// FilterByPrefixes keeps files starting with any of the given prefixes,
// de-duplicated and in their original relative order. An empty prefixes
// slice returns files unchanged (no-op filter).
func FilterByPrefixes(files, prefixes []string) []string {
	if len(prefixes) == 0 {
		return files
	}
	seen := make(map[string]bool, len(files))
	var out []string
	for _, f := range files {
		for _, p := range prefixes {
			if p == "" {
				continue
			}
			if strings.HasPrefix(f, p) && !seen[f] {
				seen[f] = true
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// FilterByApp returns files that appear to belong to any of the named apps.
func FilterByApp(files, apps []string) []string {
	if len(apps) == 0 {
		return files
	}
	set := make(map[string]bool)
	for _, a := range apps {
		set[a] = true
	}
	var out []string
	for _, f := range files {
		first := firstPathSegment(f)
		if set[first] {
			out = append(out, f)
		}
	}
	return out
}

// AuthHint returns guidance when API calls fail.
func AuthHint() string {
	return "Set GH_TOKEN or run 'gh auth login'."
}

// ExtractRepoFromURL parses the owner/repo slug from a URL.
func ExtractRepoFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "git@") {
		parts := strings.Split(raw, ":")
		if len(parts) == 2 {
			path := strings.TrimSuffix(parts[1], ".git")
			ps := strings.Split(path, "/")
			if len(ps) >= 2 {
				return ps[len(ps)-2] + "/" + ps[len(ps)-1]
			}
		}
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	ps := strings.Split(path, "/")
	if len(ps) >= 2 {
		return ps[len(ps)-2] + "/" + ps[len(ps)-1]
	}
	return ""
}

func fetchPRFiles(opts Options) ([]string, error) {
	if !hasGH() {
		return nil, fmt.Errorf("gh command not available; %s", AuthHint())
	}
	repo := ExtractRepoFromURL(opts.RepoURL)
	args := []string{"api", fmt.Sprintf("repos/%s/pulls/%s/files", repo, opts.PR), "--jq", ".[].filename"}
	out, err := exec.CommandContext(context.Background(), "gh", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api files: %w", err)
	}
	return splitLines(out), nil
}

func fetchPRAddedFiles(opts Options) ([]string, error) {
	if !hasGH() {
		return nil, fmt.Errorf("gh command not available; %s", AuthHint())
	}
	repo := ExtractRepoFromURL(opts.RepoURL)
	args := []string{"api", fmt.Sprintf("repos/%s/pulls/%s/files", repo, opts.PR), "--jq", ".[] | select(.status == \"added\") | .filename"}
	out, err := exec.CommandContext(context.Background(), "gh", args...).Output()
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

func gitDiff(baseRef string, includeDeletions bool) ([]string, error) {
	ctx := context.Background()
	if baseRef == "" {
		baseRef = "origin/main"
	}
	var args []string
	if includeDeletions {
		args = []string{"diff", "--name-status", fmt.Sprintf("%s...HEAD", baseRef)}
	} else {
		args = []string{"diff", "--name-only", "--diff-filter=d", fmt.Sprintf("%s...HEAD", baseRef)}
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		// fallback plain diff
		out, err = exec.CommandContext(ctx, "git", "diff", "--name-only").Output()
		if err != nil {
			return nil, err
		}
	}
	if includeDeletions {
		return parseNameStatus(out), nil
	}
	return splitLines(out), nil
}

func gitDiffAdded(baseRef string) ([]string, error) {
	ctx := context.Background()
	if baseRef == "" {
		baseRef = "origin/main"
	}
	out, err := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=A", fmt.Sprintf("%s...HEAD", baseRef)).Output()
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

func parseNameStatus(out []byte) []string {
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			files = append(files, parts[len(parts)-1])
		} else {
			files = append(files, line)
		}
	}
	return files
}

func splitLines(out []byte) []string {
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(l); s != "" {
			lines = append(lines, s)
		}
	}
	return lines
}

func hasGH() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func isNumericString(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func firstPathSegment(p string) string {
	p = strings.TrimPrefix(strings.TrimSpace(p), "./")
	if idx := strings.Index(p, "/"); idx != -1 {
		return p[:idx]
	}
	return p
}

func walkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// IsPACPlaceholder reports whether a string is an unresolved Pipelines-as-Code template var.
func IsPACPlaceholder(s string) bool {
	matched, _ := regexp.MatchString(`\{\{\s*.*\s*\}\}`, s)
	return matched
}
