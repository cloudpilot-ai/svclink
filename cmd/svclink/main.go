package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"k8s.io/client-go/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	api "k8s.io/kubernetes/pkg/apis/core"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/cloudpilot-ai/svclink/pkg/config"
	"github.com/cloudpilot-ai/svclink/pkg/controller"
)

var (
	syncInterval               time.Duration
	kubeconfig                 string
	includedNamespaces         []string
	syncServicesToLocalCluster bool

	rootCmd = &cobra.Command{
		Use:   "svclink",
		Short: "Kubernetes service synchronization controller",
		Long: `svclink is a Kubernetes controller that synchronizes services across multiple clusters.
It watches for ClusterLink CRDs with embedded kubeconfigs and service changes, and updates EndpointSlices accordingly.`,
		RunE: runController,
	}
)

func main() {
	klog.InitFlags(nil)

	rootCmd.Flags().DurationVar(&syncInterval, "sync-interval", config.DefaultSyncInterval, "Sync interval")
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (for local development)")
	rootCmd.Flags().StringSliceVar(&includedNamespaces, "included-namespaces", []string{}, "Global namespace filter: if specified, only services in these namespaces will be synced across all clusters (overrides ClusterLink-level inclusion rules)")
	rootCmd.Flags().BoolVar(&syncServicesToLocalCluster, "sync-services-to-local-cluster", false, "Whether to sync services from remote clusters to the local cluster")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runController(cmd *cobra.Command, args []string) error {
	currentVersion := version.Get()
	klog.Infof("Start cloudpilot svclink, version: %s, commit: %s", currentVersion.GitVersion, currentVersion.GitCommit)

	// Set up controller-runtime logger to use klog
	ctrl.SetLogger(klog.NewKlogr())

	if lo.Contains(includedNamespaces, api.NamespaceSystem) {
		return errors.New("cannot include 'kube-system' namespace; it is always excluded")
	}

	// Build config
	cfg := &config.Config{
		SyncInterval:               syncInterval,
		IncludedNamespaces:         includedNamespaces,
		SyncServicesToLocalCluster: syncServicesToLocalCluster,
	}

	// Create Kubernetes client
	restConfig, err := buildRestConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to build REST config: %w", err)
	}

	// Create controller
	ctrl, err := controller.NewController(cfg, restConfig)
	if err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		klog.Infof("Received signal %v, shutting down", sig)
		cancel()
	}()

	// Run controller
	if err := ctrl.Run(ctx); err != nil {
		klog.Errorf("Controller error: %v", err)
	}

	return nil
}

// buildRestConfig creates a REST config from kubeconfig or in-cluster config
func buildRestConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	// Try in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	return config, nil
}
