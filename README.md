# svclink

**svclink** is a Kubernetes multi-cluster service synchronization controller that enables transparent cross-cluster service discovery and load balancing by automatically aggregating service endpoints across clusters.

## 🎯 What It Does

svclink solves service discovery and load balancing challenges in Kubernetes multi-cluster environments:

### Core Capabilities

1. **Cross-Cluster Service Aggregation**
   - Automatically discovers Services and Endpoints from remote clusters
   - Aggregates service endpoints from multiple clusters to the main cluster
   - Applications can access remote cluster services just like local services

2. **Transparent Load Balancing**
   - Implements cross-cluster load balancing through standard Kubernetes EndpointSlice
   - No application code changes required
   - Supports native Kubernetes service discovery mechanisms

3. **Flexible Service Filtering**
   - Namespace-level include/exclude control
   - Fine-grained service-level filtering
   - Support for global service name exclusion (high performance)

4. **Dynamic Cluster Management**
   - Declarative management of remote clusters through CRD
   - Support for dynamic cluster addition/removal without restart
   - Real-time monitoring of cluster connection status and version information

## 🚀 Quick Start

### Prerequisites

- kubectl command-line tool
- Pod network connectivity between main cluster and remote clusters
- kubeconfig files for remote clusters (with read-only permissions)

### 30-Second Quick Deployment

```bash
# 1. Deploy CRD and Controller
kubectl apply -f https://raw.githubusercontent.com/cloudpilot-ai/svclink/main/config/crds/svclink.cloudpilot.ai_clusterlinks.yaml
kubectl apply -f https://raw.githubusercontent.com/cloudpilot-ai/svclink/main/config/deploy/deployment.yaml

# 2. Get read-only kubeconfig from remote cluster (using automated script):
# Switch to remote cluster context and run the script
./hack/export-kubeconfig.sh

# 3. Declare remote cluster (using base64 output from script)
kubectl apply -f - <<EOF
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: cluster-b
spec:
  enabled: true
  kubeconfig: xxx
EOF
```

✅ Done! All required services from the remote cluster will now automatically sync to the main cluster.

### Verify Deployment

```bash
# Check ClusterLink resources
kubectl get clusterlinks

# View detailed status
kubectl describe clusterlink cluster-a

# Check Controller logs
kubectl logs -f deployment/svclink -n cloudpilot
```

### Typical Use Cases

- **Blue-Green/Canary Deployment**: Cross-cluster traffic distribution and progressive rollout
- **Cluster Migration**: Smooth progressive cluster migration

## ✨ Core Features

- 🔄 **Automatic Sync** - Syncs all services by default (except kube-system), supports fine-grained control
- 🎯 **Efficient Aggregation** - Efficient endpoint management based on EndpointSlice API
- 📋 **Declarative Configuration** - Manage clusters through ClusterLink CRD
- 📊 **Observable Status** - Real-time monitoring of cluster connection status
- 🔌 **Plug and Play** - Dynamic cluster addition/removal without controller restart
- 🎚️ **Flexible Filtering** - Multi-level filtering strategy for precise sync scope control

## 🏗️ Architecture Design

### Overall Architecture

