// Package discoverer provides types and interfaces for service discovery across clusters.
// It defines the data structures used by the ServiceDiscoverer component to represent
// services that need to be synchronized between clusters.
package discoverer

import (
	corev1 "k8s.io/api/core/v1"
)

// ServiceInfo represents a service that needs to be synced
type ServiceInfo struct {
	Name      string
	Namespace string
	Clusters  []string        // List of cluster names where this service exists
	Service   *corev1.Service // The service object itself
}
