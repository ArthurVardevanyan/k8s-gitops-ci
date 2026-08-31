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

	// Resolve every digest up front when updating, so a shared constant can
	// be checked against all of its users rather than just the entry that
	// happens to be written first. Fetches are cached, so this is cheap.
	allDigests := map[string]string{}
	if *update {
		for _, id := range ids {
			ref := refs[id]
			src, err := f.get(ref.Path)
			if err != nil {
				continue
			}
			if d, err := upstreamref.Digest(src, ref.Functions); err == nil {
				allDigests[id] = d
			}
		}
	}
	desiredFor = func(field, id string) (string, bool) {
		if field == "ValidatedAt" {
			return *tag, true
		}
		d, ok := allDigests[id]
		return d, ok
	}

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
					found, err := updateEntry(id, got, *tag)
					if err != nil {
						fatalf("update %s: %v", id, err)
					}
					if !found {
						fatalf("update %s: no entry found in %s/**/upstream_refs.go", id, refsRoot)
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

		// Supporting citations are verified exactly like the primary one.
		// A rule whose input the API server prepares before calling the
		// function that reports the error is only as faithful as that
		// preparation, so drift in it has to fail the build too.
		for i, add := range ref.Additional {
			label := fmt.Sprintf("%s (additional[%d])", id, i)
			asrc, err := f.get(add.Path)
			if err != nil {
				missing++
				fmt.Printf("MISSING  %s\n         cannot fetch %s: %v\n", label, add.Path, err)
				continue
			}
			agot, err := upstreamref.Digest(asrc, add.Functions)
			if err != nil {
				missing++
				fmt.Printf("MISSING  %s\n         %v in %s\n", label, err, add.Path)
				continue
			}
			if agot == add.Digest {
				ok++
				continue
			}
			changed++
			fmt.Printf("CHANGED  %s\n", label)
			fmt.Printf("         %s %s\n", add.Path, strings.Join(add.Functions, ", "))
			fmt.Printf("         validated at %s, digest %s\n", add.ValidatedAt, short(add.Digest))
			fmt.Printf("         at %s, digest %s\n", *tag, short(agot))
			fmt.Printf("         supporting citation: re-read it and update the entry by hand\n")
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
// Both fields may be written either as a string literal or as an identifier
// referring to a package-level constant, and the tables use both forms: every
// entry spells the tag `ValidatedAt: validatedAt`, and several share a digest
// constant. An updater that only rewrote literals would silently do nothing
// for the tag on every entry, and nothing for a shared digest - while still
// reporting success. So an identifier is resolved to its declaration and the
// constant is rewritten instead.
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

		newEntry, rest, err := setRefField(entry, text, "Digest", digest, id, path)
		if err != nil {
			return err
		}
		text = rest
		newEntry, text, err = setRefField(newEntry, text, "ValidatedAt", tag, id, path)
		if err != nil {
			return err
		}

		// Re-locate the entry: rewriting a constant may have shifted it.
		start = strings.Index(text, key)
		end = strings.Index(text[start:], "\n\t},")
		out := text[:start] + newEntry + text[start+end:]

		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			return err
		}
		updated = true
		return nil
	})
	return updated, err
}

// desiredFor reports the value a check ID should end up with for the given
// field, so a shared constant can be checked for agreement across all its
// users before being rewritten. It is set by main once every digest for the
// run is known.
var desiredFor func(field, id string) (string, bool)

// entryStart matches the opening line of a top-level ref map entry.
var entryStart = regexp.MustCompile(`(?m)^\t"([^"]+)": \{`)

// entriesReferencing returns the check IDs whose entry sets field to the
// identifier name.
//
// Each top-level entry is isolated before its fields are read. Scanning the
// whole file with one pattern does not work: a lazy `.*?` between the ID and
// the field crosses entry boundaries, so the match starts at the first entry
// in the file and ends at the first use of the constant anywhere after it.
// The result names a check that does not use the constant and omits the one
// that does - and because regexp finds non-overlapping matches from the left,
// the real users after it are swallowed by the same match.
//
// That is not hypothetical. With two shared webhook digests in one file,
// asking who uses validatingWebhookDigest returned the first *mutating*
// entry, so `--update` compared the new digest against an unrelated check's
// desired value and rejected a legitimate update as a shared-digest conflict.
//
// Only the entry's own fields count, which is why the field must sit at
// exactly two tabs. A nested Additional ref is indented deeper, and its digest
// is not the entry's digest; treating it as one would attribute a supporting
// citation's value to the primary ref.
func entriesReferencing(file, field, name string) []string {
	out := make([]string, 0, 4)
	fieldRe := regexp.MustCompile(`(?m)^\t\t` + field + `:\s*` + name + `\b`)

	for _, loc := range entryStart.FindAllStringSubmatchIndex(file, -1) {
		id := file[loc[2]:loc[3]]
		body := file[loc[1]:]
		if end := strings.Index(body, "\n\t},"); end >= 0 {
			body = body[:end]
		}
		if fieldRe.MatchString(body) {
			out = append(out, id)
		}
	}
	return out
}

// setRefField sets one field of a ref entry to value. If the field holds a
// string literal it is rewritten in place; if it holds an identifier, the
// referenced constant declaration is rewritten instead.
//
// Rewriting a shared constant changes every entry using it, so a conflicting
// value is reported rather than applied - silently restamping unrelated
// checks would assert a human re-validated ports they never looked at.
func setRefField(entry, file, field, value, id, path string) (newEntry, newFile string, err error) {
	lit := regexp.MustCompile(field + `:(\s*)"[^"]*"`)
	if loc := lit.FindStringSubmatchIndex(entry); loc != nil {
		pad := entry[loc[2]:loc[3]]
		return entry[:loc[0]] + field + ":" + pad + strconv.Quote(value) + entry[loc[1]:], file, nil
	}

	ident := regexp.MustCompile(field + `:(\s*)([A-Za-z_][A-Za-z0-9_]*)`)
	loc := ident.FindStringSubmatchIndex(entry)
	if loc == nil {
		return entry, file, fmt.Errorf("%s: entry %s has no %s field to update", path, id, field)
	}
	name := entry[loc[4]:loc[5]]

	decl := regexp.MustCompile(`(?m)^(\s*(?:const\s+)?` + name + `\s*=\s*)"([^"]*)"`)
	dloc := decl.FindStringSubmatchIndex(file)
	if dloc == nil {
		return entry, file, fmt.Errorf("%s: entry %s references %s %q, whose declaration could not be found",
			path, id, field, name)
	}
	current := file[dloc[4]:dloc[5]]
	if current == value {
		return entry, file, nil
	}

	// A shared constant may only be rewritten when every entry using it
	// wants the same new value. Otherwise the rewrite would restamp checks
	// nobody re-validated, which is exactly the assertion --update exists
	// to make honestly.
	for _, other := range entriesReferencing(file, field, name) {
		if other == id {
			continue
		}
		want, ok := desiredFor(field, other)
		if !ok {
			return entry, file, fmt.Errorf("%s: entry %s wants %s %q, but that value lives in constant %q which entry %s also uses, and %s was not verified in this run", path, id, field, value, name, other, other)
		}
		if want != value {
			return entry, file, fmt.Errorf("%s: entries %s and %s share constant %q but need different %s values (%q vs %q); give one of them its own value", path, id, other, name, field, value, want)
		}
	}

	file = file[:dloc[3]] + strconv.Quote(value) + file[dloc[5]+1:]
	return entry, file, nil
}

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
