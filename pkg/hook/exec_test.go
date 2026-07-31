package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTempScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sh")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("writing temp script: %v", err)
	}
	return path
}

func TestRunPreBuildHook_Success(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
PRE_BUILD_HOOK=my_pre_build
my_pre_build() {
	echo "pre-build: overlay=$1 output=$2"
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RunPreBuildHook(cfg, "/tmp/app/overlays/prod", "/tmp/output/prod"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPreBuildHook_NoHookDefined(t *testing.T) {
	cfg := &Config{HasPreBuild: false, ScriptPath: "/some/path"}
	if err := RunPreBuildHook(cfg, "/tmp/overlay", "/tmp/output"); err != nil {
		t.Fatalf("expected no error when hook not defined, got: %v", err)
	}
}

func TestRunPreBuildHook_NilConfig(t *testing.T) {
	if err := RunPreBuildHook(nil, "/tmp/overlay", "/tmp/output"); err != nil {
		t.Fatalf("expected no error for nil config, got: %v", err)
	}
}

func TestRunPreBuildHook_EmptyScriptPath(t *testing.T) {
	cfg := &Config{HasPreBuild: true, PreBuildCmd: "foo", ScriptPath: ""}
	if err := RunPreBuildHook(cfg, "/tmp/overlay", "/tmp/output"); err != nil {
		t.Fatalf("expected no error for empty script path, got: %v", err)
	}
}

func TestRunPreBuildHook_ExitFailure(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
PRE_BUILD_HOOK=my_pre_build
my_pre_build() {
	echo "setup failed" >&2
	exit 1
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RunPreBuildHook(cfg, "/tmp/overlay", "/tmp/output"); err == nil {
		t.Fatal("expected error from a failing PRE_BUILD_HOOK")
	}
}

func TestRunPreBuildHook_RelativePathConvertedToAbsolute(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
PRE_BUILD_HOOK=my_pre_build
my_pre_build() {
	if [[ "$1" != /* ]]; then
		echo "ERROR: expected absolute path, got: $1" >&2
		exit 1
	fi
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RunPreBuildHook(cfg, "some-app/overlays/prod", "prod"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPostBuildHook_Success(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
POST_BUILD_HOOK=my_post_build
my_post_build() {
	echo "validating $1 $2 $3"
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RunPostBuildHook(cfg, "/tmp/test.yaml", "/tmp/app/overlays/prod"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPostBuildHook_ErrorLog(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
POST_BUILD_HOOK=my_post_build
my_post_build() {
	echo "Error: SHA mismatch: $3" >>"${ERROR_LOG}"
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = RunPostBuildHook(cfg, "/tmp/test.yaml", "/tmp/app/overlays/prod")
	if err == nil {
		t.Fatal("expected error from hook writing to ERROR_LOG")
	}
	if !strings.Contains(err.Error(), "SHA mismatch") {
		t.Errorf("expected error to contain 'SHA mismatch', got: %v", err)
	}
}

func TestRunPostBuildHook_NoHookDefined(t *testing.T) {
	cfg := &Config{HasPostBuild: false, ScriptPath: "/some/path"}
	if err := RunPostBuildHook(cfg, "/tmp/test.yaml", "/tmp/app/overlays/prod"); err != nil {
		t.Fatalf("expected no error when hook not defined, got: %v", err)
	}
}

func TestRunPostBuildHook_OutputPathIsOverlayBasename(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
POST_BUILD_HOOK=my_post_build
my_post_build() {
	if [[ "$2" != "prod" ]]; then
		echo "Error: expected OUTPUT_PATH=prod, got $2" >>"${ERROR_LOG}"
	fi
	if [[ "$(basename "$3")" != "prod" ]]; then
		echo "Error: expected OVERLAY basename=prod, got $3" >>"${ERROR_LOG}"
	fi
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RunPostBuildHook(cfg, "/tmp/builds/myapp/prod.yaml", "overlays/prod"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPostBuildHook_ExportsAppEnv(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
POST_BUILD_HOOK=my_post_build
my_post_build() {
	: "${APP:?APP not set}"
	: "${APP_TMP_DIR:?APP_TMP_DIR not set}"
	: "${ROOT_PATH:?ROOT_PATH not set}"
	: "${PIPELINE:?PIPELINE not set}"
	[[ -d "${APP_TMP_DIR}" ]] || { echo "APP_TMP_DIR is not a directory" >&2; exit 1; }
	[[ -d "${ROOT_PATH}" ]] || { echo "ROOT_PATH is not a directory" >&2; exit 1; }
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RunPostBuildHook(cfg, "/tmp/builds/myapp/sb0100.yaml", "overlays/sb0100"); err != nil {
		t.Fatalf("expected hook to succeed with app env, got: %v", err)
	}
}

func TestRunPostValidateHook_Success(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
POST_VALIDATE_HOOK=my_post_validate
my_post_validate() {
	echo "checking $1 for $2"
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RunPostValidateHook(cfg, "/tmp/builds/myapp", "myapp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPostValidateHook_ExitFailure(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
POST_VALIDATE_HOOK=my_post_validate
my_post_validate() {
	echo "fatal error" >&2
	exit 1
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RunPostValidateHook(cfg, "/tmp/builds/myapp", "myapp"); err == nil {
		t.Fatal("expected error from a failing POST_VALIDATE_HOOK")
	}
}

func TestRunPostValidateHook_NoHookDefined(t *testing.T) {
	cfg := &Config{HasPostValidate: false, ScriptPath: "/some/path"}
	if err := RunPostValidateHook(cfg, "/tmp/builds/myapp", "myapp"); err != nil {
		t.Fatalf("expected no error when hook not defined, got: %v", err)
	}
}

func TestRunHookCommand_Timeout(t *testing.T) {
	path := writeTempScript(t, `#!/usr/bin/env bash
PRE_BUILD_HOOK=my_pre_build
my_pre_build() {
	sleep 5
}
`)
	cfg, err := ParseTestScript(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	start := time.Now()
	err = runHookCommand(cfg.ScriptPath, "PRE_BUILD_HOOK", cfg.PreBuildCmd, 50*time.Millisecond, "/tmp/overlay", "/tmp/output")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second { // sanity bound, not exact
		t.Errorf("expected the hook to be killed near the timeout, took %s", elapsed)
	}
}