```txt
┌─────────────────────────────────────────────────────────────────┐
│                      Main Cluster                                 │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ svclink Controller                                       │    │
│  │                                                          │    │
│  │  ┌──────────────────────────────────────────────────┐   │    │
│  │  │ ClusterLink Controller                            │   │    │
│  │  │ - Watch ClusterLink CRD changes                  │   │    │
│  │  │ - Read kubeconfig from Secret                    │   │    │
│  │  │ - Manage remote cluster clients                  │   │    │
│  │  └──────────────────────────────────────────────────┘   │    │
│  │                                                          │    │
│  │  ┌──────────────────────────────────────────────────┐   │    │
│  │  │ Service Discoverer                                │   │    │
│  │  │ - Discover Services and Endpoints from remote    │   │    │
│  │  │   clusters                                       │   │    │
│  │  │ - Apply filtering rules (namespace/service)      │   │    │
│  │  │ - Listen to service change events                │   │    │
│  │  └──────────────────────────────────────────────────┘   │    │
│  │                                                          │    │
│  │  ┌──────────────────────────────────────────────────┐   │    │
│  │  │ Endpoint Aggregator                               │   │    │
│  │  │ - Aggregate endpoints from multiple clusters     │   │    │
│  │  │ - Create separate EndpointSlice for each cluster │   │    │
│  │  │ - Keep endpoint information synchronized         │   │    │
│  │  └──────────────────────────────────────────────────┘   │    │
│  └─────────────────────────────────────────────────────────┘    │
│                           ↓                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ EndpointSlices (one per remote cluster)                 │    │
│  │                                                          │    │
│  │  • nginx-cluster-a (2 endpoints from cluster-a)          │    │
│  │  • nginx-cluster-b (3 endpoints from cluster-b)          │    │
│  │  • api-cluster-a (4 endpoints from cluster-a)            │    │
│  └─────────────────────────────────────────────────────────┘    │
│                           ↑                                      │
│           Services automatically use all EndpointSlices for      │
│           load balancing                                         │
└─────────────────────────────────────────────────────────────────┘
         ↑                                    ↑
         │ kubeconfig                         │ kubeconfig
         │ (in Secret)                        │ (in Secret)
    ┌────┴─────┐                        ┌────┴─────┐
    │ cluster-a│                        │ cluster-b│
    │          │                        │          │
    │ Services │                        │ Services │
    │ Pods     │                        │ Pods     │
    └──────────┘                        └──────────┘
```

### Workflow

1. **Cluster Configuration Phase**
   - Administrator creates ClusterLink CRD containing remote cluster kubeconfig to declare clusters to sync
   - Controller reads configuration and establishes connections to remote clusters

2. **Service Discovery Phase**
   - Controller List/Watch Services and Endpoints from remote clusters
   - Filter services based on Service and ClusterLink filtering rules
   - Track changes to services and endpoints

3. **Endpoint Aggregation Phase**
   - Create separate EndpointSlice for each remote cluster
   - Copy endpoint information from remote clusters to main cluster
   - Keep endpoint status synchronized (ready/not ready)

4. **Service Access Phase**
   - Applications access services through Service DNS names
   - Kubernetes kube-proxy automatically discovers all EndpointSlices
   - Traffic is load balanced between local and remote endpoints

### Data Flow

```txt
Remote Cluster                Main Cluster                 Application
     │                             │                            │
     │  1. Watch Services          │                            │
     │ <────────────────────────── │                            │
     │                             │                            │
     │  2. Service/Endpoint Events │                            │
     │ ─────────────────────────> │                            │
     │                             │                            │
     │                             │  3. Create/Update          │
     │                             │     EndpointSlice          │
     │                             │ ──────────┐                │
     │                             │           │                │
     │                             │ <─────────┘                │
     │                             │                            │
     │                             │  4. Service Discovery      │
     │                             │ <────────────────────────  │
     │                             │                            │
     │                             │  5. Return Endpoints       │
     │                             │ ─────────────────────────> │
     │                             │    (local + remote)        │
     │  6. Direct Pod-to-Pod       │                            │
     │    Traffic (if network      │                            │
     │    reachable)               │                            │
     │ <──────────────────────────────────────────────────────  │
```

## 🔐 Permission Requirements

### Main Cluster Permissions

The svclink Controller requires the following permissions in the main cluster (granted via ClusterRole):

#### 1. Service-related Permissions

```yaml
# Read Services from all namespaces (for creating corresponding EndpointSlices)
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get", "list", "watch"]
# Create services across all namespaces
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["create"]
```

#### 2. EndpointSlice Management Permissions

```yaml
# Create and manage EndpointSlices (core functionality)
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

#### 3. ClusterLink CRD Permissions

```yaml
# Read ClusterLink configuration
- apiGroups: ["svclink.cloudpilot.ai"]
  resources: ["clusterlinks"]
  verbs: ["get", "list", "watch"]

# Update ClusterLink status
- apiGroups: ["svclink.cloudpilot.ai"]
  resources: ["clusterlinks/status"]
  verbs: ["get", "update", "patch"]
```

### 4. Namespace Read Permissions

```yaml
# Read Namespace information
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]
```

### 5. Namespace Create Permissions

```yaml
  # Create Namespaces
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["create"]
```

### Remote Cluster Permissions

In remote clusters, the ServiceAccount corresponding to the kubeconfig requires the following permissions:

#### 1. Service and Endpoint Read Permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: svclink-remote-reader
rules:
  # Read Service information
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get", "list", "watch"]

  # Read EndpointSlice information
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]

  # Read Namespace information
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch"]
```

