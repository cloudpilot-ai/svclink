# Cleanup EndpointSlices Script

This script allows you to delete all EndpointSlices created by svclink.

## How it Works

The script identifies svclink-managed EndpointSlices by looking for the label `cloudpilot.ai/svclink-cluster`. All EndpointSlices created by svclink have this label attached to them.

## Usage

### Quick Start with Make

```bash
# Dry-run to see what would be deleted
make cleanup-endpointslices-dry

# Delete all svclink-managed EndpointSlices (with confirmation prompt)
make cleanup-endpointslices
```

### Direct Script Usage

```bash
# Show help
./hack/cleanup-endpointslices.sh --help

# Dry-run to see what would be deleted
./hack/cleanup-endpointslices.sh --dry-run

# Delete EndpointSlices in all namespaces (interactive confirmation)
./hack/cleanup-endpointslices.sh

# Delete EndpointSlices in a specific namespace
./hack/cleanup-endpointslices.sh -n default

# Delete in all namespaces
./hack/cleanup-endpointslices.sh -A
```

## Options

- `-n, --namespace NAMESPACE` - Delete EndpointSlices in a specific namespace
- `-A, --all-namespaces` - Delete EndpointSlices in all namespaces (default)
- `-d, --dry-run` - Show what would be deleted without actually deleting
- `-h, --help` - Show help message

## Examples

### Example 1: Dry-run to preview what will be deleted

```bash
$ ./hack/cleanup-endpointslices.sh --dry-run

Searching for svclink-managed EndpointSlices...

Found 5 svclink-managed EndpointSlice(s):

NAMESPACE   NAME                    CLUSTER    SERVICE    ENDPOINTS
test-app    web-app-cluster1       cluster1   web-app    10.244.1.2,10.244.1.3
test-app    web-app-cluster2       cluster2   web-app    10.244.2.2
default     api-cluster1           cluster1   api        10.244.1.5
default     api-cluster2           cluster2   api        10.244.2.5
prod        db-cluster1            cluster1   db         10.244.1.10

DRY RUN MODE: No resources will be deleted.

The following command would be executed:
  kubectl delete endpointslices --all-namespaces -l cloudpilot.ai/svclink-cluster
```

### Example 2: Delete EndpointSlices in a specific namespace

```bash
$ ./hack/cleanup-endpointslices.sh -n test-app

Searching for svclink-managed EndpointSlices...

Found 2 svclink-managed EndpointSlice(s):

NAME                    CLUSTER    SERVICE    ENDPOINTS
web-app-cluster1       cluster1   web-app    10.244.1.2,10.244.1.3
web-app-cluster2       cluster2   web-app    10.244.2.2

This will delete 2 EndpointSlice(s).
Are you sure you want to continue? (yes/no): yes

Deleting EndpointSlices...
endpointslice "web-app-cluster1" deleted
endpointslice "web-app-cluster2" deleted

Successfully deleted 2 svclink-managed EndpointSlice(s).
```

### Example 3: Delete all EndpointSlices across all namespaces

```bash
$ ./hack/cleanup-endpointslices.sh

Searching for svclink-managed EndpointSlices...

Found 5 svclink-managed EndpointSlice(s):

NAMESPACE   NAME                    CLUSTER    SERVICE    ENDPOINTS
test-app    web-app-cluster1       cluster1   web-app    10.244.1.2,10.244.1.3
test-app    web-app-cluster2       cluster2   web-app    10.244.2.2
default     api-cluster1           cluster1   api        10.244.1.5
default     api-cluster2           cluster2   api        10.244.2.5
prod        db-cluster1            cluster1   db         10.244.1.10

This will delete 5 EndpointSlice(s).
Are you sure you want to continue? (yes/no): yes

Deleting EndpointSlices...
endpointslice "web-app-cluster1" deleted
endpointslice "web-app-cluster2" deleted
endpointslice "api-cluster1" deleted
endpointslice "api-cluster2" deleted
endpointslice "db-cluster1" deleted

Successfully deleted 5 svclink-managed EndpointSlice(s).
```

## When to Use This Script

This script is useful in the following scenarios:

1. **Clean slate**: You want to start fresh and recreate all EndpointSlices
2. **Troubleshooting**: EndpointSlices are in an inconsistent state
3. **Migration**: Moving from one svclink version to another
4. **Testing**: Cleaning up test environments
5. **Uninstallation**: Removing all traces of svclink before uninstalling

## Safety Features

- **Dry-run mode**: Preview what will be deleted before committing
- **Interactive confirmation**: Requires explicit "yes" confirmation before deletion
- **Targeted deletion**: Uses label selector to only delete svclink-managed resources
- **Namespace scoping**: Can limit deletion to specific namespaces

## Alternative: Manual Cleanup

If you prefer to use kubectl directly, you can run:

```bash
# List all svclink-managed EndpointSlices
kubectl get endpointslices --all-namespaces -l cloudpilot.ai/svclink-cluster

# Delete all svclink-managed EndpointSlices
kubectl delete endpointslices --all-namespaces -l cloudpilot.ai/svclink-cluster

# Delete in a specific namespace
kubectl delete endpointslices -n <namespace> -l cloudpilot.ai/svclink-cluster
```

## Notes

- The script uses the label `cloudpilot.ai/svclink-cluster` to identify EndpointSlices managed by svclink
- This label is automatically added to all EndpointSlices created by svclink
- The script requires `kubectl` to be installed and configured
- The script requires `jq` to be installed for JSON parsing
- EndpointSlices will be automatically recreated by svclink controller during the next sync cycle
