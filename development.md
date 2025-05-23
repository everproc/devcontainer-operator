# devcontainer-operator development

## Description
This controller provisions devcontainer-based pods on demand.

The current version of the devcontainer-operator supports a subset of [DevContainer standard](https://containers.dev/). Please check the [compatibility page](https://github.com/everproc/devcontainer-operator/blob/main/compatibility.md).

The repository that owns the .devcontainer will _always_ be mounted at `/workspace`.

```mermaid
---
title: CRD Structure
---
erDiagram
    Workspace ||--|| Definition : has
    Workspace ||--|| Deployment : produces

	Definition {
		RawSpec json
		PodTemplateSpec *
	}
	Workspace {
    GitURL string
		GitSecret string
    ContainerRegistry string
    RegistryCredentials string
		GitHashOrTag string
		Owner string
    StorageClassName string
	}
```


```mermaid
---
title: Flow
---
flowchart
	PVC@{ shape: lin-rect, label: "Create (PVC)" }
	Clone@{ label : Git Clone Pod (into PVC) (Container/1) }
	Parse@{ label : Parse Devcontainer JSON (using same PVC) (Container/2) }

	subgraph Setup
		Definition -->|Operator reads from config map and updates Definition|ConfigMap
		Definition --> PVC
		PVC --> SetupPod
		subgraph SetupPod
			Clone --> Parse
			Parse -->|ParsePod creates config map|ConfigMap
		end

	end
	subgraph WorkspaceCreation
		Workspace -->|Read PodSpecTpl|Definition
		Workspace -->|Modify and Apply PodSpecTpl|Deployment
		Workspace -->|Run PostCreationCommands|Deployment
	end



```

## Getting Started

**NOTE:** Parts of the README are still from the kubebuilder boilerplate keep this in mind.

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

If you are running this locally using kind: run `make build-and-push-utilities` before you attempt to create any resources.
If you are running this locally not using kind: run `make build-utilities`, you then need to make sure that the resulting two images are put somewhere where the cluster can pull it from.
Currently you might need to even adjust the code that defines the images if you want to pull it from a specific registry, otherwise it might try to pull it from dockerhub.
This is very alpha and will be configurable in the future

### To Deploy on the cluster

(Not recommended for testing out / local development)

Use `make run` instead for testing/local development instead (requires CRD Install section first).

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/devcontainer:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

To re-install (deletes CRD and re-applies them).

```sh
make reinstall
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/devcontainer:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

Currently this will create two sources, with each having one definition and each one workspace.
The final result should be each workspace having a deployment with one pod that you can exec into / attach VSCode to.

```sh
kubectl apply -k config/samples/
```

This will add two sources and a definition for each using a different repository.
In addition to that it spawns two workspaces for the user `julius`.
The end result will be two deployments with each `replica=1` to which you can attach VSCode (to the pod/container of the deployment).

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/devcontainer:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/devcontainer/<tag or branch>/dist/install.yaml
```
