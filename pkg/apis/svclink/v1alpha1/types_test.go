package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	api "k8s.io/kubernetes/pkg/apis/core"
)

func TestClusterLinkSpec_ShouldExcludeNamespace(t *testing.T) {
	tests := []struct {
		name             string
		spec             ClusterLinkSpec
		namespace        string
		expectedExcluded bool
		description      string
	}{
		{
			name: "exclude kube-system namespace",
			spec: ClusterLinkSpec{
				ExcludedNamespaces: []string{},
			},
			namespace:        api.NamespaceSystem,
			expectedExcluded: true,
			description:      "kube-system should always be excluded by default",
		},
		{
			name: "exclude explicitly listed namespace",
			spec: ClusterLinkSpec{
				ExcludedNamespaces: []string{"test-exclude"},
			},
			namespace:        "test-exclude",
			expectedExcluded: true,
			description:      "namespace in ExcludedNamespaces should be excluded",
		},
		{
			name: "include namespace not in any list when no inclusion list",
			spec: ClusterLinkSpec{
				ExcludedNamespaces: []string{"other"},
			},
			namespace:        "default",
			expectedExcluded: false,
			description:      "namespace should be included when not excluded and no inclusion list",
		},
		{
			name: "exclude namespace not in inclusion list",
			spec: ClusterLinkSpec{
				IncludedNamespaces: []string{"production", "staging"},
			},
			namespace:        "default",
			expectedExcluded: true,
			description:      "namespace not in IncludedNamespaces should be excluded when inclusion list is specified",
		},
		{
			name: "include namespace in inclusion list",
			spec: ClusterLinkSpec{
				IncludedNamespaces: []string{"production", "staging"},
			},
			namespace:        "production",
			expectedExcluded: false,
			description:      "namespace in IncludedNamespaces should be included",
		},
		{
			name: "exclude kube-system even when in inclusion list",
			spec: ClusterLinkSpec{
				IncludedNamespaces: []string{api.NamespaceSystem, "production"},
			},
			namespace:        api.NamespaceSystem,
			expectedExcluded: true,
			description:      "kube-system should be excluded even if explicitly included",
		},
		{
			name: "exclude namespace in both exclusion and inclusion lists",
			spec: ClusterLinkSpec{
				ExcludedNamespaces: []string{"test"},
				IncludedNamespaces: []string{"test", "production"},
			},
			namespace:        "test",
			expectedExcluded: true,
			description:      "exclusion takes precedence over inclusion",
		},
		{
			name: "multiple excluded namespaces",
			spec: ClusterLinkSpec{
				ExcludedNamespaces: []string{"ns1", "ns2", "ns3"},
			},
			namespace:        "ns2",
			expectedExcluded: true,
			description:      "should handle multiple excluded namespaces",
		},
		{
			name: "empty namespace string",
			spec: ClusterLinkSpec{
				IncludedNamespaces: []string{"production"},
			},
			namespace:        "",
			expectedExcluded: true,
			description:      "empty namespace should be excluded when inclusion list is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			excludedNS := tt.spec.ToExcludedNamespaceSet()
			includedNS := tt.spec.ToIncludedNamespaceSet()

			result := tt.spec.ShouldExcludeNamespace(tt.namespace, &excludedNS, &includedNS)

			if result != tt.expectedExcluded {
				t.Errorf("%s: expected excluded=%v, got excluded=%v", tt.description, tt.expectedExcluded, result)
			}
		})
	}
}

