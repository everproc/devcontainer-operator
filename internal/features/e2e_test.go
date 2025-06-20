package features

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"everproc.com/devcontainer/internal/parsing"
	"everproc.com/devcontainer/internal/testing_util"
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
	ctx, cancel := testing_util.GetTestCtxWithEnvTimeoutOrDefault(context.Background(), time.Minute)
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
	emptyWkspDir, err := os.MkdirTemp(os.TempDir(), "empty_workspace")
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	defer func() {
		os.RemoveAll(emptyWkspDir)
	}()
	dockerContext, err := prepareDockerBuildImageOnly(spec, installationOrder, spec.Image, cacheDir, emptyWkspDir)
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	// TODO(juf): Adjust so that this can run with podman or other OCI image builders
	cmd := exec.Command("docker", "build", "-")
	cmd.Stdin = dockerContext
	o, err := cmd.CombinedOutput()
	fmt.Println(string(o))
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
}
