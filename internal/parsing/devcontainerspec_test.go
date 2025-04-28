package parsing_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"everproc.com/devcontainer/internal/parsing"
	"github.com/stretchr/testify/assert"
)

func TestParseBasicExample(t *testing.T) {
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
	// TODO(juf): Consider: either write simple snapshot library or use existing one
	assert.Equal(t, parsing.DevContainerSpec{
		Image: "mcr.microsoft.com/devcontainers/javascript-node:1-18-bullseye",
		Features: parsing.FeatureList{
			Features: []parsing.Feature{
				{Name: "ghcr.io/devcontainers/features/docker-in-docker", Version: "2", Options: map[string]any{}},
			},
		},
		PostCreateCommand: &parsing.Cmd{
			String: "npm install -g @devcontainers/cli",
		},
		Mounts: []*parsing.Mount{
			{
				Type:     "bind",
				Source:   "dind-var-lib-docker",
				Target:   "/var/lib/docker",
				Readonly: false,
			},
		},
	}, *x)
}

func TestParseFeature(t *testing.T) {
	const f = `{
    "name": "My Feature",
    "id": "myFeature",
    "version": "1.0.0",
    "dependsOn": {
        "foo:1": {
            "flag": true
        },
        "bar:1.2.3": {},
        "baz@sha256:a4cdc44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855": {}
    }
}`

	x := &parsing.FeatureSpec{}
	err := json.Unmarshal([]byte(f), &x)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}
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
		Type:     "bind",
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
