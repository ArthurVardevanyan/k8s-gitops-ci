package core

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// This file ports the container name-format rules. The existing checks in
// container.go cover duplicate container names and duplicate port names; both
// of those assume the names are well-formed in the first place, which nothing
// verified until now.
//
// Neither rule is reachable by schema validation: in the embedded schemas
// container name and port name are plain strings with no pattern. See
// docs/SCHEMAS.md.

// containerNameInvalidCheck validates that every container name is a DNS-1123
// label, and that a name is present at all.
//
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
// (validateContainerCommon)
type containerNameInvalidCheck struct{ runtime.Meta }

func newContainerNameInvalidCheck() containerNameInvalidCheck {
	return containerNameInvalidCheck{runtime.Meta{
		RuleID:    "container/name-invalid",
		RuleTitle: "Container Name Must Be A DNS-1123 Label",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
}

func (c containerNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	finding := func(ctr runtime.ContainerWithPath, message, value string) runtime.Finding {
		return runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:      ctr.Path.Child("name").String(),
				Message:   message,
				Kind:      info.Kind,
				Name:      info.Name,
				Namespace: info.Namespace,
				Container: ctr.Container.Name,
				Value:     value,
			},
		}
	}

	var findings []runtime.Finding
	for _, ctr := range runtime.AllContainers(info) {
		// Upstream reports an absent name as Required and only validates
		// the format otherwise; the two are mutually exclusive.
		if ctr.Container.Name == "" {
			findings = append(findings, finding(ctr, "container name is required", ""))
			continue
		}

		for _, msg := range validation.IsDNS1123Label(ctr.Container.Name) {
			findings = append(findings, finding(ctr,
				fmt.Sprintf("invalid container name %q: %s", ctr.Container.Name, msg),
				ctr.Container.Name))
		}
	}

	return findings
}

// containerPortNameInvalidCheck validates the format of named container ports.
//
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
// (validateContainerPorts)
type containerPortNameInvalidCheck struct{ runtime.Meta }

func newContainerPortNameInvalidCheck() containerPortNameInvalidCheck {
	return containerPortNameInvalidCheck{runtime.Meta{
		RuleID:    "container/port-name-invalid",
		RuleTitle: "Container Port Name Must Be A Valid IANA Service Name",
		AppliesTo: runtime.HasPodSpecKinds(),
	}}
}

func (c containerPortNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	for _, ctr := range runtime.AllContainers(info) {
		// Index by position, as upstream validateContainerPorts does. A
		// name-keyed path (ports[http].name) does not exist in the manifest
		// and cannot be navigated to, which is worst when several ports are
		// wrong at once and the reader needs to tell them apart.
		for i, port := range ctr.Container.Ports {
			// Upstream only validates a port name when one is set; an
			// unnamed port is legal on a single-port container.
			if port.Name == "" {
				continue
			}

			for _, msg := range validation.IsValidPortName(port.Name) {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Finding: check.Finding{
						Path:      ctr.Path.Child("ports").Index(i).Child("name").String(),
						Message:   fmt.Sprintf("invalid port name %q: %s", port.Name, msg),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     port.Name,
					},
				})
			}
		}
	}

	return findings
}
