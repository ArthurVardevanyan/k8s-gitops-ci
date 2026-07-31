package hook

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// mainTempPrefix marks temp files Resolve(..., SourceMain) writes into the
// app directory so CleanupConfig only ever removes files it created itself.
const mainTempPrefix = "hook-main-"

// Resolve returns the hook Config for app by locating and parsing its
// test.sh. source controls where test.sh is read from:
//   - SourceMain: read from the base/target branch via `git show`, so a
//     PR can never smuggle in a weaker test.sh for its own validation run.
//   - SourcePR/SourceLocal: read from the working tree.
//
// Callers should normalize untrusted input via ResolveSource first; as a
// defense-in-depth measure, an unrecognized source here still falls back to
// SourceMain rather than the working tree.
//
// The returned Config's ScriptPath may point at a temp file (SourceMain);
// callers must defer CleanupConfig(cfg) once done resolving/running hooks.
func Resolve(app string, source Source) (*Config, error) {
	switch source {
	case SourceMain:
		return resolveFromMain(app)
	case SourcePR, SourceLocal:
		return resolveFromWorkingTree(app)
	default:
		return resolveFromMain(app)
	}
}

// resolveFromMain extracts test.sh from the base/target branch ("main")
// using `git show` and parses that content, without ever touching the
// working tree's own (possibly PR-controlled) test.sh. The extracted
// content is written to a temp file inside the app directory (rather than
// the OS temp dir) so that hook execution's `cmd.Dir`/BASH_SOURCE-relative
// path resolution behaves the same as it would for a working-tree test.sh.
func resolveFromMain(app string) (*Config, error) {
	ref := fmt.Sprintf("main:%s", FindTestScript(app))

	cmd := exec.CommandContext(context.Background(), "git", "show", ref)
	out, err := cmd.Output()
	if err != nil {
		// test.sh doesn't exist on main -> defaults, nothing to clean up.
		return DefaultConfig(), nil //nolint:nilerr // Intentional: a missing file on main is not an error.
	}

	tmpFile, err := os.CreateTemp(app, mainTempPrefix+"*.sh")
	if err != nil {
		return nil, fmt.Errorf("creating temp file for hook: %w", err)
	}
	if _, err := tmpFile.Write(out); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("writing hook script: %w", err)
	}
	_ = tmpFile.Close()
	if err := os.Chmod(tmpFile.Name(), 0o700); err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("chmod hook script: %w", err)
	}

	cfg, err := ParseTestScript(tmpFile.Name())
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, err
	}
	cfg.ScriptPath = tmpFile.Name()
	return cfg, nil
}

// resolveFromWorkingTree reads test.sh directly from the working directory.
func resolveFromWorkingTree(app string) (*Config, error) {
	return ParseTestScript(FindTestScript(app))
}

// CleanupConfig removes the temp script file Resolve(..., SourceMain) may
// have created for cfg. Safe to call on any Config (including nil, or one
// resolved from the working tree) - it only ever removes files matching the
// mainTempPrefix pattern this package itself writes.
func CleanupConfig(cfg *Config) {
	if cfg == nil || cfg.ScriptPath == "" {
		return
	}
	if strings.HasPrefix(filepath.Base(cfg.ScriptPath), mainTempPrefix) {
		_ = os.Remove(cfg.ScriptPath)
	}
}
