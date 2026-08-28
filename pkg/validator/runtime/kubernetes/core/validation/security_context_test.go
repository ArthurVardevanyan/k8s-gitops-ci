package validation

import (
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// --- privileged-allow-priv-esc tests ---

func TestPrivilegedAllowPrivEsc_Check_PrivilegedWithFalseAllowPrivEsc(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      privileged: true
      allowPrivilegeEscalation: false
`)
	check := privilegedAllowPrivEscCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "security-context/privileged-allow-priv-esc" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Container != "c" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
	if findings[0].Kind != "Pod" || findings[0].Name != "test" {
		t.Errorf("unexpected kind/name: %s/%s", findings[0].Kind, findings[0].Name)
	}
}

func TestPrivilegedAllowPrivEsc_Check_PrivilegedWithNilAllowPrivEsc(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      privileged: true
`)
	check := privilegedAllowPrivEscCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (privileged + nil allowPrivilegeEscalation), got %d", len(findings))
	}
}

func TestPrivilegedAllowPrivEsc_Check_PrivilegedWithTrueAllowPrivEsc(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      privileged: true
      allowPrivilegeEscalation: true
`)
	check := privilegedAllowPrivEscCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when privileged=true and allowPrivilegeEscalation=true, got %d", len(findings))
	}
}

func TestPrivilegedAllowPrivEsc_Check_NotPrivileged(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      privileged: false
      allowPrivilegeEscalation: false
`)
	check := privilegedAllowPrivEscCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when not privileged, got %d", len(findings))
	}
}

func TestPrivilegedAllowPrivEsc_Check_NilSecurityContext(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := privilegedAllowPrivEscCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no securityContext, got %d", len(findings))
	}
}

func TestPrivilegedAllowPrivEsc_Check_Deployment(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      containers:
      - name: c
        image: nginx
        securityContext:
          privileged: true
          allowPrivilegeEscalation: false
`)
	check := privilegedAllowPrivEscCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for Deployment, got %d", len(findings))
	}
	if findings[0].Kind != "Deployment" || findings[0].Name != "test" {
		t.Errorf("unexpected kind/name: %s/%s", findings[0].Kind, findings[0].Name)
	}
}

func TestPrivilegedAllowPrivEsc_Check_InitContainer(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  initContainers:
  - name: init-c
    image: nginx
    securityContext:
      privileged: true
      allowPrivilegeEscalation: false
  containers:
  - name: c
    image: nginx
`)
	check := privilegedAllowPrivEscCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for initContainer, got %d", len(findings))
	}
	if findings[0].Container != "init-c" {
		t.Errorf("unexpected container: %s", findings[0].Container)
	}
}

func TestPrivilegedAllowPrivEsc_Check_IDAndMetadata(t *testing.T) {
	check := privilegedAllowPrivEscCheck{}
	if got := check.ID(); got != "security-context/privileged-allow-priv-esc" {
		t.Errorf("ID() = %q, want %q", got, "security-context/privileged-allow-priv-esc")
	}
	if got := check.Title(); got != "Privileged Container Must Allow Privilege Escalation" {
		t.Errorf("Title() = %q", got)
	}
	if got := check.Category(); got != "security-context" {
		t.Errorf("Category() = %q", got)
	}
	if !check.Blocking() {
		t.Error("Blocking() should be true")
	}
	if !check.RenderSensitive() {
		t.Error("RenderSensitive() should be true")
	}
	if len(check.DocSkipper()) == 0 {
		t.Error("DocSkipper() should return PodSpec kinds")
	}
}

// --- allow-priv-esc-cap-sys-admin tests ---

func TestAllowPrivEscCapSysAdmin_Check(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        add:
        - CAP_SYS_ADMIN
`)
	check := allowPrivEscCapSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "security-context/allow-priv-esc-cap-sys-admin" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestAllowPrivEscCapSysAdmin_Check_AllowPrivEscTrue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      allowPrivilegeEscalation: true
      capabilities:
        add:
        - CAP_SYS_ADMIN
`)
	check := allowPrivEscCapSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when allowPrivilegeEscalation=true, got %d", len(findings))
	}
}

