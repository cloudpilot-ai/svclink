#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# For all commands, the working directory is the parent directory(repo root).
REPO_ROOT=$(cd $(dirname "${BASH_SOURCE[0]}")/.. && pwd)
cd "${REPO_ROOT}"

export GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

boilerplate="${REPO_ROOT}"/hack/boilerplate.go.txt

# Create boilerplate if not exists
if [ ! -f "${boilerplate}" ]; then
    cat > "${boilerplate}" <<EOF
/*
Copyright 2025 CloudPilot AI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
EOF
fi

go_path="${REPO_ROOT}/_go"
cleanup() {
  chmod -R u+w "${go_path}" 2>/dev/null || true
  rm -rf "${go_path}"
}
trap "cleanup" EXIT SIGINT

cleanup

source "${REPO_ROOT}"/hack/utils.sh
utils::create_gopath_tree "${REPO_ROOT}" "${go_path}"
export GOPATH="${go_path}"

# Install deepcopy-gen if not present
if ! command -v deepcopy-gen &> /dev/null; then
    echo "Installing deepcopy-gen..."
    go install k8s.io/code-generator/cmd/deepcopy-gen@latest
fi

echo "Generating with deepcopy-gen"
cd "${REPO_ROOT}/pkg/apis/svclink/v1alpha1"
deepcopy-gen \
  --go-header-file "${boilerplate}" \
  --output-file zz_generated.deepcopy.go \
  --bounding-dirs github.com/cloudpilot-ai/svclink/pkg/apis/svclink/v1alpha1 \
  .

cd "${REPO_ROOT}"
echo "✓ Code generation complete!"
