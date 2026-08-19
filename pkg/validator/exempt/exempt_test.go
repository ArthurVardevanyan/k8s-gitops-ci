package exempt

import "testing"

func TestExemptable(t *testing.T) {
	if !Exemptable(IDImageChecksum) {
		t.Error("image-checksum should be exemptable")
	}
	if Exemptable(IDClusterIdentity) {
		t.Error("cluster-identity should never be exemptable")
	}
}

func TestExemptable_ImageFQDNNotExemptable(t *testing.T) {
	// image-fqdn is deliberately not exemptable: an unqualified image
	// reference is almost always a mistake, and a genuine structural
	// exception (e.g. an OpenShift ImageStream-triggered bare reference)
	// belongs in a targeted skip in the check itself, not a user-managed
	// exemption.
	if Exemptable(IDImageFQDN) {
		t.Error("image-fqdn should not be exemptable")
	}
}

func TestRegisterExemptable(t *testing.T) {
	RegisterExemptable("custom")
	if !Exemptable("custom") {
		t.Error("custom should be exemptable after registration")
	}
	RegisterExemptable(IDClusterIdentity)
	if Exemptable(IDClusterIdentity) {
		t.Error("register should ignore cluster-identity")
	}
}

func TestKey(t *testing.T) {
	if got := Key(IDImageChecksum); got != AnnotationPrefix+"exempt-image-checksum" {
		t.Errorf("Key = %q", got)
	}
}

func TestAccepts(t *testing.T) {
	ann := map[string]string{Key(IDImageChecksum): "reg/repo:tag@sha256:abc"}
	if !Accepts(ann, IDImageChecksum, "reg/repo:tag@sha256:abc", nil, nil) {
		t.Error("expected match")
	}
	if Accepts(ann, IDImageChecksum, "other", nil, nil) {
		t.Error("expected no match")
	}
}

func TestAccepts_EmptyValueNeverMatches(t *testing.T) {
	// A resource with a non-nil but otherwise-unrelated annotations map,
	// and a finding whose annotationValue() is empty (e.g. both
	// Token/Value unset), must NOT be treated as exempted just because
	// the annotation key is also absent (both sides "" == "").
	ann := map[string]string{"unrelated": "annotation"}
	if Accepts(ann, IDImageChecksum, "", nil, nil) {
		t.Error("empty value must never be granted an exemption, even against a non-nil annotations map")
	}
}

func TestAccepts_EmptyAnnotationsNeverMatches(t *testing.T) {
	if Accepts(map[string]string{}, IDImageChecksum, "something", nil, nil) {
		t.Error("empty (non-nil) annotations map must not grant an exemption")
	}
}

func TestAccepts_LegitimateExemptionStillMatches(t *testing.T) {
	ann := map[string]string{Key(IDImageChecksum): "img@sha256:abc"}
	if !Accepts(ann, IDImageChecksum, "img@sha256:abc", nil, nil) {
		t.Error("expected a legitimate non-empty exact-value match to still be accepted")
	}
}

func TestFileMatches_DoesNotSubstringMatch(t *testing.T) {
	cases := []struct {
		want, file string
		expect     bool
	}{
		{"app", "myapp-config.yaml", false},
		{"app", "app-old/whatever.yaml", false},
		{"app.yaml", "app.yaml", true},
		{"app.yaml", "kubernetes/my-app/base/app.yaml", true},
		{"app.yaml", "kubernetes/my-app/base/notapp.yaml", false},
	}
	for _, c := range cases {
		sel := Selector{Check: IDImageChecksum, File: c.want}
		s := Scalar{File: c.file}
		if got := SelectorMatches(sel, s, IDImageChecksum); got != c.expect {
			t.Errorf("SelectorMatches(File:%q, file:%q) = %v, want %v", c.want, c.file, got, c.expect)
		}
	}
}

func TestSelectorMatches(t *testing.T) {
	sel := Selector{Check: IDImageChecksum, Value: "img@sha256:abc"}
	s := Scalar{Value: "img@sha256:abc", File: "deploy.yaml"}
	if !SelectorMatches(sel, s, IDImageChecksum) {
		t.Error("expected match")
	}
}

func TestEvaluate(t *testing.T) {
	s := Scalar{Value: "img@sha256:abc", File: "deploy.yaml", Path: "/spec/template/spec/containers/0/image"}
	ann := map[string]string{}
	selectors := []Selector{{Check: IDImageChecksum, Value: "img@sha256:abc"}}
	ok, applied := Evaluate(IDImageChecksum, s, ann, selectors)
	if !ok || applied.Value != "img@sha256:abc" {
		t.Errorf("expected exemption: %v %+v", ok, applied)
	}
}

