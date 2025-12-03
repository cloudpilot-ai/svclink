// Package updater manages EndpointSlice resources in the local Kubernetes cluster.
// It creates, updates, and deletes EndpointSlice objects to reflect aggregated
// endpoints from remote clusters, enabling cross-cluster service discovery.
package updater

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudpilot-ai/svclink/pkg/aggregator"
	"github.com/cloudpilot-ai/svclink/pkg/config"
)

// SliceUpdater updates EndpointSlices in the local cluster
type SliceUpdater struct {
	kubeClient client.Client
}

// NewSliceUpdater creates a new SliceUpdater
func NewSliceUpdater(ctrlClient client.Client) *SliceUpdater {
	return &SliceUpdater{
		kubeClient: ctrlClient,
	}
}

// UpdateEndpointSlices creates or updates EndpointSlices for each remote cluster
func (su *SliceUpdater) UpdateEndpointSlices(
	ctx context.Context,
	namespace, serviceName string,
	clusterEndpoints []aggregator.ClusterEndpoints,
) error {
	for _, ce := range clusterEndpoints {
		if err := su.updateSliceForCluster(ctx, namespace, serviceName, ce); err != nil {
			klog.Errorf("Failed to update EndpointSlice for cluster %s, service %s/%s: %v",
				ce.ClusterName, namespace, serviceName, err)
			// Continue with other clusters even if one fails
		}
	}

	// Clean up EndpointSlices for clusters that no longer have endpoints
	if err := su.cleanupOrphanedSlices(ctx, namespace, serviceName, clusterEndpoints); err != nil {
		klog.Errorf("Failed to cleanup orphaned slices for service %s/%s: %v", namespace, serviceName, err)
	}

	return nil
}

// updateSliceForCluster creates or updates an EndpointSlice for a specific cluster
func (su *SliceUpdater) updateSliceForCluster(
	ctx context.Context,
	namespace, serviceName string,
	ce aggregator.ClusterEndpoints,
) error {
	sliceName := fmt.Sprintf("%s-svclink-%s", serviceName, ce.ClusterName)

	// Get the service to set as owner reference
	service := &corev1.Service{}
	serviceKey := client.ObjectKey{Namespace: namespace, Name: serviceName}
	if err := su.kubeClient.Get(ctx, serviceKey, service); err != nil {
		return fmt.Errorf("failed to get service %s/%s: %w", namespace, serviceName, err)
	}

	// Set owner reference to enable garbage collection
	ownerRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Service",
		Name:       service.Name,
		UID:        service.UID,
	}

	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceName,
			Namespace: namespace,
			Labels: map[string]string{
				config.ServiceNameLabel: serviceName,
				config.ClusterLabel:     ce.ClusterName,
				config.ManagedByLabel:   config.ManagedByValue,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   ce.Endpoints,
		Ports:       ce.Ports,
	}

	// Try to get existing slice
	existing := &discoveryv1.EndpointSlice{}
	sliceKey := client.ObjectKey{Namespace: namespace, Name: sliceName}
	if err := su.kubeClient.Get(ctx, sliceKey, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get EndpointSlice: %w", err)
		}
		// Create new slice
		if err = su.kubeClient.Create(ctx, slice); err != nil {
			return fmt.Errorf("failed to create EndpointSlice: %w", err)
		}
		klog.Infof("Created EndpointSlice %s/%s for cluster %s with %d endpoints",
			namespace, sliceName, ce.ClusterName, len(ce.Endpoints))
		return nil
	}

	// Update existing slice
	existing.Endpoints = ce.Endpoints
	existing.Ports = ce.Ports
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[config.ServiceNameLabel] = serviceName
	existing.Labels[config.ClusterLabel] = ce.ClusterName
	existing.Labels[config.ManagedByLabel] = config.ManagedByValue

	if err := su.kubeClient.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update EndpointSlice: %w", err)
	}

	klog.V(4).Infof("Updated EndpointSlice %s/%s for cluster %s with %d endpoints",
		namespace, sliceName, ce.ClusterName, len(ce.Endpoints))
	return nil
}

// cleanupOrphanedSlices removes EndpointSlices for clusters that are no longer active
func (su *SliceUpdater) cleanupOrphanedSlices(
	ctx context.Context,
	namespace, serviceName string,
	activeClusterEndpoints []aggregator.ClusterEndpoints,
) error {
	// Build set of active clusters
	activeClusters := sets.NewString(lo.Map(activeClusterEndpoints, func(ce aggregator.ClusterEndpoints, _ int) string {
		return ce.ClusterName
	})...)

	// List all EndpointSlices for this service with cluster label
	selector := labels.SelectorFromSet(labels.Set{
		config.ServiceNameLabel: serviceName,
	})
	// Add requirement for cluster label existence
	clusterReq, err := labels.NewRequirement(config.ClusterLabel, selection.Exists, nil)
	if err != nil {
		return err
	}

	selector = selector.Add(*clusterReq)

	sliceList := &discoveryv1.EndpointSliceList{}
	if err := su.kubeClient.List(ctx, sliceList, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
	}); err != nil {
		return err
	}

	// Delete slices for inactive clusters
	for _, slice := range sliceList.Items {
		if slice.Labels == nil {
			continue
		}

		clusterName := slice.Labels[config.ClusterLabel]
		if activeClusters.Has(clusterName) {
			continue
		}

		if err := su.kubeClient.Delete(ctx, &slice); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete orphaned EndpointSlice %s/%s: %w",
				namespace, slice.Name, err)
		}
		klog.Infof("Deleted orphaned EndpointSlice %s/%s for cluster %s", namespace, slice.Name, clusterName)
	}

	return nil
}
