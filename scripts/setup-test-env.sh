#!/bin/bash

set -e

echo "🔧 Setting up test environment..."

# Install setup-envtest if not present
if ! command -v setup-envtest &> /dev/null; then
    echo "📦 Installing setup-envtest..."
    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
fi

# Create bin directory
mkdir -p bin/k8s

# Download and setup envtest binaries
echo "📥 Downloading Kubernetes 1.29.x test binaries..."
ENVTEST_PATH=$(setup-envtest use 1.29.x --bin-dir ./bin/k8s -p path)

echo "✅ Envtest binaries installed at: $ENVTEST_PATH"

# Verify binaries exist
if [ ! -f "$ENVTEST_PATH/etcd" ]; then
    echo "❌ Error: etcd binary not found at $ENVTEST_PATH/etcd"
    exit 1
fi

if [ ! -f "$ENVTEST_PATH/kube-apiserver" ]; then
    echo "❌ Error: kube-apiserver binary not found at $ENVTEST_PATH/kube-apiserver"
    exit 1
fi

if [ ! -f "$ENVTEST_PATH/kubectl" ]; then
    echo "❌ Error: kubectl binary not found at $ENVTEST_PATH/kubectl"
    exit 1
fi

echo "✅ All binaries verified:"
ls -lh "$ENVTEST_PATH"

echo ""
echo "🎯 To run integration tests, use:"
echo "   export KUBEBUILDER_ASSETS=$ENVTEST_PATH"
echo "   go test ./test/integration/... -v"
echo ""
echo "Or simply run:"
echo "   make test-integration"