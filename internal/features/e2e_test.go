package features

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"everproc.com/devcontainer/internal/parsing"
)

func Test_parse_and_build_docker_image(t *testing.T) {
	rawSpec := `{
    "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
    "features": {
        "ghcr.io/duduribeiro/devcontainer-features/neovim:1": {
            "version": "stable"
        }
    },
	"remoteUser": "node"
}`
	spec := &parsing.DevContainerSpec{}
	err := json.Unmarshal([]byte(rawSpec), &spec)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	g := EmptyGraph()
	root := Root()
	cacheDir, err := getOrCreateDefaultCacheDir()
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	err = resolveMany(ctx, &g, root, spec.Features.Features, cacheDir)
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	installationOrder := topologicalSort(root, g.nodes)
	dockerContext, err := PrepareDockerBuildImageOnly(spec, installationOrder, spec.Image, cacheDir)
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	cmd := exec.Command("docker", "build", "-")
	cmd.Stdin = dockerContext
	o, err := cmd.CombinedOutput()
	fmt.Println(string(o))
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
}