func TestClusterLinkSpec_ShouldExcludeService(t *testing.T) {
	tests := []struct {
		name             string
		spec             ClusterLinkSpec
		namespace        string
		serviceName      string
		expectedExcluded bool
		description      string
	}{
		{
			name:             "exclude kubernetes service by default",
			spec:             ClusterLinkSpec{},
			namespace:        "default",
			serviceName:      "kubernetes",
			expectedExcluded: true,
			description:      "kubernetes service should always be excluded by default",
		},
		{
			name: "exclude service by namespace/name combination",
			spec: ClusterLinkSpec{
				ExcludedServices: []string{"default/internal-db", "production/admin-api"},
			},
			namespace:        "default",
			serviceName:      "internal-db",
			expectedExcluded: true,
			description:      "service should be excluded when namespace/name is in ExcludedServices",
		},
		{
			name: "include service not in exclusion lists",
			spec: ClusterLinkSpec{
				ExcludedServices: []string{"default/other"},
			},
			namespace:        "default",
			serviceName:      "web-service",
			expectedExcluded: false,
			description:      "service should be included when not in any exclusion list",
		},
		{
			name: "exclude service by global name exclusion",
			spec: ClusterLinkSpec{
				ExcludedServiceNames: []string{"admin-service", "debug-tool"},
			},
			namespace:        "production",
			serviceName:      "admin-service",
			expectedExcluded: true,
			description:      "service should be excluded when name is in ExcludedServiceNames",
		},
		{
			name: "exclude service by global name in different namespace",
			spec: ClusterLinkSpec{
				ExcludedServiceNames: []string{"internal-cache"},
			},
			namespace:        "staging",
			serviceName:      "internal-cache",
			expectedExcluded: true,
			description:      "service should be excluded by name across all namespaces",
		},
		{
			name: "exclude same service name but different namespace/name combination",
			spec: ClusterLinkSpec{
				ExcludedServices: []string{"default/web-service"},
			},
			namespace:        "production",
			serviceName:      "web-service",
			expectedExcluded: false,
			description:      "service with same name in different namespace should not be excluded",
		},
		{
			name: "exclude service in both lists",
			spec: ClusterLinkSpec{
				ExcludedServices:     []string{"default/api"},
				ExcludedServiceNames: []string{"api"},
			},
			namespace:        "default",
			serviceName:      "api",
			expectedExcluded: true,
			description:      "service should be excluded if in either list",
		},
		{
			name: "multiple excluded services",
			spec: ClusterLinkSpec{
				ExcludedServices: []string{"ns1/svc1", "ns2/svc2", "ns3/svc3"},
			},
			namespace:        "ns2",
			serviceName:      "svc2",
			expectedExcluded: true,
			description:      "should handle multiple excluded services",
		},
		{
			name: "multiple excluded service names",
			spec: ClusterLinkSpec{
				ExcludedServiceNames: []string{"svc1", "svc2", "svc3"},
			},
			namespace:        "any-namespace",
			serviceName:      "svc2",
			expectedExcluded: true,
			description:      "should handle multiple excluded service names",
		},
		{
			name: "case sensitive service name matching",
			spec: ClusterLinkSpec{
				ExcludedServiceNames: []string{"MyService"},
			},
			namespace:        "default",
			serviceName:      "myservice",
			expectedExcluded: false,
			description:      "service name matching should be case sensitive",
		},
		{
			name: "empty service name",
			spec: ClusterLinkSpec{
				ExcludedServiceNames: []string{"test"},
			},
			namespace:        "default",
			serviceName:      "",
			expectedExcluded: false,
			description:      "empty service name should not match exclusions",
		},
		{
			name:             "kubernetes service in kube-system namespace",
			spec:             ClusterLinkSpec{},
			namespace:        api.NamespaceSystem,
			serviceName:      "kubernetes",
			expectedExcluded: true,
			description:      "kubernetes service in kube-system should be excluded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			excludedSvcSet := tt.spec.ToExcludedServiceSet()
			excludedSvcNameSet := tt.spec.ToExcludedServiceNameSet()

			result := tt.spec.ShouldExcludeService(tt.namespace, tt.serviceName, &excludedSvcSet, &excludedSvcNameSet)

			if result != tt.expectedExcluded {
				t.Errorf("%s: expected excluded=%v, got excluded=%v", tt.description, tt.expectedExcluded, result)
			}
		})
	}
}

