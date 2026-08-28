package validation

import (
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

	check := containerNameInvalidCheck{}
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
		},
	}

	check := containerPortNameInvalidCheck{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := check.Run([]byte(tt.manifest), "test.yaml")
			if len(findings) != tt.want {
				t.Fatalf("expected %d findings, got %d: %v", tt.want, len(findings), findings)
			}
		})
	}
}
