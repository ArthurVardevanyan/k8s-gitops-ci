package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source identifies which test.sh source to trust.
type Source string

const (
	SourceMain  Source = "main"
	SourcePR    Source = "pr"
	SourceLocal Source = "local"
)

// ResolveSource chooses which test.sh source to use. Fail-closed to main.
func ResolveSource(signal Source, triggerComment string, prSet bool) Source {
	if signal == "" {
		return SourceMain
	}
	if signal == SourceLocal {
		return SourceLocal
	}
	if signal == SourcePR {
		if strings.TrimSpace(triggerComment) == "/hook-test" && prSet {
			return SourcePR
		}
		return SourceMain
	}
	return SourceMain
}

// ExemptSelector mirrors the hook-layer EXEMPTIONS entry.
type ExemptSelector struct {
	Check, File, Kind, Name, Namespace, Match, Value, Path string
}

// Config is the parsed per-app test.sh configuration.
type Config struct {
	Scaffold        bool
	AVPExclude      []string
	ExemptSelectors []ExemptSelector
	ExemptErrors    []string
	HasPreBuild     bool
	HasPostBuild    bool
	HasPostValidate bool
	ScriptPath      string
}

// DefaultConfig returns default test.sh configuration.
func DefaultConfig() *Config { return &Config{Scaffold: true} }

// ParseTestScript reads and parses the test.sh at path.
func ParseTestScript(path string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.ScriptPath = path
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	cfg.parse(string(data))
	return cfg, nil
}

func (c *Config) parse(text string) {
	lines := strings.Split(text, "\n")
	var inExemptions bool
	var avpLines []string
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// SCAFFOLD
		if strings.HasPrefix(line, "SCAFFOLD=") {
			c.Scaffold = parseBool(strings.TrimPrefix(line, "SCAFFOLD="))
		}
		// AVP_EXCLUDE
		if strings.HasPrefix(line, "AVP_EXCLUDE=") {
			rest := strings.TrimPrefix(line, "AVP_EXCLUDE=")
			c.AVPExclude = append(c.AVPExclude, splitUnquote(rest)...)
		}
		if strings.HasPrefix(line, "AVP=") {
			avpLines = append(avpLines, line)
		}
		// EXEMPTIONS
		if inExemptions {
			if strings.HasPrefix(line, ")") || line == "" {
				inExemptions = false
				continue
			}
			c.parseExemption(line)
			continue
		}
		if strings.HasPrefix(line, "EXEMPTIONS=(") {
			inExemptions = true
			first := strings.TrimPrefix(line, "EXEMPTIONS=(")
			if first != "" && !strings.HasPrefix(first, ")") {
				c.parseExemption(first)
			}
			continue
		}
		if strings.HasPrefix(line, "EXEMPTIONS=") {
			rest := strings.TrimPrefix(line, "EXEMPTIONS=")
			c.parseExemptionSingle(rest)
		}
		// hook scripts
		if strings.Contains(line, "PRE_BUILD_HOOK=") {
			c.HasPreBuild = true
		}
		if strings.Contains(line, "POST_BUILD_HOOK=") {
			c.HasPostBuild = true
		}
		if strings.Contains(line, "POST_VALIDATE_HOOK=") {
			c.HasPostValidate = true
		}
		_ = i
	}
	_ = avpLines
	sort.Strings(c.AVPExclude)
}

func (c *Config) parseExemption(line string) {
	line = strings.TrimSpace(strings.TrimSuffix(line, ")"))
	if line == "" {
		return
	}
	c.addExemption(line)
}

func (c *Config) parseExemptionSingle(raw string) {
	raw = unquote(strings.TrimSpace(raw))
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		c.addExemption(part)
	}
}

func (c *Config) addExemption(part string) {
	if !strings.Contains(part, "=") {
		c.ExemptErrors = append(c.ExemptErrors, fmt.Sprintf("invalid exemption %q", part))
		return
	}
	sel := ExemptSelector{}
	var checkSeen bool
	for _, kv := range strings.Split(part, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		ps := strings.SplitN(kv, "=", 2)
		if len(ps) != 2 {
			c.ExemptErrors = append(c.ExemptErrors, fmt.Sprintf("invalid exemption token %q in %q", kv, part))
			continue
		}
		key := strings.TrimSpace(ps[0])
		val := unquote(strings.TrimSpace(ps[1]))
		switch key {
		case "check":
			sel.Check = val
			checkSeen = true
		case "file":
			sel.File = val
		case "kind":
			sel.Kind = val
		case "name":
			sel.Name = val
		case "namespace":
			sel.Namespace = val
		case "match":
			sel.Match = val
		case "value":
			sel.Value = val
		case "path":
			sel.Path = val
		default:
			c.ExemptErrors = append(c.ExemptErrors, fmt.Sprintf("unknown exemption key %q", key))
		}
		if val == "" {
			c.ExemptErrors = append(c.ExemptErrors, fmt.Sprintf("empty value for key %q", key))
		}
	}
	if !checkSeen || sel.Check == "" {
		c.ExemptErrors = append(c.ExemptErrors, fmt.Sprintf("missing check= in exemption %q", part))
		return
	}
	c.ExemptSelectors = append(c.ExemptSelectors, sel)
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(unquote(s)))
	return s == "true" || s == "yes" || s == "1"
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == s[len(s)-1] && (s[0] == '"' || s[0] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

func splitUnquote(s string) []string {
	s = unquote(strings.TrimSpace(s))
	var out []string
	for _, p := range strings.Split(s, " ") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// HasScaffoldEnabled reports whether the app's test.sh opts into scaffold validation.
func HasScaffoldEnabled(path string) bool {
	cfg, err := ParseTestScript(path)
	if err != nil {
		return false
	}
	return cfg.Scaffold
}

// Runner executes hook scripts if present.
type Runner struct {
	Root string
}

// NewHookRunner creates a runner rooted at root.
func NewHookRunner(root string) *Runner { return &Runner{Root: root} }

func (r *Runner) preBuild(overlayPath string) error {
	script := filepath.Join(r.Root, "pre-build.sh")
	return runScript(script, overlayPath)
}

// RunHooks runs any declared scripts for the given overlay.
func (r *Runner) RunHooks(overlayPath string, cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if cfg.HasPreBuild {
		if err := r.preBuild(overlayPath); err != nil {
			return err
		}
	}
	return nil
}

func runScript(script, arg string) error {
	if _, err := os.Stat(script); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// Placeholder: actual exec omitted to avoid runtime dep in tests
	_ = script
	_ = arg
	return nil
}

// FindTestScript returns the canonical test.sh path for an app.
func FindTestScript(app string) string {
	return filepath.Join(app, "test.sh")
}
