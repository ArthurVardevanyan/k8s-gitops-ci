package core

import (
	"slices"
	"strings"
	"testing"
)

func TestContainerNameInvalidCheck(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     int
		contains string
	}{
		{
			name: "valid container name",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: my-app
        image: nginx
`,
			want: 0,
		},
		{
			name: "uppercase container name is invalid",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: MyApp
        image: nginx
`,
			want:     1,
			contains: "invalid container name",
		},
		{
			name: "underscore container name is invalid",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: my_app
        image: nginx
`,
			want:     1,
			contains: "invalid container name",
		},
		{
			// Upstream reports an absent name as Required rather than as
			// a format error; the branches are mutually exclusive.
			name: "missing container name is required",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - image: nginx
`,
			want:     1,
			contains: "container name is required",
		},
		{
			// Init containers go through the same validateContainerCommon
			// path upstream, so they must be checked too.
			name: "invalid init container name",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      initContainers:
      - name: Bad_Init
        image: busybox
      containers:
      - name: ok
        image: nginx
`,
			want:     1,
			contains: "invalid container name",
		},
	}

	check := newContainerNameInvalidCheck()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := check.Run([]byte(tt.manifest), "test.yaml")
			if len(findings) != tt.want {
				t.Fatalf("expected %d findings, got %d: %v", tt.want, len(findings), findings)
			}
			if tt.contains != "" && !strings.Contains(findings[0].Message, tt.contains) {
				t.Errorf("expected message containing %q, got %q", tt.contains, findings[0].Message)
			}
		})
	}
}

func TestContainerPortNameInvalidCheck(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     int
		// wantPaths, where set, asserts the exact field path of every
		// finding in order. A path is the only part of a finding that tells
		// a reader which port to edit, and a count-only assertion cannot
		// tell a correct path from one that names no element of the
		// manifest at all.
		wantPaths []string
	}{
		{
			name: "valid port name",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
        ports:
        - name: http
          containerPort: 8080
`,
			want: 0,
		},
		{
			// An unnamed port is legal, so nothing is reported.
			name: "unnamed port is allowed",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
        ports:
        - containerPort: 8080
`,
			want: 0,
		},
		{
			// IANA service names are capped at 15 characters.
			name: "port name too long",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
        ports:
        - name: a-very-long-port-name
          containerPort: 8080
`,
			want: 1,
		},
		{
			// IsValidPortName returns one message per broken rule and
			// upstream emits a separate error for each, so an uppercase
			// name yields two findings: the character-set rule and the
			// "must contain at least one letter" rule.
			name: "uppercase port name is invalid",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
        ports:
        - name: HTTP
          containerPort: 8080
`,
			want: 2,
			wantPaths: []string{
				"spec.template.spec.containers[app].ports[0].name",
				"spec.template.spec.containers[app].ports[0].name",
			},
		},
		{
			// The second port is the invalid one. Keying the path by name
			// produced ports[HTTP].name, which does not exist in the
			// manifest; the index locates it.
			name: "an invalid port is indexed at its own position",
			manifest: `kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
        ports:
        - name: http
          containerPort: 8080
        - name: a-very-long-port-name
          containerPort: 8081
`,
			want:      1,
			wantPaths: []string{"spec.template.spec.containers[app].ports[1].name"},
		},
	}

	check := newContainerPortNameInvalidCheck()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := check.Run([]byte(tt.manifest), "test.yaml")
			if len(findings) != tt.want {
				t.Fatalf("expected %d findings, got %d: %v", tt.want, len(findings), findings)
			}
			if tt.wantPaths != nil {
				got := make([]string, len(findings))
				for i, f := range findings {
					got[i] = f.Path
				}
				if !slices.Equal(got, tt.wantPaths) {
					t.Errorf("paths = %v, want %v", got, tt.wantPaths)
				}
			}
		})
	}
}
