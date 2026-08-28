// Command verify-upstream-refs proves that every runtime validation check
// cites an upstream Kubernetes function that (a) exists and (b) has not
// changed since the check was last validated.
//
// It runs as a step in `task ci` via `task verify:upstream-refs`. Fetched
// sources are cached under XDG_CACHE_HOME keyed by tag, so a warm run does no
// network I/O - the same trade-off scripts/pull-schemas.sh already makes for
// its pinned artifact.
//
// The Kubernetes tag verified against is derived from the k8s.io/api version
// in go.mod, so bumping the Kubernetes dependency forces re-verification
// rather than letting the citation pin silently lag behind the typed structs
// the checks are written against.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/internal/upstreamref"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes" // registers runtime checks and their refs
)

const rawBase = "https://raw.githubusercontent.com/kubernetes/kubernetes"

func main() {
	var (
		tag      = flag.String("tag", "", "kubernetes/kubernetes tag to verify against (default: derived from k8s.io/api in go.mod)")
		cacheDir = flag.String("cache", defaultCache(), "directory to cache fetched upstream sources in")
		dump     = flag.Bool("dump", false, "print the registered refs as JSON and exit")
		update   = flag.Bool("update", false, "record the current digest and tag for refs that changed (do this only after re-reading the upstream function)")
		compute  = flag.String("compute", "", "compute the digest for a single ref instead of verifying: -compute <path> -functions <A,B>")
		fnList   = flag.String("functions", "", "comma-separated function names, used with -compute")
	)
	flag.Parse()

	// -compute exists to bootstrap a new ref. A ref cannot be registered
	// without a valid digest (RegisterAll rejects it), so the digest has to
	// be obtainable before the table entry exists.
	if *compute != "" {
		tagFor := *tag
		if tagFor == "" {
			t, err := tagFromGoMod()
			if err != nil {
				fatalf("%v", err)
			}
			tagFor = t
		}
		fns := strings.Split(*fnList, ",")
		for i := range fns {
			fns[i] = strings.TrimSpace(fns[i])
		}
		if len(fns) == 0 || fns[0] == "" {
			fatalf("-compute requires -functions")
		}
		f := &fetcher{tag: tagFor, cacheDir: *cacheDir, client: &http.Client{Timeout: 60 * time.Second}}
		src, err := f.get(*compute)
		if err != nil {
			fatalf("fetch %s: %v", *compute, err)
		}
		digest, err := upstreamref.Digest(src, fns)
		if err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("%s\t%s\n", digest, tagFor)
		return
	}

	refs := runtime.AllRefs()
	if *dump {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(refs); err != nil {
			fatalf("encode refs: %v", err)
		}
		return
	}

	if len(refs) == 0 {
		fatalf("no runtime checks are registered - nothing to verify")
	}

	if *tag == "" {
		t, err := tagFromGoMod()
		if err != nil {
			fatalf("%v", err)
		}
		*tag = t
	}

	fmt.Printf("Verifying %d upstream reference(s) against kubernetes/kubernetes %s\n\n", len(refs), *tag)

	f := &fetcher{tag: *tag, cacheDir: *cacheDir, client: &http.Client{Timeout: 60 * time.Second}}

	var missing, changed, stale, ok, updatedCount int
	ids := make([]string, 0, len(refs))
	for id := range refs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		ref := refs[id]

		src, err := f.get(ref.Path)
		if err != nil {
			missing++
			fmt.Printf("MISSING  %s\n         cannot fetch %s: %v\n", id, ref.Path, err)
			continue
		}

		got, err := upstreamref.Digest(src, ref.Functions)
		if err != nil {
			var me *upstreamref.MissingError
			if asMissing(err, &me) {
				missing++
				fmt.Printf("MISSING  %s\n         %s in %s\n", id, me.Error(), ref.Path)
				fmt.Printf("         the cited rule no longer exists upstream; this check is not a 1:1 port\n")
				continue
			}
			missing++
			fmt.Printf("ERROR    %s\n         %v\n", id, err)
			continue
		}

		switch got {
		case ref.Digest:
			ok++
			if ref.ValidatedAt != *tag {
				stale++
				if *update {
					if _, err := updateEntry(id, got, *tag); err != nil {
						fatalf("update %s: %v", id, err)
					}
				}
			}
		default:
			changed++
			fmt.Printf("CHANGED  %s\n", id)
			fmt.Printf("         %s %s\n", ref.Path, strings.Join(ref.Functions, ", "))
			fmt.Printf("         validated at %s, digest %s\n", ref.ValidatedAt, short(ref.Digest))
			fmt.Printf("         at %s, digest %s\n", *tag, short(got))
			if *update {
				found, err := updateEntry(id, got, *tag)
				if err != nil {
					fatalf("update %s: %v", id, err)
				}
				if !found {
					fatalf("update %s: no entry found in %s/**/upstream_refs.go", id, refsRoot)
				}
				fmt.Printf("         recorded new digest at %s\n", *tag)
				changed--
				updatedCount++
			} else {
				fmt.Printf("         re-read the upstream function and confirm the port is still faithful,\n")
				fmt.Printf("         then re-run with --update to record the new digest\n")
			}
		}
	}

	fmt.Printf("\n%d ok, %d changed, %d missing", ok, changed, missing)
	if updatedCount > 0 {
		fmt.Printf(", %d updated", updatedCount)
	}
	fmt.Println()
	if stale > 0 && !*update {
		fmt.Printf("%d ref(s) verified clean but recorded against an older tag; --update will restamp them to %s\n", stale, *tag)
	}

	if missing > 0 || changed > 0 {
		fmt.Fprintf(os.Stderr, "\nupstream reference verification failed\n")
		os.Exit(1)
	}
	fmt.Println("all upstream references verified")
}

