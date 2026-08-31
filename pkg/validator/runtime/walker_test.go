package runtime

import (
	"strings"
	"testing"
)

// statefulSet builds a StatefulSet whose container mounts every name in
// mounts, declaring volumes and volumeClaimTemplates as given.
func statefulSet(mounts, volumes, claims []string) []byte {
	var b strings.Builder
	b.WriteString("apiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: test\nspec:\n" +
		"  serviceName: test\n  selector:\n    matchLabels:\n      app: test\n" +
		"  template:\n    metadata:\n      labels:\n        app: test\n    spec:\n" +
		"      containers:\n        - name: c\n          image: nginx\n")
	if len(mounts) > 0 {
		b.WriteString("          volumeMounts:\n")
		for _, m := range mounts {
			b.WriteString("            - name: " + m + "\n              mountPath: /" + m + "\n")
		}
	}
	if len(volumes) > 0 {
		b.WriteString("      volumes:\n")
		for _, v := range volumes {
			b.WriteString("        - name: " + v + "\n          emptyDir: {}\n")
		}
	}
	if len(claims) > 0 {
		b.WriteString("  volumeClaimTemplates:\n")
		for _, c := range claims {
			b.WriteString("    - metadata:\n        name: " + c + "\n      spec:\n" +
				"        accessModes: [\"ReadWriteOnce\"]\n        resources:\n          requests:\n            storage: 1Gi\n")
		}
	}
	return []byte(b.String())
}

func volumeNames(t *testing.T, doc []byte) []string {
	t.Helper()
	info, err := ExtractPodSpecInfo(doc, "test.yaml")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if info == nil {
		t.Fatal("no pod spec extracted")
	}
	out := make([]string, 0, len(info.PodSpec.Volumes))
	for _, v := range info.PodSpec.Volumes {
		out = append(out, v.Name)
	}
	return out
}

// TestStatefulSetVolumeClaimTemplatesBecomeVolumes covers the merge the API
// server performs before validating a StatefulSet's pod template.
//
// A container mounting a claim template by name is the ordinary way to write
// a StatefulSet. Reading only spec.template.spec.volumes makes every one of
// them look like it mounts an undefined volume, which is what
// kubernetes/container/volume-mount-name-undefined reported against real manifests
// before this merge existed.
func TestStatefulSetVolumeClaimTemplatesBecomeVolumes(t *testing.T) {
	tests := []struct {
		name    string
		volumes []string
		claims  []string
		want    []string
	}{
		{
			name:   "claim template is visible as a volume",
			claims: []string{"data"},
			want:   []string{"data"},
		},
		{
			name:    "claim templates and declared volumes are both present",
			volumes: []string{"config"},
			claims:  []string{"data"},
			want:    []string{"data", "config"},
		},
		{
			// Upstream keeps the claim template and drops the pod-template
			// volume of the same name. Appending both instead would invent a
			// duplicate that kubernetes/volume/duplicate-volume-names would report.
			name:    "a name in both yields one volume, the claim template",
			volumes: []string{"data", "config"},
			claims:  []string{"data"},
			want:    []string{"data", "config"},
		},
		{
			// Upstream's volumesToAddForTemplates is a map keyed by name, so
			// two templates sharing one contribute a single volume, and
			// validateVolumeClaimTemplates does not reject the duplicate
			// name. Appending both synthesized a collision upstream never
			// sees, which kubernetes/volume/duplicate-volume-names then reported - a
			// finding no manifest change could satisfy and no exemption
			// could suppress.
			name:   "duplicate claim template names collapse to one volume",
			claims: []string{"data", "data"},
			want:   []string{"data"},
		},
		{
			name:    "a duplicate template name still merges with other volumes",
			volumes: []string{"config"},
			claims:  []string{"data", "data"},
			want:    []string{"data", "config"},
		},
		{
			name:    "no claim templates leaves the volumes untouched",
			volumes: []string{"config"},
			want:    []string{"config"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := volumeNames(t, statefulSet(nil, tc.volumes, tc.claims))
			if len(got) != len(tc.want) {
				t.Fatalf("volumes = %v, want %v", got, tc.want)
			}
			seen := map[string]bool{}
			for _, g := range got {
				if seen[g] {
					t.Errorf("volume %q appears twice: %v", g, got)
				}
				seen[g] = true
			}
			for _, w := range tc.want {
				if !seen[w] {
					t.Errorf("volume %q missing from %v", w, got)
				}
			}
		})
	}
}

// TestOnlyStatefulSetsGetClaimTemplateVolumes pins the merge to the one kind
// that has the field, so it cannot start inventing volumes elsewhere.
func TestOnlyStatefulSetsGetClaimTemplateVolumes(t *testing.T) {
	doc := strings.Replace(string(statefulSet(nil, []string{"config"}, []string{"data"})),
		"kind: StatefulSet", "kind: Deployment", 1)
	got := volumeNames(t, []byte(doc))
	for _, g := range got {
		if g == "data" {
			t.Errorf("a Deployment gained a volume from volumeClaimTemplates: %v", got)
		}
	}
}
