.PHONY: build test clean ko-build ko-push ko-deploy deploy codegen verify-gen-update crdgen

# Variables
KO_DOCKER_REPO ?= cloudpilotai
IMAGE_NAME ?= svclink
IMAGE_TAG ?= v0.1.0
CONTROLLER_BIN = bin/svclink
NAMESPACE ?= cloudpilot

# Build the controller binary
build:
	@echo "Building svclink controller..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o $(CONTROLLER_BIN) cmd/svclink/main.go
	@echo "Build complete: $(CONTROLLER_BIN)"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./pkg/...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	@echo "Clean complete"

# Install code generation tools
install-tools:
	@echo "Installing code generation tools..."
	./hack/toolchain.sh

# Generate code (deepcopy methods)
codegen:
	@echo "Generating deepcopy code..."
	./hack/update-codegen.sh

# Verify generated code is up-to-date
verify-gen-update:
	@echo "Verifying generated code..."
	./hack/verify-gen-update.sh

# Generate CRD manifests
crdgen:
	@echo "Generating CRD manifests..."
	./hack/update-crdgen.sh

# Check environment and tools for ko builds
check-env:
	@echo "Checking build environment..."
	@command -v git >/dev/null 2>&1 || { echo "Error: git is required but not installed"; exit 1; }
	@command -v date >/dev/null 2>&1 || { echo "Error: date command is required but not available"; exit 1; }
	@if [ ! -d ".git" ]; then echo "Warning: Not in a git repository, version info will be generic"; fi
	@echo "Environment check passed"

# Build and push image using ko (recommended)
ko-build: check-env
	@echo "Building image with ko..."
	@command -v ko >/dev/null 2>&1 || { echo "ko is not installed. Install it with: go install github.com/google/ko@latest"; exit 1; }
	export GIT_VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-unknown"); \
	export GIT_COMMIT=$$(git rev-parse HEAD 2>/dev/null || echo "unknown"); \
	export GIT_TREE_STATE=$$(if [ -z "$$(git status --porcelain 2>/dev/null)" ]; then echo "clean"; else echo "dirty"; fi); \
	export BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	KO_DOCKER_REPO=$(KO_DOCKER_REPO) ko build --bare ./cmd/svclink --tags=$(IMAGE_TAG),latest
	@echo "Image built and pushed: $(KO_DOCKER_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)"

# Build image locally with ko (without pushing)
ko-build-local: check-env
	@echo "Building image locally with ko..."
	@command -v ko >/dev/null 2>&1 || { echo "ko is not installed. Install it with: go install github.com/google/ko@latest"; exit 1; }
	export GIT_VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-unknown"); \
	export GIT_COMMIT=$$(git rev-parse HEAD 2>/dev/null || echo "unknown"); \
	export GIT_TREE_STATE=$$(if [ -z "$$(git status --porcelain 2>/dev/null)" ]; then echo "clean"; else echo "dirty"; fi); \
	export BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	KO_DOCKER_REPO=ko.local ko build --bare ./cmd/svclink --tags=$(IMAGE_TAG),latest --local
	@echo "Local image built: ko.local/$(IMAGE_NAME):$(IMAGE_TAG)"

# Deploy using ko (build and deploy in one step)
ko-deploy:
	@echo "Deploying with ko..."
	@command -v ko >/dev/null 2>&1 || { echo "ko is not installed. Install it with: go install github.com/google/ko@latest"; exit 1; }
	KO_DOCKER_REPO=$(KO_DOCKER_REPO) ko apply -f deploy/deployment.yaml
	@echo "Deployment complete"

# Deploy to Kubernetes (using pre-built image)
deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f config/deploy/deployment.yaml
	@echo "Deployment complete"

# Update go dependencies
deps:
	@echo "Updating dependencies..."
	go mod tidy
	go mod download
	@echo "Dependencies updated"

# Run locally (for development)
run:
	@echo "Running controller locally..."
	go run cmd/controller/main.go --kubeconfig=${HOME}/.kube/config -v=4

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Code formatted"

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run
	@echo "Linting complete"

# Install ko (if not already installed)
install-ko:
	@echo "Installing ko..."
	go install github.com/google/ko@latest
	@echo "ko installed successfully"

# Cleanup targets
.PHONY: cleanup-endpointslices cleanup-endpointslices-dry

# Cleanup all svclink-managed EndpointSlices
cleanup-endpointslices:
	@echo "Cleaning up svclink-managed EndpointSlices..."
	./hack/cleanup-endpointslices.sh

# Cleanup EndpointSlices (dry-run mode)
cleanup-endpointslices-dry:
	@echo "Dry-run: Showing svclink-managed EndpointSlices that would be deleted..."
	./hack/cleanup-endpointslices.sh --dry-run

# Show help
help:
	@echo "cloudpilot Makefile Commands"
	@echo ""
	@echo "Development:"
	@echo "  make build              - Build the controller binary"
	@echo "  make test               - Run unit tests"
	@echo "  make clean              - Clean build artifacts"
	@echo ""
	@echo "Code Generation:"
	@echo "  make install-tools      - Install code generation tools (first time setup)"
	@echo "  make codegen            - Generate deepcopy methods for CRD"
	@echo "  make verify-gen-update  - Verify generated code is up-to-date"
	@echo "  make crdgen             - Generate CRD YAML manifests from Go types"
	@echo ""
	@echo "ko (Recommended for building/deploying):"
	@echo "  make ko-build           - Build and push image with ko"
	@echo "  make ko-build-local     - Build image locally without pushing"
	@echo "  make ko-deploy          - Build and deploy in one step"
	@echo ""
	@echo "Deployment:"
	@echo "  make deploy             - Deploy using pre-built image"
	@echo "  make undeploy           - Remove deployment from cluster"
	@echo ""
	@echo "Cleanup:"
	@echo "  make cleanup-endpointslices     - Delete all svclink-managed EndpointSlices"
	@echo "  make cleanup-endpointslices-dry - Dry-run to show what would be deleted"
	@echo ""
	@echo "Variables:"
	@echo "  KO_DOCKER_REPO          - Docker registry for ko (default: cloudpilotai)"
	@echo "  IMAGE_TAG               - Image tag (default: v0.1.0)"
	@echo "  NAMESPACE               - Kubernetes namespace (default: cloudpilot)"

.DEFAULT_GOAL := help
