package cel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"k8s.io/apiserver/pkg/cel/library"

	"gopkg.in/yaml.v3"
)

// Rule represents a compiled CEL validation rule from a schema file.
type Rule struct {
	Program cel.Program // compiled CEL program
	Source  string      // raw CEL expression text
	Message string      // rule.message from schema
	Path    string      // JSON schema path (e.g., "properties.items.properties.endpoint")
	RuleIdx int         // index in x-kubernetes-validations array
}

// CompiledRules holds all compiled CEL rules indexed by kind and apiVersion.
type CompiledRules struct {
	// kind -> apiVersion -> rules compiled from that schema file
	Rules map[string]map[string][]*Rule
	// schemaFile -> rules for reporting (groups findings by source file)
	byFile map[string][]*Rule
}

// FileResult is per-schema-file results.
type FileResult struct {
	SchemaFile string // source CRD schema file name
	Resources  int    // resources validated
	Failures   int    // CEL rule violations
	Errors     int    // compilation/evaluation errors
	Rules      []Violation
}

// Violation is a single CEL rule violation.
type Violation struct {
	Resource string // "kind/name"
	Rule     string // CEL expression source text
	Message  string // rule.message from schema
	Field    string // field path in resource that failed
}

// Result holds validation results.
type Result struct {
	Valid, Invalid, Errors int
	Details                []FileResult
}

// Summary renders a human-readable summary.
func (r *Result) Summary() string {
	s := fmt.Sprintf("CEL Validation: Summary: %d valid, %d invalid, %d errors\n", r.Valid, r.Invalid, r.Errors)
	for _, d := range r.Details {
		switch {
		case d.Failures > 0:
			s += fmt.Sprintf("  - %s (%d failure(s)):\n", d.SchemaFile, d.Failures)
			for _, v := range d.Rules {
				s += fmt.Sprintf("    - %s: %q [%s] → %s\n", v.Resource, v.Message, v.Rule, v.Field)
			}
		case d.Errors > 0:
			s += fmt.Sprintf("  - %s (%d error(s))\n", d.SchemaFile, d.Errors)
		default:
			s += fmt.Sprintf("  - %s (%d resources, 0 failures)\n", d.SchemaFile, d.Resources)
		}
	}
	return s
}

// kubernetesCELEnv builds a single CEL environment shared across all rule
// compilations. Creating this env (which registers 7 k8s CEL libraries) is
// expensive, so it is built once and reused rather than rebuilt per rule.
// If creation fails, panic at init time — the CEL subsystem is unusable
// without the environment and there is no meaningful runtime recovery path.
var kubernetesCELEnv = func() *cel.Env {
	env, err := cel.NewEnv(
		cel.Variable("self", cel.AnyType),
		library.IP(),
		library.Lists(),
		library.Quantity(),
		library.SemverLib(),
		library.CIDR(),
		library.URLs(),
		library.Regex(),
	)
	if err != nil {
		panic(fmt.Sprintf("cel: failed to create Kubernetes CEL environment: %v", err))
	}
	return env
}()

// CompileRules walks the extracted schema directory, extracts x-kubernetes-validations
// from each JSON schema file, compiles CEL expressions with Kubernetes CEL libraries,
// and builds the kind→apiVersion→rules index. Rules containing oldSelf are skipped
// (they're for admission webhooks, not static validation).
func CompileRules(schemaDir string) (*CompiledRules, error) {
	rules := &CompiledRules{
		Rules:  make(map[string]map[string][]*Rule),
		byFile: make(map[string][]*Rule),
	}

	// Walk the standard kubeconform schema directory structure.
	subdirs := []string{
		"master-standalone-strict",
		"master-local",
		"custom-standalone-strict",
	}

	var errors []string

	for _, subdir := range subdirs {
		dirPath := filepath.Join(schemaDir, subdir)
		if _, statErr := os.Stat(dirPath); os.IsNotExist(statErr) {
			continue
		}
		walkErr := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".json") {
				if parseErr := parseSchemaFile(path, rules); parseErr != nil {
					errors = append(errors, fmt.Sprintf("failed to parse %s: %v", path, parseErr))
				}
			}
			return nil
		})
		if walkErr != nil {
			errors = append(errors, fmt.Sprintf("walk error: %v", walkErr))
		}
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("cel schema compilation encountered %d error(s)", len(errors))
	}

	return rules, nil
}

// compileCache holds CompiledRules keyed by schema directory path so that
// repeated calls for the same directory return the same cached instance.
// A package-level cache lets test binaries (which share one extraction dir
// via TestMain) compile once across dozens of RunAll calls, and ensures
// that the rendered + raw CEL passes in a single pipeline invocation share
// one compile instead of two.
var compileCache = struct {
	sync.Mutex
	m map[string]*CompiledRules
}{m: make(map[string]*CompiledRules)}