#### 2. Creating ServiceAccount and kubeconfig

Create read-only kubeconfig for remote clusters using the provided automation script:

```bash
./hack/export-kubeconfig.sh

# The script automatically:
# 1. Creates ServiceAccount: svclink (in kube-system namespace)
# 2. Creates ClusterRole: svclink-reader (read-only permissions)
# 3. Creates ClusterRoleBinding
# 4. Generates Secret token (compatible with K8s 1.24+)
# 5. Outputs base64-encoded kubeconfig

# Copy the output base64 string for use in ClusterLink
```

### Principle of Least Privilege

- ✅ Main cluster: Only requires cluster-wide read permissions + EndpointSlice write permissions + full ClusterLink permissions
- ✅ Remote clusters: Only requires read permissions, no write permissions needed
- ✅ RBAC: Follows principle of least privilege, does not grant unnecessary permissions

## 📦 Installation and Deployment

### Using Pre-built YAML

```bash
# 1. Install CRD
kubectl apply -f config/crds/svclink.cloudpilot.ai_clusterlinks.yaml

# 2. Deploy Controller
kubectl apply -f deploy/deployment.yaml

# This will create:
# - Namespace: cloudpilot
# - ServiceAccount: svclink
# - ClusterRole: cloudpilot (with necessary permissions)
# - ClusterRoleBinding: cloudpilot
# - Deployment: svclink (runs the controller)
```

### Getting Remote Cluster kubeconfig

#### Using Automation Script (Recommended)

The project provides a `hack/export-kubeconfig.sh` script to automate the creation of read-only ServiceAccount and generate kubeconfig:

```bash
# Ensure the script has execute permissions
chmod +x hack/export-kubeconfig.sh

# Use current kubectl context
./hack/export-kubeconfig.sh

# Specify a specific cluster context
./hack/export-kubeconfig.sh production-cluster

# Script output example:
# ✅ SUCCESS: Base64 Kubeconfig Generated
# ==========================================
#
# Copy the following base64 string to use in ClusterLink spec.kubeconfig:
#
# LS0tLS1CRUdJTi... (base64 encoded kubeconfig)
```

**Script Features**:

- ✅ Automatically creates ServiceAccount (kube-system/svclink)
- ✅ Creates read-only ClusterRole and ClusterRoleBinding
- ✅ Compatible with Kubernetes 1.24+ (automatically creates Secret token)
- ✅ Generates base64-encoded kubeconfig that can be used directly in ClusterLink
- ✅ Complete error handling and progress feedback

### Configuring Remote Cluster Access

```bash
# Use base64-encoded kubeconfig directly in ClusterLink
kubectl apply -f - <<EOF
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: cluster-prod
spec:
  enabled: true
  kubeconfig: xxx
EOF
```

## 📚 Usage Guide

### Command Line Parameters

For local development and advanced usage scenarios, svclink provides several command line parameters:

#### Available Parameters

```bash
svclink [flags]

Flags:
  --sync-interval duration         Sync interval for periodic reconciliation (default: 30s)
  --kubeconfig string             Path to kubeconfig file (for local development)
  --included-namespaces strings   If specified, only services in these namespaces will be synced
  --sync-services-to-local-cluster bool   Whether to sync services to the local cluster (default: false)
  -h, --help                      Help for svclink
```

#### Parameter Details

1. **`--sync-interval`**
   - Controls how often the controller performs full synchronization
   - Default: 30 seconds
   - Recommended range: 30s - 60s for production workloads
   - Example: `--sync-interval=45s`

2. **`--kubeconfig`**
   - Path to kubeconfig file for connecting to the main cluster
   - Used for local development when running outside the cluster
   - If not specified, uses in-cluster configuration
   - Example: `--kubeconfig=/path/to/kubeconfig`

3. **`--included-namespaces`** ⭐
   - **Global namespace filtering** - restricts synchronization scope to specific namespaces
   - Overrides all ClusterLink configurations for namespace inclusion
   - When specified, only services from these namespaces will be synced from **all clusters**
   - Provides performance optimization by reducing API queries to specific namespaces
   - Example: `--included-namespaces=default,production,staging`

