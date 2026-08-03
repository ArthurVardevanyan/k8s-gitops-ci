package hook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFromWorkingTree(t *testing.T) {
	dir := t.TempDir()
	app := "test-app"
	appDir := filepath.Join(dir, app)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `#!/usr/bin/env bash
SCAFFOLD=false
AVP_EXCLUDE="cluster1"
POST_BUILD_HOOK=my_post_build
`
	if err := os.WriteFile(filepath.Join(appDir, "test.sh"), []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cfg, err := Resolve(app, SourceLocal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scaffold {
		t.Error("expected Scaffold=false")
	}
	if len(cfg.AVPExclude) != 1 || cfg.AVPExclude[0] != "cluster1" {
		t.Errorf("unexpected AVPExclude: %v", cfg.AVPExclude)
	}
	if !cfg.HasPostBuild {
		t.Error("expected HasPostBuild=true")
	}
}

func TestResolveFromWorkingTree_NoTestSh(t *testing.T) {
	dir := t.TempDir()
	app := "no-hooks-app"
	if err := os.MkdirAll(filepath.Join(dir, app), 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cfg, err := Resolve(app, SourceLocal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Scaffold {
		t.Error("expected default Scaffold=true")
	}
	if cfg.HasPreBuild || cfg.HasPostBuild || cfg.HasPostValidate {
		t.Error("expected no hooks")
	}
}

func TestResolve_UnknownSource_FailsClosedToMain(t *testing.T) {
	// An unrecognized source must NOT read the working tree's test.sh. The
	// working tree here disables scaffold; since this throwaway dir has no
	// "main" git ref, an unknown source that fails closed to SourceMain
	// resolves to DefaultConfig (Scaffold=true), proving the
	// potentially-PR-controlled file was never read.
	dir := t.TempDir()
	app := "untrusted-app"
	appDir := filepath.Join(dir, app)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "test.sh"), []byte("SCAFFOLD=false\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cfg, err := Resolve(app, Source("unrecognized"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Scaffold {
		t.Error("unknown source read the working-tree test.sh (SCAFFOLD=false); expected fail-closed to main (Scaffold=true)")
	}
}

func TestCleanupConfig_NilConfig(t *testing.T) {
	CleanupConfig(nil) // must not panic
}

func TestCleanupConfig_EmptyScriptPath(t *testing.T) {
	CleanupConfig(&Config{ScriptPath: ""}) // must not panic or attempt removal
}

func TestCleanupConfig_RemovesMainTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, mainTempPrefix+"*.sh")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	CleanupConfig(&Config{ScriptPath: tmpPath})

	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("expected the temp hook script to be removed")
		_ = os.Remove(tmpPath)
	}
}

func TestCleanupConfig_LeavesNonTempFile(t *testing.T) {
	// A working-tree test.sh path must never be removed.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sh")
	if err := os.WriteFile(path, []byte("SCAFFOLD=true\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	CleanupConfig(&Config{ScriptPath: path})

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected working-tree test.sh to remain, got: %v", err)
	}
}

func TestExists_SourceLocal_True(t *testing.T) {
	dir := t.TempDir()
	app := "has-test-sh"
	appDir := filepath.Join(dir, app)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "test.sh"), []byte("SCAFFOLD=false\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if !Exists(app, SourceLocal) {
		t.Error("expected Exists to report true for a directory with a test.sh")
	}
}

func TestExists_SourceLocal_False(t *testing.T) {
	dir := t.TempDir()
	app := "no-test-sh"
	if err := os.MkdirAll(filepath.Join(dir, app), 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if Exists(app, SourceLocal) {
		t.Error("expected Exists to report false for a directory with no test.sh")
	}
}

func TestExists_SourcePR_UsesWorkingTree(t *testing.T) {
	dir := t.TempDir()
	app := "pr-app"
	appDir := filepath.Join(dir, app)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "test.sh"), []byte("SCAFFOLD=false\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if !Exists(app, SourcePR) {
		t.Error("expected SourcePR to check the working tree like SourceLocal")
	}
}
