// Package config provides configuration types and constants for the svclink controller.
// It defines the controller's configuration structure, cluster configuration,
// and annotation/label constants used for service synchronization.
package config

import "time"

// Config holds the controller runtime configuration
type Config struct {
	// SyncInterval is the interval for periodic sync operations
	SyncInterval time.Duration
	// IncludedNamespaces If specified, only services in these namespaces will be synced.
	IncludedNamespaces []string
	// SyncServicesToLocalCluster indicates whether to sync services from remote clusters to the local cluster
	SyncServicesToLocalCluster bool
}

const (
	// SyncAnnotation is the annotation key to mark services for sync
	SyncAnnotation = "cloudpilot.ai/svclink"
	// ClusterLabel is the label key to identify which cluster an EndpointSlice belongs to
	ClusterLabel = "cloudpilot.ai/svclink-cluster"
	// ServiceNameLabel is the standard Kubernetes label for service name
	ServiceNameLabel = "kubernetes.io/service-name"
	// ManagedByLabel is the standard Kubernetes label for identifying the controller managing the resource
	ManagedByLabel = "endpointslice.kubernetes.io/managed-by"
	// ManagedByValue is the value used in the managed-by label for svclink-created EndpointSlices
	ManagedByValue = "svclink.cloudpilot.ai"
	// DefaultSyncInterval is the default interval for periodic sync operations
	DefaultSyncInterval = 30 * time.Second
)
