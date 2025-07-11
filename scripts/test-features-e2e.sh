#!/bin/bash
set -e

echo "Running Features E2E Tests..."

# Check if kind cluster is running
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: No Kubernetes cluster found. Please run ./scripts/setup-e2e-cluster.sh first."
    exit 1
fi

# Check if operator is deployed
if ! kubectl get deployment devcontainer-controller-manager -n devcontainer-operator &> /dev/null; then
    echo "Error: devcontainer-operator not found. Please run ./scripts/setup-e2e-cluster.sh first."
    exit 1
fi

echo "Running features e2e tests..."
cd test/e2e
go test -v -ginkgo.focus="Features Integration" -timeout=30m

echo "Features e2e tests completed!"