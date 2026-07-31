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
	if !Accepts(ann, IDImageChecksum, "reg/repo:tag@sha256:abc") {
		t.Error("expected match")
	}
	if Accepts(ann, IDImageChecksum, "other") {
		t.Error("expected no match")
	}
}

func TestAccepts_EmptyValueNeverMatches(t *testing.T) {
	// A resource with a non-nil but otherwise-unrelated annotations map,
	// and a finding whose annotationValue() is empty (e.g. both
	// Token/Value unset), must NOT be treated as exempted just because
	// the annotation key is also absent (both sides "" == "").
	ann := map[string]string{"unrelated": "annotation"}
	if Accepts(ann, IDImageChecksum, "") {
		t.Error("empty value must never be granted an exemption, even against a non-nil annotations map")
	}
}

func TestAccepts_EmptyAnnotationsNeverMatches(t *testing.T) {
	if Accepts(map[string]string{}, IDImageChecksum, "something") {
		t.Error("empty (non-nil) annotations map must not grant an exemption")
	}
}

func TestAccepts_LegitimateExemptionStillMatches(t *testing.T) {
	ann := map[string]string{Key(IDImageChecksum): "img@sha256:abc"}
	if !Accepts(ann, IDImageChecksum, "img@sha256:abc") {
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
