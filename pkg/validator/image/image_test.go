package image

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

func TestParseImageRef(t *testing.T) {
	cases := []struct {
		raw                               string
		registry, repo, tag, digest, full string
		bare                              bool
	}{
		{"registry.io/ns/repo:tag@sha256:abc", "registry.io", "ns/repo", "tag", "sha256:abc", "", false},
		{"repo:latest", "docker.io", "repo", "latest", "", "", true},
		// port-in-registry case.
		{"myregistry:5000/myimage:v1", "myregistry:5000", "myimage", "v1", "", "", false},
		// digest-only, no tag.
		{"registry.io/repo@sha256:abc", "registry.io", "repo", "", "sha256:abc", "", false},
		// explicit docker.io host is not bare.
		{"docker.io/linuxserver/heimdall:2.8.2", "docker.io", "linuxserver/heimdall", "2.8.2", "", "", false},
		// "localhost" is a valid explicit registry host per OCI/Docker
		// reference rules even though it contains neither "." nor ":" -
		// must not be misclassified as a bare shortname.
		{"localhost/myapp:tag", "localhost", "myapp", "tag", "", "", false},
		// localhost with an explicit port already worked via the ":" rule;
		// kept here as a regression guard alongside the bare-localhost fix.
		{"localhost:5000/myapp:v1", "localhost:5000", "myapp", "v1", "", "", false},
		// Regression for change #3: a bare image with neither tag nor
		// digest must leave Tag == "" (no more implicit "latest" default -
		// the SHA-digest pinning enforcement never consulted Tag anyway).
		{"nginx", "docker.io", "nginx", "", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			ref := ParseImageRef(c.raw)
			if ref == nil {
				t.Fatalf("ParseImageRef(%q) = nil", c.raw)
			}
			if ref.Registry != c.registry || ref.Repo != c.repo || ref.Tag != c.tag || ref.Digest != c.digest {
				t.Errorf("ParseImageRef(%q) = {%q %q %q %q}", c.raw, ref.Registry, ref.Repo, ref.Tag, ref.Digest)
			}
			if ref.Bare != c.bare {
				t.Errorf("ParseImageRef(%q).Bare = %v, want %v", c.raw, ref.Bare, c.bare)
			}
		})
	}
}

func TestRepoKey(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"docker.io/linuxserver/heimdall:2.8.2", "docker.io/linuxserver/heimdall"},
		{"docker.io/linuxserver/heimdall:2.8.2@sha256:abc", "docker.io/linuxserver/heimdall"},
		// bare shortname defaults to docker.io, so its key still carries
		// that default registry.
		{"nginx:latest", "docker.io/nginx"},
		{"myregistry:5000/myimage:v1", "myregistry:5000/myimage"},
		{"registry.io/repo@sha256:abc", "registry.io/repo"},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			if got := ParseImageRef(c.raw).RepoKey(); got != c.want {
				t.Errorf("RepoKey(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
	if got := (*Ref)(nil).RepoKey(); got != "" {
		t.Errorf("RepoKey() on nil Ref = %q, want empty", got)
	}
}

func TestIsImageRef(t *testing.T) {
	// Explicit-registry refs (including docker.io) are always enforced.
	for _, raw := range []string{
		"registry.io/alpine",
		"docker.io/linuxserver/heimdall:2.8.2",
		"localhost/myapp:tag",
	} {
		if !isImageRef(ParseImageRef(raw)) {
			t.Errorf("isImageRef(%q) = false, want true", raw)
		}
	}
	// Bare shortname refs are enforced (FQDN/digest checks handle them).
	for _, raw := range []string{"alpine", "nginx:latest"} {
		if !isImageRef(ParseImageRef(raw)) {
			t.Errorf("isImageRef(%q) = false, want true", raw)
		}
	}
	// Templated/placeholder values are never enforced.
	if isImageRef(ParseImageRef("{{ .Image }}")) {
		t.Error("isImageRef of a space-containing placeholder = true, want false")
	}
	// Tekton parameter/context references are never enforced.
	for _, raw := range []string{
		"$(params.image)",
		"$(params.registry)/myimage:$(params.tag)",
		"$(context.pipelineRun.name)",
		"$(tasks.clone-results.steps.clones.status)",
		"$(steps.build.outputs.commit.image)",
		"$(resources.git.source.url)",
	} {
		if isImageRef(ParseImageRef(raw)) {
			t.Errorf("isImageRef(%q) = true, want false (Tekton ref)", raw)
		}
	}
}

func TestIsImageRef_GCPDiskImageFalsePositives(t *testing.T) {
	// Cloud-resource path strings (e.g. GCP disk image self-links) must
	// never be mistaken for an enforceable OCI image reference just because
	// they are bare slash-y strings with no tag or digest.
	cases := []string{
		"projects/rhcos-cloud/global/images/rhcos-416-94-202406251923-0-gcp-x86-64",
		"projects/my-project/zones/us-central1-a/disks/my-disk",
		"projects/cos-cloud/global/images/family/cos-stable",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if isImageRef(ParseImageRef(raw)) {
				t.Errorf("isImageRef(%q) = true, want false (registry=%q, repo=%q)", raw, ParseImageRef(raw).Registry, ParseImageRef(raw).Repo)
			}
		})
	}
}

