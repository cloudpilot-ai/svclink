#!/bin/bash

# Export kubeconfig (base64 encoded) from current cluster for ClusterLink
# Usage: ./hack/export-kubeconfig.sh [context-name]

set -e

# Function to handle errors
handle_error() {
    echo "❌ Error on line $1"
    echo "💡 Command that failed: $2"
    exit 1
}

# Trap errors
trap 'handle_error $LINENO "$BASH_COMMAND"' ERR

# Fixed configuration
NAMESPACE="kube-system"
SERVICE_ACCOUNT="svclink-reader"
CLUSTER_ROLE="svclink-reader"

# Get context
if [ -z "$1" ]; then
    CONTEXT=$(kubectl config current-context)
    echo ">>> No context specified, using current context: $CONTEXT"
else
    CONTEXT="$1"
    echo ">>> Using specified context: $CONTEXT"
fi

# Validate context exists
if ! kubectl config get-contexts "$CONTEXT" >/dev/null 2>&1; then
    echo "❌ Error: Context '$CONTEXT' not found"
    echo "Available contexts:"
    kubectl config get-contexts
    exit 1
fi

echo "✅ Context validated: $CONTEXT"

# 1. Create ServiceAccount
echo ">>> Creating ServiceAccount: $SERVICE_ACCOUNT"
if kubectl --context="$CONTEXT" apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $SERVICE_ACCOUNT
  namespace: $NAMESPACE
EOF
then
    echo "✅ ServiceAccount created/updated successfully"
else
    echo "❌ Failed to create ServiceAccount"
    exit 1
fi

# 2. Create ClusterRole (read-only permissions)
echo ">>> Creating ClusterRole: $CLUSTER_ROLE"
if kubectl --context="$CONTEXT" apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: $CLUSTER_ROLE
rules:
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]
EOF
then
    echo "✅ ClusterRole created/updated successfully"
else
    echo "❌ Failed to create ClusterRole"
    exit 1
fi

# 3. Create ClusterRoleBinding
echo ">>> Creating ClusterRoleBinding"
if kubectl --context="$CONTEXT" apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: $CLUSTER_ROLE
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: $CLUSTER_ROLE
subjects:
- kind: ServiceAccount
  name: $SERVICE_ACCOUNT
  namespace: $NAMESPACE
EOF
then
    echo "✅ ClusterRoleBinding created/updated successfully"
else
    echo "❌ Failed to create ClusterRoleBinding"
    exit 1
fi

# 4. Get token (compatible with K8s 1.24+)
echo ">>> Getting Kubernetes version"
KUBE_VERSION=$(kubectl --context="$CONTEXT" version -o json 2>/dev/null | jq -r '.serverVersion.minor' | sed 's/[^0-9]*//g' 2>/dev/null || echo "0")

echo ">>> Detected Kubernetes version: 1.$KUBE_VERSION"

if [ "$KUBE_VERSION" -ge 24 ]; then
    echo ">>> K8s 1.24+: Creating Secret token manually"
    SECRET_NAME="${SERVICE_ACCOUNT}-token"

    # Delete old Secret if exists
    if kubectl --context="$CONTEXT" get secret "$SECRET_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
        echo ">>> Deleting existing Secret: $SECRET_NAME"
        kubectl --context="$CONTEXT" delete secret "$SECRET_NAME" -n "$NAMESPACE"
    else
        echo ">>> Secret $SECRET_NAME does not exist, creating new one"
    fi

    # Create new Secret
    echo ">>> Creating Secret: $SECRET_NAME"
    if kubectl --context="$CONTEXT" apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: $SECRET_NAME
  namespace: $NAMESPACE
  annotations:
    kubernetes.io/service-account.name: $SERVICE_ACCOUNT