// CompileRulesCached is a memoized wrapper around CompileRules: calls for the
// same schema directory return the previously-compiled result.
func CompileRulesCached(schemaDir string) (*CompiledRules, error) {
	compileCache.Lock()
	defer compileCache.Unlock()
	if cached, ok := compileCache.m[schemaDir]; ok {
		return cached, nil
	}
	rules, err := CompileRules(schemaDir)
	if err != nil {
		return nil, err
	}
	compileCache.m[schemaDir] = rules
	return rules, nil
}

// parseSchemaFile reads a JSON schema file, extracts x-kubernetes-validations,
// compiles each CEL rule, and indexes it by kind/apiVersion.
func parseSchemaFile(schemaPath string, rules *CompiledRules) error {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	// Pre-filter: most schema files (94%+) have no x-kubernetes-validations;
	// skip the expensive JSON unmarshal if the key text isn't present.
	if !bytes.Contains(data, []byte("x-kubernetes-validations")) {
		return nil
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		return err
	}

	// Extract kind/apiVersion from the schema.
	kind, apiVersion, schemaFile := extractKindFromSchema(schemaPath, schema)
	if kind == "" {
		return nil
	}

	// Look for x-kubernetes-validations.
	rawRules, ok := schema["x-kubernetes-validations"].([]interface{})
	if !ok || len(rawRules) == 0 {
		return nil
	}

	versionRules := rules.Rules[kind]
	if versionRules == nil {
		versionRules = make(map[string][]*Rule)
	}

	for idx, rawRule := range rawRules {
		ruleMap, ok := rawRule.(map[string]interface{})
		if !ok {
			continue
		}

		source, _ := ruleMap["rule"].(string)
		message, _ := ruleMap["message"].(string)

		// Skip oldSelf rules - they're for validating changes, not static state.
		if strings.Contains(source, "oldSelf") {
			continue
		}

		// Compile the CEL program.
		program, err := compileCELWithEnv(source, kubernetesCELEnv)
		if err != nil {
			// Compilation failed - log and skip this rule.
			continue
		}

		rule := &Rule{
			Program: program,
			Source:  source,
			Message: message,
			Path:    extractRulePath(schemaPath),
			RuleIdx: idx,
		}

		versionRules[apiVersion] = append(versionRules[apiVersion], rule)
		rules.byFile[schemaFile] = append(rules.byFile[schemaFile], rule)
	}

	if len(versionRules) > 0 {
		rules.Rules[kind] = versionRules
	}

	return nil
}

// compileCELWithEnv compiles a CEL expression using a pre-built environment,
// avoiding the repeated cost of creating a new CEL env (with all 7 k8s
// libraries) for every rule.
func compileCELWithEnv(source string, env *cel.Env) (cel.Program, error) {
	ast, issues := env.Compile(source)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL expression: %w", issues.Err())
	}

	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	return program, nil
}

// extractKindFromSchema extracts the resource kind and apiVersion from a schema file.
// For built-in resources (master-standalone-strict/master-local), the kind is derived
// from the filename (e.g., Deployment.json → kind=Deployment, apiVersion=core/v1).
// For custom CRDs (custom-standalone-strict), the filename contains kind-group-version.
func extractKindFromSchema(schemaPath string, schema map[string]interface{}) (kind, apiVersion, schemaFile string) {
	// First, try to extract from the schema itself (apiVersion field in the JSON schema).
	if apiVer, ok := schema["apiVersion"].(string); ok && apiVer != "" {
		apiVersion = apiVer
	} else if version, ok := schema["version"].(string); ok && version != "" {
		// Some schemas use "version" instead of "apiVersion".
		if group, ok := schema["group"].(string); ok && group != "" {
			apiVersion = group + "/" + version
		} else {
			apiVersion = "v1"
		}
	}

	// Extract kind from filename.
	base := filepath.Base(schemaPath)
	ext := filepath.Ext(base)
	base = base[:len(base)-len(ext)] // remove .json extension

	// Check if this is a custom CRD schema (contains hyphens, indicating kind-group-version).
	if strings.Contains(base, "-") {
		parts := strings.SplitN(base, "-", 3)
		if len(parts) >= 3 {
			kind = parts[0]
			if apiVersion == "" {
				apiVersion = parts[1] + "/" + parts[2]
			}
		} else {
			kind = base
		}
	} else {
		// Built-in resource - kind is the filename stem.
		kind = base
	}

	// Default apiVersion for built-in resources.
	if apiVersion == "" && kind != "" {
		apiVersion = "v1"
	}

	schemaFile = filepath.Base(schemaPath)
	return kind, apiVersion, schemaFile
}

// extractRulePath determines the JSON schema path for a rule.
func extractRulePath(schemaPath string) string {
	base := filepath.Base(schemaPath)
	if strings.Contains(base, "-") {
		ext := filepath.Ext(base)
		return base[:len(base)-len(ext)]
	}
	return base
}

