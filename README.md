# devcontainer-operator

This Kubernetes operator creates reproducible developer environments based on the [DevContainer standard](https://containers.dev/). The standardized developer environment runs in a pod on your Kubernetes cluster. You can connect your IDE (like VSCode) for remote development or use a browser-based IDE similar to GitHub Codespaces.

The current version of the devcontainer-operator supports a subset of [DevContainer standard](https://containers.dev/). Please check the [compatibility page](https://github.com/everproc/devcontainer-operator/blob/main/compatibility.md).

**IMPORTANT:** This is not production-ready software. This project is in active development.

## Quickstart

The basic installation creates the `devcontainer-operator` namespace with all resources needed to run the operator. **NOTE** If you want to build Docker images on Kubernetes the build conatainer runs as privileged. Please check your Pod Security Standards for the `devcontainer-operator` namespace.

```shell
kubectl create -f https://raw.githubusercontent.com/everproc/devcontainer-operator/main/dist/install.yaml
```

Once the operator is running, create a Workspace CR with a git repository URL that contains a [devcontainer.json](https://containers.dev/).

```
apiVersion: devcontainer.everproc.com/v1alpha1
kind: Workspace
metadata:
  name: template-starter
spec:
  gitUrl: https://github.com/devcontainers/template-starter
  gitHashOrTag: main
```

Depending on your `devcontainer.json` specified IDE you can exec into the workspace container or connect your IDE over SSH or other protocols.
