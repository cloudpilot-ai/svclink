# svclink — AI coding agent quick instructions

This file explains the minimal, high-value knowledge an AI coding agent needs to be productive in this repository.

High-level summary
- svclink is a periodic multi-cluster service synchronizer: it lists ClusterLink CRDs (each contains a base64 kubeconfig), builds remote clients, discovers Services in remote clusters, aggregates their EndpointSlices, and writes EndpointSlices into the main cluster.
- The controller is NOT event-driven: one periodic sync loop (see `pkg/controller/controller.go::syncLoop`) performs full reconciliation every Config.SyncInterval (default ~30s).

Key files to read first (in order)
- `cmd/svclink/main.go` — CLI/bootstrap flags and entrypoint.
- `pkg/controller/controller.go` — startup, manager creation, and `syncLoop()` orchestration.
- `pkg/clusterlink/clusterlink.go` — how ClusterLink CRDs are listed and decoded into remote clients (`ListClusterInfo()`).
- `pkg/discoverer/service_discoverer.go` — service filtering rules and discovery across clusters.
- `pkg/aggregator/endpoint_aggregator.go` — endpoint aggregation logic and tests in `pkg/aggregator/endpoint_aggregator_test.go`.
- `pkg/updater/slice_updater.go` and `pkg/updater/service_updater.go` — EndpointSlice and optional Service creation semantics.
- `pkg/apis/svclink/v1alpha1/types.go` — CRD shapes and kubebuilder markers (edit -> run codegen).

Project-specific patterns & gotchas
- Stateless sync: all state is derived each cycle from API queries; do not add long-lived in-memory assumptions.
- Manager is used only for scheme/client caching — there is no controller-runtime reconciler per-CRD; changes are detected by re-listing CRDs each cycle.
- ClusterLink contains a base64 kubeconfig in `spec.kubeconfig`. `ListClusterInfo()` decodes this and builds a `kubernetes.Interface` per cluster; failures are logged and the cluster is skipped.
- Filtering precedence: namespace inclusion/exclusion -> specific service exclusion (`namespace/name`) -> service-name exclusion. See `ToExcluded*Set()` helpers used to precompute O(1) sets.
- EndpointSlice naming: `{service-name}-svclink-{cluster-name}`. EndpointSlices are owned by the Service (ownerRefs). Labels include `cloudpilot.ai/svclink-cluster` and `endpointslice.kubernetes.io/managed-by=svclink.cloudpilot.ai`.

Essential commands (copyable)
- Build & run locally: `make run` (runs `go run cmd/svclink/main.go`).
- Tests: `make test` (runs `go test -v ./pkg/...`).
- CRD/codegen (required after changing `pkg/apis/...`):
  - `make codegen` (deepcopy)
  - `make crdgen` (controller-gen for CRDs)
  - `make verify-gen-update` (CI check)
- Export remote kubeconfig helper: `./hack/export-kubeconfig.sh <context>` → outputs base64 kubeconfig for ClusterLink.spec.kubeconfig.
- Container image builds (uses `ko`): `make ko-build-local` / `make ko-build` / `make ko-deploy`.

Testing & validation guidance
- Unit-test focus: aggregation logic and filtering behavior. See `pkg/aggregator/*_test.go` and `pkg/apis/svclink/v1alpha1/types_test.go`.
- After edits to controller startup or manager usage, run `make test` and a quick local run `make run` with a KUBECONFIG pointing at a test cluster.

When editing CRD types
- Update `pkg/apis/svclink/v1alpha1/types.go` -> run `make codegen` and `make crdgen`. Always run `make verify-gen-update` before committing.

Small contract for agents (what edits should preserve)
- Inputs: Kubernetes API (main cluster + remote kubeconfigs in ClusterLink CRDs).
- Outputs: EndpointSlices in main cluster named `{service}-svclink-{cluster}` and optional Service objects (only when `--sync-services-to-local-cluster` is enabled).
- Error modes: per-cluster failures must be logged and not stop the whole sync; transient errors are retried on next periodic sync.

Quick debugging pointers
- Inspect ClusterLink status: `kubectl get clusterlinks` / `kubectl describe clusterlink <name>`.
- EndpointSlices: `kubectl get endpointslices -A -l endpointslice.kubernetes.io/managed-by=svclink.cloudpilot.ai`.
- Logs: `kubectl logs -f deployment/svclink -n <namespace>` (look for sync loop errors).

If something is missing or unclear, tell me which file or behavior you want documented next and I will expand this file.

