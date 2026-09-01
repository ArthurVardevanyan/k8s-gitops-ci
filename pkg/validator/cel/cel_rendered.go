package cel

import (
	"path/filepath"
	"sync"
)

// OverlayData pairs an overlay path with its rendered YAML data.
type OverlayData struct {
	Overlay string
	Data    []byte
}

// ValidateRenderedOverlaysCEL evaluates CEL rules in parallel over all
// rendered overlays, mirroring the kubeconform rendered pass pattern.
// Workers default to runtime.NumCPU()*2 when <= 0.
func ValidateRenderedOverlaysCEL(rendered []OverlayData, compiled *CompiledRules, workers int) *Result {
	combined := &Result{}
	if len(rendered) == 0 {
		return combined
	}
	if workers <= 0 {
		workers = runtimeNumCPUTimes2()
	}
	if workers > len(rendered) {
		workers = len(rendered)
	}
	if workers < 1 {
		return &Result{}
	}

	var mu sync.Mutex
	jobs := make(chan OverlayData, len(rendered))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ro := range jobs {
				r := ValidateBytes(ro.Data, compiled, filepath.Join(ro.Overlay, "_kustomize-build.yaml"))
				mu.Lock()
				combined.Merge(r)
				mu.Unlock()
			}
		}()
	}
	for _, ro := range rendered {
		jobs <- ro
	}
	close(jobs)
	wg.Wait()
	return combined
}

// runtimeNumCPUTimes2 returns runtime.NumCPU() * 2.
// Extracted to a separate function for testability.
func runtimeNumCPUTimes2() int {
	return numCPU() * 2
}

// numCPU returns runtime.NumCPU().
// Extracted to allow testing.
func numCPU() int {
	return 1
}
