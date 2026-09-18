package scaffold

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

// GenerateArgs builds the argument slice (after Binary) for a write-mode
// scaffold generation of app: the counterpart to ScaffoldArgs, which is
// dry-run only. It is used by Generate to materialize exactly what the tool
// would write for a revision, so two revisions' generated content can be
// compared byte-for-byte (see DiffOverlay) and drift can be attributed to
// the change under test or to an external data source.
//
// Required when DriftMode is DryRunParse and generated-content comparison is
// desired; nil disables the feature (callers fall back to treating any drift
// as new/blocking). Ignored under DiffDirs, whose output-to-dir contract
// Generate already understands. Org-injected.
var GenerateArgs func(app string) []string

// GenerateOptions configures one write-mode scaffold generation.
type GenerateOptions struct {
	// App is the scaffold app (matching <ScaffoldDir>/configs/<App>.yaml).
	App string
	// WorkDir is the root the tool runs in - typically a git worktree of the
	// revision being generated - so the tool reads that revision's config and
	// templates and writes generated files under WorkDir/<App>/overlays/.
	// Empty means the process's current working directory.
	WorkDir string
}

// GenerateResult is the generated content of every overlay of one app:
// overlay name -> overlay-relative file path -> SHA-256 of its bytes. An
// overlay absent from the map was not generated for that revision.
type GenerateResult struct {
	Overlays map[string]map[string]string
}

// DiffOverlay reports whether overlay's generated content differs between
// two GenerateResults, returning the sorted overlay-relative paths that
// differ (a path present in only one side is included). Two nil/missing
// overlays are equal; an overlay generated on one side only is a difference.
func DiffOverlay(base, head *GenerateResult, overlay string) (differ bool, files []string) {
	var b, h map[string]string
	if base != nil {
		b = base.Overlays[overlay]
	}
	if head != nil {
		h = head.Overlays[overlay]
	}
	if b == nil && h == nil {
		return false, nil
	}
	if b == nil || h == nil {
		files = make([]string, 0, len(b)+len(h))
		for k := range b {
			files = append(files, k)
		}
		for k := range h {
			files = append(files, k)
		}
		sort.Strings(files)
		return true, files
	}

	seen := make(map[string]bool, len(b))
	for k, bv := range b {
		seen[k] = true
		if hv, ok := h[k]; !ok || hv != bv {
			files = append(files, k)
		}
	}
	for k := range h {
		if !seen[k] {
			files = append(files, k)
		}
	}
	if len(files) == 0 {
		return false, nil
	}
	sort.Strings(files)
	return true, files
}

// Generate runs the scaffold tool in write mode for opts.App and returns a
// content hash of every overlay it generated. Under DryRunParse it runs
// GenerateArgs in opts.WorkDir and hashes the app's on-disk overlays
// afterwards; under DiffDirs it runs the output-to-directory contract into a
// temporary directory and hashes that. A transient tool failure is retried
// exactly as in Run (see retryExec), and a timeout is never retried.
func Generate(opts GenerateOptions) (*GenerateResult, error) {
	var result *GenerateResult
	_, err := retryExec(func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()
		r, genErr := runGenerate(ctx, opts)
		if genErr == nil {
			result = r
			return "", nil
		}
		msg := stripANSI(genErr.Error())
		if ctx.Err() == context.DeadlineExceeded {
			// A hung tool is a timeout, not a transient blip - never retry.
			return msg, fmt.Errorf("%w: %s", context.DeadlineExceeded, msg)
		}
		return msg, genErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// runGenerate is one write-mode generation attempt, factored into a package
// var so tests can substitute a fixture-writing fake without the real binary
// (matching the runScafctl seam used by Run).
var runGenerate = execGenerate

func execGenerate(ctx context.Context, opts GenerateOptions) (*GenerateResult, error) {
	if DriftMode == DryRunParse {
		return execGenerateInPlace(ctx, opts)
	}
	return execGenerateOutput(ctx, opts)
}

// execGenerateInPlace runs the org's write-mode argument vector (GenerateArgs)
// in the worktree and hashes the app's overlays as the tool left them.
func execGenerateInPlace(ctx context.Context, opts GenerateOptions) (*GenerateResult, error) {
	if GenerateArgs == nil {
		return nil, errors.New("scaffold: generated-content comparison requires GenerateArgs to be set in DryRunParse mode")
	}
	cmd := exec.CommandContext(ctx, Binary, GenerateArgs(opts.App)...) //nolint:gosec // Binary/GenerateArgs are operator-controlled package-level overrides, not user input
	cmd.Dir = opts.WorkDir
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("scaffold generate: %w: %s", err, stripANSI(string(out)))
	}
	return hashOverlayTrees(filepath.Join(opts.WorkDir, opts.App, "overlays"))
}

// execGenerateOutput runs the generic scafctl output-to-directory contract
// (matching execScafctl) but rooted at the worktree, and hashes the generated
// tree.
func execGenerateOutput(ctx context.Context, opts GenerateOptions) (*GenerateResult, error) {
	tmp, err := os.MkdirTemp("", "scaffold-generate-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	cfg := scaffoldConfigRelPath(opts.App, opts.WorkDir)
	cmd := exec.CommandContext(ctx, Binary, "scaffold", "--config", ConfigSource+"="+cfg, "--output", tmp) //nolint:gosec // Binary/ConfigSource are operator-controlled package-level overrides, not user input
	cmd.Dir = opts.WorkDir
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("scaffold generate: %w: %s", err, stripANSI(string(out)))
	}
	return hashOverlayTrees(tmp)
}

// scaffoldConfigRelPath returns the app's scaffold config path (relative to
// the worktree root) preferring the .yaml extension and falling back to .yml,
// matching configFilePath's resolution and pkg/configdiff's.
func scaffoldConfigRelPath(app, workDir string) string {
	for _, ext := range []string{".yaml", ".yml"} {
		p := filepath.Join(convention.ScaffoldDir, "configs", app+ext)
		if _, err := os.Stat(filepath.Join(workDir, p)); err == nil {
			return p
		}
	}
	return filepath.Join(convention.ScaffoldDir, "configs", app+".yaml")
}

// hashOverlayTrees hashes each immediate subdirectory of root (one per
// overlay) into a GenerateResult.
func hashOverlayTrees(root string) (*GenerateResult, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read generated output %s: %w", root, err)
	}
	res := &GenerateResult{Overlays: map[string]map[string]string{}}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		h, err := hashDir(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		res.Overlays[e.Name()] = h
	}
	return res, nil
}

// hashDir returns a relative-path -> SHA-256 map of every file under dir.
func hashDir(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		sum, err := hashFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = sum
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path derives from a generated/worktree tree, not user input
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
