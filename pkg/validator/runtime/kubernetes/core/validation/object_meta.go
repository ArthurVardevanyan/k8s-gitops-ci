package validation

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// This file ports the object-metadata rules the API server applies to every
// object, from ValidateObjectMetaAccessor and
// validateObjectMetaAccessorWithOptsCommon in
// k8s.io/apimachinery/pkg/api/validation/objectmeta.go.
//
// Neither half of this is reachable by JSON-schema validation: in the
// embedded schemas metadata.name is `{"type": ["string","null"]}` with no
// `pattern` and no `maxLength`, and `metadata` carries no `required` array
// at all - so kubeconform accepts a nameless object and any name string
// whatsoever. See docs/SCHEMAS.md.

// nameValidator reports validation errors for an object name. It mirrors the
// apimachinery ValidateNameFunc signature exactly, including the `prefix`
// argument: when validating metadata.generateName the API server passes
// prefix=true, which tolerates a trailing dash because the server appends a
// random suffix. Dropping that argument would falsely reject the completely
// ordinary `generateName: my-prefix-`.
type nameValidator func(name string, prefix bool) []string

// maskTrailingDash ports apimachinery's helper of the same name
// (k8s.io/apimachinery/pkg/api/validation/generic.go). The `len(name)-2`
// slice is an upstream quirk - it drops the final two characters and appends
// one - but it is reproduced verbatim because the only thing that matters is
// that a trailing dash no longer terminates the string, and diverging here
// would change which names validate.
func maskTrailingDash(name string) string {
	if len(name) > 1 && strings.HasSuffix(name, "-") {
		return name[:len(name)-2] + "a"
	}
	return name
}

func dns1123Subdomain(name string, prefix bool) []string {
	if prefix {
		name = maskTrailingDash(name)
	}
	return validation.IsDNS1123Subdomain(name)
}

func dns1123Label(name string, prefix bool) []string {
	if prefix {
		name = maskTrailingDash(name)
	}
	return validation.IsDNS1123Label(name)
}

// pathSegmentName ports content.IsPathSegmentName / IsPathSegmentPrefix
// (k8s.io/apimachinery/pkg/api/validate/content/path.go). It is deliberately
// far laxer than the DNS rules: it imposes no charset restriction and no
// length limit, rejecting only "." / ".." and names containing "/" or "%".
// RBAC objects use this, so a Role named "My.Role" is valid and must not be
// flagged.
//
// As upstream does, the exact "."/".." comparison is skipped in prefix mode,
// since an appended suffix could make such a value legal.
func pathSegmentName(name string, prefix bool) []string {
	if !prefix {
		for _, illegal := range []string{".", ".."} {
			if name == illegal {
				return []string{fmt.Sprintf("may not be '%s'", illegal)}
			}
		}
	}
	var errs []string
	for _, illegal := range []string{"/", "%"} {
		if strings.Contains(name, illegal) {
			errs = append(errs, fmt.Sprintf("may not contain '%s'", illegal))
		}
	}
	return errs
}