// ValidateBytes evaluates all compiled CEL rules against rendered YAML bytes.
// Each YAML document is checked against the rules for its kind/apiVersion.
func ValidateBytes(data []byte, compiled *CompiledRules, filename string) *Result {
	result := &Result{}

	// Parse YAML documents.
	docs := splitDocuments(data)
	for _, doc := range docs {
		if len(doc) == 0 {
			continue
		}

		var parsed map[string]interface{}
		if err := yaml.Unmarshal(doc, &parsed); err != nil {
			continue
		}

		kind, apiVersion := extractResourceMeta(parsed)
		if kind == "" {
			continue
		}

		// Look up rules for this kind/apiVersion.
		versionRules := compiled.Rules[kind]
		rules := versionRules[apiVersion]
		if len(rules) == 0 {
			// No rules for this kind/apiVersion - count as valid.
			result.Valid++
			continue
		}

		// Evaluate each rule.
		fileResult := FileResult{
			SchemaFile: filename,
			Resources:  1,
		}

		resourceSig := fmt.Sprintf("%s/%s", kind, extractName(parsed))

		for _, rule := range rules {
			val, err := evaluateRule(rule, parsed)
			if err != nil {
				fileResult.Errors++
				continue
			}

			if !isTruthy(val) {
				fieldPath := rule.Path
				if fieldPath == "" {
					fieldPath = "root"
				}
				fileResult.Rules = append(fileResult.Rules, Violation{
					Resource: resourceSig,
					Rule:     rule.Source,
					Message:  rule.Message,
					Field:    fieldPath,
				})
				fileResult.Failures++
			}
		}

		if fileResult.Failures > 0 || fileResult.Errors > 0 {
			result.Invalid++
			result.Details = append(result.Details, fileResult)
		} else {
			result.Valid++
		}
	}

	return result
}

// evaluateRule evaluates a single CEL rule against a resource map.
func evaluateRule(rule *Rule, resource map[string]interface{}) (interface{}, error) {
	val, _, err := rule.Program.Eval(map[string]interface{}{
		"self": resource,
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

// isTruthy checks if a CEL value is truthy.
func isTruthy(val interface{}) bool {
	if val == nil {
		return true
	}
	// CEL returns types.Val, check if it's a boolean false.
	if tb, ok := val.(*types.Bool); ok {
		return bool(*tb)
	}
	// Fallback: check if it's a Go bool (for tests).
	if b, ok := val.(bool); ok {
		return b
	}
	// Assume true for non-boolean values (most CEL rules return bool).
	return true
}

// extractName extracts the resource name from a parsed resource.
func extractName(parsed map[string]interface{}) string {
	metadata, ok := parsed["metadata"].(map[string]interface{})
	if !ok {
		return "<unknown>"
	}
	name, _ := metadata["name"].(string)
	if name == "" {
		return "<unknown>"
	}
	return name
}

// extractResourceMeta extracts the kind and apiVersion from a parsed resource.
func extractResourceMeta(parsed map[string]interface{}) (kind, apiVersion string) {
	kind, _ = parsed["kind"].(string)
	apiVersion, _ = parsed["apiVersion"].(string)
	return kind, apiVersion
}

// splitDocuments splits YAML data into individual documents.
func splitDocuments(data []byte) [][]byte {
	const sep = "\n---"
	if !strings.Contains(string(data), sep) {
		return [][]byte{data}
	}

	var docs [][]byte
	start := 0
	remaining := string(data)

	for {
		idx := strings.Index(remaining[start:], sep)
		if idx == -1 {
			docs = append(docs, []byte(remaining[start:]))
			break
		}
		docs = append(docs, []byte(remaining[start:start+idx]))
		start += idx + len(sep)
	}

	var nonEmpty [][]byte
	for _, d := range docs {
		if len(d) > 0 {
			nonEmpty = append(nonEmpty, d)
		}
	}
	return nonEmpty
}

// ValidateFiles validates a list of YAML files in parallel using a worker pool.
// Workers default to runtime.NumCPU()*2 when <= 0.
func ValidateFiles(files []string, compiled *CompiledRules, workers int) *Result {
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	if workers > len(files) {
		workers = len(files)
	}
	if workers < 1 {
		return &Result{}
	}

	jobs := make(chan string, len(files))
	results := make(chan *Result, len(files))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				result := ValidateBytes(data, compiled, f)
				results <- result
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results.
	combined := &Result{}
	for r := range results {
		combined.Merge(r)
	}

	return combined
}

// Merge folds src's counts and details into r.
func (r *Result) Merge(src *Result) {
	if src == nil {
		return
	}
	r.Valid += src.Valid
	r.Invalid += src.Invalid
	r.Errors += src.Errors
	r.Details = append(r.Details, src.Details...)
}
