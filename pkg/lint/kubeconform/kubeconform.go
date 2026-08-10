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

// MissingSchemaHint is appended to schema-not-found errors (e.g. pointing
// an operator at how to add a custom CRD schema for their org - see
// docs/SCHEMAS.md). Defaults to empty (no hint appended).
var MissingSchemaHint string

// KnownNonManifestFiles lists YAML/YML basenames that, by strong
// ecosystem convention, are never Kubernetes manifests - they're
// configuration files for common Go-ecosystem tooling (Task, golangci-lint,
// GoReleaser, pre-commit) that happen to live at a repo root alongside
// real Kubernetes YAML. Without this, any changeset touching one of these
// (e.g. this repo's own Taskfile.yml) trips a permanent, unavoidable
// "missing 'kind' key" kubeconform error - not a one-off exemption case
// (which would need a per-file EXEMPTIONS=(...) entry, itself gated behind
// test.sh's fail-closed SourceMain resolution - see resolveHookSource - so
// it can never take effect for the very PR that first introduces the
// entry), but a structural fact about the file that's permanently true.
//
// Exported (not a plain unexported constant list) so an org/consumer layer
// can extend it for its own tooling's YAML config files - e.g. from a
// Configure()-equivalent in its own main() - following the same
// core-data-plus-override-var shape used elsewhere in this repo (see
// docs/DEVELOPMENT.md's "core data + org-injectable override" pattern),
// even though here it's one shared map an org adds to directly rather than
// a separate override map consulted alongside a core one - there's no
// risk of an org's addition shadowing or conflicting with a core entry,
// since basenames are simply either known-non-manifest or not.
//
// This is a fast, unconditional basename skip for files that are NEVER
// manifests regardless of content. It is complementary to the content-aware
// gate (see IsManifestYAML): the content gate additionally skips any
// raw-validated file that parses to YAML with no root apiVersion/kind
// (e.g. an Ansible inventory.yml or NMState config in a flat, non-app
// directory), while a file whose basename is listed here is skipped without
// even reading it.
var KnownNonManifestFiles = map[string]bool{
	"Taskfile.yml":            true,
	"Taskfile.yaml":           true,
	".golangci.yml":           true,
	".golangci.yaml":          true,
	".goreleaser.yaml":        true,
	".goreleaser.yml":         true,
	"dependabot.yml":          true,
	"dependabot.yaml":         true,
	".pre-commit-config.yaml": true,
	".bulldozer.yml":          true,
	".bulldozer.yaml":         true,
	".policy.yml":             true,
	".policy.yaml":            true,
}

// IsKnownNonManifestFile reports whether path's basename is a known
// non-Kubernetes-manifest tooling config file (see KnownNonManifestFiles).
func IsKnownNonManifestFile(path string) bool {
	return KnownNonManifestFiles[filepath.Base(path)]
}

// missingSchemaMarker is the substring kubeconform's validator package uses
// to identify a "no schema found for this resource kind" error (see
// github.com/yannh/kubeconform/pkg/validator.validate). Matched against
// r.Err.Error() so MissingSchemaHint can be appended and so these errors
// can be deliberately left unprefixed by resource identity (see
// ValidateFileBytes) - identical missing-schema errors across many files
// should collapse into a single dedup entry, unlike genuine validation
// errors which should stay attributed per-resource.
const missingSchemaMarker = "could not find schema for"

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
	// SkippedNonManifest lists files that were not validated because they
	// carry no root-level apiVersion/kind (see IsManifestYAML) - e.g. an
	// Ansible inventory or NMState config sitting in a flat, non-app
	// directory. Surfaced as a non-blocking note so the skip is visible
	// (never silent), not counted toward Skipped (which tracks kubeconform's
	// own per-resource SkipKinds skips).
	SkippedNonManifest []string
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

// maxListedFiles caps how many file paths StringWithFiles lists inline
// before summarizing the remainder as "and N more".
const maxListedFiles = 10

func (d DeduplicatedError) String() string {
	return fmt.Sprintf("%s (%d file(s))", d.Message, d.Count)
}

