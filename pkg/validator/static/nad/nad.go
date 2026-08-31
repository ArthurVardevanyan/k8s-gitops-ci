// Package nad provides the CNI-neutral, always-on advisory tier of
// NetworkAttachmentDefinition (NAD) validation: non-gating warnings for
// likely authoring mistakes, applied uniformly to every NAD regardless of
// which CNI plugin owns it (macvlan, bridge, ipvlan, host-device, SR-IOV,
// ovn-k8s-cni-overlay, ...).
//
// It reports no hard failures. Whether spec.config is a non-empty JSON
// string that parses as a CNI configuration with a plugin "type" is decided
// by pkg/validator/runtime/k8scni's config-invalid check, which is where
// that shape requirement is both enforced and cited.
//
// Two further tiers exist but no longer live in this package:
//
//   - OVN-Kubernetes NADs (type "ovn-k8s-cni-overlay") additionally get
//     OVN's semantic rules (topology, role, subnet, and transport
//     constraints), ported from ovn-kubernetes' util.ValidateNetConf. That
//     tier moved to pkg/validator/runtime/k8scni (the
//     "k8scni/net-attach-def/ovn-netconf-invalid" check) - it is a genuine, citable
//     upstream rule the OVN-Kubernetes network controller enforces, so it
//     belongs in the runtime-validation family (always-blocking,
//     non-exemptable, with a verified UpstreamRef), not here.
//   - Non-OVN NADs get non-gating advisories for likely authoring mistakes
//     (unrecognized CNI/IPAM type, missing cniVersion). These have no
//     citable upstream function - they are this tool's own heuristics, not
//     a rule any upstream project's code enforces - so they stay here
//     rather than in the runtime family, which requires one.
//
// Dispatching on the type field (rather than a global "assume OpenShift"
// flag) fixes an earlier design that gated the OVN tier behind an
// assumeOpenshift flag and treated every non-ovn-k8s-cni-overlay NAD as
// invalid ("net-attach-def not managed by OVN"). That false-failed valid
// secondary networks such as ODF's macvlan NADs. Upstream ovn-kubernetes
// itself treats a non-OVN NAD as a skip, never a failure (see
// pkg/util/multi_network.go's ParseNetConf and its callers) - the OVN check
// in pkg/validator/runtime/k8scni mirrors that by only applying OVN rules to
// OVN NADs, and this package never re-applies OVN-specific rules at all.
package nad

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	k8scni "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/k8scni"
)

// knownCNITypes are the CNI plugin types shipped/supported on OpenShift for
// additional networks. An unrecognized type is surfaced as a non-gating
// advisory (a likely typo) rather than an error: CNI plugin types are
// open-ended (any binary on the CNI path, including third-party plugins), so
// hard-failing an unlisted type would reintroduce false positives.
var knownCNITypes = map[string]bool{
	"ovn-k8s-cni-overlay": true,
	"macvlan":             true,
	"bridge":              true,
	"ipvlan":              true,
	"host-device":         true,
	"tap":                 true,
	"vlan":                true,
	"sriov":               true,
}

// knownIPAMTypes are the IPAM plugin types commonly used on OpenShift. As with
// knownCNITypes, an unrecognized value is advisory-only.
var knownIPAMTypes = map[string]bool{
	"host-local":  true,
	"static":      true,
	"dhcp":        true,
	"whereabouts": true,
}

// ValidationError represents a NAD validation finding. A finding with Warning
// set is advisory (a likely authoring mistake) and must not gate the pipeline;
// only non-warning findings are hard failures. ValidateFiles/ValidateDir
// return the two severities separately.
type ValidationError struct {
	File    string
	Message string
	Warning bool
}

func (e ValidationError) String() string { return fmt.Sprintf("%s: %s", e.File, e.Message) }

// nadDoc is the minimal shape of a NetworkAttachmentDefinition needed for
// validation. Decoding into this avoids depending on the typed NAD client.
//
// Spec.Config is captured as json.RawMessage rather than a string so that
// decoding never fails on non-NAD documents that legitimately carry an object
// under spec.config (for example an OLM Subscription's spec.config). Every
// document in a rendered overlay file is decoded through nadDoc before Kind is
// checked, so a plain string field here would abort validation of the whole
// file - including the actual NAD - the moment a sibling document's spec.config
// happened to be an object. Only genuine NAD documents have their config
// asserted to be a string (see configString).
type nadDoc struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Config json.RawMessage `json:"config"`
	} `json:"spec"`
}

// configString asserts that the NAD's spec.config is a JSON string and returns
// its decoded value. A NetworkAttachmentDefinition's spec.config must be a
// stringified CNI netconf; anything else (e.g. an object) is a malformed NAD.
func configString(doc nadDoc) (string, error) {
	if len(doc.Spec.Config) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(doc.Spec.Config, &s); err != nil {
		return "", fmt.Errorf("spec.config must be a string")
	}
	return s, nil
}

// ValidateFiles validates all NAD YAML files in the given list, returning hard
// errors and advisory warnings separately. Files that do not declare a
// NetworkAttachmentDefinition are skipped.
func ValidateFiles(files []string) (errs, warns []ValidationError) {
	for _, f := range files {
		if !IsNADFile(f) {
			continue
		}
		fe, fw := validateNADFile(f)
		errs = append(errs, fe...)
		warns = append(warns, fw...)
	}
	return errs, warns
}

// ValidateDir validates all NAD files in a directory tree.
func ValidateDir(dir string) (errs, warns []ValidationError) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && IsNADFile(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return []ValidationError{{File: dir, Message: fmt.Sprintf("walking directory: %v", err)}}, nil
	}
	return ValidateFiles(files)
}