// nameValidators maps a Kind to the name-validation function the API server
// applies to it. Every entry is derived from a specific upstream call site;
// the rules are NOT uniform, which is the whole reason this map exists rather
// than a blanket DNS-1123 check.
//
// Kinds absent from this map are skipped rather than defaulted. Custom
// resources make up the bulk of documents in a typical GitOps repository and
// their name rules are not reliably DNS-1123, so guessing would risk blocking
// valid manifests - and these findings are non-exemptable.
var nameValidators = map[string]nameValidator{
	// NameIsDNSSubdomain (RFC 1123 subdomain, max 253 chars).
	"Pod":                            dns1123Subdomain, // core ValidatePodName
	"ConfigMap":                      dns1123Subdomain, // core ValidateConfigMapName
	"Secret":                         dns1123Subdomain, // core ValidateSecretName
	"LimitRange":                     dns1123Subdomain, // core ValidateLimitRangeName
	"ResourceQuota":                  dns1123Subdomain, // core ValidateResourceQuotaName
	"ServiceAccount":                 dns1123Subdomain, // core ValidateServiceAccountName
	"PersistentVolume":               dns1123Subdomain, // core ValidatePersistentVolumeName
	"PersistentVolumeClaim":          dns1123Subdomain, // core ValidatePersistentVolumeClaim
	"PriorityClass":                  dns1123Subdomain, // scheduling ValidatePriorityClass
	"StorageClass":                   dns1123Subdomain, // storage ValidateClassName
	"Deployment":                     dns1123Subdomain, // apps ValidateDeploymentName
	"DaemonSet":                      dns1123Subdomain, // apps ValidateDaemonSetName
	"ReplicaSet":                     dns1123Subdomain, // apps ValidateReplicaSetName
	"Job":                            dns1123Subdomain, // batch ValidateJob
	"CronJob":                        dns1123Subdomain, // batch ValidateCronJob
	"HorizontalPodAutoscaler":        dns1123Subdomain, // autoscaling ValidateHorizontalPodAutoscalerName
	"Ingress":                        dns1123Subdomain, // networking ValidateIngressName
	"IngressClass":                   dns1123Subdomain, // networking ValidateIngressClassName
	"NetworkPolicy":                  dns1123Subdomain, // networking ValidateNetworkPolicyName
	"MutatingWebhookConfiguration":   dns1123Subdomain, // admissionregistration
	"ValidatingWebhookConfiguration": dns1123Subdomain, // admissionregistration

	// NameIsDNSLabel (RFC 1123 label, max 63 chars, no dots).
	"Namespace":   dns1123Label, // core ValidateNamespaceName
	"StatefulSet": dns1123Label, // apps ValidateStatefulSetName
	// Service historically used NameIsDNS1035Label, which additionally
	// requires the name to start with a letter. KEP-5311
	// (RelaxedServiceNameValidation) moves it to NameIsDNSLabel: alpha and
	// off by default in 1.34, beta and ON by default in 1.36, GA and locked
	// on in 1.37. The relaxed rule is therefore the modern default, and it
	// is the permissive one - using it means a Service name starting with a
	// digit is never falsely blocked on a cluster where the gate is on,
	// while uppercase, underscores, dots and over-length names are still
	// caught everywhere.
	"Service": dns1123Label,

	// IsPathSegmentName - RBAC only.
	"Role":               pathSegmentName, // rbac ValidateRBACName
	"ClusterRole":        pathSegmentName, // rbac ValidateRBACName
	"RoleBinding":        pathSegmentName, // rbac ValidateRBACName
	"ClusterRoleBinding": pathSegmentName, // rbac ValidateRBACName

	// Deliberately absent:
	//   CustomResourceDefinition - its name rule is the cross-field
	//     "<plural>.<group>" form, which belongs with the other
	//     apiextensions checks rather than here.
	//   PodDisruptionBudget - policy's ValidatePodDisruptionBudget does not
	//     call ValidateObjectMeta, so the rule could not be confirmed
	//     against a specific upstream call site.
}

