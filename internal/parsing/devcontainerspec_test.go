package parsing_test

import (
	"encoding/json"
	"fmt"
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
	"postCreateCommand": "npm install -g @devcontainers/cli"
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
	fmt.Printf("Parsed: %+v", *x)
}