// nadKindRE matches a YAML `kind: NetworkAttachmentDefinition` mapping entry,
// tolerating arbitrary whitespace after the colon, optional quoting of the
// value (e.g. `kind:NetworkAttachmentDefinition`,
// `kind: "NetworkAttachmentDefinition"`), and a trailing line comment.
//
// The comment case is load-bearing rather than cosmetic: this predicate gates
// ValidateFiles, so a NAD whose kind line ends in a comment was not merely
// absent from the report - it was never validated at all. A false negative
// here is silent by construction, since the file it skips is the only thing
// that would have reported anything.
//
// The space before "#" is required, because YAML only starts a comment after
// whitespace: `kind: NetworkAttachmentDefinition#x` is the scalar
// "NetworkAttachmentDefinition#x", which is a different kind and must not
// match.
var nadKindRE = regexp.MustCompile(`(?m)^\s*kind:\s*["']?NetworkAttachmentDefinition["']?(?:\s+#.*)?\s*$`)

// IsNADFile reports whether path is a YAML file whose content declares
// `kind: NetworkAttachmentDefinition`. Unlike a bare extension check, this
// requires actually reading the file, so a non-existent or unreadable path
// is reported as "not a NAD file" rather than erroring here — ValidateFiles/
// ValidateDir surface read errors themselves once a file is dispatched to
// validateNADFile.
func IsNADFile(path string) bool {
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return ContainsNAD(data)
}

// ContainsNAD reports whether the given YAML bytes declare at least one
// `kind: NetworkAttachmentDefinition` document. It's the in-memory
// counterpart to IsNADFile, letting callers that already hold a (possibly
// multi-document) rendered-overlay buffer detect whether a NAD is present
// without re-reading it from disk - used to decide whether the NAD report
// section should be rendered at all (an overlay chain with no NAD gets no
// section rather than a "0 NADs, all good" stub).
func ContainsNAD(data []byte) bool {
	return nadKindRE.Match(data)
}

// validateNADFile decodes every document in the file and validates each
// NetworkAttachmentDefinition, dispatching on the CNI plugin type declared in
// spec.config. Non-NAD documents (e.g. an OLM Subscription whose spec.config
// is an object) are skipped by kind and never break decoding.
func validateNADFile(path string) (errs, warns []ValidationError) {
	data, err := os.ReadFile(path)
	if err != nil {
		return []ValidationError{{File: path, Message: fmt.Sprintf("read error: %v", err)}}, nil
	}

	dec := kyaml.NewYAMLToJSONDecoder(bytes.NewReader(data))
	for {
		var doc nadDoc
		if decErr := dec.Decode(&doc); decErr != nil {
			if errors.Is(decErr, io.EOF) {
				break
			}
			errs = append(errs, ValidationError{File: path, Message: fmt.Sprintf("decoding YAML: %v", decErr)})
			return errs, warns
		}
		if doc.Kind != "NetworkAttachmentDefinition" {
			continue
		}
		warns = append(warns, validateNAD(path, doc)...)
	}
	return errs, warns
}

// validateNAD produces this section's advisories for one NAD.
//
// It reports no hard failures. Whether spec.config parses at all is decided by
// the runtime check "k8scni/net-attach-def/config-invalid", which covers every
// case this gate used to reject - an unreadable or empty config, one that is
// not valid JSON, one that is not a CNI configuration, and a missing plugin
// "type" in either config shape. Reporting them here as well put one
// malformed config in two sections as two blocking findings, leaving a reader
// to work out that they were the same defect and which layer to act on.
//
// It still has to parse the config, because the advisories below are about its
// contents. When that fails it returns nothing and lets the runtime check
// report it, rather than describing a config it could not read.
func validateNAD(path string, doc nadDoc) (warns []ValidationError) {
	id := doc.Metadata.Namespace + "/" + doc.Metadata.Name
	warnf := func(msg string) {
		warns = append(warns, ValidationError{File: path, Message: fmt.Sprintf("NetworkAttachmentDefinition %s: %s", id, msg), Warning: true})
	}

	cfg, err := configString(doc)
	if err != nil {
		return warns
	}

	// The same parser the config-invalid runtime check uses, rather than a
	// second implementation of it. Re-parsing here meant the two tiers could
	// judge a different value for one NAD: a config this package read well
	// enough to call the plugin type unrecognized, while the runtime check
	// could not read it at all, produced an advisory about a config that had
	// already been reported as unparseable.
	//
	// Deferring to it also means an unreadable config yields nothing here.
	// That is deliberate - describing the contents of a config that does not
	// parse is guesswork, and the runtime check has already said so.
	probe, err := k8scni.ProbeConfig(cfg)
	if err != nil {
		return warns
	}
	cniType := strings.TrimSpace(probe.CNIType)
	ipam := strings.TrimSpace(probe.IPAMType)
	if cniType == "" {
		return warns
	}

	// Non-gating advisories for likely authoring mistakes. OVN-Kubernetes
	// NADs get these too - unlike the superseded design, they are no longer
	// exempted from this tier by an early return, since these advisories
	// have never depended on OVN's own hard semantic rules (which live in
	// pkg/validator/runtime/k8scni's "k8scni/net-attach-def/ovn-netconf-invalid" check).
	if !knownCNITypes[cniType] {
		warnf(fmt.Sprintf("unrecognized CNI type %q (typo? not gating)", cniType))
	}
	if ipam != "" && !knownIPAMTypes[ipam] {
		warnf(fmt.Sprintf("unrecognized IPAM type %q (typo? not gating)", ipam))
	}
	if strings.TrimSpace(probe.CNIVersion) == "" {
		warnf("spec.config is missing the recommended \"cniVersion\" field (not gating)")
	}
	return warns
}
