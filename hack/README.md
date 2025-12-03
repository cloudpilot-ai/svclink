# hack/ - Code Generation and Tooling Scripts

This directory contains code generation and verification scripts for the svclink project.

## Directory Structure

```text
hack/
├── README.md              # This document
├── boilerplate.go.txt     # Code file header template
├── toolchain.sh           # Install code generation tools
├── utils.sh               # Common utility functions
├── update-codegen.sh      # Generate deepcopy code
├── update-crdgen.sh       # Generate CRD YAML
└── verify-gen-update.sh   # Verify generated code
```

## Quick Start

### Initial Setup

If using code generation tools for the first time, install dependencies:

```bash
./hack/toolchain.sh
```

This will install:

- `deepcopy-gen` - Generate DeepCopy methods
- `controller-gen` - Generate CRD manifests

### Generate Code

```bash
# Generate deepcopy methods
make codegen

# Generate CRD YAML
make crdgen

# Verify generated code is up-to-date
make verify-gen-update
```

## Script Descriptions

### toolchain.sh

Installs tools required for code generation.

**Purpose:**

- Install `deepcopy-gen` for generating DeepCopy methods
- Install `controller-gen` for generating CRD YAML

**Usage:**

```bash
./hack/toolchain.sh
```

**Note:** Individual generation scripts will auto-install missing tools, but it's recommended to run this manually on first use.

### update-codegen.sh

Generates deepcopy methods required by Kubernetes code generators.

**Purpose:**

- Generate `zz_generated.deepcopy.go` for `ClusterLink` CRD
- Implement DeepCopy methods required by `runtime.Object` interface
- Use standard GOPATH structure for code generation

**Usage:**

```bash
./hack/update-codegen.sh
```

**Dependencies:**

- `k8s.io/code-generator/cmd/deepcopy-gen`

Script will auto-install missing tools.

**How it Works:**

1. Create temporary GOPATH structure (`_go/`)
2. Symlink project to standard Go package path
3. Run `deepcopy-gen` to generate code
4. Clean up temporary files

### update-crdgen.sh

Generates CRD YAML manifest files from Go type definitions.

**Purpose:**

- Use `controller-gen` to generate CRDs from Go structs
- Output to `deploy/crd.yaml`
- Include OpenAPI v3 schema validation

**Usage:**

```bash
./hack/update-crdgen.sh
```

**Dependencies:**

- `sigs.k8s.io/controller-tools/cmd/controller-gen`

Script will auto-install missing tools.

### verify-gen-update.sh

Verifies that generated code is up-to-date.

**Purpose:**

- Check generated code needs updating in CI/CD
- Ensure committed code includes latest generated files

**Usage:**

```bash
./hack/verify-gen-update.sh
```

Returns 0 on success, 1 on failure.

## Workflow

### After Modifying CRD Definitions

1. Edit `pkg/apis/svclink/v1alpha1/types.go`
2. Run code generation:

   ```bash
   ./hack/update-codegen.sh
   ```

3. To update CRD YAML:

   ```bash
   ./hack/update-crdgen.sh
   ```

4. Verify generation:

   ```bash
   ./hack/verify-gen-update.sh
   ```

5. Commit changes (including generated files)

### CI Integration

Add verification step to CI workflow:

```yaml
# .github/workflows/ci.yml
- name: Verify generated code
  run: ./hack/verify-gen-update.sh
```

## File Structure

```text
hack/
├── README.md              # This file
├── update-codegen.sh      # Generate deepcopy code
├── update-crdgen.sh       # Generate CRD YAML
├── verify-gen-update.sh   # Verify generated code
└── boilerplate.go.txt     # Copyright header template (auto-generated)
```

## Common Issues

### Tool Installation Fails

If automatic installation fails, install manually:

```bash
# Install deepcopy-gen
go install k8s.io/code-generator/cmd/deepcopy-gen@latest

# Install controller-gen
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
```

### Generated Files in Wrong Location

Scripts automatically move generated files to the correct location. If this fails:

1. Check `REPO_ROOT` environment variable
2. Ensure running scripts from project root or `hack/` directory

### DeepCopy Method Conflicts

If you manually wrote deepcopy methods, the code generator will overwrite them. Solutions:

1. Remove manually written methods
2. Use `// +k8s:deepcopy-gen=false` tag for types that don't need generation

## References

- [Kubernetes Code Generator](https://github.com/kubernetes/code-generator)
- [Controller Tools](https://github.com/kubernetes-sigs/controller-tools)
- [Kubebuilder Book](https://book.kubebuilder.io/)