// StringWithFiles renders the same summary as String, plus an inline,
// defensively-deduplicated, capped listing of the affected files. Kept
// separate from String (rather than always including files inline) so
// callers that only want the compact summary aren't forced to also render
// a potentially long file list.
func (d DeduplicatedError) StringWithFiles() string {
	s := d.String()
	if len(d.Files) == 0 {
		return s
	}
	files := dedupeStrings(d.Files)
	extra := ""
	if len(files) > maxListedFiles {
		extra = fmt.Sprintf(", and %d more", len(files)-maxListedFiles)
		files = files[:maxListedFiles]
	}
	return s + "\n    files: " + strings.Join(files, ", ") + extra
}

// dedupeStrings returns sl with duplicate entries removed, preserving
// first-seen order. Defensive: Result.Deduplicate already avoids adding
// duplicate file paths per DeduplicatedError, but StringWithFiles
// shouldn't assume that invariant holds for every caller-constructed
// DeduplicatedError.
func dedupeStrings(sl []string) []string {
	seen := make(map[string]bool, len(sl))
	out := make([]string, 0, len(sl))
	for _, s := range sl {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
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
	r.SkippedNonManifest = append(r.SkippedNonManifest, src.SkippedNonManifest...)
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
		s += "  - " + d.StringWithFiles() + "\n"
	}
	if n := len(r.SkippedNonManifest); n > 0 {
		files := dedupeStrings(r.SkippedNonManifest)
		extra := ""
		if len(files) > maxListedFiles {
			extra = fmt.Sprintf(", and %d more", len(files)-maxListedFiles)
			files = files[:maxListedFiles]
		}
		s += fmt.Sprintf("  - skipped %d non-manifest YAML file(s) (no apiVersion/kind): %s%s\n", n, strings.Join(files, ", "), extra)
	}
	return s
}

// ExtractSchemas extracts the embedded schema archive and returns the
// directory containing the archive's top-level "kubernetes-json-schema"
// folder (i.e. the direct parent of custom-standalone-strict,
// master-standalone-strict, and master-local), ready to be used as
// Options.SchemaDir / passed to SchemaLocations.
//
// ExtractSchemas is a package var so an org/consumer layer can override the
// schema source at startup - e.g. to supply schemas from an OCI-pulled tar or
// a pre-populated directory - without the binary needing the `embedschemas`
// build tag. The default reads the (optionally-embedded) archive; when built
// without `embedschemas` it returns schemas.ErrNoEmbeddedArchive, and callers
// (pipeline Setup, phases) fall back gracefully.
var ExtractSchemas = defaultExtractSchemas