func TestValidateBytes_NoPinning(t *testing.T) {
	data := []byte(`kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - image: registry.io/repo:tag
`)
	errs := ValidateBytes(data, "x.yaml")
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "SHA digest") {
		t.Errorf("expected unpinned error: %v", errs)
	}
}

func TestValidateBytes_Pinned(t *testing.T) {
	data := []byte(`kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - image: registry.io/repo@sha256:abc
`)
	errs := ValidateBytes(data, "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestValidateBytes_SkipsNonOCI(t *testing.T) {
	data := []byte(`kind: ConfigMap
metadata:
  name: test
data:
  image: projects/rhcos-cloud/global/images/rhcos-416-94-202406251923-0-gcp-x86-64
`)
	errs := ValidateBytes(data, "test.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors for a non-OCI-looking image ref, got: %v", errs)
	}
}

func TestValidateBytesRaw_IgnoresAnnotationExemption(t *testing.T) {
	// Regression: ValidateBytesRaw must return every unpinned image
	// regardless of any annotation exemption - exemption evaluation is
	// the shared engine's job now, not this package's.
	data := []byte(`kind: Deployment
metadata:
  name: d
  annotations:
    gitops-ci.k8s.io/exempt-image-checksum: registry.io/repo:tag
spec:
  template:
    spec:
      containers:
      - image: registry.io/repo:tag
`)
	errs := ValidateBytesRaw(data, "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 unfiltered finding even with an exempting annotation present: %v", errs)
	}
	if errs[0].Annotations["gitops-ci.k8s.io/exempt-image-checksum"] != "registry.io/repo:tag" {
		t.Errorf("expected the resource's annotations to be attached to the finding: %v", errs[0].Annotations)
	}
}

func TestValidateBytesRaw_Pinned_NoFindings(t *testing.T) {
	data := []byte(`kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - image: registry.io/repo@sha256:abc
`)
	if errs := ValidateBytesRaw(data, "x.yaml"); len(errs) != 0 {
		t.Errorf("expected no findings for a pinned image: %v", errs)
	}
}

func TestValidateBytesRaw_DockerIORegistered_RequiresDigest(t *testing.T) {
	// Regression: an unpinned image with an explicit docker.io host must
	// no longer be silently skipped. Reported originally against
	// docker.io/linuxserver/heimdall:2.8.2.
	pinned := []byte(`kind: StatefulSet
metadata:
  name: heimdall
spec:
  template:
    spec:
      containers:
      - name: heimdall
        image: docker.io/linuxserver/heimdall:2.8.2@sha256:abc
`)
	if errs := ValidateBytesRaw(pinned, "heimdall.yaml"); len(errs) != 0 {
		t.Errorf("expected no findings for a pinned docker.io image: %v", errs)
	}

	unpinned := []byte(`kind: StatefulSet
metadata:
  name: heimdall
spec:
  template:
    spec:
      containers:
      - name: heimdall
        image: docker.io/linuxserver/heimdall:2.8.2
`)
	errs := ValidateBytesRaw(unpinned, "heimdall.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding for an unpinned docker.io image, got: %v", errs)
	}
	if errs[0].Image != "docker.io/linuxserver/heimdall:2.8.2" {
		t.Errorf("unexpected image in finding: %q", errs[0].Image)
	}
	if errs[0].Repo != "docker.io/linuxserver/heimdall" {
		t.Errorf("expected the finding's Repo to be the tag-independent repo key, got %q", errs[0].Repo)
	}
}

