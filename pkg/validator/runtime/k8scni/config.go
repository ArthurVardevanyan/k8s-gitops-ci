package k8scni

import (
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// configInvalidCheck validates that a NAD's spec.config parses as a
// structurally valid CNI configuration, independent of which CNI plugin
// owns it. See package doc comment and Probe/ProbeConfig for how.
type configInvalidCheck struct{ runtime.Meta }

func newConfigInvalidCheck() configInvalidCheck {
	return configInvalidCheck{runtime.Meta{
		RuleID:    "k8scni/net-attach-def/config-invalid",
		RuleTitle: "NetworkAttachmentDefinition spec.config Must Be A Valid CNI Configuration",
		AppliesTo: nadKinds,
	}}
}

func (c configInvalidCheck) Run(data []byte, _ string) []runtime.Finding {
	var doc nadDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// Malformed YAML is caught elsewhere (yaml-syntax, kubeconform);
		// this check has nothing to add when the document itself doesn't
		// even parse.
		return nil
	}

	cfg, err := configString(doc.Spec.Config)
	if err != nil {
		return []runtime.Finding{configFinding(c, doc, err.Error())}
	}
	if strings.TrimSpace(cfg) == "" {
		return []runtime.Finding{configFinding(c, doc, "spec.config must not be empty")}
	}
	if _, err := ProbeConfig(cfg); err != nil {
		return []runtime.Finding{configFinding(c, doc, "spec.config: "+err.Error())}
	}
	return nil
}

func configFinding(c runtime.Check, doc nadDoc, msg string) runtime.Finding {
	return runtime.NewFinding(c, check.Finding{
		Kind:      "NetworkAttachmentDefinition",
		Name:      doc.Metadata.Name,
		Namespace: doc.Metadata.Namespace,
		Path:      "spec.config",
		Message:   msg,
	})
}
