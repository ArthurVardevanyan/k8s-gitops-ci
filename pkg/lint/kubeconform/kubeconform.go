package kubeconform

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	kfv "github.com/yannh/kubeconform/pkg/validator"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/lint/kubeconform/schemas"
)

// MissingSchemaHint is appended to schema-not-found errors.
var MissingSchemaHint string

// Options configures kubeconform validation.
type Options struct {
	SchemaLocations      []string
	SkipKinds            []string
	Strict               bool
	KubernetesVersion    string
	IgnoreMissingSchemas bool
	UseSchemas           bool
	SchemaDir            string
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		SchemaLocations: []string{
			"default",
			"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		},
		SkipKinds:         []string{"ExternalSecret", "AnalysisTemplate", "Rollout"},
		Strict:            true,
		KubernetesVersion: "1.29.0",
		UseSchemas:        true,
	}
}

// Result holds validation results.
type Result struct {
	Valid, Invalid, Errors, Skipped int
	Details                         []FileResult
}

// FileResult is per-file results.
type FileResult struct {
	Filename, Status                                   string
	Errors                                             []string
	ValidCount, InvalidCount, ErrorCount, SkippedCount int
}

// DeduplicatedError groups identical errors across files.
type DeduplicatedError struct {
	Message string
	Count   int
	Files   []string
}

func (d DeduplicatedError) String() string {
	s := fmt.Sprintf("%s (%d file(s))", d.Message, d.Count)
	if len(d.Files) > 0 {
		files := d.Files
		extra := ""
		if len(files) > 10 {
			extra = fmt.Sprintf(", and %d more", len(files)-10)
			files = files[:10]
		}
		s += "\n    files: " + strings.Join(files, ", ") + extra
	}
	return s
}

// Merge folds src's counts and details into r.
func (r *Result) Merge(src *Result) {
	if src == nil {
		return
	}
	r.Valid += src.Valid
	r.Invalid += src.Invalid
	r.Errors += src.Errors
	r.Skipped += src.Skipped
	r.Details = append(r.Details, src.Details...)
}