4. **`--sync-services-to-local-cluster`**
   - Whether to sync services to the local cluster (main cluster)
   - Default: false
   - When set to true, services from remote clusters will also be synced to the local cluster
   - Useful for scenarios where local access to remote services is required
   - Example: `--sync-services-to-local-cluster=true`

#### Usage Examples

##### Local Development

```bash
# Run locally with custom kubeconfig
./svclink --kubeconfig=$HOME/.kube/config --sync-interval=15s

# Sync only specific namespaces for development
./svclink --kubeconfig=$HOME/.kube/config --included-namespaces=default,test
```

##### Production Deployment with Namespace Filtering

```bash
# Only sync production-related namespaces (reduces overhead)
./svclink --included-namespaces=production,staging,monitoring --sync-interval=60s
```

##### Performance-Optimized Configuration

```bash
# Minimize sync scope for large clusters
./svclink --included-namespaces=app-tier,data-tier --sync-interval=45s
```

#### Important Notes

- **Global vs ClusterLink Filtering**: The `--included-namespaces` flag applies **globally** to all clusters, while ClusterLink's `spec.includedNamespaces` applies per-cluster
- **Performance Impact**: Using `--included-namespaces` significantly improves performance in large clusters by avoiding full cluster service discovery
- **Precedence**: Command-line `--included-namespaces` takes precedence - if specified, ClusterLink namespace filtering is ignored
- **Empty List**: If `--included-namespaces` is not specified, all namespaces (except kube-system) are processed according to individual ClusterLink configurations

### Basic Usage

#### 1. Creating ClusterLink

```yaml
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: production-east
spec:
  # Whether to enable synchronization (default: true)
  enabled: true
  # kubeconfig (base64 encoded)
  kubeconfig: LS0tLS1CRUd... (omitted)
```

#### 2. Viewing Cluster Status

```bash
# List all ClusterLinks
kubectl get clusterlinks

# Example output:
# NAME              ENABLED   VERSION   STATUS    AGE
# production-east   true      v1.28.0   Ready     5m
# production-west   true      v1.27.2   Ready     3m

# View detailed status
kubectl describe clusterlink production-east -n cloudpilot

# Example output:
# Status:
#   Conditions:
#     Last Transition Time:  2024-01-15T10:30:00Z
#     Message:              Cluster connection established
#     Reason:               ClusterReady
#     Status:               True
#     Type:                 Ready
#   Version:                v1.28.0
```

#### 3. Verifying Service Synchronization

```bash
# View synchronized EndpointSlices
kubectl get endpointslices -n default

# Example output:
# NAME                    ADDRESSTYPE   PORTS   ENDPOINTS   AGE
# nginx-local            IPv4          80      3           10m
# nginx-production-east  IPv4          80      2           5m
# nginx-production-west  IPv4          80      2           3m

# View EndpointSlice details
kubectl describe endpointslice nginx-production-east -n default
```

### Service Filtering Configuration

svclink provides multi-level service filtering capabilities, ordered by priority from highest to lowest:

1. **kube-system namespace** - Always excluded (hardcoded)
2. **includedNamespaces** - Whitelist: Only sync specified namespaces
3. **excludedNamespaces** - Blacklist: Exclude specified namespaces
4. **excludedServices** - Exclude specific services (format: `namespace/service-name`)
5. **excludedServiceNames** - Globally exclude service names (all namespaces)

#### Example 1: Exclude Specific Namespaces

```yaml
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: cluster-prod
  namespace: cloudpilot
spec:
  enabled: true
  excludedNamespaces:
    - monitoring          # Exclude monitoring-related services
    - logging             # Exclude logging-related services
    - internal-tools      # Exclude internal tools
```

#### Example 2: Sync Only Specific Namespaces (Whitelist)

```yaml
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: cluster-prod
  namespace: cloudpilot
spec:
  enabled: true
  includedNamespaces:
    - default            # Only sync these three namespaces
    - production
    - staging
```

#### Example 3: Exclude Specific Services

```yaml
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: cluster-prod
  namespace: cloudpilot
spec:
  enabled: true
  excludedServices:
    - default/internal-db          # Exclude internal-db in default namespace
    - production/admin-api         # Exclude admin-api in production namespace
    - staging/debug-service        # Exclude debug-service in staging namespace
```

