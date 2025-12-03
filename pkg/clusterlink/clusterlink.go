// Package clusterlink manages ClusterLink CRDs and builds remote cluster clients.
// It handles listing ClusterLink resources, decoding embedded kubeconfigs,
// creating Kubernetes clients for remote clusters, and updating connection status.
package clusterlink

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	svclinkv1alpha1 "github.com/cloudpilot-ai/svclink/pkg/apis/svclink/v1alpha1"
)

func ListClusterInfo(ctx context.Context, kubeClient client.Client) (map[string]*ClusterInfo, error) {
	var cks svclinkv1alpha1.ClusterLinkList
	if err := kubeClient.List(ctx, &cks); err != nil {
		return nil, err
	}

	clusterInfos := make(map[string]*ClusterInfo, len(cks.Items))
	for _, clusterLink := range cks.Items {
		clusterInfo := &ClusterInfo{
			Name:        clusterLink.Name,
			Enabled:     clusterLink.Spec.Enabled,
			ClusterLink: clusterLink,
		}

		kubeconfigData, err := base64.StdEncoding.DecodeString(clusterLink.Spec.Kubeconfig)
		if err != nil {
			klog.Errorf("Failed to decode kubeconfig for cluster %s: %v", clusterLink.Name, err)
			updateClusterStatus(ctx, kubeClient, &clusterLink, false, "", fmt.Sprintf("Failed to decode kubeconfig: %v", err))
			continue
		}

		client, version, err := buildClientWithVersion(kubeconfigData)
		if err != nil {
			klog.Errorf("Failed to build client for cluster %s: %v", clusterLink.Name, err)
			updateClusterStatus(ctx, kubeClient, &clusterLink, false, "", fmt.Sprintf("Failed to build client: %v", err))
			continue
		}

		clusterInfo.Client = client
		clusterInfos[clusterLink.Name] = clusterInfo
		updateClusterStatus(ctx, kubeClient, &clusterInfo.ClusterLink, true, version, "")
	}
	return clusterInfos, nil
}

// ClusterInfo holds information about a remote cluster
type ClusterInfo struct {
	Name        string
	Enabled     bool
	Client      kubernetes.Interface
	ClusterLink svclinkv1alpha1.ClusterLink
}

// buildClientWithVersion creates a Kubernetes client from kubeconfig data and fetches the cluster version
func buildClientWithVersion(kubeconfigData []byte) (kubernetes.Interface, string, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create client: %w", err)
	}

	// Try to get the cluster version
	version := ""
	versionInfo, err := client.Discovery().ServerVersion()
	if err != nil {
		klog.V(4).Infof("Failed to get cluster version: %v", err)
	} else {
		version = versionInfo.GitVersion
	}

	return client, version, nil
}

func updateClusterStatus(ctx context.Context, kubeClient client.Client, cluster *svclinkv1alpha1.ClusterLink, connected bool, version, errorMsg string) {
	cluster.Status.Connected = connected
	cluster.Status.Version = version
	cluster.Status.Error = errorMsg

	if connected {
		now := metav1.NewTime(time.Now())
		cluster.Status.LastConnected = &now
	}

	// Update conditions
	cluster.Status.Conditions = buildConditions(connected, errorMsg)

	// Apply status update using controller-runtime client
	if err := kubeClient.Status().Update(ctx, cluster); err != nil {
		// Ignore not found errors - the resource may have been deleted
		if client.IgnoreNotFound(err) != nil {
			klog.Errorf("Failed to update status for ClusterLink %s: %v", cluster.Name, err)
		}
		return
	}

	klog.V(4).Infof("Updated status for ClusterLink %s (connected=%v)", cluster.Name, connected)
}

func buildConditions(connected bool, errorMsg string) []svclinkv1alpha1.ClusterLinkCondition {
	now := metav1.NewTime(time.Now())
	var conditions []svclinkv1alpha1.ClusterLinkCondition

	if connected {
		conditions = append(conditions, svclinkv1alpha1.ClusterLinkCondition{
			Type:               svclinkv1alpha1.ClusterLinkReady,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: now,
			Reason:             "Connected",
			Message:            "Successfully connected to remote cluster",
		})
	} else {
		conditions = append(conditions, svclinkv1alpha1.ClusterLinkCondition{
			Type:               svclinkv1alpha1.ClusterLinkReady,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: now,
			Reason:             "ConnectionFailed",
			Message:            "Failed to connect to remote cluster",
		})

		if errorMsg != "" {
			conditions = append(conditions, svclinkv1alpha1.ClusterLinkCondition{
				Type:               svclinkv1alpha1.ClusterLinkError,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: now,
				Reason:             "Error",
				Message:            errorMsg,
			})
		}
	}

	return conditions
}

func UpdateClusterSyncError(ctx context.Context, kubeClient client.Client, clusterInfo *ClusterInfo, clusterName string, syncError error) {
	var errorMsg string
	if syncError != nil {
		errorMsg = fmt.Sprintf("Service sync error: %v", syncError)
	}
	// Always update status - either with error or clear it (empty string)
	updateClusterStatus(ctx, kubeClient, &clusterInfo.ClusterLink, true, clusterInfo.ClusterLink.Status.Version, errorMsg)
}
