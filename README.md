# devcontainer-operator

This Kubernetes operator creates reproducible developer environments based on the [DevContainer standard](https://containers.dev/). The standardized developer environment runs in a pod on your Kubernetes cluster. You can connect your IDE (like VSCode) for remote development or use a browser-based IDE similar to GitHub Codespaces.

The current version of the devcontainer-operator supports a subset of [DevContainer standard](https://containers.dev/). Please check the [compatibility page](https://github.com/everproc/devcontainer-operator/blob/main/compatibility.md).

**IMPORTANT:** This is not production-ready software. This project is in active development.

## Quickstart

The basic installation creates the `devcontainer-operator` namespace with all resources needed to run the operator. **NOTE** If you want to build Docker images on Kubernetes the build conatainer runs as privileged. Please check your Pod Security Standards for the `devcontainer-operator` namespace.

```shell
kubectl create -f https://raw.githubusercontent.com/everproc/devcontainer-operator/main/dist/install.yaml
```

Once the operator is running, create a K8s secret to enable the operator to push to your private registry.

```
kubectl create secret generic docker-secret \
    --from-file=.dockerconfigjson=<path/to/.docker/config.json \
    --type=kubernetes.io/dockerconfigjson
```

Add the image pull secret to the service account, so that K8s can pull the new image from your private registry.

```
kubectl patch serviceaccount default -p '{"imagePullSecrets": [{"name": "docker-secret"}]}'
```

Now create Workspace CR with a git repository URL that contains a [devcontainer.json](https://containers.dev/)

```
apiVersion: devcontainer.everproc.com/v1alpha1
kind: Workspace
metadata:
  name: template-starter
spec:
  gitUrl: https://github.com/devcontainers/template-starter
  gitHashOrTag: main
  containerRegistry: "your-private-registry-url"
  registryCredentials: "docker-secret"
```

Depending on your `devcontainer.json` specified IDE you can exec into the workspace container or connect your IDE over SSH or other protocols.