// Deduplicate de-duplicates result errors.
func (r *Result) Deduplicate() []DeduplicatedError {
	seen := make(map[string]*DeduplicatedError)
	var order []string
	for _, d := range r.Details {
		for _, e := range d.Errors {
			if de, ok := seen[e]; ok {
				de.Count++
				if len(de.Files) < 10 && !contains(de.Files, d.Filename) {
					de.Files = append(de.Files, d.Filename)
				}
				continue
			}
			seen[e] = &DeduplicatedError{Message: e, Count: 1, Files: []string{d.Filename}}
			order = append(order, e)
		}
	}
	out := make([]DeduplicatedError, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

// Summary renders a human-readable summary.
func (r *Result) Summary() string {
	s := fmt.Sprintf("Summary: %d valid, %d invalid, %d errors, %d skipped\n", r.Valid, r.Invalid, r.Errors, r.Skipped)
	for _, d := range r.Deduplicate() {
		s += "  - " + d.String() + "\n"
	}
	return s
}

// ExtractSchemas extracts the embedded schema archive and returns the
// directory containing the archive's top-level "kubernetes-json-schema"
// folder (i.e. the direct parent of custom-standalone-strict,
// master-standalone-strict, and master-local), ready to be used as
// Options.SchemaDir / passed to SchemaLocations.
func ExtractSchemas() (dir string, cleanup func(), err error) {
	extracted, cleanup, err := schemas.Extract()
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(extracted, "kubernetes-json-schema"), cleanup, nil
}

// SchemaLocations returns schema location templates for the local, embedded
// kubernetes-json-schema archive rooted at schemaBase (see ExtractSchemas).
//
// The archive has a fixed, kubernetes-version-independent layout:
//   - master-standalone-strict/  builtin Kubernetes resources, using
//     kubeconform's own {{.ResourceKind}}{{.KindSuffix}}.json convention.
//   - master-local/  additional builtin/apiextensions schemas not present in
//     master-standalone-strict (e.g. the top-level CustomResourceDefinition
//     object itself), using the same {{.ResourceKind}}{{.KindSuffix}}.json
//     convention.
//   - custom-standalone-strict/  third-party & OKD/OpenShift CRDs, using a
//     flat {kind}-{full-group}-{version}.json convention. Note this does
//     NOT use {{.KindSuffix}}: KindSuffix truncates multi-segment API groups
//     (e.g. "machineconfiguration.openshift.io" -> "machineconfiguration"),
//     so it can never match filenames like
//     "kubeletconfig-machineconfiguration.openshift.io-v1.json".
func SchemaLocations(schemaBase string) []string {
	return []string{
		filepath.Join(schemaBase, "master-standalone-strict", "{{.ResourceKind}}{{.KindSuffix}}.json"),
		filepath.Join(schemaBase, "master-local", "{{.ResourceKind}}{{.KindSuffix}}.json"),
		filepath.Join(schemaBase, "custom-standalone-strict", "{{.ResourceKind}}-{{.Group}}-{{.ResourceAPIVersion}}.json"),
	}
}

// NewValidator creates a kubeconform validator.
func NewValidator(opts Options) (kfv.Validator, error) {
	sl := opts.SchemaLocations
	if opts.SchemaDir != "" {
		sl = append(SchemaLocations(opts.SchemaDir), sl...)
	}
	skipKinds := make(map[string]struct{})
	for _, k := range opts.SkipKinds {
		skipKinds[k] = struct{}{}
	}
	return kfv.New(sl, kfv.Opts{
		SkipKinds:            skipKinds,
		KubernetesVersion:    opts.KubernetesVersion,
		Strict:               opts.Strict,
		IgnoreMissingSchemas: opts.IgnoreMissingSchemas,
	})
}

// ValidateFileBytes validates bytes with a given validator.
func ValidateFileBytes(v kfv.Validator, filename string, data []byte) FileResult {
	rc := io.NopCloser(bytes.NewReader(data))
	defer rc.Close()
	results := v.Validate(filename, rc)
	fr := FileResult{Filename: filename}
	for _, r := range results {
		switch r.Status {
		case kfv.Valid:
			fr.Status = "valid"
			fr.ValidCount++
		case kfv.Invalid:
			fr.Status = "invalid"
			fr.InvalidCount++
			if r.Err != nil {
				fr.Errors = append(fr.Errors, r.Err.Error())
			}
			for _, ve := range r.ValidationErrors {
				fr.Errors = append(fr.Errors, ve.Msg)
			}
		case kfv.Error:
			fr.Status = "error"
			fr.ErrorCount++
			if r.Err != nil {
				fr.Errors = append(fr.Errors, r.Err.Error())
			}
		case kfv.Skipped:
			fr.Status = "skipped"
			fr.SkippedCount++
		case kfv.Empty:
			fr.Status = "empty"
		}
	}
	if fr.Status == "" {
		fr.Status = "empty"
	}
	return fr
}

// ValidateBytes validates in-memory YAML content (e.g. a rendered
// `kustomize build` manifest stream) as a single named unit. name is used
// only for reporting (it need not exist on disk).
func ValidateBytes(name string, data []byte, opts Options) (*Result, error) {
	v, err := NewValidator(opts)
	if err != nil {
		return nil, err
	}
	res := &Result{}
	updateResult(res, ValidateFileBytes(v, name, data))
	return res, nil
}

// ValidateFiles validates a list of files.
func ValidateFiles(files []string, opts Options) (*Result, error) {
	v, err := NewValidator(opts)
	if err != nil {
		return nil, err
	}
	return validateFilesPar(v, files), nil
}

// ValidateDir validates all YAML files under dir, extracting schemas if needed.
func ValidateDir(dir string, opts Options) (*Result, func(), error) {
	cleanup := func() {}
	if opts.SchemaDir == "" {
		schemaDir, c, err := ExtractSchemas()
		if err != nil {
			return nil, nil, err
		}
		opts.SchemaDir = schemaDir
		cleanup = c
	}
	v, err := NewValidator(opts)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil //nolint:nilerr // filepath.Walk convention: skip entry, keep walking
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		return nil
	})
	return validateFilesPar(v, files), cleanup, nil
}

func validateFilesPar(v kfv.Validator, files []string) *Result {
	workers := runtime.NumCPU() * 2
	if workers > len(files) {
		workers = len(files)
	}
	if workers == 0 {
		return &Result{}
	}
	jobs := make(chan string, len(files))
	var mu sync.Mutex
	res := &Result{}
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
				fr := ValidateFileBytes(v, f, data)
				mu.Lock()
				updateResult(res, fr)
				mu.Unlock()
			}
		}()
	}
	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	return res
}

func updateResult(r *Result, fr FileResult) {
	r.Details = append(r.Details, fr)
	switch fr.Status {
	case "valid":
		r.Valid += fr.ValidCount
	case "invalid":
		r.Invalid += fr.InvalidCount
	case "error":
		r.Errors += fr.ErrorCount
	case "skipped":
		r.Skipped += fr.SkippedCount
	}
}

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}
