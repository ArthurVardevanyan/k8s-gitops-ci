// Package nad provides NetworkAttachmentDefinition (NAD) validation.
//
// Validation dispatches on the CNI plugin type declared in each NAD's
// spec.config (a stringified CNI netconf), rather than on a global platform
// assumption:
//
//   - Structural (always, org/CNI-neutral): spec.config must be a non-empty
//     JSON string, must parse as valid JSON (a single CNI config object or a
//     conflist), and must declare a non-empty plugin "type".
//   - OVN-Kubernetes NADs (type "ovn-k8s-cni-overlay"): OVN's semantic rules
//     (topology, role, subnet, and transport constraints) are additionally
//     applied. These run wherever such a NAD is authored - the type field is
//     self-describing, so no "assume OpenShift" flag is needed.
//   - Non-OVN NADs (macvlan, bridge, ipvlan, host-device, SR-IOV, ...): their
//     config is owned by the respective CNI plugin, so no hard semantic checks
//     are applied. Only non-gating advisories are surfaced for likely
//     authoring mistakes (unrecognized CNI/IPAM type, missing cniVersion).
//
// Dispatching on the type field fixes an earlier design that gated the OVN
// tier behind an assumeOpenshift flag and treated every
// non-ovn-k8s-cni-overlay NAD as invalid ("net-attach-def not managed by
// OVN"). That false-failed valid secondary networks such as ODF's macvlan
// NADs. Upstream ovn-kubernetes itself treats a non-OVN NAD as a skip, never a
// failure (see pkg/util/multi_network.go's ParseNetConf and its callers) -
// this package mirrors that by only applying OVN rules to OVN NADs.
//
// The OVN tier imports only the lightweight ovn-kubernetes netconf parsing
// packages (pkg/config, pkg/types); the semantic rule set is ported from
// ovn-kubernetes' util.ValidateNetConf so the heavyweight pkg/util dependency
// tree (netlink, nftables, frr-k8s, ...) is avoided.
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

	ovnconfig "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/config"
	ovntypes "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/types"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// ovnCNIType is the CNI plugin type of an OVN-Kubernetes-managed NAD. Only
// NADs declaring this type are subjected to OVN semantic validation.
const ovnCNIType = "ovn-k8s-cni-overlay"

// knownCNITypes are the CNI plugin types shipped/supported on OpenShift for
// additional networks. An unrecognized type is surfaced as a non-gating
// advisory (a likely typo) rather than an error: CNI plugin types are
// open-ended (any binary on the CNI path, including third-party plugins), so
// hard-failing an unlisted type would reintroduce false positives.
var knownCNITypes = map[string]bool{
	ovnCNIType:    true,
	"macvlan":     true,
	"bridge":      true,
	"ipvlan":      true,
	"host-device": true,
	"tap":         true,
	"vlan":        true,
	"sriov":       true,
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

// ipamProbe extracts an IPAM plugin's type without asserting the rest of its
// (plugin-specific) shape.
type ipamProbe struct {
	Type string `json:"type"`
}

// pluginProbe is one entry of a CNI conflist's plugins array. IPAM is kept raw
// so a plugin-specific IPAM block never breaks decoding.
type pluginProbe struct {
	Type string          `json:"type"`
	IPAM json.RawMessage `json:"ipam"`
}

// cniProbe is the subset of a (stringified) CNI netconf needed to dispatch
// validation by plugin type. It accepts both a single plugin config (top-level
// type/ipam) and a conflist (plugins[]).
type cniProbe struct {
	CNIVersion string          `json:"cniVersion"`
	Type       string          `json:"type"`
	IPAM       json.RawMessage `json:"ipam"`
	Plugins    []pluginProbe   `json:"plugins"`
}

// ipamTypeOf best-effort extracts an ipam.type from a raw IPAM block. A
// non-object or absent block yields "".
func ipamTypeOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var i ipamProbe
	if err := json.Unmarshal(raw, &i); err != nil {
		return ""
	}
	return strings.TrimSpace(i.Type)
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
// tolerating arbitrary whitespace after the colon and optional quoting of the
// value (e.g. `kind:NetworkAttachmentDefinition`, `kind: "NetworkAttachmentDefinition"`).
var nadKindRE = regexp.MustCompile(`(?m)^\s*kind:\s*["']?NetworkAttachmentDefinition["']?\s*$`)

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
		e, w := validateNAD(path, doc)
		errs = append(errs, e...)
		warns = append(warns, w...)
	}
	return errs, warns
}