func defaultExtractSchemas() (dir string, cleanup func(), err error) {
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
//
// This directory is deliberately hardcoded as "-strict" rather than using
// kubeconform's {{.StrictSuffix}} template variable: scripts/pull-schemas.sh
// only ever populates a custom-standalone-strict directory (there is no
// non-strict custom-schema variant in the upstream kubernetes-json-schema
// source this repo tracks), so {{.StrictSuffix}} would resolve to a
// nonexistent "custom-standalone" directory whenever Options.Strict is
// false, breaking every custom/CRD schema lookup in non-strict mode for no
// compensating benefit.
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

// statusPrecedence ranks a multi-document file's aggregate Status: a file
// containing several YAML documents with differing outcomes (e.g. one
// invalid resource alongside otherwise-valid ones) is reported under
// whichever status is most severe, not whichever document happened to be
// validated last - a plain last-write-wins assignment would let a single
// invalid document's status be silently overwritten by a later valid one
// in the same file (and vice versa), a genuine correctness bug.
var statusPrecedence = []string{"error", "invalid", "valid", "skipped", "empty"}

func aggregateStatus(seen map[string]bool) string {
	for _, s := range statusPrecedence {
		if seen[s] {
			return s
		}
	}
	return "empty"
}

// resourceSignaturePrefix returns a "Kind \"name\": " prefix identifying
// res, or "" if a signature can't be determined (e.g. the document failed
// to parse enough to extract kind/name). Used to keep genuine validation
// errors attributed to the resource that produced them, so identically-
// worded errors from different resources don't collapse together in
// Result.Deduplicate.
func resourceSignaturePrefix(res *kfv.Result) string {
	sig, err := res.Resource.Signature()
	if err != nil || sig == nil {
		return ""
	}
	return fmt.Sprintf("%s %q: ", sig.Kind, sig.Name)
}

// ValidateFileBytes validates bytes with a given validator.
func ValidateFileBytes(v kfv.Validator, filename string, data []byte) FileResult {
	rc := io.NopCloser(bytes.NewReader(data))
	defer rc.Close()
	results := v.Validate(filename, rc)
	fr := FileResult{Filename: filename}
	seenStatus := make(map[string]bool, len(results))
	for i := range results {
		r := results[i]
		switch r.Status {
		case kfv.Valid:
			seenStatus["valid"] = true
			fr.ValidCount++
		case kfv.Invalid:
			seenStatus["invalid"] = true
			fr.InvalidCount++
			prefix := resourceSignaturePrefix(&r)
			if r.Err != nil {
				fr.Errors = append(fr.Errors, prefix+r.Err.Error())
			}
			for _, ve := range r.ValidationErrors {
				fr.Errors = append(fr.Errors, prefix+ve.Msg)
			}
		case kfv.Error:
			seenStatus["error"] = true
			fr.ErrorCount++
			if r.Err != nil {
				fr.Errors = append(fr.Errors, formatSchemaAwareError(&r))
			}
		case kfv.Skipped:
			seenStatus["skipped"] = true
			fr.SkippedCount++
		case kfv.Empty:
			seenStatus["empty"] = true
		}
	}
	fr.Status = aggregateStatus(seenStatus)
	return fr
}

// formatSchemaAwareError formats r.Err, appending MissingSchemaHint and
// deliberately omitting the resource-signature prefix when r.Err is a
// "could not find schema" error (so identical missing-schema errors across
// many files collapse into one Result.Deduplicate entry instead of being
// kept separate per resource), and prefixing with the resource signature
// otherwise (so genuine validation errors stay attributed per-resource).
func formatSchemaAwareError(r *kfv.Result) string {
	msg := r.Err.Error()
	if strings.Contains(msg, missingSchemaMarker) {
		if MissingSchemaHint != "" {
			msg += "; " + MissingSchemaHint
		}
		return msg
	}
	return resourceSignaturePrefix(r) + msg
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

// ValidateFiles validates a list of files. The returned error signals a
// genuine setup failure (e.g. NewValidator failing to construct) - it is
// deliberately nil whenever validation itself ran, even if res.Invalid or
// res.Errors is nonzero. Callers must inspect those Result fields directly
// to detect validation failures, not this error: at least two real call
// sites (pkg/validator/phases.go's raw kubeconform lint step and its
// rendered-overlay pass, both gated on `err == nil`) already correctly
// branch on res.Invalid/res.Errors and would
// silently drop findings from their reports if this returned a non-nil
// error whenever validation found problems.
func ValidateFiles(files []string, opts Options) (*Result, error) {
	v, err := NewValidator(opts)
	if err != nil {
		return nil, err
	}
	manifests, skipped := partitionManifests(files)
	res := validateFilesPar(v, manifests)
	res.SkippedNonManifest = append(res.SkippedNonManifest, skipped...)
	return res, nil
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
	manifests, skipped := partitionManifests(files)
	res := validateFilesPar(v, manifests)
	res.SkippedNonManifest = append(res.SkippedNonManifest, skipped...)
	return res, cleanup, nil
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

// updateResult folds fr's per-document counters into r's running totals.
// This sums every counter unconditionally rather than switching on
// fr.Status and adding only the matching bucket - a multi-document file
// mixing e.g. one invalid and one valid document has both ValidCount and
// InvalidCount set on the same FileResult (fr.Status reflects only the
// most severe outcome, see aggregateStatus), and gating on fr.Status here
// would silently drop whichever counter didn't match the aggregate status.
func updateResult(r *Result, fr FileResult) {
	r.Details = append(r.Details, fr)
	r.Valid += fr.ValidCount
	r.Invalid += fr.InvalidCount
	r.Errors += fr.ErrorCount
	r.Skipped += fr.SkippedCount
}

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}
