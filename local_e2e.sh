#!/bin/bash
export REGISTRY=localhost:5001
export REPOSITORY=devcontainer-operator
export IMG_TAG=0.0.7
set -exo pipefail
go mod tidy
./scripts/kind-docker.sh
kubectl wait --for=condition=Ready nodes --all --timeout=300s
make push-operator-registry
make push-utilities-registry
make deploy
kubectl wait --for=condition=Available deployment/devcontainer-controller-manager -n devcontainer-operator --timeout=300s
PROMETHEUS_INSTALL_SKIP=true CERT_MANAGER_INSTALL_SKIP=true make test-e2e
