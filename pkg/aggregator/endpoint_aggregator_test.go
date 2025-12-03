package aggregator

import (
	"context"
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/cloudpilot-ai/svclink/pkg/config"
)

// TestGetEndpointsFromCluster_SkipsSyncedSlices verifies that EndpointSlices
// created by svclink (with cloudpilot.ai/svclink-cluster label) are skipped to prevent
// circular synchronization when multiple clusters run svclink.
func TestGetEndpointsFromCluster_SkipsSyncedSlices(t *testing.T) {
	ctx := context.Background()

	// Create test EndpointSlices
	nativeSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-abc123",
			Namespace: "default",
			Labels: map[string]string{
				"kubernetes.io/service-name": "test-service",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.1.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
			},
			{
				Addresses: []string{"10.0.1.2"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name: stringPtr("http"),
				Port: int32Ptr(8080),
			},
		},
	}

	// This slice was created by svclink from another cluster
	syncedSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-cluster-b",
			Namespace: "default",
			Labels: map[string]string{
				"kubernetes.io/service-name": "test-service",
				config.ClusterLabel:          "cluster-b", // This marks it as synced
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.2.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name: stringPtr("http"),
				Port: int32Ptr(8080),
			},
		},
	}

	// Create fake client with both slices
	fakeClient := fake.NewSimpleClientset(nativeSlice, syncedSlice)

	// Create aggregator (no longer needs localClient)
	aggregator := &EndpointAggregator{}

	// Get endpoints
	endpoints, ports, err := aggregator.getEndpointsFromCluster(ctx, fakeClient, "default", "test-service")
	if err != nil {
		t.Fatalf("getEndpointsFromCluster failed: %v", err)
	}

	// Verify only native endpoints are returned (synced slice should be skipped)
	if len(endpoints) != 2 {
		t.Errorf("Expected 2 endpoints (from native slice only), got %d", len(endpoints))
	}

	// Verify endpoints are from the native slice
	expectedAddresses := map[string]bool{"10.0.1.1": true, "10.0.1.2": true}
	for _, ep := range endpoints {
		if len(ep.Addresses) == 0 {
			t.Error("Endpoint has no addresses")
			continue
		}
		addr := ep.Addresses[0]
		if !expectedAddresses[addr] {
			t.Errorf("Unexpected endpoint address: %s (should only have native endpoints)", addr)
		}
	}

	// Verify ports
	if len(ports) != 1 {
		t.Errorf("Expected 1 port, got %d", len(ports))
	}
}

// TestGetEndpointsFromCluster_WithOnlySyncedSlices verifies behavior when
// all EndpointSlices are managed by svclink (edge case).
func TestGetEndpointsFromCluster_WithOnlySyncedSlices(t *testing.T) {
	ctx := context.Background()

	// Only synced slices exist (no native endpoints)
	syncedSlice1 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-cluster-b",
			Namespace: "default",
			Labels: map[string]string{
				"kubernetes.io/service-name": "test-service",
				config.ClusterLabel:          "cluster-b",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.2.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name: stringPtr("http"),
				Port: int32Ptr(8080),
			},
		},
	}

	syncedSlice2 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-cluster-c",
			Namespace: "default",
			Labels: map[string]string{
				"kubernetes.io/service-name": "test-service",
				config.ClusterLabel:          "cluster-c",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.3.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name: stringPtr("http"),
				Port: int32Ptr(8080),
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(syncedSlice1, syncedSlice2)

	aggregator := &EndpointAggregator{}

	// Get endpoints
	endpoints, ports, err := aggregator.getEndpointsFromCluster(ctx, fakeClient, "default", "test-service")
	if err != nil {
		t.Fatalf("getEndpointsFromCluster failed: %v", err)
	}

	// Should return empty (all slices are synced and should be skipped)
	if len(endpoints) != 0 {
		t.Errorf("Expected 0 endpoints (all are synced), got %d", len(endpoints))
	}

	// Ports should also be empty since no native slices were processed
	if len(ports) != 0 {
		t.Errorf("Expected 0 ports (all slices were skipped), got %d", len(ports))
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