----
Small note: merged and condensed from prior long instructions — preserves codegen, sync behavior, ClusterLink kubeconfig pattern, EndpointSlice naming, and the authoritative list of starter files.
# svclink AI Coding Agent Instructions

## Project Overview

**svclink** is a Kubernetes multi-cluster service synchronization controller that enables cross-cluster service discovery by aggregating endpoints from remote clusters into the main cluster via EndpointSlices.

### Core Architecture Pattern

The controller follows a **5-component orchestration model**:
1. **ClusterLink Manager** (`pkg/clusterlink/`) - Lists ClusterLink CRDs and builds remote cluster clients from embedded kubeconfigs
2. **ServiceDiscoverer** (`pkg/discoverer/`) - Discovers services across clusters using hierarchical filtering rules
3. **EndpointAggregator** (`pkg/aggregator/`) - Collects endpoints from remote clusters' EndpointSlices
4. **SliceUpdater** (`pkg/updater/slice_updater.go`) - Creates/updates EndpointSlices with cluster-specific suffixes (`{service-name}-svclink-{cluster-name}`)
5. **ServiceUpdater** (`pkg/updater/service_updater.go`) - Optionally creates Service objects in main cluster (when `--sync-services-to-local-cluster` flag is set)

### Key Data Flow

```
ClusterLink CRD (base64 kubeconfig) → clusterlink.ListClusterInfo() → Remote k8s.Client
  ↓
ServiceDiscoverer (filters services) → EndpointAggregator (collect endpoints)
  ↓
ServiceUpdater (optional) → Service objects in main cluster
  ↓
SliceUpdater → EndpointSlices in main cluster (one per remote cluster)
```

**Periodic Sync Architecture**:
- Single `syncLoop()` runs every `Config.SyncInterval` (default: 30s)
- Each sync cycle: List ClusterLinks → Discover services → Aggregate endpoints → Update EndpointSlices
- **No event-driven reconciler**: Controller-runtime Manager is initialized but only used for scheme registration and client creation

## Development Conventions

### Package Structure & Naming
- `cmd/svclink/main.go` - Cobra CLI entry point with signal handling
- `pkg/controller/controller.go` - Main orchestration (`Controller.Run()` coordinates all components, `syncLoop()` runs periodic sync)
- `pkg/clusterlink/clusterlink.go` - ClusterLink CRD listing and remote client building (`ListClusterInfo()`, `UpdateClusterSyncError()`)
- `pkg/apis/svclink/v1alpha1/types.go` - CRD definitions with kubebuilder markers (cluster-scoped resource)
- `pkg/config/types.go` - Controller config, constants for labels/annotations (`cloudpilot.ai/svclink-*`)
- `pkg/updater/slice_updater.go` - EndpointSlice CRUD operations
- `pkg/updater/service_updater.go` - Service synchronization to local cluster (optional feature)
- `hack/*.sh` - Bash scripts for codegen, CRD generation, and kubeconfig export

### ClusterLink CRD Configuration Pattern

ClusterLink embeds kubeconfig and filtering directly in spec (no separate Secret):
```go
type ClusterLinkSpec struct {
    Enabled bool                  // Runtime enable/disable switch
    Kubeconfig string             // Base64-encoded kubeconfig
    ExcludedNamespaces []string   // Namespace blocklist
    IncludedNamespaces []string   // Namespace allowlist (empty = all except excluded)
    ExcludedServices []string     // Format: "namespace/service-name"
    ExcludedServiceNames []string // Global service name blocklist
}
```

**Filtering precedence**: Namespace inclusion/exclusion → Service exclusion → Service name exclusion
**Defaults**: `kube-system` namespace and `kubernetes` service always excluded (hardcoded in `ToExcluded*Set()` methods)

### Controller-Runtime Integration

**Manager setup** (`controller.NewController`):
1. Create scheme with `k8s.io/client-go/kubernetes/scheme` + custom `svclinkv1alpha1` types
2. Create controller-runtime Manager with scheme
3. Manager provides controller-runtime client for main cluster operations
4. Manager cache is synced before starting sync loop

**No reconciler pattern**: Unlike typical controller-runtime projects, svclink uses:
- Manager only for client creation and caching (not for event-driven reconciliation)
- Simple periodic sync loop (`syncLoop()`) that calls `clusterlink.ListClusterInfo()` each cycle
- ClusterLink changes detected through periodic listing, not watches

**Client pattern**: Use controller-runtime `client.Client` for main cluster, `kubernetes.Interface` (client-go) for remote clusters

## Build & Development Workflow

### Code Generation (Required for CRD changes)

