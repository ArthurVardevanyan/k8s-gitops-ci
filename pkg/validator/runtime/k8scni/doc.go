// Package k8scni provides runtime validation of NetworkAttachmentDefinition
// (NAD) manifests.
//
// Two checks live here, both dispatched only for
// NetworkAttachmentDefinition documents (see nadKinds):
//
//   - k8scni/net-attach-def/config-invalid parses spec.config through
//     containernetworking/cni's own reference parser - the same library
//     every CNI-compliant container runtime (containerd, CRI-O) uses to
//     load a plugin config before invoking it - rather than reimplementing
//     the CNI Specification's config-shape rules by hand. This is CNI-
//     plugin-neutral: it fires identically for macvlan, bridge, SR-IOV,
//     ovn-k8s-cni-overlay, or any other declared "type".
//   - k8scni/net-attach-def/ovn-netconf-invalid additionally applies OVN-Kubernetes'
//     semantic rules (topology, role, subnet, and transport constraints) to
//     NADs whose CNI type is ovn-k8s-cni-overlay. It calls
//     ovn-kubernetes's own config.ParseNetConf directly - which itself
//     treats a non-OVN NAD as a no-op skip (config.ErrorAttachDefNotOvnManaged),
//     never a failure - so a valid secondary network owned by a different
//     CNI plugin (e.g. ODF's macvlan NADs) is simply not this check's
//     concern, exactly matching how the OVN-Kubernetes network controller
//     itself behaves.
package k8scni