// validateNAD applies the structural gates common to every NAD and then
// dispatches on the CNI plugin type: OVN NADs get OVN's semantic rules,
// everything else gets advisory-only checks.
func validateNAD(path string, doc nadDoc) (errs, warns []ValidationError) {
	id := doc.Metadata.Namespace + "/" + doc.Metadata.Name
	hardf := func(msg string) {
		errs = append(errs, ValidationError{File: path, Message: fmt.Sprintf("NetworkAttachmentDefinition %s: %s", id, msg)})
	}
	warnf := func(msg string) {
		warns = append(warns, ValidationError{File: path, Message: fmt.Sprintf("NetworkAttachmentDefinition %s: %s", id, msg), Warning: true})
	}

	cfg, err := configString(doc)
	if err != nil {
		hardf(err.Error())
		return errs, warns
	}
	if strings.TrimSpace(cfg) == "" {
		hardf("spec.config is empty")
		return errs, warns
	}

	// spec.config must be valid JSON (a single CNI config object or a
	// conflist). This org/CNI-neutral gate catches malformed configs for every
	// plugin type, not just OVN - previously only OVN NADs were JSON-checked.
	if !json.Valid([]byte(cfg)) {
		hardf("spec.config is not valid JSON")
		return errs, warns
	}
	var probe cniProbe
	if err := json.Unmarshal([]byte(cfg), &probe); err != nil {
		hardf(fmt.Sprintf("spec.config is not a valid CNI configuration: %v", err))
		return errs, warns
	}

	// Resolve the effective plugin type and ipam from either shape (a conflist
	// dispatches on its first plugin, like ovn-kubernetes' own parser).
	cniType := strings.TrimSpace(probe.Type)
	ipam := ipamTypeOf(probe.IPAM)
	if len(probe.Plugins) > 0 {
		cniType = strings.TrimSpace(probe.Plugins[0].Type)
		ipam = ipamTypeOf(probe.Plugins[0].IPAM)
		for i, p := range probe.Plugins {
			if strings.TrimSpace(p.Type) == "" {
				hardf(fmt.Sprintf("spec.config plugins[%d] is missing a CNI \"type\"", i))
				return errs, warns
			}
		}
	}
	if cniType == "" {
		hardf("spec.config is missing a CNI \"type\"")
		return errs, warns
	}

	// OVN-Kubernetes NADs get OVN's semantic validation. Everything else is
	// owned by its CNI plugin; only surface non-gating advisories.
	if cniType == ovnCNIType {
		if e := validateOVNNetConf(path, id, cfg); e != nil {
			errs = append(errs, *e)
		}
		return errs, warns
	}

	if !knownCNITypes[cniType] {
		warnf(fmt.Sprintf("unrecognized CNI type %q (typo? not gating)", cniType))
	}
	if ipam != "" && !knownIPAMTypes[ipam] {
		warnf(fmt.Sprintf("unrecognized IPAM type %q (typo? not gating)", ipam))
	}
	if strings.TrimSpace(probe.CNIVersion) == "" {
		warnf("spec.config is missing the recommended \"cniVersion\" field (not gating)")
	}
	return errs, warns
}

