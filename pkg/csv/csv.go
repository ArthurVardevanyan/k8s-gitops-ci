package csv

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Mismatch records a startingCSV-folder version mismatch.
type Mismatch struct {
	File       string
	FolderName string
	CSVVersion string
}

var versionRe = regexp.MustCompile(`^[vV]?\d`)

// CheckStartingCSVFolderMatch validates startingCSV versions match their folder.
func CheckStartingCSVFolderMatch(changedFiles []string) ([]Mismatch, error) {
	var out []Mismatch
	for _, f := range changedFiles {
		if !strings.Contains(f, "/components/") || !strings.HasSuffix(f, "kustomization.yaml") {
			continue
		}
		version, err := parseStartingCSV(f)
		if err != nil || version == "" {
			continue
		}
		folder := folderForFile(f)
		csvVersion := stripV(csvVersionPart(version))
		fldVersion := stripV(folder)
		if csvVersion != fldVersion {
			out = append(out, Mismatch{File: f, FolderName: folder, CSVVersion: csvVersion})
		}
	}
	return out, nil
}

// FormatMismatches formats mismatches for display.
func FormatMismatches(mismatches []Mismatch) string {
	if len(mismatches) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("startingCSV version does not match the component folder name:\n")
	for _, m := range mismatches {
		fmt.Fprintf(&b, "  - %s: folder %q vs CSV %q\n", m.File, m.FolderName, m.CSVVersion)
	}
	b.WriteString("Update the component folder name or the startingCSV value.")
	return b.String()
}

func parseStartingCSV(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	const maxScan = 512 * 1024
	buf := make([]byte, maxScan)
	scanner.Buffer(buf, maxScan)
	foundPath := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "path:") && strings.Contains(line, "/spec/startingCSV") {
			foundPath = true
			continue
		}
		if foundPath {
			if strings.HasPrefix(line, "-") || line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.Contains(line, "value:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.Trim(strings.TrimSpace(parts[1]), `"'`), nil
				}
			} else {
				foundPath = false
			}
		}
	}
	return "", scanner.Err()
}

func folderForFile(file string) string {
	idx := strings.Index(file, "/components/")
	if idx == -1 {
		return filepath.Base(filepath.Dir(file))
	}
	rest := file[idx+len("/components/"):]
	first := strings.SplitN(rest, "/", 2)[0]
	if versionRe.MatchString(first) {
		return first
	}
	return filepath.Base(filepath.Dir(file))
}

func stripV(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 0 && (s[0] == 'v' || s[0] == 'V') {
		return s[1:]
	}
	return s
}

func csvVersionPart(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "."); idx != -1 {
		return strings.TrimSpace(s[idx+1:])
	}
	return s
}
