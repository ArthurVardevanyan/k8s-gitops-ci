package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
)

func TestDir(t *testing.T) {
	if got := Dir(); got != ".scafctl/configs" {
		t.Errorf("Dir() = %q", got)
	}
}

func TestSortOverrideKeys(t *testing.T) {
	dir := t.TempDir()
	old := convention.ScaffoldDir
	convention.ScaffoldDir = dir
	defer func() { convention.ScaffoldDir = old }()
	_ = os.MkdirAll(Dir(), 0o755)

	input := `overlayDefinitions:
  overrides:
    zzz: 1
    aaa: 2
`
	path := filepath.Join(Dir(), "app.yaml")
	_ = os.WriteFile(path, []byte(input), 0o644)

	unsorted, err := CheckSortOrder()
	if err != nil || len(unsorted) != 1 {
		t.Fatalf("expected 1 unsorted file, got %v err %v", unsorted, err)
	}

	count, err := SortConfigs()
	if err != nil || count != 1 {
		t.Fatalf("SortConfigs error %v count %d", err, count)
	}

	unsorted, err = CheckSortOrder()
	if err != nil || len(unsorted) != 0 {
		t.Fatalf("expected sorted after fix: %v err %v", unsorted, err)
	}
}

func TestSortOverrideKeys_AlreadySorted(t *testing.T) {
	dir := t.TempDir()
	old := convention.ScaffoldDir
	convention.ScaffoldDir = dir
	defer func() { convention.ScaffoldDir = old }()
	_ = os.MkdirAll(Dir(), 0o755)

	input := `overlayDefinitions:
  overrides:
    aaa: 1
    zzz: 2
`
	path := filepath.Join(Dir(), "app.yaml")
	_ = os.WriteFile(path, []byte(input), 0o644)

	unsorted, err := CheckSortOrder()
	if err != nil || len(unsorted) != 0 {
		t.Fatalf("expected sorted, got %v err %v", unsorted, err)
	}
}

func TestSortOverrideKeys_NoOverrides(t *testing.T) {
	data := []byte("other:\n  key: value\n")
	sorted, err := sortBytes(data)
	if err != nil || string(sorted) != string(data) {
		t.Fatalf("expected unchanged: %q err %v", sorted, err)
	}
}

func TestSortConfigs_MissingDir(t *testing.T) {
	dir := t.TempDir()
	old := convention.ScaffoldDir
	convention.ScaffoldDir = dir
	defer func() { convention.ScaffoldDir = old }()

	_, err := SortConfigs()
	if err == nil {
		t.Fatal("expected missing dir error")
	}
}

func TestFormatUnsortedError(t *testing.T) {
	s := FormatUnsortedError([]string{"a.yaml", "b.yaml"})
	if s == "" || !strings.Contains(s, "sort-configs") {
		t.Fatalf("unexpected formatted error: %q", s)
	}
}