// validateOVNNetConf parses an ovn-k8s-cni-overlay NAD's spec.config and
// applies OVN-Kubernetes' semantic rules. The rule set is ported from
// ovn-kubernetes/go-controller/pkg/util.ValidateNetConf (keep in sync when the
// pinned ovn-kubernetes version changes). Runtime-only checks that depend on
// live cluster state (uplink gateway mode, dynamic transit-subnet defaulting)
// are intentionally omitted as they are not statically knowable.
func validateOVNNetConf(path, id, cfg string) *ValidationError {
	// Every failure is scoped to this NAD's namespace/name so a finding stays
	// unambiguous when a rendered file carries multiple NAD documents (the
	// common case). Individual messages therefore omit the id themselves.
	fail := func(msg string) *ValidationError {
		return &ValidationError{File: path, Message: fmt.Sprintf("NetworkAttachmentDefinition %s: %s", id, msg)}
	}

	netconf, err := ovnconfig.ParseNetConf([]byte(cfg))
	if err != nil {
		return fail(fmt.Sprintf("invalid OVN netconf: %v", err))
	}

	if netconf.Name != ovntypes.DefaultNetworkName && netconf.NADName != id {
		return fail(fmt.Sprintf("netAttachDefName in config (%s) does not match the NAD's namespace/name", netconf.NADName))
	}

	if err := ovnconfig.ValidateNetConfNameFields(netconf); err != nil {
		return fail(err.Error())
	}

	if netconf.AllowPersistentIPs && netconf.Topology == ovntypes.Layer3Topology {
		return fail("layer3 topology does not allow persistent IPs")
	}

	if netconf.Role != "" && netconf.Role != ovntypes.NetworkRoleSecondary && netconf.Topology == ovntypes.LocalnetTopology {
		return fail(fmt.Sprintf("unexpected network field \"role\" %s for \"localnet\" topology, "+
			"localnet topology does not allow network roles to be set since its always a secondary network", netconf.Role))
	}

	if netconf.Role != "" && netconf.Role != ovntypes.NetworkRolePrimary && netconf.Role != ovntypes.NetworkRoleSecondary {
		return fail(fmt.Sprintf("invalid network role value %s", netconf.Role))
	}

	if netconf.IPAM.Type != "" {
		return fail("unsupported ipam key")
	}

	if netconf.Transport != "" &&
		netconf.Transport != ovntypes.NetworkTransportNoOverlay &&
		netconf.Transport != ovntypes.NetworkTransportEVPN {
		return fail(fmt.Sprintf("invalid transport %q: must be one of %q", netconf.Transport,
			[]string{ovntypes.NetworkTransportNoOverlay, ovntypes.NetworkTransportEVPN}))
	}

	if netconf.OutboundSNAT != "" {
		if netconf.Transport != ovntypes.NetworkTransportNoOverlay {
			return fail(fmt.Sprintf("outboundSNAT is only valid when transport is %q", ovntypes.NetworkTransportNoOverlay))
		}
		if netconf.OutboundSNAT != ovntypes.NoOverlaySNATEnabled &&
			netconf.OutboundSNAT != ovntypes.NoOverlaySNATDisabled {
			return fail(fmt.Sprintf("invalid outboundSNAT %q: must be one of %q", netconf.OutboundSNAT,
				[]string{ovntypes.NoOverlaySNATEnabled, ovntypes.NoOverlaySNATDisabled}))
		}
	}

	if netconf.JoinSubnet != "" && netconf.Topology == ovntypes.LocalnetTopology {
		return fail("localnet topology does not allow specifying join-subnet as services are not supported")
	}

	if netconf.Role == ovntypes.NetworkRolePrimary && netconf.Subnets == "" && netconf.Topology == ovntypes.Layer2Topology {
		return fail("the subnet attribute must be defined for layer2 primary user defined networks")
	}

	if netconf.InfrastructureSubnets != "" && netconf.Topology != ovntypes.Layer2Topology {
		return fail("infrastructureSubnets is only supported for layer2 topology")
	}

	if netconf.ReservedSubnets != "" && netconf.Topology != ovntypes.Layer2Topology {
		return fail("reservedSubnets is only supported for layer2 topology")
	}

	if netconf.DefaultGatewayIPs != "" && netconf.Topology != ovntypes.Layer2Topology {
		return fail("defaultGatewayIPs is only supported for layer2 topology")
	}

	return nil
}
