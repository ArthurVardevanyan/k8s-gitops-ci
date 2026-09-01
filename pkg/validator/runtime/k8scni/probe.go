package k8scni

import (
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/libcni"
	"sigs.k8s.io/yaml"
)

// nadKinds is shared by every check in this package: all validate a
// NetworkAttachmentDefinition's spec.config, so the dispatcher never hands
// them a document of any other kind.
var nadKinds = []string{"NetworkAttachmentDefinition"}

// nadDoc is the minimal shape of a NetworkAttachmentDefinition needed by
// this package.
//
// The dispatcher only invokes Run for documents whose kind is
// NetworkAttachmentDefinition (see Check.Kinds/nadKinds), so decoding here
// always targets a genuine NAD - unlike the superseded pkg/validator/nad
// package, which walked every document in a rendered file by hand and had
// to guard against a sibling document (e.g. an OLM Subscription) that
// legitimately carries an object under spec.config.
type nadDoc struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Config json.RawMessage `json:"config"`
	} `json:"spec"`
}

// configString asserts that a NAD's spec.config is a JSON string (its
// required shape per the NetworkAttachmentDefinition CRD: a stringified CNI
// netconf) and returns its decoded value.
//
// Decodes with sigs.k8s.io/yaml (a superset of JSON), not encoding/json:
// this package lives under pkg/validator/runtime, whose structural rule
// (TestRuntimeChecksDoNotDecodeWithEncodingJSON) requires it repo-wide so a
// check can never silently decode-fail on the YAML documents checks
// normally receive. raw here already holds an embedded JSON value (spec.config
// itself, extracted by the caller's own sigs.k8s.io/yaml decode of the outer
// document) rather than a top-level manifest, but the same decoder still
// applies correctly since JSON is valid YAML.
func configString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("spec.config must be a string")
	}
	return s, nil
}

// Probe is the subset of a parsed CNI netconf both checks in this package,
// and pkg/validator/static/cni's advisory (unrecognized-type warnings),
// need: the effective plugin type and IPAM type - resolved from either a
// single-plugin config or a conflist's first plugin, the same dispatch a
// real CNI runtime performs - and the declared cniVersion.
//
// Exported so pkg/validator/nad's advisories share exactly this parsing rather
// than re-implementing it. They did re-implement it once, and the two tiers
// duly judged a different value for the same NAD: a config this parser
// rejected outright was still advised on, by a second parser that had read it
// well enough to call its plugin type unrecognized.
type Probe struct {
	CNIType    string
	IPAMType   string
	CNIVersion string
}

// hasPluginsKey reports whether raw CNI netconf bytes declare a top-level
// "plugins" array - the same test a CNI-compliant parser (including
// ovn-kubernetes's own pkg/config/cni.go ParseNetConf) uses to dispatch
// between a conflist and a single-plugin config, since a conflist and a
// single config are unmarshaled by different libcni entry points.
func hasPluginsKey(cfg []byte) bool {
	var probe struct {
		Plugins json.RawMessage `json:"plugins"`
	}
	return yaml.Unmarshal(cfg, &probe) == nil && probe.Plugins != nil
}

// ProbeConfig parses cfg (a NAD's decoded spec.config) via
// containernetworking/cni's own reference parser rather than reimplementing
// the CNI Specification's config-shape rules by hand. That parser genuinely
// rejects a missing "type" (single config: libcni.NetworkPluginConfFromBytes)
// or an empty/absent "plugins" list, a missing name, or an unset "type" on
// any entry within it (conflist: libcni.ConfListFromBytes, which wraps
// NetworkConfFromBytes) - see the
// k8scni/net-attach-def/config-invalid UpstreamRef in upstream_refs.go for the exact
// functions and citation kind.
func ProbeConfig(cfg string) (Probe, error) {
	raw := []byte(cfg)

	if hasPluginsKey(raw) {
		list, err := libcni.ConfListFromBytes(raw)
		if err != nil {
			return Probe{}, err
		}
		var cniType, ipamType string
		if len(list.Plugins) > 0 && list.Plugins[0].Network != nil {
			cniType = list.Plugins[0].Network.Type
			ipamType = list.Plugins[0].Network.IPAM.Type
		}
		return Probe{CNIType: cniType, IPAMType: ipamType, CNIVersion: list.CNIVersion}, nil
	}

	// NetworkPluginConfFromBytes directly, not the deprecated ConfFromBytes
	// wrapper (which does nothing but call this).
	conf, err := libcni.NetworkPluginConfFromBytes(raw)
	if err != nil {
		return Probe{}, err
	}
	var cniType, ipamType, cniVersion string
	if conf.Network != nil {
		cniType = conf.Network.Type
		ipamType = conf.Network.IPAM.Type
		cniVersion = conf.Network.CNIVersion
	}
	return Probe{CNIType: cniType, IPAMType: ipamType, CNIVersion: cniVersion}, nil
}
