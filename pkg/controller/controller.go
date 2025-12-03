// Package controller implements the main orchestration logic for svclink.
// It coordinates service discovery, endpoint aggregation, and EndpointSlice updates
// across multiple Kubernetes clusters in a continuous reconciliation loop.
package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilserrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudpilot-ai/svclink/pkg/aggregator"
	apisdiscoverer "github.com/cloudpilot-ai/svclink/pkg/apis/discoverer"
	svclinkv1alpha1 "github.com/cloudpilot-ai/svclink/pkg/apis/svclink/v1alpha1"
	"github.com/cloudpilot-ai/svclink/pkg/clusterlink"
	"github.com/cloudpilot-ai/svclink/pkg/config"
	"github.com/cloudpilot-ai/svclink/pkg/discoverer"
	"github.com/cloudpilot-ai/svclink/pkg/updater"
)

// Controller is the main svclink controller
type Controller struct {
	ctrlClient client.Client

	cfg               *config.Config
	manager           ctrl.Manager
	serviceDiscoverer *discoverer.ServiceDiscoverer
	aggregator        *aggregator.EndpointAggregator
	sliceUpdater      *updater.SliceUpdater
	serviceUpdater    *updater.ServiceUpdater
}

// newScheme creates and registers all required schemes
func newScheme() (*runtime.Scheme, error) {
	runtimeScheme := runtime.NewScheme()

	// Add Kubernetes core types (includes core/v1, apps/v1, discovery/v1, etc.)
	if err := scheme.AddToScheme(runtimeScheme); err != nil {
		return nil, fmt.Errorf("failed to add core scheme: %w", err)
	}

	// Add our custom types (ClusterLink CRD)
	if err := svclinkv1alpha1.AddToScheme(runtimeScheme); err != nil {
		return nil, fmt.Errorf("failed to add svclink scheme: %w", err)
	}

	return runtimeScheme, nil
}

// NewController creates a new Controller with controller-runtime Manager
func NewController(cfg *config.Config, restConfig *rest.Config) (*Controller, error) {
	// Create and register schemes
	runtimeScheme, err := newScheme()
	if err != nil {
		return nil, err
	}

	// Create controller-runtime manager
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: runtimeScheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	serviceDiscoverer := discoverer.NewServiceDiscoverer(mgr.GetClient())
	aggregator := aggregator.NewEndpointAggregator(mgr.GetClient())
	sliceUpdater := updater.NewSliceUpdater(mgr.GetClient())
	serviceUpdater := updater.NewServiceUpdater(mgr.GetClient())

	return &Controller{
		ctrlClient: mgr.GetClient(),

		cfg:               cfg,
		manager:           mgr,
		serviceDiscoverer: serviceDiscoverer,
		aggregator:        aggregator,
		sliceUpdater:      sliceUpdater,
		serviceUpdater:    serviceUpdater,
	}, nil
}

// Run starts the controller
func (c *Controller) Run(ctx context.Context) error {
	klog.Info("Starting svclink controller")

	// Start the controller-runtime manager (handles ClusterLink events)
	go func() {
		klog.Info("Starting controller-runtime manager")
		if err := c.manager.Start(ctx); err != nil {
			klog.Fatalf("Failed to start manager: %v", err)
		}
	}()

	// Wait for manager cache to sync
	if !c.manager.GetCache().WaitForCacheSync(ctx) {
		return fmt.Errorf("failed to sync manager cache")
	}
	klog.Info("Manager cache synced")

	// Start sync loop for service synchronization
	go c.syncLoop(ctx)

	<-ctx.Done()
	klog.Info("Shutting down svclink controller")
	return nil
}

// syncLoop runs the sync process periodically
func (c *Controller) syncLoop(ctx context.Context) {
	// Run sync immediately and then periodically
	wait.UntilWithContext(ctx, c.sync, c.cfg.SyncInterval)
}

