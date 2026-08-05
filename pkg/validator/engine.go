package validator

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// docSource maps a unique doc hash back to the files that contain it.
type docSource struct {
	data  []byte
	files []string
}

// filterDisabled removes checks whose ID is present in disabled.
func filterDisabled(checks []check.Check, disabled map[string]bool) []check.Check {
	if len(disabled) == 0 {
		return checks
	}
	out := make([]check.Check, 0, len(checks))
	for _, c := range checks {
		if !disabled[c.ID()] {
			out = append(out, c)
		}
	}
	return out
}

// runDocChecks evaluates ScopeDoc checks over raw changed source files.
//
// It runs in two tiers to implement the dual-pass (raw + rendered) model:
//
//   - Non-render-sensitive checks run over every given file, as before.
//   - Render-sensitive checks (see check.RenderSensitive) are authoritative
//     on the rendered overlay stream, evaluated separately by
//     runDocChecksRendered. Here they only run over files that are NOT
//     covered by any successfully-rendered overlay (renderedFiles) - a
//     brand-new component not yet wired into any kustomization.yaml, or a
//     file whose overlay failed to build - so a violation in a not-yet-
//     wired-up manifest is never silently skipped, while a raw fragment that
//     IS composed into a rendered overlay (e.g. a base with
//     `image: <PATCHED_BY_KUSTOMIZE>`, patched at build time) is judged only
//     on its rendered result and never produces a raw false positive.
//
// renderedFiles is the set of raw source paths that participate in at least
// one successfully-rendered overlay (cleaned paths). When empty (no overlays
// built, or the rendered pass is disabled), render-sensitive checks fall
// back to running over all files - matching the pre-dual-pass behavior.
func runDocChecks(files []string, renderedFiles map[string]bool, selectors []exempt.Selector, workers int, disabled map[string]bool) check.Result {
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	all := filterDisabled(check.ByScope(check.ScopeDoc), disabled)
	renderSensitive, rawOnly := check.PartitionByRenderSensitivity(all)

	// rawOnly checks run over every file; renderSensitive checks run only
	// over files not covered by a rendered overlay (the fallback tier).
	uncovered := files
	if len(renderedFiles) > 0 {
		uncovered = make([]string, 0, len(files))
		for _, f := range files {
			if !renderedFiles[filepath.Clean(f)] {
				uncovered = append(uncovered, f)
			}
		}
	}

	var combined check.Result
	rawResult := runDocCheckPass(files, rawOnly, selectors, workers, false)
	combined.Findings = append(combined.Findings, rawResult.Findings...)
	combined.Exempted = append(combined.Exempted, rawResult.Exempted...)

	if len(renderSensitive) > 0 && len(uncovered) > 0 {
		fbResult := runDocCheckPass(uncovered, renderSensitive, selectors, workers, false)
		combined.Findings = append(combined.Findings, fbResult.Findings...)
		combined.Exempted = append(combined.Exempted, fbResult.Exempted...)
	}
	return combined
}

// runDocChecksRendered evaluates render-sensitive ScopeDoc checks over the
// kustomize/AVP-rendered overlay stream - the authoritative source of truth
// for those checks. Each finding carries its overlay origin in File (like
// runKyvernoValidation's remap). Resource-level direct/indirect
// classification is handled by classifyResourceCompliance in the section
// composer, not by blanket ForcedDirect here.
func runDocChecksRendered(outputs []renderedOverlay, selectors []exempt.Selector, workers int, disabled map[string]bool) check.Result {
	if len(outputs) == 0 {
		return check.Result{}
	}
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	all := filterDisabled(check.ByScope(check.ScopeDoc), disabled)
	renderSensitive, _ := check.PartitionByRenderSensitivity(all)
	if len(renderSensitive) == 0 {
		return check.Result{}
	}

	jobs := make(chan renderedOverlay, len(outputs))
	var mu sync.Mutex
	var combined check.Result
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				for _, doc := range splitDocuments(j.data) {
					if len(doc) == 0 || isKyvernoPolicyDoc(doc) {
						continue
					}
					findings, exempted := evaluateRenderedDoc(doc, j.overlay, renderSensitive, selectors)
					mu.Lock()
					combined.Findings = append(combined.Findings, findings...)
					combined.Exempted = append(combined.Exempted, exempted...)
					mu.Unlock()
				}
			}
		}()
	}
	for _, o := range outputs {
		jobs <- o
	}
	close(jobs)
	wg.Wait()
	return combined
}

