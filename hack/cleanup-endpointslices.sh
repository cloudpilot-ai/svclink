#!/bin/bash

# cleanup-endpointslices.sh
# This script deletes all EndpointSlices created by svclink
# It identifies svclink-managed EndpointSlices by the label: cloudpilot.ai/svclink-cluster

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
LABEL_SELECTOR="cloudpilot.ai/svclink-cluster"
DRY_RUN=false
NAMESPACE=""
ALL_NAMESPACES=false

# Print usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Delete all EndpointSlices created by svclink.

OPTIONS:
    -n, --namespace NAMESPACE    Delete EndpointSlices in a specific namespace
    -A, --all-namespaces        Delete EndpointSlices in all namespaces (default)
    -d, --dry-run               Show what would be deleted without actually deleting
    -h, --help                  Show this help message

EXAMPLES:
    # Delete all svclink EndpointSlices in all namespaces (with confirmation)
    $0

    # Delete svclink EndpointSlices in a specific namespace
    $0 -n default

    # Dry run to see what would be deleted
    $0 --dry-run

    # Delete in all namespaces without confirmation
    $0 -A

EOF
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -A|--all-namespaces)
            ALL_NAMESPACES=true
            shift
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo -e "${RED}Error: Unknown option $1${NC}"
            usage
            ;;
    esac
done

# Build kubectl command
KUBECTL_CMD="kubectl get endpointslices"

if [ -n "$NAMESPACE" ]; then
    KUBECTL_CMD="$KUBECTL_CMD -n $NAMESPACE"
else
    KUBECTL_CMD="$KUBECTL_CMD --all-namespaces"
fi

KUBECTL_CMD="$KUBECTL_CMD -l $LABEL_SELECTOR"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl is not installed or not in PATH${NC}"
    exit 1
fi

# Get list of EndpointSlices
echo -e "${YELLOW}Searching for svclink-managed EndpointSlices...${NC}"
echo ""

# List EndpointSlices
SLICES=$(eval "$KUBECTL_CMD -o json" 2>/dev/null)
COUNT=$(echo "$SLICES" | jq -r '.items | length')

if [ "$COUNT" -eq 0 ]; then
    echo -e "${GREEN}No svclink-managed EndpointSlices found.${NC}"
    exit 0
fi

echo -e "${YELLOW}Found $COUNT svclink-managed EndpointSlice(s):${NC}"
echo ""

# Display EndpointSlices in a table
if [ -n "$NAMESPACE" ]; then
    eval "$KUBECTL_CMD -o custom-columns=NAME:.metadata.name,CLUSTER:.metadata.labels.cloudpilot\\.ai/svclink-cluster,SERVICE:.metadata.labels.kubernetes\\.io/service-name,ENDPOINTS:.endpoints[*].addresses"
else
    eval "$KUBECTL_CMD -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,CLUSTER:.metadata.labels.cloudpilot\\.ai/svclink-cluster,SERVICE:.metadata.labels.kubernetes\\.io/service-name,ENDPOINTS:.endpoints[*].addresses"
fi

echo ""

# Dry run mode
if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}DRY RUN MODE: No resources will be deleted.${NC}"
    echo ""
    echo "The following command would be executed:"
    if [ -n "$NAMESPACE" ]; then
        echo "  kubectl delete endpointslices -n $NAMESPACE -l $LABEL_SELECTOR"
    else
        echo "  kubectl delete endpointslices --all-namespaces -l $LABEL_SELECTOR"
    fi
    exit 0
fi

# Confirmation prompt
echo -e "${YELLOW}This will delete $COUNT EndpointSlice(s).${NC}"
read -p "Are you sure you want to continue? (yes/no): " -r
echo ""

if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo -e "${YELLOW}Operation cancelled.${NC}"
    exit 0
fi

# Delete EndpointSlices
echo -e "${YELLOW}Deleting EndpointSlices...${NC}"

if [ -n "$NAMESPACE" ]; then
    kubectl delete endpointslices -n "$NAMESPACE" -l "$LABEL_SELECTOR"
else
    kubectl delete endpointslices --all-namespaces -l "$LABEL_SELECTOR"
fi

echo ""
echo -e "${GREEN}Successfully deleted $COUNT svclink-managed EndpointSlice(s).${NC}"
