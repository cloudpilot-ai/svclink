#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# For all commands, the working directory is the parent directory(repo root).
REPO_ROOT=$(cd $(dirname "${BASH_SOURCE[0]}")/.. && pwd)
cd "${REPO_ROOT}"

export GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

# Install controller-gen if not present
if ! command -v controller-gen &> /dev/null; then
    echo "Installing controller-gen..."
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
fi

echo "Generating CRD manifests with controller-gen"
controller-gen crd paths=./pkg/apis/svclink/... output:crd:dir=./config/crds
