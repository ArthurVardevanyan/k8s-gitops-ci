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

const rawBase = "https://raw.githubusercontent.com"

func main() {
	var (
		tag      = flag.String("tag", "", "version to verify/compute against, applied to every ref regardless of Repo when set. Default (empty): kubernetes/kubernetes resolves its own version from k8s.io/api in go.mod, and any other repo resolves from its own go.mod requirement instead (see -repo)")
		cacheDir = flag.String("cache", defaultCache(), "directory to cache fetched upstream sources in")
		dump     = flag.Bool("dump", false, "print the registered refs as JSON and exit")
		update   = flag.Bool("update", false, "record the current digest and version (a release tag, or a commit SHA for a repo other than kubernetes/kubernetes) for refs that changed (do this only after re-reading the upstream function)")
		compute  = flag.String("compute", "", "compute the digest for a single ref instead of verifying: -compute <path> -functions <A,B>")
		fnList   = flag.String("functions", "", "comma-separated function names, used with -compute")
		repoFlag = flag.String("repo", runtime.DefaultRepo, "owner/name of the upstream repository, used with -compute")
	)
	flag.Parse()

	// -compute exists to bootstrap a new ref. A ref cannot be registered
	// without a valid digest (RegisterAll rejects it), so the digest has to
	// be obtainable before the table entry exists.
	if *compute != "" {
		tagFor, err := versionFor(*repoFlag, *tag)
		if err != nil {
			fatalf("%v", err)
		}
		fns := strings.Split(*fnList, ",")
		for i := range fns {
			fns[i] = strings.TrimSpace(fns[i])
		}
		if len(fns) == 0 || fns[0] == "" {
			fatalf("-compute requires -functions")
		}
		f := &fetcher{cacheDir: *cacheDir, client: &http.Client{Timeout: 60 * time.Second}}
		src, err := f.get(*repoFlag, tagFor, *compute)
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

	// userTag is exactly what the user passed via -tag ("" if they didn't).
	// It is kept separate from the per-repo versions resolved below so that
	// an explicit -tag overrides every ref's version regardless of Repo -
	// consistent with how -compute already treats it (tagFor := *tag,
	// applied to whatever -repo was given) - while an unset -tag still lets
	// each repo resolve its own default independently.
	userTag := *tag
	defaultRepoVersion, err := versionFor(runtime.DefaultRepo, userTag)
	if err != nil {
		fatalf("%v", err)
	}

	fmt.Printf("Verifying %d upstream reference(s) (kubernetes/kubernetes %s; other repos resolved individually from go.mod unless -tag overrides them)\n\n", len(refs), defaultRepoVersion)

	f := &fetcher{cacheDir: *cacheDir, client: &http.Client{Timeout: 60 * time.Second}}

	// versionCache memoizes each repo's resolved version (a go.mod parse
	// per distinct repo, not per ref).
	versionCache := map[string]string{runtime.DefaultRepo: defaultRepoVersion}
	versionForCached := func(repo string) (string, error) {
		if v, ok := versionCache[repo]; ok {
			return v, nil
		}
		v, err := versionFor(repo, userTag)
		if err != nil {
			return "", err
		}
		versionCache[repo] = v
		return v, nil
	}

	var missing, changed, stale, ok, updatedCount int
	ids := make([]string, 0, len(refs))
	for id := range refs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Resolve every digest up front when updating, so a shared constant can
	// be checked against all of its users rather than just the entry that
	// happens to be written first. Fetches are cached, so this is cheap.
	// Only RefKindRewrite refs have a digest to recompute; RefKindImport
	// has nothing here to write back.
	allDigests := map[string]string{}
	if *update {
		for _, id := range ids {
			ref := refs[id]
			if ref.EffectiveKind() != runtime.RefKindRewrite {
				continue
			}
			repo := ref.EffectiveRepo()
			version, err := versionForCached(repo)
			if err != nil {
				continue
			}
			src, err := f.get(repo, version, ref.Path)
			if err != nil {
				continue
			}
			if d, err := upstreamref.Digest(src, ref.Functions); err == nil {
				allDigests[id] = d
			}
		}
	}
	desiredFor = func(field, id string) (string, bool) {
		ref, ok := refs[id]
		if !ok {
			return "", false
		}
		if field == "ValidatedAt" {
			v, err := versionForCached(ref.EffectiveRepo())
			return v, err == nil
		}
		d, ok := allDigests[id]
		return d, ok
	}

	for _, id := range ids {
		ref := refs[id]
		repo := ref.EffectiveRepo()
		version, verr := versionForCached(repo)
		if verr != nil {
			missing++
			fmt.Printf("MISSING  %s\n         resolving version for repo %q: %v\n", id, repo, verr)
			continue
		}

		src, err := f.get(repo, version, ref.Path)
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

		// RefKindImport calls the cited code directly - go.mod and the
		// compiler already pin and verify exactly what runs, so there is no
		// recorded digest to compare against. Confirming the fetch above
		// succeeded and every cited function actually exists there is the
		// whole check for this kind.
		if ref.EffectiveKind() == runtime.RefKindImport {
			ok++
			continue
		}

		switch got {
		case ref.Digest:
			ok++
			if ref.ValidatedAt != version {
				stale++
				if *update {
					found, err := updateEntry(id, got, version)
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
			fmt.Printf("         at %s, digest %s\n", version, short(got))
			if *update {
				found, err := updateEntry(id, got, version)
				if err != nil {
					fatalf("update %s: %v", id, err)
				}
				if !found {
					fatalf("update %s: no entry found in %s/**/upstream_refs.go", id, refsRoot)
				}
				fmt.Printf("         recorded new digest at %s\n", version)
				changed--
				updatedCount++
			} else {
				fmt.Printf("         re-read the upstream function and confirm the port is still faithful,\n")
				fmt.Printf("         then re-run with --update to record the new digest\n")
			}
		}

		// Supporting citations are verified exactly like the primary one,
		// each resolved against its own Repo/Kind - an Additional entry may
		// cite a different upstream repository than its parent.
		for i, add := range ref.Additional {
			label := fmt.Sprintf("%s (additional[%d])", id, i)
			addRepo := add.EffectiveRepo()
			addVersion, averr := versionForCached(addRepo)
			if averr != nil {
				missing++
				fmt.Printf("MISSING  %s\n         resolving version for repo %q: %v\n", label, addRepo, averr)
				continue
			}
			asrc, err := f.get(addRepo, addVersion, add.Path)
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
			if add.EffectiveKind() == runtime.RefKindImport {
				ok++
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
			fmt.Printf("         at %s, digest %s\n", addVersion, short(agot))
			fmt.Printf("         supporting citation: re-read it and update the entry by hand\n")
		}
	}

	fmt.Printf("\n%d ok, %d changed, %d missing", ok, changed, missing)
	if updatedCount > 0 {
		fmt.Printf(", %d updated", updatedCount)
	}
	fmt.Println()
	if stale > 0 && !*update {
		fmt.Printf("%d ref(s) verified clean but recorded against an older version; --update will restamp them\n", stale)
	}

	if missing > 0 || changed > 0 {
		fmt.Fprintf(os.Stderr, "\nupstream reference verification failed\n")
		os.Exit(1)
	}
	fmt.Println("all upstream references verified")
}

// cachePathFor joins cacheDir/repo/version/path and verifies the result
// still lives under cacheDir, rejecting a repo, version, or path (an
// UpstreamRef field, or a -compute/-repo flag) that is absolute or that
// escapes via "..". filepath.Join alone cleans "." and ".." segments
// syntactically but happily produces a path outside cacheDir if enough ".."
// segments are present, so the cleaned result is walked back up to cacheDir
// to confirm containment before any file I/O is attempted. An absolute
// component is rejected explicitly rather than relied on to be neutralized
// by Join (which only preserves a leading "/" specially for its *first*
// argument - true today for repo/version/path here, since none of them is
// ever first, but that's an accident of argument order, not a guarantee
// this function should depend on).
func cachePathFor(cacheDir, repo, version, path string) (string, error) {
	base, err := filepath.Abs(cacheDir)
	if err != nil {
		return "", fmt.Errorf("resolving cache dir %q: %w", cacheDir, err)
	}
	for name, part := range map[string]string{"repo": repo, "version": version, "path": path} {
		if filepath.IsAbs(filepath.FromSlash(part)) {
			return "", fmt.Errorf("refusing to use cache path: %s %q must not be absolute", name, part)
		}
	}
	joined := filepath.Join(base, filepath.FromSlash(repo), version, filepath.FromSlash(path))
	rel, err := filepath.Rel(base, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to use cache path %q: escapes cache dir %q (repo=%q version=%q path=%q)",
			joined, base, repo, version, path)
	}
	return joined, nil
}

// fetcher retrieves upstream files at a pinned repo+version, caching them on
// disk so a re-run is offline and so one file backing many refs is fetched
// once. Keyed by (repo, version, path) so refs into different upstream
// repositories never collide.
type fetcher struct {
	cacheDir string
	client   *http.Client
	memo     map[string][]byte
}

func (f *fetcher) get(repo, version, path string) ([]byte, error) {
	if f.memo == nil {
		f.memo = map[string][]byte{}
	}
	key := repo + "@" + version + ":" + path
	if b, ok := f.memo[key]; ok {
		return b, nil
	}

	cache, err := cachePathFor(f.cacheDir, repo, version, path)
	if err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(cache); err == nil {
		f.memo[key] = b
		return b, nil
	}

	url := fmt.Sprintf("%s/%s/%s/%s", rawBase, repo, version, path)
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
	f.memo[key] = b
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

// pseudoVersionCommit matches a Go module pseudo-version and captures its
// trailing abbreviated commit hash, e.g. "e63fce3cf15d" out of
// "v0.0.0-20260827164301-e63fce3cf15d" - the form go.mod pins an untagged
// commit of a dependency to. That hash is the git ref raw.githubusercontent.com
// actually understands; the pseudo-version string itself is not a valid ref.
//
// All three documented pseudo-version forms are matched (see
// pkg/validator/runtime/upstream.go's pseudoVersionPattern, which this
// mirrors): the bare "vX.0.0-<ts>-<hash>" form (no earlier version), and the
// two forms with an additional "<pre>." segment between the version core and
// the timestamp ("vX.Y.Z-0.<ts>-<hash>" built on a tagged release, or
// "vX.Y.Z-pre.0.<ts>-<hash>" built on a prerelease) - e.g.
// "v1.1.2-0.20180830191138-d8f796af33cc" (github.com/davecgh/go-spew in this
// repo's own go.mod) is the second form, which the original bare-only regex
// here did not match.
var pseudoVersionCommit = regexp.MustCompile(`^v\d+\.\d+\.\d+-(?:[0-9A-Za-z]+(?:\.[0-9A-Za-z]+)*\.)?\d{14}-([0-9a-f]{12})$`)

// versionFor resolves the upstream ref (tag or commit) to fetch repo's
// source at. If explicitTag is set it wins outright (mirroring --tag's
// existing kubernetes/kubernetes override) - except that an explicit
// override targeting kubernetes/kubernetes specifically must still be a real
// Kubernetes release tag (v1.<minor>.<patch>): that repo's RefKindRewrite
// refs can only ever record a tag shaped like that as ValidatedAt, so
// --update would otherwise write in whatever was passed here and RegisterAll
// would panic the next time it runs (runtime.UpstreamRef.Validate rejects
// it) - a confusing failure far from the flag that caused it. Any other
// repo's ValidatedAt format is looser (commit SHA, pseudo-version, or an
// ordinary semver tag - see runtime.ValidateValidatedAt), and -compute
// legitimately wants to fetch an arbitrary ref (a branch name, "HEAD", ...)
// before a ValidatedAt is ever recorded for a brand-new ref, so this is not
// guarded for non-default repos. Otherwise:
//   - repo == runtime.DefaultRepo uses the existing k8s.io/api-derived
//     staging-module convention (tagFromGoMod).
//   - any other repo is resolved from its own go.mod requirement, exactly
//     the same "bump the dependency, re-verification is forced" rationale
//     tagFromGoMod already applies to kubernetes/kubernetes - generalized
//     here since a citation into a second upstream project (e.g.
//     ovn-kubernetes) is pinned the same way, just via a plain module
//     require line instead of the staging-module version convention.
func versionFor(repo, explicitTag string) (string, error) {
	if explicitTag != "" {
		if repo == runtime.DefaultRepo {
			if err := runtime.ValidateValidatedAt(repo, explicitTag); err != nil {
				return "", fmt.Errorf("-tag %q is not usable as %s's version: %w", explicitTag, repo, err)
			}
		}
		return explicitTag, nil
	}
	if repo == runtime.DefaultRepo {
		return tagFromGoMod()
	}
	return moduleVersionForRepo(repo)
}

// moduleVersionForRepo finds the go.mod requirement whose module path is
// repo (as "github.com/<repo>") or a subdirectory of it - Go modules that
// publish from a subdirectory of their repository (e.g. ovn-kubernetes's
// go-controller) still have their module path prefixed by the repository
// root - and returns the git ref that requirement pins: the tag itself for
// an ordinary semver requirement, or the trailing commit hash for a Go
// pseudo-version (an untagged commit).
func moduleVersionForRepo(repo string) (string, error) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("read go.mod (run from the repository root): %w", err)
	}
	prefix := "github.com/" + repo
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		modPath := fields[0]
		if modPath != prefix && !strings.HasPrefix(modPath, prefix+"/") {
			continue
		}
		v := fields[1]
		if m := pseudoVersionCommit.FindStringSubmatch(v); m != nil {
			return m[1], nil
		}
		return v, nil
	}
	return "", fmt.Errorf("no go.mod requirement found for repo %q (expected module path %q or a subdirectory of it)", repo, prefix)
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

// refsRoot is where the per-package upstream_refs.go tables live, one level
// above the per-upstream-family directories (kubernetes/, and any sibling
// family such as k8scni/ added later) so a new family's tables are picked up
// without editing this constant.
const refsRoot = "pkg/validator/runtime"

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