// runDocCheckPass evaluates the given checks over files, one worker per
// unique document. Shared by the raw pass and its render-sensitive fallback.
func runDocCheckPass(files []string, checks []check.Check, selectors []exempt.Selector, workers int, _ bool) check.Result {
	if len(checks) == 0 || len(files) == 0 {
		return check.Result{}
	}
	docs := indexDocuments(files)
	type job struct {
		hash string
		doc  *docSource
	}
	jobs := make(chan job, len(docs))
	var mu sync.Mutex
	var combined check.Result
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				findings, exempted := evaluateDoc(j.doc.data, j.doc.files, checks, selectors)
				mu.Lock()
				combined.Findings = append(combined.Findings, findings...)
				combined.Exempted = append(combined.Exempted, exempted...)
				mu.Unlock()
			}
		}()
	}
	for h, d := range docs {
		jobs <- job{hash: h, doc: d}
	}
	close(jobs)
	wg.Wait()
	return combined
}

// runOverlayChecks drives ScopeOverlay checks.
func runOverlayChecks(overlays []string, cluster string, selectors []exempt.Selector, workers int, disabled map[string]bool) check.Result {
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	checks := filterDisabled(check.ByScope(check.ScopeOverlay), disabled)
	if len(checks) == 0 {
		return check.Result{}
	}
	type job struct{ overlay, cluster string }
	jobs := make(chan job, len(overlays))
	var mu sync.Mutex
	var combined check.Result
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				for _, c := range checks {
					oc, ok := c.(check.OverlayCheck)
					if !ok {
						continue
					}
					findings := oc.CheckOverlay(j.overlay, j.cluster)
					findings, exempted := fanOut(findings, []string{j.overlay}, selectors)
					mu.Lock()
					combined.Findings = append(combined.Findings, findings...)
					combined.Exempted = append(combined.Exempted, exempted...)
					mu.Unlock()
				}
			}
		}()
	}
	for _, o := range overlays {
		jobs <- job{overlay: o, cluster: cluster}
	}
	close(jobs)
	wg.Wait()
	return combined
}

func indexDocuments(files []string) map[string]*docSource {
	docs := make(map[string]*docSource)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, doc := range splitDocuments(data) {
			if isKyvernoPolicyDoc(doc) {
				continue
			}
			h := fmt.Sprintf("%x", sha256.Sum256(doc))
			if docs[h] == nil {
				docs[h] = &docSource{data: doc}
			}
			docs[h].files = append(docs[h].files, f)
		}
	}
	for _, d := range docs {
		sort.Strings(d.files)
		d.files = uniqueStrings(d.files)
	}
	return docs
}

func splitDocuments(data []byte) [][]byte {
	const sep = "\n---"
	if !bytes.Contains(data, []byte(sep)) {
		return [][]byte{bytes.TrimSpace(data)}
	}
	var docs [][]byte
	start := 0
	for {
		i := bytes.Index(data[start:], []byte(sep))
		if i == -1 {
			docs = append(docs, bytes.TrimSpace(data[start:]))
			break
		}
		docs = append(docs, bytes.TrimSpace(data[start:start+i]))
		start += i + len(sep)
	}
	var nonEmpty [][]byte
	for _, d := range docs {
		if len(d) > 0 {
			nonEmpty = append(nonEmpty, d)
		}
	}
	return nonEmpty
}

func evaluateDoc(doc []byte, files []string, checks []check.Check, selectors []exempt.Selector) ([]check.Finding, []exempt.Applied) {
	var findings []check.Finding
	var exempted []exempt.Applied
	var kind string
	var kindLoaded bool
	for _, c := range checks {
		dc, ok := c.(check.DocCheck)
		if !ok {
			continue
		}
		if skipper, ok := c.(check.DocSkipper); ok {
			if !kindLoaded {
				kind = quickKind(doc)
				kindLoaded = true
			}
			if skipper.SkipDoc(kind) {
				continue
			}
		}
		res := dc.CheckDoc(doc, "")
		res, ex := fanOut(res, files, selectors)
		findings = append(findings, res...)
		exempted = append(exempted, ex...)
	}
	return findings, exempted
}

