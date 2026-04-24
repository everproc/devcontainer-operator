# E2E Tests for DevContainer Features

This directory contains end-to-end tests for the devcontainer-operator features functionality.

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/) installed
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed
- [Docker](https://docs.docker.com/get-docker/) installed and running

## Quick Start

1. **Setup the test environment:**
   ```bash
   ./scripts/setup-e2e-cluster.sh
   ```
   This will:
   - Create a Kind cluster with local registry
   - Build and load the operator image
   - Deploy the operator
   - Install CRDs

2. **Run all e2e tests:**
   ```bash
   make test-e2e
   ```

3. **Run only features tests:**
   ```bash
   ./scripts/test-features-e2e.sh
   ```

4. **Run specific test patterns:**
   ```bash
   cd test/e2e
   go test -v -ginkgo.focus="Features Integration"
   go test -v -ginkgo.focus="single feature"
   ```

## Test Structure

### Features E2E Tests (`features_e2e_test.go`)

Tests the complete features integration workflow:

- **Single Feature Test**: Creates a workspace with one devcontainer feature
- **Multiple Features Test**: Tests workspaces with multiple features
- **Feature Options Test**: Tests features with custom configuration options
- **Dependency Resolution Test**: Tests feature dependency ordering
- **Caching Test**: Verifies feature caching works correctly
- **Error Handling Test**: Tests invalid feature references
- **Docker Context Test**: Verifies Docker build context generation
- **Installation Order Test**: Tests topological sorting of features

### Test Scenarios

Each test creates a workspace in a dedicated namespace and verifies:

1. Workspace creation succeeds
2. Definition ConfigMap is created with features data
3. Build job completes successfully
4. Deployment is created and becomes ready
5. Features are installed in correct order
6. Proper cleanup occurs

## Debugging Tests

### View test logs:
```bash
kubectl logs -n features-test -l app.kubernetes.io/name=devcontainer
```

### Check workspace status:
```bash
kubectl get workspace -n features-test
kubectl describe workspace workspace-name -n features-test
```

### Check build jobs:
```bash
kubectl get jobs -n features-test
kubectl logs job/build-job-name -n features-test
```

### Check definitions:
```bash
kubectl get configmaps -n features-test -l app.kubernetes.io/name=devcontainer
```

## Cleanup

To clean up the test environment:
```bash
kind delete cluster
```

## Adding New Tests

When adding new feature tests:

1. Follow the existing test structure in `features_e2e_test.go`
2. Use the `features-test` namespace for isolation
3. Include proper cleanup in `AfterEach` or test body
4. Test both success and failure scenarios
5. Verify all Kubernetes resources are created correctly
6. Check that features are processed in the expected order

## Common Issues

- **Timeout errors**: Increase timeout values for slow networks
- **Image pull errors**: Ensure local registry is running and accessible
- **Permission errors**: Check RBAC configuration
- **Feature resolution errors**: Verify feature references are valid and accessible