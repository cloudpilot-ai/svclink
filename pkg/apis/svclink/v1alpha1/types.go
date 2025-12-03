package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Included NS",type=string,JSONPath=`.spec.includedNamespaces`,priority=1
// +kubebuilder:printcolumn:name="Excluded NS",type=string,JSONPath=`.spec.excludedNamespaces`,priority=1
// +kubebuilder:printcolumn:name="Excluded Services",type=string,JSONPath=`.spec.excludedServices`,priority=1
// +kubebuilder:printcolumn:name="Excluded Service Names",type=string,JSONPath=`.spec.excludedServiceNames`,priority=1
// +kubebuilder:printcolumn:name="Connected",type=boolean,JSONPath=`.status.connected`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Last Connected",type=date,JSONPath=`.status.lastConnected`

// ClusterLink is a specification for a linked Kubernetes cluster
type ClusterLink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec ClusterLinkSpec `json:"spec"`
	// +optional
	Status ClusterLinkStatus `json:"status,omitempty"`
}

// ClusterLinkSpec defines the desired state of ClusterLink
type ClusterLinkSpec struct {
	// Enabled indicates whether this cluster should be actively synced
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Kubeconfig is the base64 encoded kubeconfig for accessing the remote cluster
	// +required
	Kubeconfig string `json:"kubeconfig"`

	// ExcludedNamespaces is a list of namespaces that should not be synced.
	// Services in these namespaces will be ignored.
	// Note: kube-system is always excluded by default and does not need to be specified here.
	// +optional
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`

	// IncludedNamespaces is a list of namespaces that should be synced.
	// If specified, only services in these namespaces will be synced.
	// If empty, all namespaces except kube-system and ExcludedNamespaces will be synced.
	// Note: kube-system is always excluded even if listed here.
	// +optional
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`

	// ExcludedServices is a list of service names (in format namespace/service-name) that should not be synced.
	// This allows fine-grained control to exclude specific services in specific namespaces.
	// Note: Services in kube-system are always excluded regardless of this setting.
	// Example: ["default/internal-db", "production/admin-api"]
	// +optional
	ExcludedServices []string `json:"excludedServices,omitempty"`

	// ExcludedServiceNames is a list of service names that should not be synced in ALL namespaces.
	// This is more efficient than listing the same service in multiple namespaces in ExcludedServices.
	// Note: The 'kubernetes' service is always excluded by default and does not need to be specified here.
	// Example: ["admin-service", "internal-cache", "debug-tool"]
	// +optional
	ExcludedServiceNames []string `json:"excludedServiceNames,omitempty"`
}

// ClusterLinkStatus defines the observed state of ClusterLink
type ClusterLinkStatus struct {
	// Connected indicates whether the cluster is currently reachable
	// +optional
	Connected bool `json:"connected"`

	// LastConnected is the timestamp of the last successful connection
	// +optional
	LastConnected *metav1.Time `json:"lastConnected,omitempty"`

	// Error contains the last error message if connection failed
	// +optional
	Error string `json:"error,omitempty"`

	// Version is the Kubernetes version of the remote cluster
	// +optional
	Version string `json:"version,omitempty"`

	// Conditions represent the latest available observations of the cluster's state
	// +optional
	Conditions []ClusterLinkCondition `json:"conditions,omitempty"`
}

// ClusterLinkCondition describes the state of a linked cluster
type ClusterLinkCondition struct {
	// Type of condition
	Type ClusterLinkConditionType `json:"type"`

	// Status of the condition (True, False, Unknown)
	Status metav1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a brief reason for the condition's last transition
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about the transition
	// +optional
	Message string `json:"message,omitempty"`
}

// ClusterLinkConditionType defines the type of condition
type ClusterLinkConditionType string

const (
	// ClusterLinkReady indicates the cluster is ready and reachable
	ClusterLinkReady ClusterLinkConditionType = "Ready"

	// ClusterLinkError indicates there's an error with the cluster
	ClusterLinkError ClusterLinkConditionType = "Error"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterLinkList is a list of ClusterLink resources
type ClusterLinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterLink `json:"items"`
}

func (cls *ClusterLinkSpec) ToExcludedNamespaceSet() sets.Set[string] {
	excludedNS := sets.New(cls.ExcludedNamespaces...)
	excludedNS.Insert(api.NamespaceSystem) // Always exclude kube-system
	return excludedNS
}

func (cls *ClusterLinkSpec) ToIncludedNamespaceSet() sets.Set[string] {
	return sets.New(cls.IncludedNamespaces...)
}

func (cls *ClusterLinkSpec) ToExcludedServiceSet() sets.Set[string] {
	return sets.New(cls.ExcludedServices...)
}

func (cls *ClusterLinkSpec) ToExcludedServiceNameSet() sets.Set[string] {
	excludedSvcNames := sets.New(cls.ExcludedServiceNames...)
	excludedSvcNames.Insert("kubernetes") // Always exclude the kubernetes service
	return excludedSvcNames
}

// ShouldExcludeNamespace determines whether a namespace should be excluded from synchronization.
// It evaluates exclusion/inclusion rules in the following order:
// 1. Namespace is explicitly excluded
// 2. Namespace is not in the included list (if IncludedNamespaces is specified)
// Parameters accept pre-computed sets for efficient O(1) lookups.
// Returns true if the namespace should be excluded, false otherwise.
func (cls *ClusterLinkSpec) ShouldExcludeNamespace(namespace string, excludedNS, includedNS *sets.Set[string]) bool {
	// Exclude if namespace is in the exclusion list
	if excludedNS.Has(namespace) {
		return true
	}

	// Exclude if namespace is not in the inclusion list (when inclusion list is specified)
	if includedNS.Len() > 0 && !includedNS.Has(namespace) {
		return true
	}

	return false
}

// ShouldExcludeService determines whether a service should be excluded from synchronization.
// It evaluates exclusion/inclusion rules in the following order:
//  1. Service is explicitly excluded by namespace/name combination
//  2. Service name is globally excluded across all namespaces
//
// Parameters accept pre-computed sets for efficient O(1) lookups.
// Returns true if the service should be excluded, false otherwise.
func (cls *ClusterLinkSpec) ShouldExcludeService(namespace, serviceName string, excludedSvcSet, excludedSvcNameSet *sets.Set[string]) bool {
	// Exclude if exact namespace/service combination matches
	fullName := namespace + "/" + serviceName
	if excludedSvcSet.Has(fullName) {
		return true
	}

	// Exclude if service name is globally excluded
	if excludedSvcNameSet.Has(serviceName) {
		return true
	}

	return false
}
