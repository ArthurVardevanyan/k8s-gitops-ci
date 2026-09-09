package validator

import (
	"os"
	"sync"
	"testing"
)

// chdirMu serializes CWD changes across parallel tests to prevent ENOENT
// errors when one test changes CWD to a temp dir that gets cleaned up while
// another test (or tool like prettier) tries to read the CWD. Only serializes
// tests that actually call chdirTemp; others run in parallel.
var chdirMu sync.Mutex

// chdirTemp changes the current working directory to a new temp directory
// and restores the original CWD when the test completes. Tests must call
// this (instead of calling os.Chdir directly) to avoid race conditions
// with other parallel tests.
func chdirTemp(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	chdirMu.Lock()
	t.Cleanup(func() { chdirMu.Unlock() })
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	return d
}