// objectMetaKinds returns the sorted set of kinds these checks apply to.
func objectMetaKinds() []string {
	kinds := make([]string, 0, len(nameValidators))
	for k := range nameValidators {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

// objectMeta is the subset of metadata these checks read.
type objectMeta struct {
	Name         string `json:"name"`
	GenerateName string `json:"generateName"`
	Namespace    string `json:"namespace"`
}

func parseObjectMeta(data []byte) (kind string, meta objectMeta, ok bool) {
	var doc struct {
		Kind     string     `json:"kind"`
		Metadata objectMeta `json:"metadata"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", objectMeta{}, false
	}
	return doc.Kind, doc.Metadata, true
}

// objectMetaNameInvalidCheck validates metadata.name and metadata.generateName
// against the name-validation function the API server applies to the object's
// kind.
//
// Source: k8s.io/apimachinery/pkg/api/validation/objectmeta.go
// (ValidateObjectMetaAccessor)
type objectMetaNameInvalidCheck struct{}

func (c objectMetaNameInvalidCheck) ID() string { return "core/object-meta-name-invalid" }
func (c objectMetaNameInvalidCheck) Title() string {
	return "Object Name Must Be Valid For Its Kind"
}
func (c objectMetaNameInvalidCheck) Category() string      { return "core" }
func (c objectMetaNameInvalidCheck) Blocking() bool        { return true }
func (c objectMetaNameInvalidCheck) RenderSensitive() bool { return true }
func (c objectMetaNameInvalidCheck) Kinds() []string       { return objectMetaKinds() }

func (c objectMetaNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	kind, meta, ok := parseObjectMeta(data)
	if !ok {
		return nil
	}
	validate, known := nameValidators[kind]
	if !known {
		return nil
	}

	finding := func(path, message, value string) runtime.Finding {
		return runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:      path,
				Message:   message,
				Kind:      kind,
				Name:      meta.Name,
				Namespace: meta.Namespace,
				Value:     value,
			},
		}
	}

	var findings []runtime.Finding

	// Upstream validates generateName whenever it is set, independently of
	// name (objectmeta.go: `if len(meta.GetGenerateName()) != 0`).
	if meta.GenerateName != "" {
		for _, msg := range validate(meta.GenerateName, true) {
			findings = append(findings, finding("metadata.generateName",
				fmt.Sprintf("metadata.generateName: invalid value %q: %s", meta.GenerateName, msg),
				meta.GenerateName))
		}
	}

	// Upstream requires a name only when generateName is also absent, and
	// reports it as "name or generateName is required". An object supplying
	// only generateName is valid.
	if meta.Name == "" {
		if meta.GenerateName == "" {
			findings = append(findings, finding("metadata.name",
				"metadata.name: name or generateName is required", ""))
		}
		return findings
	}

	for _, msg := range validate(meta.Name, false) {
		findings = append(findings, finding("metadata.name",
			fmt.Sprintf("metadata.name: invalid value %q: %s", meta.Name, msg),
			meta.Name))
	}
	return findings
}

// objectMetaNamespaceInvalidCheck validates the *format* of metadata.namespace
// when one is set: a namespace name must be a valid DNS-1123 label.
//
// It deliberately does NOT check whether a namespace is present or forbidden
// for the object's scope. That rule (namespaced objects must set a namespace,
// cluster-scoped objects must not) is already owned by the exemptable
// "namespace" static check, which carries the generated cluster resource-scope
// map. Duplicating it here would double-report and would make an exemptable
// policy decision unexemptable.
//
// Source: k8s.io/apimachinery/pkg/api/validation/objectmeta.go
// (validateObjectMetaAccessorWithOptsCommon -> ValidateNamespaceName)
type objectMetaNamespaceInvalidCheck struct{}

func (c objectMetaNamespaceInvalidCheck) ID() string { return "core/object-meta-namespace-invalid" }
func (c objectMetaNamespaceInvalidCheck) Title() string {
	return "Object Namespace Must Be A Valid DNS Label"
}
func (c objectMetaNamespaceInvalidCheck) Category() string      { return "core" }
func (c objectMetaNamespaceInvalidCheck) Blocking() bool        { return true }
func (c objectMetaNamespaceInvalidCheck) RenderSensitive() bool { return true }
func (c objectMetaNamespaceInvalidCheck) Kinds() []string       { return objectMetaKinds() }

func (c objectMetaNamespaceInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	kind, meta, ok := parseObjectMeta(data)
	if !ok {
		return nil
	}
	if _, known := nameValidators[kind]; !known {
		return nil
	}
	if meta.Namespace == "" {
		return nil
	}

	var findings []runtime.Finding
	for _, msg := range validation.IsDNS1123Label(meta.Namespace) {
		findings = append(findings, runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:      "metadata.namespace",
				Message:   fmt.Sprintf("metadata.namespace: invalid value %q: %s", meta.Namespace, msg),
				Kind:      kind,
				Name:      meta.Name,
				Namespace: meta.Namespace,
				Value:     meta.Namespace,
			},
		})
	}
	return findings
}