// evaluateRenderedDoc runs render-sensitive checks against a single rendered
// document. Findings are attributed to the overlay path (overlay) - the
// thing a reviewer can actually open. Resource-level direct/indirect
// classification is handled later by isDirectFinding in the section
// composer, not by blanket ForcedDirect here. Checks that implement
// check.RenderedDocCheck (e.g. placeholder, which enables AVP scanning on
// rendered input) use CheckRenderedDoc; the rest use CheckDoc.
func evaluateRenderedDoc(doc []byte, overlay string, checks []check.Check, selectors []exempt.Selector) ([]check.Finding, []exempt.Applied) {
	var findings []check.Finding
	var exempted []exempt.Applied
	var kind string
	var kindLoaded bool
	for _, c := range checks {
		dc, ok := c.(check.DocCheck)
		if !ok {
			continue
		}
		if skipper, ok := c.(check.DocSkipper); ok {
			if !kindLoaded {
				kind = quickKind(doc)
				kindLoaded = true
			}
			if skipper.SkipDoc(kind) {
				continue
			}
		}
		var res []check.Finding
		if rdc, ok := c.(check.RenderedDocCheck); ok {
			res = rdc.CheckRenderedDoc(doc, "")
		} else {
			res = dc.CheckDoc(doc, "")
		}
		res, ex := fanOut(res, []string{overlay}, selectors)
		findings = append(findings, res...)
		exempted = append(exempted, ex...)
	}
	return findings, exempted
}

// exemptions per (finding, file) pair. It returns both the surviving
// (non-exempted) findings and the accepted exemptions, so callers can
// record an audit-trail entry instead of silently discarding why a finding
// didn't appear in the report.
func fanOut(findings []check.Finding, files []string, selectors []exempt.Selector) ([]check.Finding, []exempt.Applied) {
	var out []check.Finding
	var exempted []exempt.Applied
	for _, f := range findings {
		for _, file := range files {
			f2 := f
			f2.File = file
			scalar := f2.Scalar()
			if ok, applied := exempt.Evaluate(f2.CheckID, scalar, f2.Annotations, selectors); ok {
				exempted = append(exempted, applied)
				continue
			}
			out = append(out, f2)
		}
	}
	return out, exempted
}

func uniqueStrings(sl []string) []string {
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

// filterKyvernoTestFixtureDirs drops files living in a directory that also
// contains a kyverno-test.yaml - a Kyverno CLI test manifest whose paired
// resource fixtures are deliberately non-compliant by design (e.g. a Pod
// intentionally missing a required field, to exercise a policy's "should
// fail" case) - from compliance doc-check input, since those fixtures
// aren't real workloads and shouldn't be held to podspec/psa/namespace/etc.
// standards. Checked against the filesystem (not just the given files
// list), since the kyverno-test.yaml sibling may not itself be part of the
// current changeset. Only affects the compliance doc-check pass; the file
// is still validated by kubeconform/Kyverno through their own paths.
func filterKyvernoTestFixtureDirs(files []string) []string {
	dirIsFixture := make(map[string]bool)
	out := make([]string, 0, len(files))
	for _, f := range files {
		dir := filepath.Dir(f)
		isFixture, cached := dirIsFixture[dir]
		if !cached {
			_, err := os.Stat(filepath.Join(dir, "kyverno-test.yaml"))
			isFixture = err == nil
			dirIsFixture[dir] = isFixture
		}
		if isFixture {
			continue
		}
		out = append(out, f)
	}
	return out
}

// detectSourceFiles marks files that are directly changed as forced-direct.
func detectSourceFiles(changed []string) map[string]bool {
	m := make(map[string]bool)
	for _, c := range changed {
		m[filepath.Clean(c)] = true
	}
	return m
}

// finalizeCompliance splits findings into blocking vs warning tables.
func finalizeCompliance(findings []check.Finding, changedFiles map[string]bool) (blocking, warning []check.Finding) {
	var direct, indirect []check.Finding
	for _, f := range findings {
		if f.ForcedDirect || changedFiles[f.File] {
			direct = append(direct, f)
		} else {
			indirect = append(indirect, f)
		}
	}
	return direct, indirect
}