func TestAllowPrivEscCapSysAdmin_Check_NoCapSysAdmin(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        add:
        - NET_BIND_SERVICE
`)
	check := allowPrivEscCapSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when cap is not CAP_SYS_ADMIN, got %d", len(findings))
	}
}

func TestAllowPrivEscCapSysAdmin_Check_NilSecurityContext(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := allowPrivEscCapSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no securityContext, got %d", len(findings))
	}
}

// --- run-as-non-root-user-zero tests ---

func TestRunAsNonRootUserZero_Check(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      runAsNonRoot: true
      runAsUser: 0
`)
	check := runAsNonRootUserZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "security-context/run-as-non-root-user-zero" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestRunAsNonRootUserZero_Check_RootUserNotZero(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
`)
	check := runAsNonRootUserZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when runAsUser != 0, got %d", len(findings))
	}
}

func TestRunAsNonRootUserZero_Check_RunAsNonRootFalse(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      runAsNonRoot: false
      runAsUser: 0
`)
	check := runAsNonRootUserZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when runAsNonRoot=false, got %d", len(findings))
	}
}

func TestRunAsNonRootUserZero_Check_NilRunAsUser(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      runAsNonRoot: true
`)
	check := runAsNonRootUserZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when runAsUser is nil, got %d", len(findings))
	}
}

func TestRunAsNonRootUserZero_Check_NilSecurityContext(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := runAsNonRootUserZeroCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no securityContext, got %d", len(findings))
	}
}

// --- run-as-non-root-user-zero-pod-level tests ---

func TestRunAsNonRootUserZeroPodLevel_Check_PodLevelRootUser(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 0
  containers:
  - name: c
    image: nginx
`)
	check := runAsNonRootUserZeroPodLevelCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for container without explicit runAsUser, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "security-context/run-as-non-root-user-zero-pod-level" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestRunAsNonRootUserZeroPodLevel_Check_ContainerOverride(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 0
  containers:
  - name: c
    image: nginx
    securityContext:
      runAsUser: 1000
`)
	check := runAsNonRootUserZeroPodLevelCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when container has explicit runAsUser override, got %d", len(findings))
	}
}

func TestRunAsNonRootUserZeroPodLevel_Check_NoPodSecurityContext(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := runAsNonRootUserZeroPodLevelCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no pod securityContext, got %d", len(findings))
	}
}

func TestRunAsNonRootUserZeroPodLevel_Check_Deployment(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 0
      containers:
      - name: c
        image: nginx
`)
	check := runAsNonRootUserZeroPodLevelCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for Deployment, got %d: %v", len(findings), findings)
	}
	if findings[0].Kind != "Deployment" || findings[0].Name != "test" {
		t.Errorf("unexpected kind/name: %s/%s", findings[0].Kind, findings[0].Name)
	}
}

func TestRunAsNonRootUserZeroPodLevel_Check_PodLevelRunAsNonRootFalse(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  securityContext:
    runAsNonRoot: false
    runAsUser: 0
  containers:
  - name: c
    image: nginx
`)
	check := runAsNonRootUserZeroPodLevelCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when pod-level runAsNonRoot=false, got %d", len(findings))
	}
}

func TestRunAsNonRootUserZeroPodLevel_Check_PodLevelRunAsUserNotZero(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
  containers:
  - name: c
    image: nginx
`)
	check := runAsNonRootUserZeroPodLevelCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when pod-level runAsUser != 0, got %d", len(findings))
	}
}

// --- invalid-privilege-escalation-field tests ---

func TestInvalidPrivilegeEscalationField_Check_Nil(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      privileged: false
`)
	check := invalidPrivilegeEscalationFieldCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (nil allowPrivilegeEscalation), got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "security-context/invalid-privilege-escalation-field" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestInvalidPrivilegeEscalationField_Check_ExplicitTrue(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      privileged: false
      allowPrivilegeEscalation: true
`)
	check := invalidPrivilegeEscalationFieldCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when allowPrivilegeEscalation=true, got %d", len(findings))
	}
}

func TestInvalidPrivilegeEscalationField_Check_ExplicitFalse(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      privileged: false
      allowPrivilegeEscalation: false
`)
	check := invalidPrivilegeEscalationFieldCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when allowPrivilegeEscalation=false, got %d", len(findings))
	}
}

func TestInvalidPrivilegeEscalationField_Check_NilSecurityContext(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := invalidPrivilegeEscalationFieldCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no securityContext, got %d", len(findings))
	}
}

// --- capabilities-drop-all-add-sys-admin tests ---

func TestCapabilitiesDropAllAddSysAdmin_Check(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      capabilities:
        drop:
        - ALL
        add:
        - CAP_SYS_ADMIN
`)
	check := capabilitiesDropAllAddSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "security-context/capabilities-drop-all-add-sys-admin" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
}