func TestValidateFQDNBytesRaw(t *testing.T) {
	bare := []byte(`kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - image: nginx:latest
`)
	errs := ValidateFQDNBytesRaw(bare, "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 FQDN finding for a bare shortname, got: %v", errs)
	}
	if errs[0].Image != "nginx:latest" || !strings.Contains(errs[0].Message, "explicit registry") {
		t.Errorf("unexpected FQDN finding: %+v", errs[0])
	}

	qualified := []byte(`kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - image: docker.io/nginx:latest
`)
	if errs := ValidateFQDNBytesRaw(qualified, "x.yaml"); len(errs) != 0 {
		t.Errorf("expected no FQDN finding for an explicitly-registered image: %v", errs)
	}

	gcpLink := []byte(`kind: ConfigMap
metadata:
  name: test
data:
  image: projects/rhcos-cloud/global/images/rhcos-416-94-202406251923-0-gcp-x86-64
`)
	if errs := ValidateFQDNBytesRaw(gcpLink, "x.yaml"); len(errs) != 0 {
		t.Errorf("expected no FQDN finding for a GCP self-link: %v", errs)
	}
}

func TestValidateFQDNBytesRaw_SkipsTektonParamRefs(t *testing.T) {
	data := []byte(`apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: build-task
spec:
  params:
    - name: image
      type: string
  steps:
    - image: $(params.image)
      name: build
`)
	errs := ValidateFQDNBytesRaw(data, "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no FQDN findings for Tekton param refs, got: %v", errs)
	}
}

func TestDeduplicate(t *testing.T) {
	errs := []ValidationError{
		{Kind: "Deployment", Name: "d", Image: "registry.io/repo:tag"},
		{Kind: "Deployment", Name: "d", Image: "registry.io/repo:tag"},
		{Kind: "Deployment", Name: "other", Image: "registry.io/repo2:tag"},
	}
	got := Deduplicate(errs)
	if len(got) != 2 {
		t.Fatalf("expected 2 deduplicated groups, got %d: %v", len(got), got)
	}
	byImage := map[string]DeduplicatedError{}
	for _, d := range got {
		byImage[d.Image] = d
	}
	if byImage["registry.io/repo:tag"].Count != 2 {
		t.Errorf("expected count 2 for the duplicated image, got %v", byImage["registry.io/repo:tag"])
	}
	if byImage["registry.io/repo2:tag"].Count != 1 {
		t.Errorf("expected count 1 for the unique image, got %v", byImage["registry.io/repo2:tag"])
	}
}

func TestDeduplicate_Empty(t *testing.T) {
	if got := Deduplicate(nil); len(got) != 0 {
		t.Errorf("expected empty result for no findings, got %v", got)
	}
}

func TestExtractImagesFromFile(t *testing.T) {
	images := ExtractImagesFromFile("testdata/good-pinned.yaml")
	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d: %v", len(images), images)
	}
}

func TestVerifyTagDigest_Match(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:abc")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	ref := &Ref{Registry: strings.TrimPrefix(ts.URL, "https://"), Repo: "repo", Tag: "latest", Digest: "sha256:abc"}
	client := ts.Client()
	if err := VerifyTagDigest(ref, client); err != nil {
		t.Errorf("expected match: %v", err)
	}
}

func TestVerifyTagDigest_Mismatch(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:actual")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	ref := &Ref{
		Registry: strings.TrimPrefix(ts.URL, "https://"), Repo: "repo", Tag: "latest",
		Digest: "sha256:expected", Raw: "repo:latest@sha256:expected",
	}
	err := VerifyTagDigest(ref, ts.Client())
	if err == nil {
		t.Fatal("expected an error for mismatched digest")
	}
	if !strings.Contains(err.Message, "resolves to") {
		t.Errorf("expected 'resolves to' message, got: %s", err.Message)
	}
}

