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

// runDocChecks evaluates all ScopeDoc checks once per unique doc and fans out.
func runDocChecks(files []string, selectors []exempt.Selector, workers int, disabled map[string]bool) check.Result {
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	docs := indexDocuments(files)
	checks := filterDisabled(check.ByScope(check.ScopeDoc), disabled)
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
	for _, c := range checks {
		dc, ok := c.(check.DocCheck)
		if !ok {
			continue
		}
		res := dc.CheckDoc(doc, "")
		res, ex := fanOut(res, files, selectors)
		findings = append(findings, res...)
		exempted = append(exempted, ex...)
	}
	return findings, exempted
}

// fanOut expands each finding across every file it was found in, evaluating
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
