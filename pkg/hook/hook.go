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
	// PreBuildCmd/PostBuildCmd/PostValidateCmd hold the value assigned to
	// PRE_BUILD_HOOK=/POST_BUILD_HOOK=/POST_VALIDATE_HOOK= (e.g. the
	// function or command name to invoke - "run_my_script" in
	// PRE_BUILD_HOOK=run_my_script). RunPreBuildHook/RunPostBuildHook/
	// RunPostValidateHook (exec.go) source ScriptPath and invoke this
	// value as a command, so it may name either a shell function defined
	// elsewhere in the same test.sh or an external executable on PATH.
	PreBuildCmd     string
	PostBuildCmd    string
	PostValidateCmd string
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
		// hook scripts: PRE_BUILD_HOOK=<cmd> / POST_BUILD_HOOK=<cmd> /
		// POST_VALIDATE_HOOK=<cmd> (optionally "export "-prefixed, like
		// SCAFFOLD=/AVP_EXCLUDE=). <cmd> names a shell function (defined
		// elsewhere in the same test.sh) or external command that
		// RunPreBuildHook/RunPostBuildHook/RunPostValidateHook (exec.go)
		// invoke after sourcing the script.
		if v, ok := hookDirectiveValue(line, "PRE_BUILD_HOOK="); ok {
			c.PreBuildCmd = v
			c.HasPreBuild = v != ""
		}
		if v, ok := hookDirectiveValue(line, "POST_BUILD_HOOK="); ok {
			c.PostBuildCmd = v
			c.HasPostBuild = v != ""
		}
		if v, ok := hookDirectiveValue(line, "POST_VALIDATE_HOOK="); ok {
			c.PostValidateCmd = v
			c.HasPostValidate = v != ""
		}
		_ = i
	}
	_ = avpLines
	sort.Strings(c.AVPExclude)
}

// hookDirectiveValue matches a "<key><rest>" or "export <key><rest>" line
// and returns the unquoted, trimmed value after key (e.g. key
// "PRE_BUILD_HOOK=" against "export PRE_BUILD_HOOK=run_my_script" returns
// ("run_my_script", true)). ok is false when line doesn't start with key
// (after stripping an optional "export " prefix).
func hookDirectiveValue(line, key string) (value string, ok bool) {
	rest := strings.TrimPrefix(line, "export ")
	if !strings.HasPrefix(rest, key) {
		return "", false
	}
	return unquote(strings.TrimSpace(strings.TrimPrefix(rest, key))), true
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

// FindTestScript returns the canonical test.sh path for an app.
func FindTestScript(app string) string {
	return filepath.Join(app, "test.sh")
}
