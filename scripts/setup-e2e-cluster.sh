#!/bin/bash
set -e

echo "Setting up Kind cluster with local registry for devcontainer-operator testing..."

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo "Error: kind is not installed. Please install kind first."
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed. Please install kubectl first."
    exit 1
fi

# Run the kind-docker setup script
echo "Running kind-docker setup..."
./scripts/kind-docker.sh

echo "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s

echo "Building and loading operator image..."
make docker-build IMG=example.com/devcontainer:v0.0.1
kind load docker-image example.com/devcontainer:v0.0.1

echo "Building and loading utility images..."
make load-utilities-via-kind

echo "Installing CRDs..."
make install

echo "Deploying operator..."
make deploy IMG=example.com/devcontainer:v0.0.1

echo "Waiting for operator to be ready..."
kubectl wait --for=condition=Available deployment/devcontainer-controller-manager -n devcontainer-system --timeout=300s

echo "Setup complete! You can now run e2e tests with:"
echo "  make test-e2e"
echo ""
echo "To run only features tests:"
echo "  go test ./test/e2e/ -v -ginkgo.focus=\"Features Integration\""
