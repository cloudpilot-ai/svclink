#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Install the required tools for code generation

echo "Installing code generation tools for svclink..."
echo ""

echo "Installing deepcopy-gen..."
go install k8s.io/code-generator/cmd/deepcopy-gen@latest

echo "Installing controller-gen..."
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

echo ""
echo "✓ All code generation tools installed successfully!"
echo ""
echo "Installed tools:"
echo "  - deepcopy-gen (k8s.io/code-generator)"
echo "  - controller-gen (controller-tools)"
echo ""
echo "Usage:"
echo "  make codegen  - Generate deepcopy methods"
echo "  make crdgen   - Generate CRD YAML manifests"

