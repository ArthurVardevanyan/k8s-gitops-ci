package image

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseImageRef(t *testing.T) {
	cases := []struct {
		raw                               string
		registry, repo, tag, digest, full string
	}{
		{"registry.io/ns/repo:tag@sha256:abc", "registry.io", "ns/repo", "tag", "sha256:abc", ""},
		{"repo:latest", "docker.io", "repo", "latest", "", ""},
	}
	for _, c := range cases {
		ref := ParseImageRef(c.raw)
		if ref == nil {
			t.Fatalf("ParseImageRef(%q) = nil", c.raw)
		}
		if ref.Registry != c.registry || ref.Repo != c.repo || ref.Tag != c.tag || ref.Digest != c.digest {
			t.Errorf("ParseImageRef(%q) = {%q %q %q %q}", c.raw, ref.Registry, ref.Repo, ref.Tag, ref.Digest)
		}
	}
}

func TestIsOCIImageRef(t *testing.T) {
	if isOCIImageRef(ParseImageRef("alpine")) {
		t.Error("docker shortname is not OCI")
	}
	if !isOCIImageRef(ParseImageRef("registry.io/alpine")) {
		t.Error("registry ref is OCI")
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
	data := []byte(`containers:
- image: registry.io/a:b
- image: registry.io/c:d
`)
	imgs := extractImagesFromBytes(data)
	if len(imgs) != 2 {
		t.Errorf("expected 2 images: %v", imgs)
	}
}

func TestVerifyTagDigest_Match(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:abc")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	ref := &Ref{Registry: strings.TrimPrefix(ts.URL, "https://"), Repo: "repo", Tag: "latest"}
	client := ts.Client()
	if err := VerifyTagDigest(ref, client); err != nil {
		t.Errorf("expected match: %v", err)
	}
}