func TestClusterLinkSpec_ToExcludedNamespaceSet(t *testing.T) {
	tests := []struct {
		name               string
		spec               ClusterLinkSpec
		expectedNamespaces []string
	}{
		{
			name: "always includes kube-system",
			spec: ClusterLinkSpec{
				ExcludedNamespaces: []string{"test"},
			},
			expectedNamespaces: []string{api.NamespaceSystem, "test"},
		},
		{
			name: "empty exclusion list only has kube-system",
			spec: ClusterLinkSpec{
				ExcludedNamespaces: []string{},
			},
			expectedNamespaces: []string{api.NamespaceSystem},
		},
		{
			name: "does not duplicate kube-system",
			spec: ClusterLinkSpec{
				ExcludedNamespaces: []string{api.NamespaceSystem, "test"},
			},
			expectedNamespaces: []string{api.NamespaceSystem, "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.ToExcludedNamespaceSet()

			expected := sets.New(tt.expectedNamespaces...)
			if !result.Equal(expected) {
				t.Errorf("expected set %v, got %v", expected, result)
			}
		})
	}
}

func TestClusterLinkSpec_ToIncludedNamespaceSet(t *testing.T) {
	tests := []struct {
		name               string
		spec               ClusterLinkSpec
		expectedNamespaces []string
	}{
		{
			name: "empty included namespaces",
			spec: ClusterLinkSpec{
				IncludedNamespaces: []string{},
			},
			expectedNamespaces: []string{},
		},
		{
			name: "multiple included namespaces",
			spec: ClusterLinkSpec{
				IncludedNamespaces: []string{"production", "staging", "dev"},
			},
			expectedNamespaces: []string{"production", "staging", "dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.ToIncludedNamespaceSet()

			expected := sets.New(tt.expectedNamespaces...)
			if !result.Equal(expected) {
				t.Errorf("expected set %v, got %v", expected, result)
			}
		})
	}
}

func TestClusterLinkSpec_ToExcludedServiceSet(t *testing.T) {
	tests := []struct {
		name             string
		spec             ClusterLinkSpec
		expectedServices []string
	}{
		{
			name: "empty excluded services",
			spec: ClusterLinkSpec{
				ExcludedServices: []string{},
			},
			expectedServices: []string{},
		},
		{
			name: "multiple excluded services",
			spec: ClusterLinkSpec{
				ExcludedServices: []string{"default/svc1", "production/svc2"},
			},
			expectedServices: []string{"default/svc1", "production/svc2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.ToExcludedServiceSet()

			expected := sets.New(tt.expectedServices...)
			if !result.Equal(expected) {
				t.Errorf("expected set %v, got %v", expected, result)
			}
		})
	}
}

func TestClusterLinkSpec_ToExcludedServiceNameSet(t *testing.T) {
	tests := []struct {
		name                 string
		spec                 ClusterLinkSpec
		expectedServiceNames []string
	}{
		{
			name: "always includes kubernetes service",
			spec: ClusterLinkSpec{
				ExcludedServiceNames: []string{"admin"},
			},
			expectedServiceNames: []string{"kubernetes", "admin"},
		},
		{
			name: "empty exclusion list only has kubernetes",
			spec: ClusterLinkSpec{
				ExcludedServiceNames: []string{},
			},
			expectedServiceNames: []string{"kubernetes"},
		},
		{
			name: "does not duplicate kubernetes",
			spec: ClusterLinkSpec{
				ExcludedServiceNames: []string{"kubernetes", "admin"},
			},
			expectedServiceNames: []string{"kubernetes", "admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.ToExcludedServiceNameSet()

			expected := sets.New(tt.expectedServiceNames...)
			if !result.Equal(expected) {
				t.Errorf("expected set %v, got %v", expected, result)
			}
		})
	}
}
