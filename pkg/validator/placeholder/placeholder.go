package placeholder

import (
	"bufio"
	"fmt"
	"io"
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

// sentinelPatterns are precompiled once at package init, rather than
// recompiled per sentinel per line of every file scanned (the previous
// approach was a real performance issue on large repos).
var sentinelPatterns []*regexp.Regexp

func init() {
	for _, s := range Sentinels {
		// (?i): sentinel matching is case-insensitive - a lowercase
		// "changeme" or "fixme" left in rendered YAML is just as much an
		// unresolved placeholder as the canonical uppercase form.
		sentinelPatterns = append(sentinelPatterns, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(s)+`\b`))
	}
}

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

// ValidateReaderWithOptions validates placeholders from a reader. r is a
// plain io.Reader (not *os.File) so callers with in-memory content (e.g.
// the check-engine adapters in pkg/validator, which have already-decoded
// document bytes) don't need to write a temp file just to call this.
func ValidateReaderWithOptions(r io.Reader, source string, opts Options) []ValidationError {
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
			errs = append(errs, ValidationError{File: source, Line: lineNo, Match: m, Context: trimmed})
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
	for _, re := range sentinelPatterns {
		// Only the first match per sentinel per line is reported (not
		// every occurrence), and the reported match is always uppercased
		// regardless of the source line's casing - matches how downstream
		// consumers of this package expect sentinel findings to be
		// reported.
		if m := re.FindString(line); m != "" {
			matches = append(matches, strings.ToUpper(m))
		}
	}
	// Intentionally not deduplicated for the angle-bracket/AVP placeholder
	// patterns above: a line with the same placeholder token twice (e.g.
	// "<PATH>/<PATH>/file") should yield two findings, not one - each
	// occurrence is a separate unresolved placeholder. This does NOT apply
	// to the sentinel patterns just above, which intentionally report only
	// the first match per sentinel per line (see comment above).
	return matches
}
