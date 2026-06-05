#!bash
set -exo pipefail

CONTAINER_TOOL=${CONTAINER_TOOL:-podman}

echo "Deleting kind cluster..."
kind delete cluster || true

echo "Removing local registry container..."
${CONTAINER_TOOL} rm -f kind-registry || true

echo "Teardown complete."
