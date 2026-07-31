package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestDiscoverApps(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	mustWrite(t, filepath.Join(".scafctl", "configs", "app-a.yaml"), "{}\n")
	mustWrite(t, filepath.Join(".scafctl", "configs", "app-b.yml"), "{}\n")

	apps := DiscoverApps()
	if len(apps) != 2 || apps[0] != "app-a" || apps[1] != "app-b" {
		t.Errorf("unexpected apps: %v", apps)
	}
}

func TestDiscoverApps_NoScaffoldDir(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if apps := DiscoverApps(); apps != nil {
		t.Errorf("expected no apps, got %v", apps)
	}
}

func TestGenerateScaffoldTable_SortedAndFormatted(t *testing.T) {
	table := GenerateScaffoldTable([]StatusRow{
		{App: "zeta", Overlay: "dev", Status: "✅ ok"},
		{App: "alpha", Overlay: "prod", Status: "❌ drift"},
		{App: "alpha", Overlay: "dev", Status: "✅ ok"},
	})
	if !strings.Contains(table, tableStart) || !strings.Contains(table, tableEnd) {
		t.Errorf("expected table markers, got:\n%s", table)
	}
	// alpha/dev must sort before alpha/prod, which sorts before zeta/dev.
	iAlphaDev := strings.Index(table, "| alpha | dev |")
	iAlphaProd := strings.Index(table, "| alpha | prod |")
	iZetaDev := strings.Index(table, "| zeta | dev |")
	if iAlphaDev >= iAlphaProd || iAlphaProd >= iZetaDev {
		t.Errorf("expected sorted App-then-Overlay order, got:\n%s", table)
	}
}

func TestStatusRowsForApp_Classification(t *testing.T) {
	summary := &Summary{
		MismatchFiles:   []string{"dev"},
		SkippedClusters: []string{"staging"},
	}
	rows := statusRowsForApp("myapp", []string{"dev", "staging", "prod"}, summary)
	byOverlay := map[string]string{}
	for _, r := range rows {
		byOverlay[r.Overlay] = r.Status
	}
	if byOverlay["dev"] != "❌ drift" {
		t.Errorf("dev status = %q, want drift", byOverlay["dev"])
	}
	if byOverlay["staging"] != "⏭️ skipped" {
		t.Errorf("staging status = %q, want skipped", byOverlay["staging"])
	}
	if byOverlay["prod"] != "✅ ok" {
		t.Errorf("prod status = %q, want ok", byOverlay["prod"])
	}
}

func TestStatusRowsForApp_ExecFailureAppliesToUnclassifiedOverlays(t *testing.T) {
	summary := &Summary{Errors: []string{"scafctl not found"}}
	rows := statusRowsForApp("myapp", []string{"dev"}, summary)
	if len(rows) != 1 || rows[0].Status != "❌ error" {
		t.Errorf("expected an error status row, got %+v", rows)
	}
}

func TestUpdateReadmeStatus_NoReadmeIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := UpdateReadmeStatus(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateReadmeStatus_RegeneratesFromRealDiscovery(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	mustWrite(t, "README.md", "# Readme\n")
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")

	withFakeScafctl(t, func(_ context.Context, _, outputDir string) error {
		genDir := filepath.Join(outputDir, "dev")
		if err := os.MkdirAll(genDir, 0o750); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(genDir, "kustomization.yaml"), []byte("resources: []\n"), 0o600)
	})

	if err := UpdateReadmeStatus(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "| myapp | dev | ✅ ok |") {
		t.Errorf("expected a passing myapp/dev row, got:\n%s", content)
	}
}

// ── CheckReadmeStatus ────────────────────────────────────────────────────────

func TestCheckReadmeStatus_NoReadme(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	current, diff := CheckReadmeStatus()
	if !current || diff != "" {
		t.Errorf("expected current=true for a missing README, got current=%v diff=%q", current, diff)
	}
}

func TestCheckReadmeStatus_NoMarkerIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	mustWrite(t, "README.md", "# Readme\nNo scaffold table here.\n")
	current, diff := CheckReadmeStatus()
	if !current || diff != "" {
		t.Errorf("expected current=true when there's no marker, got current=%v diff=%q", current, diff)
	}
}

func TestCheckReadmeStatus_UpToDate(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")
	table := GenerateScaffoldTable([]StatusRow{{App: "myapp", Overlay: "dev", Status: "✅ ok"}})
	mustWrite(t, "README.md", "# Readme\n\n"+table)

	current, diff := CheckReadmeStatus()
	if !current {
		t.Errorf("expected current=true, got diff=%q", diff)
	}
}

func TestCheckReadmeStatus_MissingRow(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "prod", "kustomization.yaml"), "resources: []\n")
	// README only lists "dev" - "prod" is missing (e.g. a new cluster added
	// without running update-scaffold-status).
	table := GenerateScaffoldTable([]StatusRow{{App: "myapp", Overlay: "dev", Status: "✅ ok"}})
	mustWrite(t, "README.md", "# Readme\n\n"+table)

	current, diff := CheckReadmeStatus()
	if current {
		t.Fatal("expected current=false for a missing row")
	}
	if !strings.Contains(diff, "myapp/prod") {
		t.Errorf("expected the diff to mention myapp/prod, got: %s", diff)
	}
}

func TestCheckReadmeStatus_StaleRow(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	mustWrite(t, filepath.Join(".scafctl", "configs", "myapp.yaml"), "{}\n")
	mustWrite(t, filepath.Join("myapp", "overlays", "dev", "kustomization.yaml"), "resources: []\n")
	// README lists a "removed" overlay that no longer exists on disk.
	table := GenerateScaffoldTable([]StatusRow{
		{App: "myapp", Overlay: "dev", Status: "✅ ok"},
		{App: "myapp", Overlay: "removed", Status: "✅ ok"},
	})
	mustWrite(t, "README.md", "# Readme\n\n"+table)

	current, diff := CheckReadmeStatus()
	if current {
		t.Fatal("expected current=false for a stale row")
	}
	if !strings.Contains(diff, "myapp/removed") {
		t.Errorf("expected the diff to mention myapp/removed, got: %s", diff)
	}
}

func TestReplaceMarkerSection_NoExistingMarker(t *testing.T) {
	got := replaceMarkerSection("# Readme\n", tableStart, tableEnd, tableStart+"\ncontent\n"+tableEnd+"\n")
	if !strings.Contains(got, "# Readme") || !strings.Contains(got, "content") {
		t.Errorf("unexpected result: %q", got)
	}
}

func TestReplaceMarkerSection_ReplacesExisting(t *testing.T) {
	content := "# Readme\n" + tableStart + "\nPREVIOUS\n" + tableEnd + "\nfooter\n"
	got := replaceMarkerSection(content, tableStart, tableEnd, tableStart+"\nUPDATED\n"+tableEnd)
	if strings.Contains(got, "PREVIOUS") {
		t.Errorf("expected previous content replaced, got: %q", got)
	}
	if !strings.Contains(got, "UPDATED") || !strings.Contains(got, "footer") {
		t.Errorf("expected new content + footer preserved, got: %q", got)
	}
}