func TestVerifyTagDigest_NoTag(t *testing.T) {
	// This package's current contract: VerifyTagDigest requires a tag to
	// resolve against the registry - an empty Tag is reported as an error
	// (distinct from the caller-side skip in VerifyFileTagDigests, which
	// never calls VerifyTagDigest at all when ref.Digest == "").
	ref := &Ref{Registry: "example.com", Repo: "repo", Tag: "", Digest: "sha256:abc"}
	err := VerifyTagDigest(ref, http.DefaultClient)
	if err == nil {
		t.Fatal("expected an error when Tag is empty")
	}
	if !strings.Contains(err.Message, "missing tag") {
		t.Errorf("expected a missing-tag message, got: %s", err.Message)
	}
}

func TestVerifyFileTagDigests_SkipsUnpinned(t *testing.T) {
	// Regression for change #3: removing the Tag="latest" default must not
	// affect this skip - it keys off Digest, not Tag.
	errs := VerifyFileTagDigests("testdata/bad-no-digest.yaml", http.DefaultClient)
	if len(errs) != 0 {
		t.Errorf("expected no digest-verification errors for images with no digest at all, got: %v", errs)
	}
}

// --- testdata-fixture-driven tests -------------------------------------

func TestValidateFile_AllPinned(t *testing.T) {
	errs := ValidateFile("testdata/good-pinned.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors for fully pinned images, got: %v", errs)
	}
}

func TestValidateFile_NoPinning(t *testing.T) {
	errs := ValidateFile("testdata/bad-no-digest.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors for unpinned images, got %d: %v", len(errs), errs)
	}
	for _, e := range errs {
		if !strings.Contains(e.Message, "not pinned") {
			t.Errorf("expected 'not pinned' message, got: %s", e.Message)
		}
	}
}

func TestValidateFile_Mixed(t *testing.T) {
	errs := ValidateFile("testdata/mixed-tekton.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (unpinned step), got %d: %v", len(errs), errs)
	}
	if errs[0].Image != "registry.io/tools:latest" {
		t.Errorf("expected error for the unpinned tools image, got: %s", errs[0].Image)
	}
}

func TestValidateFile_OCIVolume(t *testing.T) {
	errs := ValidateFile("testdata/oci-volume.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (unpinned OCI volume image), got %d: %v", len(errs), errs)
	}
	if errs[0].Image != "registry.io/homelab/clair-action-db:latest" {
		t.Errorf("expected the unpinned volume image to be flagged, got: %q", errs[0].Image)
	}
	if errs[0].Kind != "Task" || errs[0].Name != "clair-action" {
		t.Errorf("expected the finding to identify the owning resource, got: %+v", errs[0])
	}
}

func TestValidateFile_NoImages(t *testing.T) {
	errs := ValidateFile("testdata/no-images.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors for a file with no images, got: %v", errs)
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	errs := ValidateFile("testdata/does-not-exist.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors for a missing file, got: %v", errs)
	}
}

func TestValidateFile_ArgoCDSplitVersion_SiblingScoping(t *testing.T) {
	// Regression for change #1: an unrelated nested image (the sidecar)
	// appears in document order between the "image" and "version" keys of
	// the top-level spec mapping, but is NOT itself in that mapping. The
	// fix must key the digest off the same-mapping "image" sibling
	// (spec.image), not "whichever image string was most recently seen
	// during the recursive walk" (which would misattach the digest to the
	// sidecar instead and leave spec.image looking unpinned).
	errs := ValidateFile("testdata/argocd-split-version.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding (the genuinely-unpinned sidecar), got %d: %v", len(errs), errs)
	}
	if errs[0].Image != "registry.io/sidecar:latest" {
		t.Errorf("expected the finding to be for the sidecar image (spec.image must have combined with the digest instead), got: %q", errs[0].Image)
	}
}