func TestPathMatches(t *testing.T) {
	cases := []struct {
		sel, find string
		want      bool
	}{
		{"/spec/template/spec/containers/0/image", "spec/template/spec/containers/0/image", true},
		{"containers[].image", "spec/template/spec/containers/0/image", true},
		{"containers[].image", "spec/containers/5/image", true},
		{"spec.replicas", "spec/replicas", true},
		{"spec.replicas", "metadata/labels", false},
	}
	for _, c := range cases {
		if got := pathMatches(c.sel, c.find); got != c.want {
			t.Errorf("pathMatches(%q,%q) = %v want %v", c.sel, c.find, got, c.want)
		}
	}
}

func TestPathMatches_ArrayIndexPinning(t *testing.T) {
	// Regression: a selector with a literal array index must pin to
	// exactly that index, not degrade to "any index" the way an empty
	// bracket "[]" wildcard does.
	if !pathMatches("containers[1].image", "spec.containers[1].image") {
		t.Error("expected selector to match its own pinned index")
	}
	if pathMatches("containers[1].image", "spec.containers[0].image") {
		t.Error("expected selector NOT to match a different index")
	}
	if pathMatches("containers[1].image", "spec.containers[2].image") {
		t.Error("expected selector NOT to match a different index")
	}
}

func TestPathMatches_EmptyBracketStillWildcards(t *testing.T) {
	// The explicit "[]" wildcard form must still match any index - only
	// a literal numeric index should pin.
	if !pathMatches("containers[].image", "spec.containers[0].image") {
		t.Error("expected [] to match index 0")
	}
	if !pathMatches("containers[].image", "spec.containers[7].image") {
		t.Error("expected [] to match index 7")
	}
}

func TestNormalizePath_PreservesLiteralIndex(t *testing.T) {
	if got := normalizePath("containers[2].image"); got != "containers/2/image" {
		t.Errorf("normalizePath = %q, want %q", got, "containers/2/image")
	}
	if got := normalizePath("containers[].image"); got != "containers/*/image" {
		t.Errorf("normalizePath = %q, want %q", got, "containers/*/image")
	}
}

func TestEvaluate_TokenPreferredOverValue(t *testing.T) {
	// A check may emit a human-readable Value for display while matching
	// exemptions against a more stable Token (e.g. a foreign-cluster
	// token vs. its raw display string). The annotation must match
	// against Token when set, not Value.
	s := Scalar{Value: "display text (cluster-a)", Token: "cluster-a", File: "x.yaml"}
	ann := map[string]string{Key(IDImageChecksum): "cluster-a"}
	ok, applied := Evaluate(IDImageChecksum, s, ann, nil)
	if !ok {
		t.Fatalf("expected the annotation to match against Token, got ok=%v", ok)
	}
	if applied.Token != "cluster-a" || applied.Value != "display text (cluster-a)" {
		t.Errorf("unexpected applied exemption: %+v", applied)
	}
}

func TestEvaluate_ValueUsedWhenTokenEmpty(t *testing.T) {
	s := Scalar{Value: "img@sha256:abc", File: "x.yaml"}
	ann := map[string]string{Key(IDImageChecksum): "img@sha256:abc"}
	ok, _ := Evaluate(IDImageChecksum, s, ann, nil)
	if !ok {
		t.Error("expected the annotation to match against Value when Token is empty")
	}
}

func TestDirMatches(t *testing.T) {
	cases := []struct {
		dir, file string
		expect    bool
	}{
		{".tekton", ".tekton/pr.yaml", true},
		{".tekton", ".tekton", true},
		{".tekton", "apps/foo/.tekton/pr.yaml", false}, // root-anchored only
		{".tekton", "nottekton/pr.yaml", false},
		{".tekton", ".tektonfoo/pr.yaml", false}, // must not substring-match
		{"", ".tekton/pr.yaml", false},
	}
	for _, c := range cases {
		if got := dirMatches(c.dir, c.file); got != c.expect {
			t.Errorf("dirMatches(%q,%q) = %v want %v", c.dir, c.file, got, c.expect)
		}
	}
}

func TestSelectorMatches_Dir(t *testing.T) {
	sel := Selector{Check: "sync-options", Kind: "PipelineRun", Dir: ".tekton"}
	if !SelectorMatches(sel, Scalar{Kind: "PipelineRun", File: ".tekton/pr.yaml"}, "sync-options") {
		t.Error("expected selector to match a PipelineRun under .tekton/")
	}
	if SelectorMatches(sel, Scalar{Kind: "PipelineRun", File: "apps/foo/.tekton/pr.yaml"}, "sync-options") {
		t.Error("expected selector NOT to match a nested .tekton/ directory")
	}
	if SelectorMatches(sel, Scalar{Kind: "Pipeline", File: ".tekton/pr.yaml"}, "sync-options") {
		t.Error("expected selector NOT to match a different Kind")
	}
}