**CRITICAL**: Always run after modifying `pkg/apis/svclink/v1alpha1/types.go`:
```bash
make codegen           # Generates zz_generated.deepcopy.go using deepcopy-gen
make crdgen            # Generates config/crds/*.yaml using controller-gen
make verify-gen-update # CI check - fails if generated code is stale
```

**What they do**:
- `hack/update-codegen.sh` - Installs deepcopy-gen, generates deepcopy methods for runtime.Object
- `hack/update-crdgen.sh` - Uses controller-gen to parse kubebuilder markers → OpenAPI schema
- `hack/verify-gen-update.sh` - Diffs generated files against committed versions

### Build & Deploy

```bash
# Local development (requires kubeconfig)
make run  # Runs: go run cmd/svclink/main.go --kubeconfig=$KUBECONFIG

# Container builds (ko is preferred - no Dockerfile needed)
make ko-build-local  # Builds to ko.local registry (no push)
make ko-build        # Builds and pushes to $KO_DOCKER_REPO (default: cloudpilotai)
make ko-deploy       # Builds and applies config/deploy/deployment.yaml

# Override registry: KO_DOCKER_REPO=myregistry make ko-build
```

**ko advantages**:
- Automatically injects Git version info from environment vars
- No Dockerfile maintenance
- Direct integration with `go build` flags

### Remote Cluster Setup

**Script**: `./hack/export-kubeconfig.sh [context-name]`
**What it does**:
1. Creates ServiceAccount `svclink-reader` in `kube-system` namespace
2. Creates ClusterRole with minimal RBAC: `get/list/watch` on `services`, `endpointslices`
3. Extracts SA token and cluster CA cert
4. Outputs **base64-encoded kubeconfig** ready for ClusterLink spec

**Usage**:
```bash
# In remote cluster context
./hack/export-kubeconfig.sh cluster-b | pbcopy
# Paste into ClusterLink YAML: spec.kubeconfig: <base64-string>
```

## Critical Implementation Patterns

### EndpointSlice Naming & Ownership

**Naming**: `{service-name}-svclink-{cluster-name}` (e.g., `nginx-svclink-cluster-a`)
**Why**: Prevents collisions when multiple clusters have same service; enables per-cluster tracking

**Owner References**: EndpointSlices are owned by the Service object via `metav1.OwnerReference`
- Enables automatic garbage collection when Service is deleted
- See `slice_updater.go:updateSliceForCluster()` for implementation

**Labels**:
```go
cloudpilot.ai/svclink-cluster: <cluster-name>  // Identifies source cluster
kubernetes.io/service-name: <service-name>     // Standard service association
endpointslice.kubernetes.io/managed-by: svclink.cloudpilot.ai  // Ownership marker
```

### Filtering Logic Implementation

**Performance pattern**: Pre-compute sets once per sync cycle, pass pointers to filtering methods:
```go
// In ServiceDiscoverer.DiscoverServices():
excludedNS := clusterLink.Spec.ToExcludedNamespaceSet()
includedNS := clusterLink.Spec.ToIncludedNamespaceSet()
excludedSvcSet := clusterLink.Spec.ToExcludedServiceSet()
excludedSvcNameSet := clusterLink.Spec.ToExcludedServiceNameSet()

// O(1) lookups in ShouldExcludeNamespace/ShouldExcludeService
if clusterLink.Spec.ShouldExcludeNamespace(ns, &excludedNS, &includedNS) {
    continue
}
```

**Why**: Avoids O(n) slice scans per service; critical for clusters with hundreds of services

**Default exclusions** (hardcoded in `ToExcluded*Set()`):
- Namespace: `kube-system` (always added to excludedNS set)
- Service name: `kubernetes` (always added to excludedSvcNameSet)

### Error Handling & Graceful Degradation

**Cluster-level failures** (`clusterlink.ListClusterInfo()`):
```go
for ci := range cks.Items {
    if err := buildClient(kubeconfig); err != nil {
        // Error logged, status updated, cluster skipped
        continue
    }
}
```
- Individual cluster connection failures don't halt sync
- Status subresource updated with error details via `updateClusterStatus()`

**Service sync failures** (`controller.sync()`):
- Endpoint aggregation errors logged, sync continues
- `sliceUpdater.UpdateEndpointSlices()` processes remaining clusters

**No retries in sync loop**: Relies on periodic full sync (every 30s) to retry failed operations

### State Management

