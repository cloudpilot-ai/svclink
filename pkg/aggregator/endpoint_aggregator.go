// Package aggregator collects endpoint information from multiple Kubernetes clusters.
// It aggregates Pod endpoints from EndpointSlices across all clusters where a service exists,
// organizing them by cluster for synchronized endpoint distribution.
package aggregator

import (
	"context"
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudpilot-ai/svclink/pkg/clusterlink"
	"github.com/cloudpilot-ai/svclink/pkg/config"
)

// EndpointAggregator aggregates endpoints from multiple clusters
type EndpointAggregator struct {
	kubeClient client.Client
}

// NewEndpointAggregator creates a new EndpointAggregator
func NewEndpointAggregator(kubeClient client.Client) *EndpointAggregator {
	return &EndpointAggregator{
		kubeClient: kubeClient,
	}
}

// ClusterEndpoints represents endpoints from a specific cluster
type ClusterEndpoints struct {
	ClusterName string
	Endpoints   []discoveryv1.Endpoint
	Ports       []discoveryv1.EndpointPort
}

// AggregateEndpoints collects endpoints for a service from all clusters
func (ea *EndpointAggregator) AggregateEndpoints(ctx context.Context, namespace, serviceName string, clusters []string, clusterInfos map[string]*clusterlink.ClusterInfo) ([]ClusterEndpoints, error) {
	var results []ClusterEndpoints

	for _, clusterName := range clusters {
		clusterInfo, ok := clusterInfos[clusterName]
		if !ok {
			klog.V(4).Infof("Cluster %s not found or not enabled, skipping", clusterName)
			continue
		}

		endpoints, ports, err := ea.getEndpointsFromCluster(ctx, clusterInfo.Client, namespace, serviceName)
		if err != nil {
			klog.Warningf("Failed to get endpoints from cluster %s for service %s/%s: %v",
				clusterInfo.Name, namespace, serviceName, err)
			continue
		}

		if len(endpoints) > 0 {
			results = append(results, ClusterEndpoints{
				ClusterName: clusterInfo.Name,
				Endpoints:   endpoints,
				Ports:       ports,
			})
			klog.V(4).Infof("Aggregated %d endpoints from cluster %s for service %s/%s",
				len(endpoints), clusterInfo.Name, namespace, serviceName)
		}
	}

	return results, nil
}

// getEndpointsFromCluster retrieves endpoints from a single cluster
func (ea *EndpointAggregator) getEndpointsFromCluster(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, serviceName string,
) ([]discoveryv1.Endpoint, []discoveryv1.EndpointPort, error) {
	// Get EndpointSlices for the service
	sliceList, err := client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kubernetes.io/service-name=%s", serviceName),
	})
	if err != nil {
		return nil, nil, err
	}

	var allEndpoints []discoveryv1.Endpoint
	var ports []discoveryv1.EndpointPort

	for _, slice := range sliceList.Items {
		// Skip EndpointSlices created by svclink to avoid circular synchronization
		// These slices have the cloudpilot.ai/svclink-cluster label
		if _, isSyncedSlice := slice.Labels[config.ClusterLabel]; isSyncedSlice {
			klog.V(5).Infof("Skipping svclink managed EndpointSlice %s/%s (cluster: %s)",
				slice.Namespace, slice.Name, slice.Labels[config.ClusterLabel])
			continue
		}

		// Collect endpoints from native Kubernetes EndpointSlices only
		allEndpoints = append(allEndpoints, slice.Endpoints...)

		// Use ports from the first slice (they should be the same across slices)
		if len(ports) == 0 && len(slice.Ports) > 0 {
			ports = slice.Ports
		}
	}

	// Filter only ready endpoints
	var readyEndpoints []discoveryv1.Endpoint
	for _, ep := range allEndpoints {
		if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
			readyEndpoints = append(readyEndpoints, ep)
		}
	}

	return readyEndpoints, ports, nil
}
