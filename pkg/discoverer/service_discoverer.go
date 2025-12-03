// Package discoverer discovers Kubernetes services across multiple clusters.
// By default, all services are synchronized except those in kube-system namespace.
// Services can be controlled using ClusterLink spec:
// - spec.excludedNamespaces: list of namespaces to exclude
// - spec.includedNamespaces: if specified, only sync these namespaces
// - spec.excludedServices: list of services (namespace/name) to exclude
package discoverer

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudpilot-ai/svclink/pkg/apis/discoverer"
	"github.com/cloudpilot-ai/svclink/pkg/clusterlink"
)

// ServiceDiscoverer discovers services across all clusters (excluding kube-system)
type ServiceDiscoverer struct {
	kubeClient client.Client
}

// NewServiceDiscoverer creates a new ServiceDiscoverer
func NewServiceDiscoverer(kubeClient client.Client) *ServiceDiscoverer {
	return &ServiceDiscoverer{
		kubeClient: kubeClient,
	}
}

// DiscoverServices discovers all services across all clusters and returns them
func (sd *ServiceDiscoverer) DiscoverServices(ctx context.Context, clusterInfos map[string]*clusterlink.ClusterInfo, includedNamespaces []string) (map[string]*discoverer.ServiceInfo, error) {
	services := make(map[string]*discoverer.ServiceInfo)
	includedNS := sets.New(includedNamespaces...)

	for clusterName, clusterInfo := range clusterInfos {
		err := sd.discoverInCluster(ctx, clusterName, clusterInfo, services, includedNS)

		// Always update cluster status: either with error or clear error (nil means success)
		clusterlink.UpdateClusterSyncError(ctx, sd.kubeClient, clusterInfo, clusterName, err)

		if err != nil {
			klog.Errorf("Failed to discover services in cluster %s: %v", clusterName, err)
			continue
		}
	}

	klog.Infof("Discovered %d services across %d remote clusters", len(services), len(clusterInfos))
	return services, nil
}

// discoverInCluster discovers services in a single cluster
func (sd *ServiceDiscoverer) discoverInCluster(ctx context.Context, clusterName string,
	clusterInfo *clusterlink.ClusterInfo,
	services map[string]*discoverer.ServiceInfo,
	cfgIncludedNamespaces sets.Set[string],
) error {
	spec := clusterInfo.ClusterLink.Spec

	excludedNS := spec.ToExcludedNamespaceSet()
	includedNS := spec.ToIncludedNamespaceSet()
	excludedSvc := spec.ToExcludedServiceSet()
	excludedSvcName := spec.ToExcludedServiceNameSet()

	nsList, err := clusterInfo.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to list namespaces in cluster %s: %v", clusterName, err)
		return err
	}

	for ni := range nsList.Items {
		namespace := nsList.Items[ni].Name

		if cfgIncludedNamespaces.Len() > 0 && !cfgIncludedNamespaces.Has(namespace) {
			// If includedNamespaces is specified, skip services not in that set
			klog.V(4).Infof("Namespace %s skipped as not in included namespaces", namespace)
			continue
		}

		// Check if namespace should be excluded based on all exclusion/inclusion rules
		if spec.ShouldExcludeNamespace(namespace, &excludedNS, &includedNS) {
			klog.V(4).Infof("Namespace %s excluded from sync in cluster %s",
				namespace, clusterName)
			continue
		}

		svcList, err := clusterInfo.Client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to list services in namespace %s of cluster %s: %v",
				namespace, clusterName, err)
			return err
		}

		for _, svc := range svcList.Items {
			serviceName := svc.Name

			// Check if service should be excluded based on all exclusion/inclusion rules
			if spec.ShouldExcludeService(namespace, serviceName, &excludedSvc, &excludedSvcName) {
				klog.V(4).Infof("Service %s/%s excluded from sync in cluster %s",
					namespace, serviceName, clusterName)
				continue
			}

			// Add or update service info
			key := namespace + "/" + serviceName
			svcInfo, exists := services[key]
			if !exists || svcInfo == nil {
				svcInfo = &discoverer.ServiceInfo{
					Name:      serviceName,
					Namespace: namespace,
					Clusters:  []string{},
				}
				services[key] = svcInfo
			}
			svcInfo.Clusters = append(svcInfo.Clusters, clusterName)
			svcInfo.Service = &svc

			klog.V(4).Infof("Found service %s in cluster %s", key, clusterName)
		}
	}

	return nil
}