// TestValidateBytes_ImageVersionPairing covers the operator split-pinning
// convention where a container image is pinned across an "image" field and
// a sibling "version" field holding a sha256 digest (e.g. ArgoCD).
func TestValidateBytes_ImageVersionPairing(t *testing.T) {
	const digest = "sha256:e6a5a60222c5a594bcbc197923d15d6cf1debbe39703689c1d7e3535faff5254"

	widget := func(version string) string {
		return `apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
spec:
  image: registry.io/example/widget
  version: ` + version + "\n"
	}

	tests := []struct {
		name     string
		yaml     string
		wantErrs int
	}{
		{
			name:     "version is a sha256 digest - pinned",
			yaml:     widget(digest),
			wantErrs: 0,
		},
		{
			name:     "version is a plain tag - not pinned",
			yaml:     widget("v2.13.0"),
			wantErrs: 1,
		},
		{
			name: "image already carries digest, version present - pinned",
			yaml: `apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
spec:
  image: registry.io/example/widget@` + digest + `
  version: ` + digest + "\n",
			wantErrs: 0,
		},
		{
			name: "image with no sibling version - not pinned",
			yaml: `apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
spec:
  image: registry.io/example/widget
`,
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateBytes([]byte(tt.yaml), "test.yaml")
			if len(errs) != tt.wantErrs {
				t.Fatalf("expected %d error(s), got %d: %v", tt.wantErrs, len(errs), errs)
			}
		})
	}
}

func TestValidateBytes_SplitRepository_Version(t *testing.T) {
	// NVIDIA-style: image + repository + version (non-digest tag).
	// Should combine to FQDN image with tag; pinning check flags it.
	const y = `apiVersion: nvidia.com/v1
kind: ClusterPolicy
metadata:
  name: gpu
spec:
  operator:
    initContainer:
      image: cuda
      repository: nvcr.io/nvidia
      version: 13.0.0-base-ubi9
`
	errs := ValidateBytes([]byte(y), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 pinning error, got %d: %v", len(errs), errs)
	}
	if errs[0].Image != "nvcr.io/nvidia/cuda:13.0.0-base-ubi9" {
		t.Errorf("unexpected combined image: %q", errs[0].Image)
	}
}

func TestValidateBytes_SplitRepository_Tag(t *testing.T) {
	// NVIDIA-style with explicit "tag" field instead of "version".
	const y = `apiVersion: nvidia.com/v1
kind: ClusterPolicy
metadata:
  name: gpu
spec:
  toolkit:
    image: toolkit
    repository: nvcr.io/nvidia
    tag: latest
`
	errs := ValidateBytes([]byte(y), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 pinning error, got %d: %v", len(errs), errs)
	}
	if errs[0].Image != "nvcr.io/nvidia/toolkit:latest" {
		t.Errorf("unexpected combined image: %q", errs[0].Image)
	}
}