**Stateless design**: All state derived from Kubernetes API server queries each sync cycle
**ClusterInfo cache**: Built fresh each sync via `clusterlink.ListClusterInfo()` which:
- Lists all ClusterLink CRDs
- Decodes embedded base64 kubeconfigs
- Builds `kubernetes.Interface` clients for each enabled cluster
- Returns `map[string]*ClusterInfo` with cluster name → client mappings

**No persistent storage**: Controller restart fully rebuilds state from CRDs

## Integration Points

### Controller-Runtime Lifecycle

**Startup sequence** (`controller.Run()`):
1. Start Manager in goroutine → caches ClusterLink CRDs
2. Wait for Manager cache sync (`WaitForCacheSync`)
3. Start `controller.syncLoop()` → periodic service sync every 30s
4. Block on `<-ctx.Done()` until SIGTERM/SIGINT

**Simple periodic sync**: Single loop that performs full reconciliation:
- `clusterlink.ListClusterInfo()` lists all ClusterLinks and builds clients
- `serviceDiscoverer.DiscoverServices()` discovers services across all clusters
- `serviceUpdater.SyncServicesToLocalCluster()` optionally creates Service objects (if `--sync-services-to-local-cluster` flag set)
- `sliceUpdater.UpdateEndpointSlices()` creates/updates EndpointSlices per cluster

### Kubernetes API Interactions

**Main cluster** (controller-runtime client):
- Read: ClusterLinks (cluster-scoped), Services (all namespaces)
- Write: EndpointSlices (create/update/delete), ClusterLink status updates

**Remote clusters** (client-go kubernetes.Interface):
- Read-only: Services, EndpointSlices (via `kubernetes.Interface.DiscoveryV1().EndpointSlices(ns).List()`)
- RBAC required: `get/list/watch` on `services`, `endpointslices`

**Authentication**: Embedded base64 kubeconfig in ClusterLink spec → decoded in `clusterlink.ListClusterInfo()`

### Network Requirements

**Critical distinction**: Controller only syncs service metadata (endpoints), not actual traffic routing
**Pod network connectivity** required between clusters for applications to reach endpoints
**Not provided by svclink**: CNI mesh, VPN, service mesh (Istio/Linkerd)

## Testing & Validation

### Test Structure
- Unit tests: `pkg/aggregator/endpoint_aggregator_test.go`, `pkg/apis/svclink/v1alpha1/types_test.go`
- Focus areas: Endpoint aggregation logic, filtering rules (especially edge cases with inclusion/exclusion)
- Run: `make test` (executes `go test -v ./pkg/...`)

### Debugging Workflow

```bash
# 1. Check ClusterLink status
kubectl get clusterlinks  # Check Enabled, Connected, Version columns
kubectl describe clusterlink <name>  # See status.error, status.conditions

# 2. Check controller logs
kubectl logs -f deployment/svclink -n cloudpilot  # Look for "Failed to sync cluster" errors

# 3. Verify EndpointSlices created
kubectl get endpointslices -A --selector='endpointslice.kubernetes.io/managed-by=svclink.cloudpilot.ai'
kubectl describe endpointslice <service-name>-svclink-<cluster-name> -n <namespace>

# 4. Check Service endpoints
kubectl get endpoints <service-name> -n <namespace>  # Should aggregate local + remote endpoints
```

**Common issues**:
- ClusterLink shows `Connected: false` → Check kubeconfig validity, RBAC permissions
- EndpointSlices not created → Check service filtering rules, namespace inclusion/exclusion
- Endpoints empty → Verify pod network connectivity, check remote cluster EndpointSlices

## Dependencies & Constraints

**Go version**: 1.24.10 (see `go.mod` - supports go workspaces, generics)
**Kubernetes**: 1.19+ (EndpointSlice API became stable in 1.19)
**Key dependencies**:
- `sigs.k8s.io/controller-runtime` v0.22.4 - CRD reconciliation framework
- `k8s.io/client-go` v0.34.1 - Remote cluster API access
- `k8s.io/apimachinery` v0.34.1 - Scheme, runtime.Object interfaces
- `github.com/samber/lo` v1.52.0 - Functional utils (filtering, mapping)
- `github.com/spf13/cobra` v1.10.1 - CLI framework

**Tooling**:
- `ko` (optional, for image builds) - Install: `go install github.com/google/ko@latest`
- `controller-gen` (for CRD generation) - Auto-installed by `make crdgen`
- `deepcopy-gen` (for deepcopy methods) - Auto-installed by `make codegen`

**RBAC constraints**: Remote clusters only need read access to `services` and `endpointslices` (enforced by `export-kubeconfig.sh`)
