// Package nad provides NetworkAttachmentDefinition (NAD) validation.
//
// It offers two tiers of validation:
//
//   - Structural (default): lightweight YAML checks — the resource is a
//     NetworkAttachmentDefinition and its spec.config field is present and
//     non-empty. These checks are org/CNI-neutral.
//   - OVN-aware (opt-in via assumeOpenshift): parses spec.config as an
//     OVN-Kubernetes CNI netconf and applies OVN's semantic rules (topology,
//     role, subnet, and transport constraints). This assumes NADs target the
//     OVN-Kubernetes CNI, which is the CNI on OpenShift clusters - the same
//     assumption already made by Options.AssumeOpenShift for the
//     sync-options check (see pkg/validator/syncopts.AssumeOpenShift), so
//     this tier reuses that flag rather than introducing a second one.
//
// The OVN-aware tier imports only the lightweight ovn-kubernetes netconf
// parsing packages (pkg/config, pkg/types); the semantic rule set is
// ported from ovn-kubernetes' util.ValidateNetConf so the heavyweight
// pkg/util dependency tree (netlink, nftables, frr-k8s, ...) is avoided.
package nad

import (
	"bufio"
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

// ValidationError represents a NAD validation failure.
type ValidationError struct {
	File    string
	Message string
}

func (e ValidationError) String() string { return fmt.Sprintf("%s: %s", e.File, e.Message) }

// nadDoc is the minimal shape of a NetworkAttachmentDefinition needed for
// validation. Decoding into this avoids depending on the typed NAD client.
//
// Spec.Config is captured as json.RawMessage rather than a string so that
// decoding never fails on non-NAD documents that legitimately carry an object
// under spec.config (for example an OLM Subscription's spec.config). The
// OVN-aware validator decodes every document in a rendered overlay file
// through nadDoc before checking Kind, so a plain string field here would
// abort validation of the whole file - including the actual NAD - the moment
// a sibling document's spec.config happened to be an object. The kind is
// checked before the config is interpreted; only genuine NAD documents have
// their config asserted to be a string (see configString).
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

// configString asserts that the NAD's spec.config is a JSON string and
// returns its decoded value. A NetworkAttachmentDefinition's spec.config must
// be a stringified CNI netconf; anything else is a malformed NAD.
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

// ValidateFiles validates all NAD YAML files in the given list. When
// assumeOpenshift is true, OVN-aware netconf validation is applied in
// addition to the always-on structural checks.
func ValidateFiles(files []string, assumeOpenshift bool) []ValidationError {
	var errs []ValidationError
	for _, f := range files {
		if !IsNADFile(f) {
			continue
		}
		if e := validateNADFile(f, assumeOpenshift); len(e) > 0 {
			errs = append(errs, e...)
		}
	}
	return errs
}

// ValidateDir validates all NAD files in a directory tree.
func ValidateDir(dir string, assumeOpenshift bool) []ValidationError {
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
		return []ValidationError{{File: dir, Message: fmt.Sprintf("walking directory: %v", err)}}
	}
	return ValidateFiles(files, assumeOpenshift)
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

// validateNADFile dispatches to the OVN-aware or structural validator.
func validateNADFile(path string, assumeOpenshift bool) []ValidationError {
	if assumeOpenshift {
		return validateNADFileOVN(path)
	}
	return validateNADFileStructural(path)
}

// validateNADFileOVN parses every NetworkAttachmentDefinition document in the
// file as an OVN-Kubernetes netconf and applies OVN's semantic validation.
func validateNADFileOVN(path string) []ValidationError {
	data, err := os.ReadFile(path)
	if err != nil {
		return []ValidationError{{File: path, Message: fmt.Sprintf("read error: %v", err)}}
	}

	var errs []ValidationError
	dec := kyaml.NewYAMLToJSONDecoder(bytes.NewReader(data))
	for {
		var doc nadDoc
		if decErr := dec.Decode(&doc); decErr != nil {
			if errors.Is(decErr, io.EOF) {
				break
			}
			errs = append(errs, ValidationError{File: path, Message: fmt.Sprintf("decoding YAML: %v", decErr)})
			return errs
		}
		if doc.Kind != "NetworkAttachmentDefinition" {
			continue
		}
		if e := validateNetConf(path, doc); e != nil {
			errs = append(errs, *e)
		}
	}
	return errs
}

// validateNetConf parses the NAD's spec.config as an OVN netconf and applies
// OVN-Kubernetes' semantic rules. The rule set is ported from
// ovn-kubernetes/go-controller/pkg/util.ValidateNetConf (keep in sync when the
// pinned ovn-kubernetes version changes). Runtime-only checks that depend on
// live cluster state (uplink gateway mode, dynamic transit-subnet defaulting)
// are intentionally omitted as they are not statically knowable.
func validateNetConf(path string, doc nadDoc) *ValidationError {
	fail := func(msg string) *ValidationError {
		return &ValidationError{File: path, Message: msg}
	}

	cfg, err := configString(doc)
	if err != nil {
		return fail(fmt.Sprintf("invalid NetworkAttachmentDefinition %s/%s: %v", doc.Metadata.Namespace, doc.Metadata.Name, err))
	}

	netconf, err := ovnconfig.ParseNetConf([]byte(cfg))
	if err != nil {
		return fail(fmt.Sprintf("invalid NetworkAttachmentDefinition %s/%s: %v", doc.Metadata.Namespace, doc.Metadata.Name, err))
	}

	nadName := fmt.Sprintf("%s/%s", doc.Metadata.Namespace, doc.Metadata.Name)
	if netconf.Name != ovntypes.DefaultNetworkName && netconf.NADName != nadName {
		return fail(fmt.Sprintf("net-attach-def name (%s) is inconsistent with config (%s)", nadName, netconf.NADName))
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
		return fail(fmt.Sprintf("error parsing Network Attachment Definition %s: unsupported ipam key", nadName))
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

// validateNADFileStructural performs lightweight structural checks: the
// spec.config field must be present and non-empty. This tier is CNI-neutral.
func validateNADFileStructural(path string) []ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return []ValidationError{{File: path, Message: fmt.Sprintf("read error: %v", err)}}
	}
	defer f.Close() //nolint:errcheck // Best-effort close on read-only file

	var errs []ValidationError
	scanner := bufio.NewScanner(f)
	hasConfig := false
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "config:") {
			hasConfig = true
			value := strings.TrimSpace(strings.TrimPrefix(line, "config:"))
			if value == "" || value == "''" || value == `""` {
				errs = append(errs, ValidationError{
					File:    path,
					Message: fmt.Sprintf("line %d: spec.config is empty", lineNum),
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, ValidationError{
			File:    path,
			Message: fmt.Sprintf("scan error: %v", err),
		})
	}

	if !hasConfig {
		errs = append(errs, ValidationError{
			File:    path,
			Message: "spec.config field not found in NetworkAttachmentDefinition",
		})
	}

	return errs
}
