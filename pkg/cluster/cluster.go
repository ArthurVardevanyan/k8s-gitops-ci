package cluster

// ClusterProject is a generic, schema-free description of a cluster's identity.
//
//nolint:revive // "cluster.ClusterProject" reads clearly at call sites across packages.
type ClusterProject struct {
	Name             string
	ProjectID        string
	ProjectNumber    string
	NetworkProjectID string
}

// ProjectIndex is a reverse lookup index for cluster identities.
type ProjectIndex struct {
	IDToCluster     map[string]string
	NumberToCluster map[string]string
	SharedProjects  map[string]bool
}

// BuildProjectIndex builds a ProjectIndex from cluster metadata.
func BuildProjectIndex(clusters []ClusterProject) ProjectIndex {
	index := ProjectIndex{
		IDToCluster:     make(map[string]string),
		NumberToCluster: make(map[string]string),
		SharedProjects:  make(map[string]bool),
	}
	for _, c := range clusters {
		name := c.Name
		if c.ProjectID != "" {
			index.IDToCluster[c.ProjectID] = name
		}
		if c.ProjectNumber != "" {
			index.NumberToCluster[c.ProjectNumber] = name
		}
		if c.NetworkProjectID != "" {
			index.SharedProjects[c.NetworkProjectID] = true
		}
	}
	return index
}
