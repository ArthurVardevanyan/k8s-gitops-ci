package placeholder

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Options controls placeholder validation.
type Options struct {
	CheckAVP bool
}

// ValidationError records an unresolved placeholder.
type ValidationError struct {
	File    string
	Line    int
	Match   string
	Context string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s:%d: unresolved placeholder %q", e.File, e.Line, e.Match)
}

// Sentinels are uppercase sentinel values flagged as placeholders.
var Sentinels = []string{
	"CHANGEME", "CHANGE_ME", "PATCH_ME", "FIXME", "FIX_ME", "XXX", "PLACEHOLDER",
}

var knownNonPlaceholders = map[string]bool{
	"EOF": true, "EOT": true, "EOS": true, "END": true, "HTML": true,
	"BR": true, "HR": true, "LI": true, "UL": true, "OL": true, "TR": true,
	"TD": true, "TH": true, "H1": true, "H2": true, "H3": true, "H4": true,
	"H5": true, "H6": true,
}

var (
	placeholderRe  = regexp.MustCompile(`<([A-Z][A-Z0-9_-]*)>`)
	placeholderRe2 = regexp.MustCompile(`<([a-z]+-[a-z]+-[a-z]+)>`)
	avpRe          = regexp.MustCompile(`<(path|vault|aws|gcp):[^>]+>`)
)

// ValidateFile validates placeholders in a file with default options.
func ValidateFile(path string) []ValidationError {
	return ValidateFileWithOptions(path, Options{CheckAVP: true})
}

// ValidateFileWithOptions validates placeholders with options.
func ValidateFileWithOptions(path string, opts Options) []ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ValidateReaderWithOptions(f, path, opts)
}

// ValidateReaderWithOptions validates placeholders from a reader.
func ValidateReaderWithOptions(r *os.File, source string, opts Options) []ValidationError {
	var errs []ValidationError
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, m := range findPlaceholders(line, opts) {
			errs = append(errs, ValidationError{File: source, Line: lineNo, Match: m})
		}
	}
	_ = scanner.Err()
	return errs
}

func findPlaceholders(line string, opts Options) []string {
	var matches []string
	for _, re := range []*regexp.Regexp{placeholderRe, placeholderRe2} {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			if knownNonPlaceholders[m[1]] {
				continue
			}
			matches = append(matches, m[0])
		}
	}
	if opts.CheckAVP {
		matches = append(matches, avpRe.FindAllString(line, -1)...)
	}
	for _, s := range Sentinels {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(s) + `\b`)
		matches = append(matches, re.FindAllString(line, -1)...)
	}
	return dedup(matches)
}

func dedup(sl []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range sl {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
