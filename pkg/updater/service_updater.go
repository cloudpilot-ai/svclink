package updater

import (
	"context"

	"github.com/cloudpilot-ai/svclink/pkg/config"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apiserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudpilot-ai/svclink/pkg/apis/discoverer"
)

type ServiceUpdater struct {
	ctrlClient client.Client
}

func NewServiceUpdater(ctrlClient client.Client) *ServiceUpdater {
	return &ServiceUpdater{
		ctrlClient: ctrlClient,
	}
}

// SyncServicesToLocalCluster ensures that services existing in remote clusters are created in the local cluster.
func (su *ServiceUpdater) SyncServicesToLocalCluster(ctx context.Context, services map[string]*discoverer.ServiceInfo) error {
	namespaceServiceMap := su.groupServicesByNamespace(services)

	for ns, serviceNames := range namespaceServiceMap {
		var namespace corev1.Namespace
		if err := su.ctrlClient.Get(ctx, client.ObjectKey{Name: ns}, &namespace); err != nil {
			if !apiserrors.IsNotFound(err) {
				return err
			}

			if err := su.ctrlClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}); err != nil {
				return err
			}

			klog.Infof("Created namespace %s as it does not exist in local cluster", ns)
		}

		existingServices, err := su.getExistingServiceNames(ctx, ns)
		if err != nil {
			return err
		}

		for _, name := range serviceNames {
			if _, exists := existingServices[name]; exists {
				continue
			}

			serviceInfo := services[ns+"/"+name]
			if serviceInfo == nil {
				continue
			}

			if err := su.createMissingService(ctx, ns, name, serviceInfo); err != nil {
				return err
			}
		}
	}

	return nil
}

// groupServicesByNamespace organizes services by namespace.
func (su *ServiceUpdater) groupServicesByNamespace(services map[string]*discoverer.ServiceInfo) map[string][]string {
	namespaceServiceMap := make(map[string][]string)
	for _, serviceInfo := range services {
		if serviceInfo == nil {
			continue
		}

		if _, exists := namespaceServiceMap[serviceInfo.Namespace]; !exists {
			namespaceServiceMap[serviceInfo.Namespace] = []string{}
		}
		namespaceServiceMap[serviceInfo.Namespace] = append(namespaceServiceMap[serviceInfo.Namespace], serviceInfo.Name)
	}
	return namespaceServiceMap
}

// getExistingServiceNames retrieves the names of existing services in the specified namespace.
func (su *ServiceUpdater) getExistingServiceNames(ctx context.Context, namespace string) (map[string]struct{}, error) {
	svcList := &corev1.ServiceList{}
	if err := su.ctrlClient.List(ctx, svcList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	return lo.SliceToMap(svcList.Items, func(svc corev1.Service) (string, struct{}) {
		return svc.Name, struct{}{}
	}), nil
}

// createMissingService creates a service in the local cluster if it doesn't exist.
func (su *ServiceUpdater) createMissingService(ctx context.Context, namespace, name string, serviceInfo *discoverer.ServiceInfo) error {
	if serviceInfo.Service == nil {
		return nil
	}

	annotations := serviceInfo.Service.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[config.SyncAnnotation] = "true"

	newSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      serviceInfo.Service.Labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports:    serviceInfo.Service.Spec.Ports,
			Selector: serviceInfo.Service.Spec.Selector,
		},
	}

	if err := su.ctrlClient.Create(ctx, newSvc); err != nil {
		return err
	}
	klog.Infof("Created service %s/%s as it exists in remote clusters", namespace, name)
	return nil
}
