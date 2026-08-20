package changeset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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

// PRFile is a single entry from the GitHub "list pull request files" API.
type PRFile struct {
	Filename string `json:"filename"`
	Status   string `json:"status"` // "added", "modified", "removed", "renamed", ...
}

// GetChangedFiles returns changed files based on the configured source.
func GetChangedFiles(opts Options) ([]string, error) {
	if opts.PR != "" && opts.RepoURL != "" && isNumericString(opts.PR) {
		return getChangedFilesFromPR(opts)
	}
	return gitDiff(opts.BaseRef, opts.IncludeDeletions)
}

func getChangedFilesFromPR(opts Options) ([]string, error) {
	files, err := fetchPRFiles(opts)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(files))
	for _, f := range files {
		if !opts.IncludeDeletions && f.Status == "removed" {
			continue
		}
		result = append(result, f.Filename)
	}
	sort.Strings(result)
	return result, nil
}

// GetAddedFiles returns only added files from the changeset.
func GetAddedFiles(opts Options) ([]string, error) {
	if opts.PR != "" && opts.RepoURL != "" && isNumericString(opts.PR) {
		return getAddedFilesFromPR(opts)
	}
	return gitDiffAdded(opts.BaseRef)
}

func getAddedFilesFromPR(opts Options) ([]string, error) {
	files, err := fetchPRFiles(opts)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, f := range files {
		if f.Status == "added" {
			result = append(result, f.Filename)
		}
	}
	sort.Strings(result)
	return result, nil
}

// GetAllFiles returns all git-tracked files plus untracked files that are
// not ignored by .gitignore (via `git ls-files` + `git ls-files
// --others --exclude-standard`), falling back to a plain directory walk only
// when git itself is unavailable or the current directory isn't a git repo.
func GetAllFiles() ([]string, error) {
	var files []string

	// Tracked files.
	trackedOut, trackedErr := exec.CommandContext(context.Background(), "git", "ls-files").Output()
	if trackedErr == nil {
		files = append(files, splitLines(trackedOut)...)
	}

	// Untracked files that are not .gitignore-d.
	untrackedOut, untrackedErr := exec.CommandContext(context.Background(), "git", "ls-files", "--others", "--exclude-standard").Output()
	if untrackedErr == nil {
		files = append(files, splitLines(untrackedOut)...)
	}

	// If both git commands failed, fall back to a plain directory walk.
	if trackedErr != nil && untrackedErr != nil {
		return walkDir(".")
	}

	sort.Strings(files)

	// De-duplicate (a file can theoretically appear in both outputs
	// if it was just staged — ls-files without flags lists tracked,
	// --others lists untracked, but there's no harm in deduplicating).
	seen := make(map[string]bool, len(files))
	var deduped []string
	for _, f := range files {
		if !seen[f] {
			seen[f] = true
			deduped = append(deduped, f)
		}
	}
	return deduped, nil
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

// ExcludeByPrefixes drops files starting with any of the given prefixes,
// returning the remainder in their original relative order. An empty prefixes
// slice returns files unchanged (no-op filter).
func ExcludeByPrefixes(files, prefixes []string) []string {
	if len(prefixes) == 0 {
		return files
	}
	var out []string
	for _, f := range files {
		matched := false
		for _, p := range prefixes {
			if p != "" && strings.HasPrefix(f, p) {
				matched = true
				break
			}
		}
		if !matched {
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

// fetchPRFiles fetches the full list of PR files (with status) from the
// GitHub API in one paginated call. --paginate ensures PRs with more than
// one page of files (>30, gh's default page size) aren't silently
// truncated. Callers filter/derive whatever subset they need from the
// returned status field rather than issuing a second, differently-jq'd API
// call, avoiding both an extra request and status-string duplication.
func fetchPRFiles(opts Options) ([]PRFile, error) {
	if !hasGH() {
		return nil, fmt.Errorf("gh command not available; %s", AuthHint())
	}
	repo := ExtractRepoFromURL(opts.RepoURL)
	if repo == "" {
		return nil, fmt.Errorf("could not extract repo from URL: %s", opts.RepoURL)
	}
	out, err := exec.CommandContext(
		context.Background(), "gh", "api", "--paginate",
		"-H", "Accept: application/vnd.github+json",
		"-H", "X-GitHub-Api-Version: 2022-11-28",
		fmt.Sprintf("repos/%s/pulls/%s/files", repo, opts.PR),
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api files: %w%s", err, ghResponseHint(out))
	}
	var files []PRFile
	if err := json.Unmarshal(out, &files); err != nil {
		// gh returned a non-JSON body (commonly an HTML error page) with a
		// zero exit code - surface a hint instead of a cryptic JSON error.
		return nil, fmt.Errorf("parsing PR files response: %w%s", err, ghResponseHint(out))
	}
	return files, nil
}

// ghResponseHint inspects a gh api response body and, if it doesn't look
// like JSON (e.g. an HTML error page), returns a short diagnostic suffix
// pointing at the most common cause: an invalid/expired gh token or the
// wrong gh host.
func ghResponseHint(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return ""
	}
	return " (gh returned a non-JSON response; check `gh auth status` - the token may be invalid/expired or pointed at the wrong host)"
}

// gitDiff returns changed files for local (non-PR) mode. When baseRef is
// empty (the common local-dev case), it diffs the working tree - the union
// of unstaged and staged changes - which is what a developer running
// test-all locally against uncommitted work expects to see. When baseRef is
// explicitly set (e.g. merge-queue runs comparing against the target
// branch), it diffs baseRef...HEAD instead.
func gitDiff(baseRef string, includeDeletions bool) ([]string, error) {
	diffFilter := "--diff-filter=d"
	if includeDeletions {
		diffFilter = ""
	}
	if baseRef != "" {
		return gitDiffBaseRef(baseRef, diffFilter)
	}
	return gitDiffWorkingTree(diffFilter), nil
}

func gitDiffAdded(baseRef string) ([]string, error) {
	if baseRef != "" {
		return gitDiffBaseRef(baseRef, "--diff-filter=A")
	}
	return gitDiffWorkingTree("--diff-filter=A"), nil
}

// gitDiffBaseRef diffs HEAD against baseRef using three-dot syntax
// (`git diff <baseRef>...HEAD`), finding what HEAD introduced since the
// merge-base - exactly what a merge-queue commit contains relative to the
// target branch.
func gitDiffBaseRef(baseRef, diffFilter string) ([]string, error) {
	args := []string{"diff", "--name-only", baseRef + "...HEAD"}
	if diffFilter != "" {
		args = append(args, diffFilter)
	}
	out, err := exec.CommandContext(context.Background(), "git", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff %s...HEAD: %w\n%s", baseRef, err, strings.TrimSpace(string(out)))
	}
	result := splitLines(out)
	sort.Strings(result)
	return result, nil
}

// gitDiffWorkingTree returns the deduplicated, sorted union of unstaged and
// staged changed files (`git diff` + `git diff --cached`), each filtered by
// diffFilter (e.g. "--diff-filter=d", "--diff-filter=A", or "" for none).
func gitDiffWorkingTree(diffFilter string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, cached := range []bool{false, true} {
		args := []string{"diff", "--name-only"}
		if cached {
			args = []string{"diff", "--cached", "--name-only"}
		}
		if diffFilter != "" {
			args = append(args, diffFilter)
		}
		out, _ := exec.CommandContext(context.Background(), "git", args...).Output()
		for _, f := range splitLines(out) {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	sort.Strings(result)
	return result
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