func TestValidateBytes_SplitRepository_Digest(t *testing.T) {
	// NVIDIA-style with version containing sha256 digest — should be pinned.
	const digest = "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	const y = `apiVersion: nvidia.com/v1
kind: ClusterPolicy
metadata:
  name: gpu
spec:
  driver:
    repository: nvcr.io/nvidia
    image: driver
    version: ` + digest + "\n"
	errs := ValidateBytes([]byte(y), "test.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors for pinned image, got %d: %v", len(errs), errs)
	}
}

func TestValidateBytes_SplitRepository_RepoImageBothTagged(t *testing.T) {
	// Both repository and image carry tags — version digest should win.
	const digest = "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	const y = `apiVersion: example.com/v1
kind: App
metadata:
  name: app
spec:
  image: myapp:v1
  repository: registry.io/repo:v2
  version: ` + digest + "\n"
	errs := ValidateBytes([]byte(y), "test.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateFQDNBytesRaw_SplitRepository(t *testing.T) {
	// FQDN check should pass when repository provides the registry.
	const y = `apiVersion: nvidia.com/v1
kind: ClusterPolicy
metadata:
  name: gpu
spec:
  operator:
    initContainer:
      image: cuda
      repository: nvcr.io/nvidia
      version: 13.0.0-base-ubi9
  driver:
    repository: my-registry.example.com/homelab
    image: nvidia/driver
    version: 580.173.02
`
	errs := ValidateFQDNBytesRaw([]byte(y), "test.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no FQDN findings, got %d: %v", len(errs), errs)
	}
}

func TestValidateFQDNBytesRaw_SplitRepository_BareRepo(t *testing.T) {
	// repository without a registry host → combined image still bare.
	const y = `apiVersion: example.com/v1
kind: App
metadata:
  name: app
spec:
  repository: homelab
  image: myapp
`
	errs := ValidateFQDNBytesRaw([]byte(y), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 FQDN finding, got %d: %v", len(errs), errs)
	}
	if errs[0].Image != "homelab/myapp" {
		t.Errorf("unexpected combined image: %q", errs[0].Image)
	}
}

func TestValidateBytes_SplitRepository_OnlyRepository_NoImage(t *testing.T) {
	// repository without image — no image extracted.
	const y = `apiVersion: example.com/v1
kind: App
metadata:
  name: app
spec:
  repository: nvcr.io/nvidia
`
	errs := ValidateBytes([]byte(y), "test.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateBytes_SplitRepository_Precedence_DigestOverVersionOverTag(t *testing.T) {
	// When version (non-digest) and tag both exist, tag takes precedence.
	// When version is a digest, digest takes precedence over tag.
	const digest = "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	t.Run("digest wins over tag", func(t *testing.T) {
		y := `apiVersion: example.com/v1
kind: App
metadata:
  name: app
spec:
  image: myapp
  repository: registry.io/repo
  version: ` + digest + `
  tag: latest
`
		errs := ValidateBytes([]byte(y), "test.yaml")
		if len(errs) != 0 {
			t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
		}
	})

	t.Run("tag used when version is non-digest", func(t *testing.T) {
		y := `apiVersion: example.com/v1
kind: App
metadata:
  name: app
spec:
  image: myapp
  repository: registry.io/repo
  version: v1.2.3
  tag: latest
`
		errs := ValidateBytes([]byte(y), "test.yaml")
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
		if errs[0].Image != "registry.io/repo/myapp:latest" {
			t.Errorf("unexpected image: %q", errs[0].Image)
		}
	})
}

func TestValidateBytesWithExemptions_SplitRepository(t *testing.T) {
	// Annotation exemption should work with combined image.
	key := exempt.Key(exempt.IDImageChecksum)
	y := `apiVersion: nvidia.com/v1
kind: ClusterPolicy
metadata:
  name: gpu
  annotations:
    ` + key + `: "nvcr.io/nvidia/cuda:13.0.0-base-ubi9"
spec:
  operator:
    initContainer:
      image: cuda
      repository: nvcr.io/nvidia
      version: 13.0.0-base-ubi9
`
	errs, exempted := ValidateBytesWithExemptions([]byte(y), "test.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	if len(exempted) != 1 || exempted[0].Image != "nvcr.io/nvidia/cuda:13.0.0-base-ubi9" {
		t.Errorf("unexpected exempted: %v", exempted)
	}
}

func TestValidateBytesRaw_IgnoresAnnotationExemption_SplitRepository(t *testing.T) {
	// ValidateBytesRaw returns unfiltered findings even with annotation exemption.
	const y = `apiVersion: nvidia.com/v1
kind: ClusterPolicy
metadata:
  name: gpu
  annotations:
    gitops-ci.k8s.io/exempt-image-checksum: "nvcr.io/nvidia/cuda:13.0.0-base-ubi9"
spec:
  operator:
    initContainer:
      image: cuda
      repository: nvcr.io/nvidia
      version: 13.0.0-base-ubi9
`
	errs := ValidateBytesRaw([]byte(y), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 unfiltered finding, got %d: %v", len(errs), errs)
	}
	if errs[0].Image != "nvcr.io/nvidia/cuda:13.0.0-base-ubi9" {
		t.Errorf("unexpected image: %q", errs[0].Image)
	}
}

func TestValidateBytesWithExemptions(t *testing.T) {
	const unpinned = "registry.io/caas/test:latest"
	key := exempt.Key(exempt.IDImageChecksum)

	base := func(annotation string) string {
		y := `apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: kyverno-require-image-checksum
`
		if annotation != "" {
			y += "  annotations:\n    " + annotation + "\n"
		}
		y += `spec:
  containers:
    - image: ` + unpinned + "\n"
		return y
	}

	tests := []struct {
		name         string
		yaml         string
		wantErrs     int
		wantExempted int
	}{
		{
			name:         "exact image value accepts",
			yaml:         base(key + `: "` + unpinned + `"`),
			wantErrs:     0,
			wantExempted: 1,
		},
		{
			name:         "different image value still enforces",
			yaml:         base(key + `: "registry.io/caas/other:latest"`),
			wantErrs:     1,
			wantExempted: 0,
		},
		{
			name:         "empty value still enforces",
			yaml:         base(key + `: ""`),
			wantErrs:     1,
			wantExempted: 0,
		},
		{
			name:         "boolean-style value no longer accepts",
			yaml:         base(key + `: "true"`),
			wantErrs:     1,
			wantExempted: 0,
		},
		{
			name:         "no annotation still enforces",
			yaml:         base(""),
			wantErrs:     1,
			wantExempted: 0,
		},
		{
			name:         "unrelated annotation still enforces",
			yaml:         base(`example.com/other: "` + unpinned + `"`),
			wantErrs:     1,
			wantExempted: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs, exempted := ValidateBytesWithExemptions([]byte(tt.yaml), "test.yaml")
			if len(errs) != tt.wantErrs {
				t.Errorf("errors: got %d, want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if len(exempted) != tt.wantExempted {
				t.Errorf("exempted: got %d, want %d: %v", len(exempted), tt.wantExempted, exempted)
			}
			if tt.wantExempted > 0 {
				// Regression for change #2: ExemptedImage now carries
				// Kind/Name, not just File/Image.
				if exempted[0].Kind != "Canary" || exempted[0].Name != "kyverno-require-image-checksum" {
					t.Errorf("unexpected exempted identity: %+v", exempted[0])
				}
				if exempted[0].Image != unpinned {
					t.Errorf("unexpected exempted image: %q", exempted[0].Image)
				}
			}
		})
	}
}

func TestValidateBytesWithExemptions_PerImageGranularity(t *testing.T) {
	// An exception naming one image must not exempt a second unpinned
	// image on the same resource.
	key := exempt.Key(exempt.IDImageChecksum)
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: two-images
  annotations:
    ` + key + `: "registry.io/caas/one:latest"
spec:
  template:
    spec:
      containers:
        - image: registry.io/caas/one:latest
        - image: registry.io/caas/two:latest
`
	errs, exempted := ValidateBytesWithExemptions([]byte(yaml), "test.yaml")
	if len(exempted) != 1 || exempted[0].Image != "registry.io/caas/one:latest" {
		t.Fatalf("expected only image one exempted, got %v", exempted)
	}
	if len(errs) != 1 || errs[0].Image != "registry.io/caas/two:latest" {
		t.Fatalf("expected image two to still error, got %v", errs)
	}
}