func TestCapabilitiesDropAllAddSysAdmin_Check_MultipleAddedCaps(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      capabilities:
        drop:
        - ALL
        add:
        - NET_BIND_SERVICE
        - SYS_PTRACE
`)
	check := capabilitiesDropAllAddSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (ANY caps added when ALL dropped), got %d: %v", len(findings), findings)
	}
}

func TestCapabilitiesDropAllAddSysAdmin_Check_NoAddedCaps(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      capabilities:
        drop:
        - ALL
`)
	check := capabilitiesDropAllAddSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no caps added, got %d", len(findings))
	}
}

func TestCapabilitiesDropAllAddSysAdmin_Check_NotDropAll(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      capabilities:
        drop:
        - NET_BIND_SERVICE
        add:
        - CAP_SYS_ADMIN
`)
	check := capabilitiesDropAllAddSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when not dropping ALL, got %d", len(findings))
	}
}

func TestCapabilitiesDropAllAddSysAdmin_Check_NilSecurityContext(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
`)
	check := capabilitiesDropAllAddSysAdminCheck{}
	findings := check.Run(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings when no securityContext, got %d", len(findings))
	}
}

// --- ValidateSecurityContext integration tests ---

func TestValidateSecurityContext_MultipleViolations(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c1
    image: nginx
    securityContext:
      privileged: true
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
        add:
        - CAP_SYS_ADMIN
`)
	findings := ValidateSecurityContext(data, "test.yaml")
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings (privileged + cap-sys-admin + caps-drop-all), got %d: %v", len(findings), findings)
	}
	ruleIDs := make(map[string]bool)
	for _, f := range findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs["security-context/privileged-allow-priv-esc"] {
		t.Error("missing privileged-allow-priv-esc finding")
	}
	if !ruleIDs["security-context/allow-priv-esc-cap-sys-admin"] {
		t.Error("missing allow-priv-esc-cap-sys-admin finding")
	}
	if !ruleIDs["security-context/capabilities-drop-all-add-sys-admin"] {
		t.Error("missing capabilities-drop-all-add-sys-admin finding")
	}
}

func TestValidateSecurityContext_Clean(t *testing.T) {
	data := []byte(`kind: Pod
metadata:
  name: test
spec:
  containers:
  - name: c
    image: nginx
    securityContext:
      privileged: false
      allowPrivilegeEscalation: false
      runAsNonRoot: true
      runAsUser: 1000
      capabilities:
        drop:
        - ALL
`)
	findings := ValidateSecurityContext(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for clean pod, got %d: %v", len(findings), findings)
	}
}

func TestValidateSecurityContext_NonWorkload(t *testing.T) {
	data := []byte(`kind: Service
metadata:
  name: test
spec:
  ports:
  - port: 80
`)
	findings := ValidateSecurityContext(data, "test.yaml")
	if len(findings) != 0 {
		t.Errorf("expected no findings for Service, got %d: %v", len(findings), findings)
	}
}

func TestValidateSecurityContext_InvalidYAML(t *testing.T) {
	data := []byte(`not valid yaml {{`)
	findings := ValidateSecurityContext(data, "test.yaml")
	if findings != nil {
		t.Errorf("expected nil for invalid YAML, got %v", findings)
	}
}

// --- Check interface implementation verification ---

func TestAllChecksImplementCheckInterface(t *testing.T) {
	checks := []runtime.Check{
		privilegedAllowPrivEscCheck{},
		allowPrivEscCapSysAdminCheck{},
		runAsNonRootUserZeroCheck{},
		runAsNonRootUserZeroPodLevelCheck{},
		invalidPrivilegeEscalationFieldCheck{},
		capabilitiesDropAllAddSysAdminCheck{},
	}

	for _, c := range checks {
		if c.ID() == "" {
			t.Errorf("check %T has empty ID", c)
		}
		if c.Title() == "" {
			t.Errorf("check %T has empty Title", c)
		}
		if c.Category() == "" {
			t.Errorf("check %T has empty Category", c)
		}
		if !c.Blocking() {
			t.Errorf("check %T should be blocking", c)
		}
		if !c.RenderSensitive() {
			t.Errorf("check %T should render sensitive", c)
		}
		if len(c.DocSkipper()) == 0 {
			t.Errorf("check %T should have DocSkipper", c)
		}
	}
}

func TestRegister(t *testing.T) {
	// Verify that Register() can be called without panicking
	// (this tests that all checks have unique IDs and valid metadata)
	Register()
}