#### Example 4: Globally Exclude Service Names

```yaml
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: cluster-prod
  namespace: cloudpilot
spec:
  enabled: true
  excludedServiceNames:
    - admin-service      # Exclude this service name in all namespaces
    - internal-cache     # Exclude this service name in all namespaces
    - debug-tool         # Exclude this service name in all namespaces
    - kubernetes         # kubernetes service is excluded by default, no need to configure
```

**Note**: The `kubernetes` service and `kube-system` namespace are always excluded and do not need explicit configuration.

#### Example 5: Combined Filtering Strategy

```yaml
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: cluster-prod
  namespace: cloudpilot
spec:
  enabled: true

  # Only sync these namespaces
  includedNamespaces:
    - default
    - production
    - staging

  # Exclude specific services within the above namespaces
  excludedServices:
    - production/internal-api      # Internal API in production environment not synced

  # Exclude these service names in all namespaces
  excludedServiceNames:
    - admin-panel                  # All admin panels not synced
    - metrics-collector            # All metrics collectors not synced
```

### Cluster Management Operations

#### Adding New Cluster

```bash
# Step 1: Get kubeconfig from new cluster
./hack/export-kubeconfig.sh

# Step 2: Method 2 - Embed kubeconfig (use script output directly)
kubectl apply -f - <<EOF
apiVersion: svclink.cloudpilot.ai/v1alpha1
kind: ClusterLink
metadata:
  name: new-cluster
  namespace: cloudpilot
spec:
  enabled: true
  kubeconfig: xxx
EOF
```

#### Disable/Enable Cluster Synchronization

```bash
# Disable cluster (stop sync, but don't delete existing EndpointSlices)
kubectl patch clusterlink production-east -n cloudpilot \
  --type merge -p '{"spec":{"enabled":false}}'

# Re-enable cluster
kubectl patch clusterlink production-east -n cloudpilot \
  --type merge -p '{"spec":{"enabled":true}}'
```

#### Delete Cluster

```bash
# Delete ClusterLink (will clean up associated EndpointSlices)
kubectl delete clusterlink production-east -n cloudpilot

# If using Secret, optionally clean up entries in Secret
kubectl edit secret remote-clusters-kubeconfig -n cloudpilot
# Manually delete corresponding key
```

#### Update Cluster Configuration

```bash
# Update filtering rules
kubectl edit clusterlink production-east -n cloudpilot

# Or use patch
kubectl patch clusterlink production-east -n cloudpilot \
  --type merge -p '{"spec":{"excludedNamespaces":["monitoring","logging"]}}'
```

### Monitoring and Troubleshooting

#### Check Controller Status

```bash
# View Pod status
kubectl get pods -n cloudpilot -l app=svclink

# View logs
kubectl logs -f deployment/svclink -n cloudpilot

# View recent events
kubectl get events -n cloudpilot --sort-by='.lastTimestamp'
```

#### Common Issue Troubleshooting

##### Issue 1: ClusterLink Status is NotReady

```bash
# Check detailed error information
kubectl describe clusterlink <name>

# Common causes:
# 1. Invalid or expired kubeconfig
# 2. Network connectivity issues
# 3. Insufficient permissions
```

##### Issue 2: EndpointSlice Not Created

```bash
# Check if remote cluster has corresponding Service
kubectl get svc -A --kubeconfig=/path/to/remote.kubeconfig

# Check if filtering rules exclude this service
kubectl get clusterlink <name> -n cloudpilot -o yaml

# View Controller logs
kubectl logs deployment/svclink -n cloudpilot | grep <service-name>
```

##### Issue 3: Cross-cluster Access Failed

```bash
# Check Pod network connectivity
kubectl run test-pod --image=nicolaka/netshoot -it --rm -- /bin/bash
# Ping remote cluster Pod IP inside the Pod

# Check if addresses in EndpointSlice are correct
kubectl describe endpointslice <name> -n <namespace>

# Check Service endpoints
kubectl get endpoints <service-name> -n <namespace>
```

## 🗑️ Uninstall and Cleanup

### Complete svclink Uninstall

