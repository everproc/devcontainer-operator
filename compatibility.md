# Development Container Specification Compatibility

This page documents devcontainer standard supported by the devcontainer-operator. For reference documentation on any given feature, see the [corresponding documentation](https://containers.dev/implementors/json_reference/).

## General devcontainer.json properties

| Property | Supported |
| ------------- | ------------- |
| `name` | :white_check_mark: |
| `forwardPorts` | |
| `portsAttributes` | |
| `otherPortsAttributes` | |
| `containerEnv` | |
| `remoteEnv` | |
| `remoteUser` | |
| `containerUser` | |
| `updateRemoteUserUID` | |
| `userEnvProbe` | |
| `overrideCommand` | |
| `shutdownAction` | |
| `init` | |
| `privileged` | |
| `capAdd` | |
| `securityOpt` | |
| `mounts` | :white_check_mark: |
| `features` | :construction: |
| `overrideFeatureInstallOrder` | |
| `customizations` | |

## Scenario specific properties

| Property | Supported |
| ------------- | ------------- |
| `image` | :white_check_mark: |
| `build.dockerfile` | :white_check_mark: |
| `build.context` | |
| `build.args` | |
| `build.options` | |
| `build.target` | |
| `build.cacheFrom` | |
| `appPort` | |
| `workspaceMount` | |
| `workspaceFolder` | |
| `runArgs` | |

## Lifecycle scripts

| Property | Supported |
| ------------- | ------------- |
| `initializeCommand` | |
| `onCreateCommand` | |
| `updateContentCommand` | |
| `postCreateCommand` | :white_check_mark: |
| `postStartCommand` | |
| `postAttachCommand` | |
| `waitFor` | |

## Minimum host requirements

| Property | Supported |
| ------------- | ------------- |
| `hostRequirements.cpus` | |
| `hostRequirements.memory` | |
| `hostRequirements.storage` | |
| `hostRequirements.gpu` | |


## Port attributes

| Property | Supported |
| ------------- | ------------- |
| `label` | |
| `protocol` | |
| `onAutoForward` | |
| `requireLocalPort` | |
| `elevateIfNeeded` | |