// sync performs one sync cycle
func (c *Controller) sync(ctx context.Context) {
	klog.Info("Starting sync cycle")

	clusterInfos, err := clusterlink.ListClusterInfo(ctx, c.ctrlClient)
	if err != nil {
		klog.Errorf("Failed to list cluster info: %v", err)
		return
	}

	// Discover which remote clusters have these services
	klog.Info("Discovering services across clusters")
	services, err := c.serviceDiscoverer.DiscoverServices(ctx, clusterInfos, c.cfg.IncludedNamespaces)
	if err != nil {
		klog.Errorf("Failed to discover services: %v", err)
		return
	}

	if c.cfg.SyncServicesToLocalCluster {
		klog.Info("Syncing services to local cluster")
		if err := c.serviceUpdater.SyncServicesToLocalCluster(ctx, services); err != nil {
			klog.Errorf("Failed to update services in local cluster: %v", err)
			return
		}
	} else {
		filteredServices, err := c.filterServicesExistingInLocalCluster(ctx, c.cfg.IncludedNamespaces, services)
		if err != nil {
			klog.Errorf("Failed to filter services: %v", err)
			return
		}
		services = filteredServices
	}

	// For each service, aggregate endpoints and update EndpointSlices
	klog.Info("Aggregating endpoints and updating EndpointSlices")
	errs := make([]error, 0)
	for key, svcInfo := range services {
		if err := c.syncService(ctx, svcInfo, clusterInfos); err != nil {
			errs = append(errs, fmt.Errorf("failed to sync service %s: %v", key, err))
		}
	}

	if len(errs) > 0 {
		klog.Errorf("Sync cycle completed with errors: %v", utilserrors.NewAggregate(errs))
		return
	}

	klog.Infof("Sync cycle completed, processed %d services", len(services))
}

// syncService syncs a single service
func (c *Controller) syncService(ctx context.Context, svcInfo *apisdiscoverer.ServiceInfo, clusterInfos map[string]*clusterlink.ClusterInfo) error {
	klog.V(4).Infof("Syncing service %s/%s from clusters: %v",
		svcInfo.Namespace, svcInfo.Name, svcInfo.Clusters)

	// Aggregate endpoints from all clusters
	clusterEndpoints, err := c.aggregator.AggregateEndpoints(
		ctx,
		svcInfo.Namespace,
		svcInfo.Name,
		svcInfo.Clusters,
		clusterInfos,
	)
	if err != nil {
		return err
	}

	// Update EndpointSlices
	if err := c.sliceUpdater.UpdateEndpointSlices(
		ctx,
		svcInfo.Namespace,
		svcInfo.Name,
		clusterEndpoints,
	); err != nil {
		return err
	}

	return nil
}

// filterServicesExistingInLocalCluster filters the services map to only include services
// that exist in the local cluster. This ensures EndpointSlices are only created for
// services that have a corresponding Service object in the local cluster.
func (c *Controller) filterServicesExistingInLocalCluster(ctx context.Context, includedNamespaces []string, services map[string]*apisdiscoverer.ServiceInfo) (map[string]*apisdiscoverer.ServiceInfo, error) {
	var svcList corev1.ServiceList
	if err := c.ctrlClient.List(ctx, &svcList); err != nil {
		return nil, err
	}

	// Build a set of local services for efficient lookup
	localServices := make(map[string]struct{})
	includedNSSet := sets.New(includedNamespaces...)

	for _, svc := range svcList.Items {
		// Check if the service is in an included namespace
		if includedNSSet.Len() > 0 && !includedNSSet.Has(svc.Namespace) {
			continue
		}
		key := svc.Namespace + "/" + svc.Name
		localServices[key] = struct{}{}
	}

	// Filter services to only include those that exist locally
	filtered := make(map[string]*apisdiscoverer.ServiceInfo)
	for key, svcInfo := range services {
		if _, existsLocally := localServices[key]; existsLocally {
			filtered[key] = svcInfo
		}
	}

	return filtered, nil
}