```bash
# 1. Delete all ClusterLink resources
kubectl delete clusterlinks --all

# 2. Wait for Controller to clean up associated EndpointSlices (about 5-10 seconds)
sleep 10

# 3. Delete Controller Deployment
kubectl delete deployment svclink -n cloudpilot

# 4. Delete RBAC resources
kubectl delete clusterrolebinding svclink
kubectl delete clusterrole svclink
kubectl delete serviceaccount svclink -n cloudpilot

# 5. Delete CRD (will delete all ClusterLink instances)
kubectl delete crd clusterlinks.svclink.cloudpilot.ai
```

### Cleanup Leftover EndpointSlices

If you need to manually clean up EndpointSlices created by svclink:

```bash
# Use the cleanup script provided by the project (recommended)
./hack/cleanup-endpointslices.sh --help

# Preview EndpointSlices to be deleted (dry-run)
./hack/cleanup-endpointslices.sh --dry-run

# Clean up all svclink-managed EndpointSlices in all namespaces
./hack/cleanup-endpointslices.sh

# Clean up only specific namespace
./hack/cleanup-endpointslices.sh -n default

# Or use Makefile
make cleanup-endpointslices-dry    # Preview
make cleanup-endpointslices         # Execute cleanup
```

The cleanup script will delete all EndpointSlices containing the following labels:

- `svclink.cloudpilot.ai/cluster=*`

For more details, please refer to: [docs/cleanup-endpointslices.md](./docs/cleanup-endpointslices.md)

## ⚠️ Limitations and Considerations

### Technical Limitations

1. **Network Connectivity Requirements**
   - ❌ Requires main cluster Pods to directly access remote cluster Pod IPs
   - ✅ Suitable for scenarios with same VPC, VPN interconnection, or dedicated line connections
   - ❌ Not suitable for completely isolated network environments

2. **Service Type Limitations**
   - ✅ Supported: ClusterIP type Services
   - ❌ Not supported: Headless Services (`clusterIP: None`)
   - ⚠️ LoadBalancer/NodePort: Only syncs Pod endpoints, not external IPs

### Functional Limitations

1. **Service Discovery**
   - Only syncs Service Pod endpoints
   - Does not sync ExternalName type Services
   - Does not sync external IPs from Endpoints

2. **Status Synchronization**
   - Endpoint status (ready/not ready) is synchronized
   - Pod deletion has brief delay (depends on `sync-interval`)
   - Does not guarantee strong consistency, uses eventual consistency model

3. **Namespaces**
   - kube-system namespace is always excluded
   - Remote cluster and main cluster namespaces need to have the same name

### Performance Considerations

1. **Scalability**
   - Recommended number of remote clusters: ≤ 10
   - Recommended total number of synced Services: ≤ 1000
   - Recommended number of endpoints per Service: ≤ 100

2. **Sync Latency**
   - Normal conditions: < 5 seconds (depends on sync-interval)
   - Network jitter may increase latency
   - Recommended sync-interval setting: 30s - 60s

### Security Considerations

1. **Credential Management**
   - ⚠️ kubeconfig contains sensitive information and should be properly secured
   - ✅ Recommend using read-only ServiceAccount for remote clusters
   - ✅ Regularly rotate ServiceAccount tokens

2. **Access Control**
   - Follow principle of least privilege
   - Only grant necessary read permissions to remote clusters
   - Regularly audit RBAC configurations

3. **Network Security**
   - Ensure inter-cluster communication is encrypted (TLS)
   - Consider using network policies to limit cross-cluster traffic
   - Monitor abnormal cross-cluster access

### Known Issues

1. **EndpointSlice Naming**
   - EndpointSlice name format: `{service-name}-{cluster-name}`
   - Names longer than 63 characters will be truncated
   - May cause name conflicts between EndpointSlices of different services

2. **Cluster Deletion**
   - When deleting ClusterLink, associated EndpointSlices will be cleaned up
   - If Controller is not running, EndpointSlices may be left behind
   - Use `hack/cleanup-endpointslices.sh` for manual cleanup

3. **Network Partition**
   - When remote cluster network is unreachable, EndpointSlices are not immediately deleted
   - May cause request timeouts, recommend configuring reasonable timeout values

## 🤝 Community and Support

- GitHub Issues: [https://github.com/cloudpilot-ai/svclink/issues](https://github.com/cloudpilot-ai/svclink/issues)
