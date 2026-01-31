#!/bin/bash
set -e

echo "🔧 Generating CRDs and manifests..."

# Get project root (macOS compatible)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo "📂 Working directory: $PROJECT_ROOT"

# Check if controller-gen is installed
if ! command -v controller-gen &> /dev/null; then
    echo "❌ controller-gen is not installed"
    echo "Installing controller-gen..."
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
    export PATH="$PATH:$(go env GOPATH)/bin"
fi

CONTROLLER_GEN_VERSION=$(controller-gen --version 2>&1 || echo "unknown")
echo "✅ controller-gen version: $CONTROLLER_GEN_VERSION"

# Create output directories
mkdir -p config/crd/bases
mkdir -p config/rbac
mkdir -p config/webhook
mkdir -p deploy/kustomize/base
mkdir -p deploy/helm/ksit/templates

# Generate CRDs
echo "📝 Generating CRDs..."
controller-gen crd \
    paths="./api/..." \
    output:crd:artifacts:config=config/crd/bases

if [ $? -eq 0 ]; then
    echo "✅ CRDs generated successfully"
    ls -lh config/crd/bases/
else
    echo "❌ CRD generation failed"
    exit 1
fi

# Generate deepcopy functions
echo "📝 Generating deepcopy functions..."
controller-gen object \
    paths="./api/..."

if [ $? -eq 0 ]; then
    echo "✅ Deepcopy functions generated"
else
    echo "❌ Deepcopy generation failed"
    exit 1
fi

# Generate RBAC manifests
echo "📝 Generating RBAC manifests..."
controller-gen rbac:roleName=manager-role \
    paths="./pkg/controller/..." \
    output:rbac:artifacts:config=config/rbac

if [ $? -eq 0 ]; then
    echo "✅ RBAC manifests generated"
    ls -lh config/rbac/
else
    echo "⚠️  RBAC generation failed (non-critical)"
fi

# Generate webhook manifests
echo "📝 Generating webhook manifests..."
controller-gen webhook \
    paths="./internal/webhook/..." \
    output:webhook:artifacts:config=config/webhook

if [ $? -eq 0 ]; then
    echo "✅ Webhook manifests generated"
else
    echo "⚠️  Webhook generation failed (non-critical)"
fi

# Copy CRDs to deployment locations
echo "📋 Copying CRDs to deployment locations..."

# Copy to kustomize
if [ -f config/crd/bases/*.yaml ]; then
    cp config/crd/bases/*.yaml deploy/kustomize/base/crds.yaml 2>/dev/null || true
    echo "✅ CRDs copied to deploy/kustomize/base/"
fi

# Copy to helm
if [ -f config/crd/bases/*.yaml ]; then
    cat config/crd/bases/*.yaml > deploy/helm/ksit/templates/crds.yaml
    echo "✅ CRDs copied to deploy/helm/ksit/templates/"
fi

# Validate generated files
echo "🔍 Validating generated CRDs..."
for crd in config/crd/bases/*.yaml; do
    if [ -f "$crd" ]; then
        echo "  ✓ $(basename $crd)"
        # Validate YAML syntax
        if command -v yamllint &> /dev/null; then
            yamllint -d relaxed "$crd" 2>&1 | grep -v "^$" || true
        fi
    fi
done

echo ""
echo "✅ CRD generation complete!"
echo ""
echo "📁 Generated files:"
echo "  CRDs:     config/crd/bases/"
echo "  RBAC:     config/rbac/"
echo "  Webhook:  config/webhook/"
echo "  Kustomize: deploy/kustomize/base/"
echo "  Helm:     deploy/helm/ksit/templates/"
echo ""
echo "📚 Next steps:"
echo "  1. Review CRDs: cat config/crd/bases/ksit.io_integrations.yaml"
echo "  2. Install CRDs: kubectl apply -f config/crd/bases/"
echo "  3. Or use: make install"