func TestSelectorMatches_Token(t *testing.T) {
	sel := Selector{Check: IDImageChecksum, Value: "cluster-a"}
	s := Scalar{Value: "display text (cluster-a)", Token: "cluster-a", File: "x.yaml"}
	if !SelectorMatches(sel, s, IDImageChecksum) {
		t.Error("expected selector Value to match against Token, not the display Value")
	}
	// A selector matching the raw display Value (not the Token) must NOT
	// match, since Token takes precedence when set.
	selOnDisplay := Selector{Check: IDImageChecksum, Value: "display text (cluster-a)"}
	if SelectorMatches(selOnDisplay, s, IDImageChecksum) {
		t.Error("expected selector matching the display Value to NOT match once Token takes precedence")
	}
}

func TestAccepts_MatchAlias(t *testing.T) {
	// A repo-level annotation (docker.io/linuxserver/heimdall) must exempt
	// a finding whose exact Value is a tagged reference of that repo
	// (docker.io/linuxserver/heimdall:2.8.2), via MatchAliases, without
	// needing to be updated on every tag bump.
	ann := map[string]string{Key(IDImageChecksum): "docker.io/linuxserver/heimdall"}
	if !Accepts(ann, IDImageChecksum, "docker.io/linuxserver/heimdall:2.8.2", []string{"docker.io/linuxserver/heimdall"}, nil) {
		t.Error("expected the repo-level annotation to match via the alias")
	}
	// The exact full-ref annotation must still work too (back-compat).
	annFull := map[string]string{Key(IDImageChecksum): "docker.io/linuxserver/heimdall:2.8.2"}
	if !Accepts(annFull, IDImageChecksum, "docker.io/linuxserver/heimdall:2.8.2", []string{"docker.io/linuxserver/heimdall"}, nil) {
		t.Error("expected the exact full-ref annotation to still match")
	}
	// An unrelated repo sharing a prefix must not match - this must be an
	// anchored, exact-string alias match, not a substring/prefix check.
	annOther := map[string]string{Key(IDImageChecksum): "docker.io/linuxserver/heimdall-extra"}
	if Accepts(annOther, IDImageChecksum, "docker.io/linuxserver/heimdall:2.8.2", []string{"docker.io/linuxserver/heimdall"}, nil) {
		t.Error("expected an unrelated repo name to NOT match")
	}
	// An empty alias must never grant a match, mirroring the fail-closed
	// behavior for an empty value.
	if Accepts(map[string]string{Key(IDImageChecksum): ""}, IDImageChecksum, "docker.io/linuxserver/heimdall:2.8.2", []string{"docker.io/linuxserver/heimdall"}, nil) {
		t.Error("expected an empty annotation to never match, even with aliases present")
	}
}

func TestSelectorMatches_ValueAlias(t *testing.T) {
	sel := Selector{Check: IDImageChecksum, Value: "docker.io/linuxserver/heimdall"}
	s := Scalar{Value: "docker.io/linuxserver/heimdall:2.8.2", MatchAliases: []string{"docker.io/linuxserver/heimdall"}}
	if !SelectorMatches(sel, s, IDImageChecksum) {
		t.Error("expected a selector Value matching an alias to match")
	}
	other := Scalar{Value: "docker.io/linuxserver/heimdall-extra:1.0", MatchAliases: []string{"docker.io/linuxserver/heimdall-extra"}}
	if SelectorMatches(sel, other, IDImageChecksum) {
		t.Error("expected a selector Value NOT to match an unrelated repo's alias")
	}
}

func TestSelectorMatches_MatchAlias(t *testing.T) {
	sel := Selector{Check: IDImageChecksum, Match: "linuxserver"}
	s := Scalar{Value: "docker.io/linuxserver/heimdall:2.8.2", MatchAliases: []string{"docker.io/linuxserver/heimdall"}}
	if !SelectorMatches(sel, s, IDImageChecksum) {
		t.Error("expected a substring selector to match against the alias too")
	}
}

func TestEvaluate_MatchAlias_HeimdallScenario(t *testing.T) {
	// End-to-end regression for the reported scenario: an annotation
	// naming just the repo (no tag) must exempt image-checksum findings
	// for any tag/digest of that repo.
	s := Scalar{
		Value:        "docker.io/linuxserver/heimdall:2.8.2",
		MatchAliases: []string{"docker.io/linuxserver/heimdall"},
		File:         "statefulset.yaml",
	}
	ann := map[string]string{Key(IDImageChecksum): "docker.io/linuxserver/heimdall"}
	ok, applied := Evaluate(IDImageChecksum, s, ann, nil)
	if !ok {
		t.Fatalf("expected the repo-level annotation to exempt the tagged image finding, got ok=%v", ok)
	}
	if applied.Value != "docker.io/linuxserver/heimdall:2.8.2" {
		t.Errorf("expected Applied.Value to record the full tagged reference, got %q", applied.Value)
	}
}

