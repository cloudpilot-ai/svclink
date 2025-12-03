#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Run the codegen update script.
echo "Running hack/update-codegen.sh"
./hack/update-codegen.sh

# Run the crdgen update script.
echo "Running hack/update-crdgen.sh"
./hack/update-crdgen.sh

# Check for any uncommitted changes in the repository (only if in a git repo).
if git rev-parse --git-dir > /dev/null 2>&1; then
    if ! git diff --quiet; then
        git --no-pager diff
        echo "Changes detected after running hack/update-codegen.sh and hack/updatecrdgen.sh. Please review and commit the changes."
        exit 1
    fi
    echo "No changes detected after running update scripts."
else
    echo "Not a git repository. Skipping change detection."
fi
