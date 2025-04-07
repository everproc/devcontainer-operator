package parsing_test

import (
	"encoding/json"
	"fmt"
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
	"postCreateCommand": "npm install -g @devcontainers/cli"
}`
	x := &parsing.DevContainerSpec{}
	err := json.Unmarshal([]byte(f), &x)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}
	fmt.Printf("Parsed: %+v", *x)
	v, err := json.Marshal(x)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}
	fmt.Printf("Parsed: %s", string(v))
}
