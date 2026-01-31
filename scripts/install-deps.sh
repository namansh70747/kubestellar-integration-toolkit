#!/bin/bash
set -e

echo "📦 Installing KubeStellar Integration Toolkit dependencies..."

# Get project root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Check Go version
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or later"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.21"

echo "✅ Go version: $GO_VERSION"

# Initialize or update go.mod
echo "📝 Updating go.mod..."
go mod tidy

# Install core dependencies
echo "📦 Installing core dependencies..."
go get -u sigs.k8s.io/controller-runtime@v0.17.0
go get -u k8s.io/client-go@v0.29.0
go get -u k8s.io/apimachinery@v0.29.0
go get -u k8s.io/api@v0.29.0

# Install logging
echo "📦 Installing logging dependencies..."
go get -u github.com/go-logr/logr@v1.4.1

# Install prometheus client
echo "📦 Installing Prometheus dependencies..."
go get -u github.com/prometheus/client_golang@v1.18.0
go get -u github.com/prometheus/common@v0.45.0

# Install testing frameworks
echo "📦 Installing testing frameworks..."
go get -u github.com/stretchr/testify@v1.8.4
go get -u github.com/onsi/ginkgo/v2@v2.14.0
go get -u github.com/onsi/gomega@v1.30.0

# Install YAML support
echo "📦 Installing YAML dependencies..."
go get -u gopkg.in/yaml.v3@v3.0.1

# Install additional utilities
echo "📦 Installing additional utilities..."
go get -u github.com/spf13/cobra@latest
go get -u github.com/spf13/viper@latest

# Download all dependencies
echo "⬇️  Downloading dependencies..."
go mod download

# Verify dependencies
echo "✔️  Verifying dependencies..."
go mod verify

# Install development tools
echo "🛠️  Installing development tools..."

# controller-gen
echo "Installing controller-gen..."
go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0

# kustomize
echo "Installing kustomize..."
go install sigs.k8s.io/kustomize/kustomize/v5@latest

# golangci-lint
echo "Installing golangci-lint..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2

# goimports
echo "Installing goimports..."
go install golang.org/x/tools/cmd/goimports@latest

# mockgen (for testing)
echo "Installing mockgen..."
go install github.com/golang/mock/mockgen@latest

# setup-envtest (for integration tests)
echo "Installing setup-envtest..."
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Add GOPATH/bin to PATH
GOPATH=$(go env GOPATH)
export PATH="$PATH:$GOPATH/bin"

echo ""
echo "✅ All dependencies installed successfully!"
echo ""
echo "📋 Installed tools:"
command -v controller-gen && echo "  ✓ controller-gen: $(command -v controller-gen)"
command -v kustomize && echo "  ✓ kustomize: $(command -v kustomize)"
command -v golangci-lint && echo "  ✓ golangci-lint: $(command -v golangci-lint)"
command -v goimports && echo "  ✓ goimports: $(command -v goimports)"
command -v mockgen && echo "  ✓ mockgen: $(command -v mockgen)"
echo ""
echo "💡 Add this to your shell profile (~/.bashrc or ~/.zshrc):"
echo "   export PATH=\"\$PATH:\$(go env GOPATH)/bin\""
echo ""
echo "📚 Next steps:"
echo "  1. Run: source ~/.zshrc (or ~/.bashrc)"
echo "  2. Run: ./scripts/setup.sh"