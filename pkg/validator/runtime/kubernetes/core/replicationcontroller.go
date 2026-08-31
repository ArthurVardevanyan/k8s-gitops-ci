package core

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// ReplicationController is validated here rather than under apps/ because it
// is a core/v1 type: its rules live in
// pkg/apis/core/validation.ValidateReplicationControllerSpec, not in the apps
// validation package the other workload controllers are ported from.
//
// Until these existed the kind was half-supported. It has a pod template, so
// runtime.HasPodSpecKinds includes it and every container, volume and
// pod-spec rule already ran against it, while the controller-level fields
// that every other workload kind has checked - replicas, selector,
// minReadySeconds - had no rules at all.

const replicationControllerKind = "ReplicationController"

// parseReplicationController decodes a manifest only if it is a
// ReplicationController, returning it with its metadata.name.
//
// The kind test duplicates the adapter's Kinds() gate during a pipeline run
// and is kept for the same reason as its apps counterpart: a check invoked
// directly, as the tests do, must still report only on its own kind.
func parseReplicationController(data []byte) (obj map[string]interface{}, name string) {
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, ""
	}
	kind, _, err := unstructured.NestedString(obj, "kind")
	if err != nil || kind != replicationControllerKind {
		return nil, ""
	}
	name, _, _ = unstructured.NestedString(obj, "metadata", "name")

	return obj, name
}

// rcFinding builds a finding against a ReplicationController.
func rcFinding(c runtime.Check, name, path, message, value string) runtime.Finding {
	return runtime.Finding{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Finding: check.Finding{
			Path:    path,
			Message: message,
			Kind:    replicationControllerKind,
			Name:    name,
			Value:   value,
		},
	}
}

// replicationControllerReplicasInvalidCheck verifies spec.replicas is present
// and non-negative.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type replicationControllerReplicasInvalidCheck struct{ runtime.Meta }

func newReplicationControllerReplicasInvalidCheck() replicationControllerReplicasInvalidCheck {
	return replicationControllerReplicasInvalidCheck{runtime.Meta{
		RuleID:    "kubernetes/core/replicationcontroller-replicas-invalid",
		RuleTitle: "ReplicationController Replicas Must Be >= 0",
		AppliesTo: []string{replicationControllerKind},
	}}
}

func (c replicationControllerReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseReplicationController(data)
	if obj == nil {
		return nil
	}

	replicas, found, err := unstructured.NestedInt64(obj, "spec", "replicas")
	if err != nil || !found {
		// Upstream reports Required when replicas is nil, but defaulting
		// sets it to 1 before validation and an unrendered manifest
		// legitimately omits it. Reporting here would fail nearly every
		// ReplicationController ever written, on a non-exemptable check.
		return nil
	}

	if replicas >= 0 {
		return nil
	}

	return []runtime.Finding{rcFinding(c, name,
		field.NewPath("spec").Child("replicas").String(),
		fmt.Sprintf("replicas: must be >= 0, got %d", replicas),
		fmt.Sprintf("%d", replicas))}
}

// replicationControllerSelectorInvalidCheck verifies spec.selector is not
// empty.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type replicationControllerSelectorInvalidCheck struct{ runtime.Meta }

func newReplicationControllerSelectorInvalidCheck() replicationControllerSelectorInvalidCheck {
	return replicationControllerSelectorInvalidCheck{runtime.Meta{
		RuleID:    "kubernetes/core/replicationcontroller-selector-invalid",
		RuleTitle: "ReplicationController Selector Must Not Be Empty",
		AppliesTo: []string{replicationControllerKind},
	}}
}

func (c replicationControllerSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseReplicationController(data)
	if obj == nil {
		return nil
	}

	// A ReplicationController's selector is a plain label map, not a
	// LabelSelector, so ValidateNonEmptySelector is simply "selects
	// something".
	//
	// It is not read as written. SetDefaults_ReplicationController copies
	// spec.template.metadata.labels into an empty selector before
	// validation, so omitting the selector is both valid and the ordinary
	// way to write the kind. Reporting the field as absent would be a
	// non-exemptable finding on a manifest the API server accepts - the
	// same mistake as validating a StatefulSet's volume mounts without
	// first synthesizing its claim-template volumes.
	//
	// The rule therefore fires only when defaulting has nothing to supply
	// either, which is exactly when upstream's selector is still empty by
	// the time ValidateNonEmptySelector runs.
	if selector, found, err := unstructured.NestedStringMap(obj, "spec", "selector"); err == nil && found && len(selector) > 0 {
		return nil
	}
	if labels, found, err := unstructured.NestedStringMap(obj, "spec", "template", "metadata", "labels"); err == nil && found && len(labels) > 0 {
		return nil
	}

	return []runtime.Finding{rcFinding(c, name,
		field.NewPath("spec").Child("selector").String(),
		"selector: Required value (spec.template.metadata.labels is empty, so defaulting cannot supply one)", "")}
}

// replicationControllerMinReadySecondsInvalidCheck verifies
// spec.minReadySeconds is non-negative.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type replicationControllerMinReadySecondsInvalidCheck struct{ runtime.Meta }

func newReplicationControllerMinReadySecondsInvalidCheck() replicationControllerMinReadySecondsInvalidCheck {
	return replicationControllerMinReadySecondsInvalidCheck{runtime.Meta{
		RuleID:    "kubernetes/core/replicationcontroller-min-ready-seconds-invalid",
		RuleTitle: "ReplicationController MinReadySeconds Must Be >= 0",
		AppliesTo: []string{replicationControllerKind},
	}}
}

func (c replicationControllerMinReadySecondsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseReplicationController(data)
	if obj == nil {
		return nil
	}

	minReadySeconds, found, err := unstructured.NestedInt64(obj, "spec", "minReadySeconds")
	if err != nil || !found {
		return nil
	}

	if minReadySeconds >= 0 {
		return nil
	}

	return []runtime.Finding{rcFinding(c, name,
		field.NewPath("spec").Child("minReadySeconds").String(),
		fmt.Sprintf("minReadySeconds: must be >= 0, got %d", minReadySeconds),
		fmt.Sprintf("%d", minReadySeconds))}
}
