package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// PaC trigger classes carried via the --hook-source signal ({{ event_type }}).
// These are not hook Sources; ResolveSource maps them to one.
const (
	eventTypePullRequest = "pull_request" // PR open/sync → trusted base branch (main)
	eventTypePush        = "push"         // merge-queue push → checked-out merge commit
	eventTypeOnComment   = "on-comment"   // matched on-comment annotation (e.g. /hook-test)
)

// hookTestCommentRe matches the /hook-test gitops comment, optionally followed
// by whitespace-separated arguments (e.g. "/hook-test pp2000"). The command
// must be followed by whitespace or end-of-string so a prefix like
// /hook-test-evil is never treated as the allow-listed command. PaC escapes
// newlines in {{ trigger_comment }} to a literal "\n", so a leading-anchored
// match reliably identifies the command.
var hookTestCommentRe = regexp.MustCompile(`^/hook-test(\s|$)`)

// isHookTestComment reports whether the triggering comment is the /hook-test
// command. The comment body is the value of PaC's {{ trigger_comment }}.
func isHookTestComment(triggerComment string) bool {
	return hookTestCommentRe.MatchString(strings.TrimSpace(triggerComment))
}

// failClosed resolves to the trusted source: SourceMain when a PR is in play
// (prSet), otherwise SourceLocal (local dev).
func failClosed(prSet bool) Source {
	if prSet {
		return SourceMain
	}
	return SourceLocal
}

// ResolveSource maps a trigger signal into the hook Source used to read test.sh.
//
// The signal is either an explicit override (main, pr, local) or a PaC
// event_type carried via {{ event_type }}:
//
//   - main/pr/local        → returned unchanged (explicit override, e.g. local dev)
//   - pull_request         → SourceMain  (trusted base branch)
//   - push (merge queue)   → SourceLocal (the checked-out, approved merge commit)
//   - on-comment           → SourcePR only when triggerComment is an allow-listed
//     command (/hook-test); any other comment fails closed
//   - empty / unrecognized → fails closed
//
// "Fails closed" means: resolve to SourceMain whenever a PR is in play (prSet),
// so a malformed, unknown, or non-allow-listed signal can never cause CI to
// execute a PR-controlled test.sh. With no PR (local dev) it resolves to
// SourceLocal.
//
// triggerComment is the body of the gitops comment (PaC {{ trigger_comment }}),
// empty for non-comment triggers. It is only consulted for the on-comment class.
func ResolveSource(signal Source, triggerComment string, prSet bool) Source {
	switch signal {
	case SourceMain, SourcePR, SourceLocal:
		return signal
	case eventTypePush:
		return SourceLocal
	case eventTypePullRequest:
		return SourceMain
	case eventTypeOnComment:
		if isHookTestComment(triggerComment) {
			return SourcePR
		}
		return failClosed(prSet)
	default:
		return failClosed(prSet)
	}
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
	// MisdeclaredHooks records the reserved hook names
	// (PRE_BUILD_HOOK/POST_BUILD_HOOK/POST_VALIDATE_HOOK) that test.sh defines
	// as a bash function but never wires via the directive assignment form
	// (<HOOK>=<fn>). Such a function definition is invisible to the parser's
	// directive scan, so its hook silently never runs - callers should surface
	// this as a blocking error so a dead validation gate can't ship. See
	// hookDirectiveValue and pkg/validator/hookMisdeclaredErrors.
	MisdeclaredHooks []string
	HasPreBuild      bool
	HasPostBuild     bool
	HasPostValidate  bool
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
	// funcDefns tracks which reserved hook names are declared as bash function
	// definitions (the DIRECTIVE_OUTPUT form above parses only the assignment
	// directive <HOOK>=<fn>; a `POST_VALIDATE_HOOK() {...}` function definition
	// with no directive is detected here and reported via MisdeclaredHooks).
	funcDefns := make(map[string]bool, 3)
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip optional "export " prefix uniformly across all directives so
		// authors can write e.g. "export EXEMPTIONS=(...)" or
		// "export SCAFFOLD=false" consistently with hook command directives
		// (hookDirectiveValue already does this for PRE/POST_BUILD_HOOK etc.).
		line = strings.TrimPrefix(line, "export ")
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
		// Reserved-name function definition detection: a line like
		// POST_VALIDATE_HOOK() {...} or `function POST_VALIDATE_HOOK` defines
		// a bash function but ISN'T the assignment directive the parser looks
		// for, so the hook wouldn't be wired. Record it for the fail-loud
		// reporting below (never auto-run it).
		if name, ok := hookFuncDefName(line); ok {
			funcDefns[name] = true
		}
		_ = i
	}
	_ = avpLines
	sort.Strings(c.AVPExclude)
	c.checkMisdeclaredHooks(funcDefns)
}

// hookFuncDefName reports whether line is a bash function definition whose
// name is a reserved hook directive (PRE_BUILD_HOOK/POST_BUILD_HOOK/
// POST_VALIDATE_HOOK), returning that name and true. Both `NAME()` / `NAME ()`
// and `function NAME` (with or without `()`) forms are recognized. Comments
// and directive-assignment lines are handled upstream, so only bare function
// definitions reach here.
func hookFuncDefName(line string) (string, bool) {
	// Strip an optional "function " keyword, then take the first
	// whitespace-delimited token - the function name - terminated by an
	// optional "(" or body brace "{" (handles "NAME()", "NAME () {",
	// "function NAME" and "function NAME() {" alike).
	rest := strings.TrimSpace(strings.TrimPrefix(line, "function "))
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", false
	}
	first := strings.TrimRight(fields[0], "() {")
	first = strings.TrimSpace(first)
	for _, name := range []string{"PRE_BUILD_HOOK", "POST_BUILD_HOOK", "POST_VALIDATE_HOOK"} {
		if first == name {
			return name, true
		}
	}
	return "", false
}

// checkMisdeclaredHooks reports any reserved hook name that was declared as a
// function definition but lacks its directive assignment (the only form that
// actually wires a hook), prefixed with the corrective guidance.
func (c *Config) checkMisdeclaredHooks(funcDefns map[string]bool) {
	type hookName struct {
		name string
		has  bool
	}
	all := []hookName{
		{"PRE_BUILD_HOOK", c.HasPreBuild},
		{"POST_BUILD_HOOK", c.HasPostBuild},
		{"POST_VALIDATE_HOOK", c.HasPostValidate},
	}
	for _, h := range all {
		if funcDefns[h.name] && !h.has {
			c.MisdeclaredHooks = append(c.MisdeclaredHooks,
				h.name+" is defined as a function but has no <HOOK>=<fn> directive; declare '"+h.name+"=<fn>' and rename the function so it runs")
		}
	}
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
	line = unquote(strings.TrimSpace(strings.TrimSuffix(line, ")")))
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
