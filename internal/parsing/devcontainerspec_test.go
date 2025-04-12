package parsing_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"everproc.com/devcontainer/internal/parsing"
)

func TestAbc(t *testing.T) {
	const f = `{
	"image": "mcr.microsoft.com/devcontainers/javascript-node:1-18-bullseye",
	"features": {
		"ghcr.io/devcontainers/features/docker-in-docker:2": {}
	},
	"customizations": {
		"vscode": {
			"extensions": [
				"mads-hartmann.bash-ide-vscode",
				"dbaeumer.vscode-eslint"
			]
		}
	},
	"postCreateCommand": "npm install -g @devcontainers/cli",
	"mounts": [
		{
		"src": "dind-var-lib-docker",
		"target": "/var/lib/docker",
		"type": "bind"
		}
	]
}`
	x := &parsing.DevContainerSpec{}
	err := json.Unmarshal([]byte(f), &x)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}
	fmt.Printf("Unmarshal parsed: %+v", *x)
	v, err := json.Marshal(x)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}
	fmt.Printf("Marshal parsed: %s", string(v))
}

func TestParseMounts(t *testing.T) {
	const f = `{
	"image": "mcr.microsoft.com/devcontainers/javascript-node:1-18-bullseye",
	"features": {
		"ghcr.io/devcontainers/features/docker-in-docker:2": {}
	},
	"customizations": {
		"vscode": {
			"extensions": [
				"mads-hartmann.bash-ide-vscode",
				"dbaeumer.vscode-eslint"
			]
		}
	},
	"postCreateCommand": "npm install -g @devcontainers/cli",
	"mounts": [
		{
		"source": "dind-var-lib-docker",
		"destination": "/var/lib/docker",
		"type": "bind",
		"readonly": true
		},
		{
		"src": "dind-var-lib-docker",
		"dst": "/var/lib/docker",
		"type": "bind",
		"ro": true
		},
		{
		"src": "dind-var-lib-docker",
		"target": "/var/lib/docker",
		"type": "bind",
		"readonly": true
		}
	]
}`
	result := parsing.DevContainerSpec{}
	expected := parsing.Mount{
		Source:   "dind-var-lib-docker",
		Target:   "/var/lib/docker",
		Type:     "volume",
		Readonly: true,
	}
	err := json.Unmarshal([]byte(f), &result)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}

	if !reflect.DeepEqual(expected, *result.Mounts[0]) {
		t.Errorf("parsing mounts was incorrect\ngot: %v\n\nwant: %v", *result.Mounts[0], expected)
	}
	if !reflect.DeepEqual(expected, *result.Mounts[1]) {
		t.Errorf("parsing mounts was incorrect\ngot: %v\n\nwant: %v", *result.Mounts[1], expected)
	}
	if !reflect.DeepEqual(expected, *result.Mounts[2]) {
		t.Errorf("parsing mounts was incorrect\ngot: %v\n\nwant: %v", *result.Mounts[2], expected)
	}
}
