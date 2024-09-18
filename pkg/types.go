package pkg

// ClusterOutput holds the list of cluster ARNs returned by ECS
type ClusterOutput struct {
	ClusterArns []string `json:"clusterArns"`
}

// ServiceOutput holds the list of service ARNs returned by ECS
type ServiceOutput struct {
	ServiceArns []string `json:"serviceArns"`
}

// ServiceDetails contains details about ECS services, including the cluster they belong to
type ServiceDetails struct {
	Cluster      string `json:"cluster"` // Add Cluster field
	ServiceName  string `json:"serviceName"`
	RunningCount int64  `json:"runningCount"`
	DesiredCount int64  `json:"desiredCount"`
}
