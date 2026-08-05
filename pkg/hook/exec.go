package hook

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	preBuildTimeout     = 60 * time.Second
	postBuildTimeout    = 60 * time.Second
	postValidateTimeout = 120 * time.Second
)

// RunPreBuildHook executes the app's PRE_BUILD_HOOK (if defined) once per
// overlay, before that overlay is built. overlayPath is the overlay
// directory; outputPath is where its rendered manifest will be written.
func RunPreBuildHook(cfg *Config, overlayPath, outputPath string) error {
	if cfg == nil || !cfg.HasPreBuild || cfg.ScriptPath == "" {
		return nil
	}
	absOverlay, err := filepath.Abs(overlayPath)
	if err != nil {
		return fmt.Errorf("resolving overlay path: %w", err)
	}
	return runHookCommand(cfg.ScriptPath, "PRE_BUILD_HOOK", cfg.PreBuildCmd, preBuildTimeout, absOverlay, outputPath)
}

// RunPostBuildHook executes the app's POST_BUILD_HOOK (if defined) once per
// successfully-built overlay, with three args:
//
//	$1 YAML_FILE — absolute path to the rendered YAML
//	$2 OUTPUT_PATH — the overlay basename (e.g. "prod-east")
//	$3 OVERLAY — absolute path to the overlay directory
func RunPostBuildHook(cfg *Config, yamlFile, overlayPath string) error {
	if cfg == nil || !cfg.HasPostBuild || cfg.ScriptPath == "" {
		return nil
	}
	absYAML, err := filepath.Abs(yamlFile)
	if err != nil {
		return fmt.Errorf("resolving yaml path: %w", err)
	}
	absOverlay, err := filepath.Abs(overlayPath)
	if err != nil {
		return fmt.Errorf("resolving overlay path: %w", err)
	}
	outputPath := filepath.Base(overlayPath)
	return runHookCommand(cfg.ScriptPath, "POST_BUILD_HOOK", cfg.PostBuildCmd, postBuildTimeout, absYAML, outputPath, absOverlay)
}

// RunPostValidateHook executes the app's POST_VALIDATE_HOOK (if defined)
// once, after every overlay for the app has been built.
func RunPostValidateHook(cfg *Config, buildDir, appName string) error {
	if cfg == nil || !cfg.HasPostValidate || cfg.ScriptPath == "" {
		return nil
	}
	absBuildDir, err := filepath.Abs(buildDir)
	if err != nil {
		return fmt.Errorf("resolving build dir path: %w", err)
	}
	return runHookCommand(cfg.ScriptPath, "POST_VALIDATE_HOOK", cfg.PostValidateCmd, postValidateTimeout, absBuildDir, appName)
}

// runHookCommand sources scriptPath and invokes cmdName (the value of
// PRE_BUILD_HOOK=/POST_BUILD_HOOK=/POST_VALIDATE_HOOK=, a shell function
// defined elsewhere in the script or an external command on PATH) with
// args, under a timeout. hookName is only used for error messages.
func runHookCommand(scriptPath, hookName, cmdName string, timeout time.Duration, args ...string) error {
	quotedArgs := ""
	for _, a := range args {
		quotedArgs += fmt.Sprintf(" %q", a)
	}

	// Hooks may write structured errors to ERROR_LOG instead of (or in
	// addition to) a non-zero exit, so a hook can report multiple
	// problems in one run.
	errLog, err := os.CreateTemp("", "hook-errors-*")
	if err != nil {
		return fmt.Errorf("creating hook error log: %w", err)
	}
	defer os.Remove(errLog.Name()) //nolint:errcheck // Best-effort cleanup
	_ = errLog.Close()

	// Per-app environment that hooks may reference (APP, ROOT_PATH,
	// APP_TMP_DIR, ERROR_LOG, PIPELINE), mirroring the conventions of the
	// wider GitOps repo's other tooling that also sources test.sh.
	scriptDir := filepath.Dir(scriptPath)
	appName := filepath.Base(scriptDir)
	appTmpDir, err := os.MkdirTemp("", "hook-"+appName+"-")
	if err != nil {
		return fmt.Errorf("creating hook temp dir: %w", err)
	}
	defer os.RemoveAll(appTmpDir) //nolint:errcheck // Best-effort cleanup

	// cmd.Dir is set to scriptDir below, so source the script by basename
	// (a full path here would be resolved against scriptDir a second time).
	scriptBase := filepath.Base(scriptPath)
	rootPath, err := filepath.Abs(scriptDir)
	if err != nil {
		return fmt.Errorf("resolving app root path: %w", err)
	}
	script := fmt.Sprintf(
		`set -o errexit; set -o pipefail; set -o nounset; `+
			`export APP=%q ROOT_PATH=%q APP_TMP_DIR=%q ERROR_LOG=%q PIPELINE=yes; `+
			`source %q; %q%s`,
		appName, rootPath, appTmpDir, errLog.Name(), scriptBase, cmdName, quotedArgs,
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", script) //nolint:gosec // trusted test.sh content; see docs/SECURITY.md
	cmd.Dir = scriptDir
	// Run the hook in its own process group so a timeout kills the whole
	// tree (the hook's own descendants, e.g. a backgrounded/forgotten
	// child process), not just the immediate bash process - otherwise a
	// grandchild that inherited the stdout/stderr pipe can keep cmd.Wait
	// blocked well past ctx's deadline even after bash itself is killed.
	// The process-group setup/kill is platform-specific; see
	// exec_unix.go/exec_windows.go.
	setProcGroup(cmd)
	cmd.Cancel = func() error {
		return killProcessGroup(cmd)
	}
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("hook %s (%s) timed out after %s", hookName, cmdName, timeout)
	}
	if runErr != nil {
		if errContent, readErr := os.ReadFile(errLog.Name()); readErr == nil && len(errContent) > 0 {
			return fmt.Errorf("hook %s (%s) failed:\n%s", hookName, cmdName, string(errContent))
		}
		msg := stderr.String()
		if msg == "" {
			msg = stdout.String()
		}
		return fmt.Errorf("hook %s (%s) failed: %w\n%s", hookName, cmdName, runErr, msg)
	}

	// A hook may write to ERROR_LOG without exiting non-zero.
	if errContent, err := os.ReadFile(errLog.Name()); err == nil && len(errContent) > 0 {
		return fmt.Errorf("hook %s (%s) reported errors:\n%s", hookName, cmdName, string(errContent))
	}

	return nil
}