func TestValidateBytesWithExemptions_MultiDoc(t *testing.T) {
	key := exempt.Key(exempt.IDImageChecksum)
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: exempt-me
  annotations:
    ` + key + `: "registry.io/caas/exempt:latest"
spec:
  template:
    spec:
      containers:
        - image: registry.io/caas/exempt:latest
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: enforce-me
spec:
  template:
    spec:
      containers:
        - image: registry.io/caas/enforce:latest
`
	errs, exempted := ValidateBytesWithExemptions([]byte(yaml), "test.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for non-exempt doc, got %d: %v", len(errs), errs)
	}
	if errs[0].Name != "enforce-me" {
		t.Errorf("expected error on enforce-me, got %q", errs[0].Name)
	}
	if len(exempted) != 1 {
		t.Fatalf("expected 1 exemption, got %d: %v", len(exempted), exempted)
	}
	if exempted[0].Name != "exempt-me" {
		t.Errorf("expected exemption on exempt-me, got %q", exempted[0].Name)
	}
}

func TestValidateBytes_BackCompatWithAnnotation(t *testing.T) {
	// ValidateBytes must still drop accepted images (returning only errors).
	key := exempt.Key(exempt.IDImageChecksum)
	yaml := `apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: c
  annotations:
    ` + key + `: "registry.io/caas/test:latest"
spec:
  containers:
    - image: registry.io/caas/test:latest
`
	if errs := ValidateBytes([]byte(yaml), "test.yaml"); len(errs) != 0 {
		t.Fatalf("expected 0 errors for annotated resource, got %d: %v", len(errs), errs)
	}
}