type: kubernetes.io/service-account-token
EOF
    then
        echo "✅ Secret created successfully"
    else
        echo "❌ Failed to create Secret"
        exit 1
    fi

    echo ">>> Waiting for token to be populated..."
    sleep 5

    # Retry getting token with timeout
    RETRY_COUNT=0
    MAX_RETRIES=10
    while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
        TOKEN=$(kubectl --context="$CONTEXT" get secret "$SECRET_NAME" -n "$NAMESPACE" \
            -o jsonpath='{.data.token}' 2>/dev/null | base64 -d 2>/dev/null || echo "")

        if [ -n "$TOKEN" ]; then
            echo "✅ Token retrieved successfully"
            break
        fi

        echo ">>> Waiting for token... (attempt $((RETRY_COUNT + 1))/$MAX_RETRIES)"
        sleep 2
        RETRY_COUNT=$((RETRY_COUNT + 1))
    done

    if [ -z "$TOKEN" ]; then
        echo "❌ Failed to get token after $MAX_RETRIES attempts"
        echo ">>> Please check if the ServiceAccount token controller is running"
        exit 1
    fi
else
    echo ">>> K8s < 1.24: Using auto-generated token"
    sleep 3  # Wait for ServiceAccount token auto-generation

    SECRET_NAME=$(kubectl --context="$CONTEXT" get serviceaccount "$SERVICE_ACCOUNT" -n "$NAMESPACE" \
        -o jsonpath='{.secrets[0].name}' 2>/dev/null || echo "")

    if [ -z "$SECRET_NAME" ]; then
        echo "❌ No secret found for ServiceAccount $SERVICE_ACCOUNT"
        echo ">>> ServiceAccount may not have auto-generated secrets"
        exit 1
    fi

    echo ">>> Found secret: $SECRET_NAME"
    TOKEN=$(kubectl --context="$CONTEXT" get secret "$SECRET_NAME" -n "$NAMESPACE" \
        -o jsonpath='{.data.token}' | base64 -d)

    if [ -z "$TOKEN" ]; then
        echo "❌ Failed to extract token from secret $SECRET_NAME"
        exit 1
    fi

    echo "✅ Token retrieved successfully"
fi

# 5. Get cluster information
echo ">>> Getting cluster information"

CLUSTER_NAME=$(kubectl --context="$CONTEXT" config view -o jsonpath="{.contexts[?(@.name==\"$CONTEXT\")].context.cluster}" 2>/dev/null || echo "")
if [ -z "$CLUSTER_NAME" ]; then
    echo "❌ Failed to get cluster name for context $CONTEXT"
    exit 1
fi
echo ">>> Cluster name: $CLUSTER_NAME"

CLUSTER_SERVER=$(kubectl --context="$CONTEXT" config view -o jsonpath="{.clusters[?(@.name==\"$CLUSTER_NAME\")].cluster.server}" 2>/dev/null || echo "")
if [ -z "$CLUSTER_SERVER" ]; then
    echo "❌ Failed to get cluster server for cluster $CLUSTER_NAME"
    exit 1
fi
echo ">>> Cluster server: $CLUSTER_SERVER"

CLUSTER_CA=$(kubectl --context="$CONTEXT" config view --raw -o jsonpath="{.clusters[?(@.name==\"$CLUSTER_NAME\")].cluster.certificate-authority-data}" 2>/dev/null || echo "")
if [ -z "$CLUSTER_CA" ]; then
    echo "❌ Failed to get cluster certificate authority data"
    exit 1
fi
echo "✅ Cluster information retrieved successfully"

# 6. Generate kubeconfig and output base64
echo ">>> Generating kubeconfig"
echo ""
echo "==========================================="
echo "✅ SUCCESS: Base64 Kubeconfig Generated"
echo "==========================================="
echo ""
echo "Copy the following base64 string to use in ClusterLink spec.kubeconfig:"
echo ""

cat <<EOF | base64 -w 0
apiVersion: v1
kind: Config
clusters:
- name: $CLUSTER_NAME
  cluster:
    server: $CLUSTER_SERVER
    certificate-authority-data: $CLUSTER_CA
contexts:
- name: $SERVICE_ACCOUNT@$CLUSTER_NAME
  context:
    cluster: $CLUSTER_NAME
    user: $SERVICE_ACCOUNT
    namespace: default
current-context: $SERVICE_ACCOUNT@$CLUSTER_NAME
users:
- name: $SERVICE_ACCOUNT
  user:
    token: $TOKEN
EOF