func TestAccepts_CommaSeparated(t *testing.T) {
	ann := map[string]string{Key(IDImageChecksum): "cuda,nvidia/driver"}
	// Comma-splitting is opt-in via ExemptAnnotationValues being non-empty.
	// Each finding carries only its own image in ExemptAnnotationValues.
	if !Accepts(ann, IDImageChecksum, "cuda", nil, []string{"cuda"}) {
		t.Error("expected first comma-separated entry to match")
	}
	if !Accepts(ann, IDImageChecksum, "nvidia/driver", nil, []string{"nvidia/driver"}) {
		t.Error("expected second comma-separated entry to match")
	}
	// A third image not in the annotation should NOT match.
	if Accepts(ann, IDImageChecksum, "toolkit.image", nil, []string{"toolkit.image"}) {
		t.Error("expected third image to NOT match")
	}
}

func TestAccepts_CommaSeparatedWithWhitespace(t *testing.T) {
	ann := map[string]string{Key(IDImageChecksum): "cuda, nvidia/driver ,toolkit.image"}
	if !Accepts(ann, IDImageChecksum, "cuda", nil, []string{"cuda"}) {
		t.Error("expected entry with leading whitespace trimmed to match")
	}
	if !Accepts(ann, IDImageChecksum, "nvidia/driver", nil, []string{"nvidia/driver"}) {
		t.Error("expected entry with surrounding whitespace trimmed to match")
	}
	if !Accepts(ann, IDImageChecksum, "toolkit.image", nil, []string{"toolkit.image"}) {
		t.Error("expected entry with trailing whitespace trimmed to match")
	}
}

func TestAccepts_CommaSeparated_MatchAlias(t *testing.T) {
	// Comma-separated annotation where one entry matches via alias (repo-level).
	ann := map[string]string{Key(IDImageChecksum): "cuda,docker.io/linuxserver/heimdall"}
	if !Accepts(ann, IDImageChecksum, "cuda", nil, []string{"cuda"}) {
		t.Error("expected first entry to match exact value")
	}
	if !Accepts(ann, IDImageChecksum, "docker.io/linuxserver/heimdall:2.8.2", []string{"docker.io/linuxserver/heimdall"}, []string{"docker.io/linuxserver/heimdall:2.8.2"}) {
		t.Error("expected second entry to match via repo-level alias")
	}
}

func TestAccepts_CommaSeparated_EmptyEntries(t *testing.T) {
	// Trailing commas and multiple commas should produce empty entries that are ignored.
	ann := map[string]string{Key(IDImageChecksum): "cuda,,,"}
	if !Accepts(ann, IDImageChecksum, "cuda", nil, []string{"cuda"}) {
		t.Error("expected non-empty entry to match despite trailing commas")
	}
}

func TestAccepts_CommaSeparated_AllEmptyAfterSplit(t *testing.T) {
	ann := map[string]string{Key(IDImageChecksum): ",,,  ,,"}
	if Accepts(ann, IDImageChecksum, "something", nil, []string{"something"}) {
		t.Error("expected all-empty annotation entries to never match")
	}
}

func TestEvaluate_CommaSeparated(t *testing.T) {
	// End-to-end: comma-separated annotation exempts multiple images.
	// Each Scalar carries only its own image in ExemptAnnotationVals.
	s1 := Scalar{Value: "cuda", File: "cluster-policy.yaml", ExemptAnnotationVals: []string{"cuda"}}
	s2 := Scalar{Value: "nvidia/driver", File: "cluster-policy.yaml", ExemptAnnotationVals: []string{"nvidia/driver"}}
	s3 := Scalar{Value: "toolkit.image", File: "cluster-policy.yaml", ExemptAnnotationVals: []string{"toolkit.image"}}
	ann := map[string]string{Key(IDImageChecksum): "cuda,nvidia/driver"}

	ok1, applied1 := Evaluate(IDImageChecksum, s1, ann, nil)
	if !ok1 || applied1.Value != "cuda" {
		t.Errorf("expected cuda to be exempted: ok=%v, applied=%+v", ok1, applied1)
	}

	ok2, applied2 := Evaluate(IDImageChecksum, s2, ann, nil)
	if !ok2 || applied2.Value != "nvidia/driver" {
		t.Errorf("expected nvidia/driver to be exempted: ok=%v, applied=%+v", ok2, applied2)
	}

	ok3, _ := Evaluate(IDImageChecksum, s3, ann, nil)
	if ok3 {
		t.Error("expected toolkit.image to NOT be exempted")
	}
}