// fetcher retrieves upstream files at a pinned tag, caching them on disk so a
// re-run is offline and so one file backing many refs is fetched once.
type fetcher struct {
	tag      string
	cacheDir string
	client   *http.Client
	memo     map[string][]byte
}

func (f *fetcher) get(path string) ([]byte, error) {
	if f.memo == nil {
		f.memo = map[string][]byte{}
	}
	if b, ok := f.memo[path]; ok {
		return b, nil
	}

	cache := filepath.Join(f.cacheDir, f.tag, filepath.FromSlash(path))
	if b, err := os.ReadFile(cache); err == nil {
		f.memo[path] = b
		return b, nil
	}

	url := fmt.Sprintf("%s/%s/%s", rawBase, f.tag, path)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err == nil {
		_ = os.WriteFile(cache, b, 0o644)
	}
	f.memo[path] = b
	return b, nil
}

// tagFromGoMod derives the kubernetes/kubernetes tag from the k8s.io/api
// version in go.mod. Staging modules are published as v0.X.Y alongside the
// v1.X.Y release they were cut from, so k8s.io/api v0.36.3 corresponds to
// kubernetes v1.36.3.
func tagFromGoMod() (string, error) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("read go.mod (run from the repository root): %w", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || fields[0] != "k8s.io/api" {
			continue
		}
		v := strings.TrimPrefix(fields[1], "v0.")
		if v == fields[1] {
			return "", fmt.Errorf("unexpected k8s.io/api version %q, cannot derive a kubernetes tag", fields[1])
		}
		return "v1." + v, nil
	}
	return "", fmt.Errorf("k8s.io/api not found in go.mod")
}

func defaultCache() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "k8s-gitops-ci", "upstream")
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "k8s-gitops-ci", "upstream")
}

func asMissing(err error, target **upstreamref.MissingError) bool {
	return errors.As(err, target)
}

// refsRoot is where the per-package upstream_refs.go tables live.
const refsRoot = "pkg/validator/runtime/kubernetes"

// updateEntry rewrites the Digest and ValidatedAt of a single map entry in
// the per-package upstream_refs.go tables. The entry is located by its check
// ID key, so unrelated entries are never touched.
//
// Re-validation is deliberately a human step: --update records that a person
// re-read the upstream function and confirmed the port is still faithful. The
// tool only automates the bookkeeping, because a digest match proves upstream
// did not change - not that our port was ever correct.
func updateEntry(id, digest, tag string) (bool, error) {
	var updated bool
	err := filepath.Walk(refsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Base(path) != "upstream_refs.go" || updated {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(src)

		key := fmt.Sprintf("%q: {", id)
		start := strings.Index(text, key)
		if start < 0 {
			return nil
		}
		end := strings.Index(text[start:], "\n\t},")
		if end < 0 {
			return fmt.Errorf("%s: malformed entry for %s", path, id)
		}
		entry := text[start : start+end]

		replaced := digestLine.ReplaceAllString(entry, "Digest:      "+strconv.Quote(digest))
		replaced = validatedLine.ReplaceAllString(replaced, "ValidatedAt: "+strconv.Quote(tag))
		if replaced == entry {
			return nil
		}

		out := text[:start] + replaced + text[start+end:]
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			return err
		}
		updated = true
		return nil
	})
	return updated, err
}

var (
	digestLine    = regexp.MustCompile(`Digest:\s*"[^"]*"`)
	validatedLine = regexp.MustCompile(`ValidatedAt:\s*"[^"]*"`)
)

func short(digest string) string {
	d := strings.TrimPrefix(digest, "sha256:")
	if len(d) > 12 {
		d = d[:12]
	}
	return d
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
