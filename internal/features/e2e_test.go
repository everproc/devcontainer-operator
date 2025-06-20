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
	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)
	ctx, cancel := testing_util.GetTestCtxWithEnvTimeoutOrDefault(context.Background(), time.Minute)
	defer cancel()
	g := EmptyGraph()
	root := Root()
	cacheDir, err := getOrCreateDefaultCacheDir()
	assert.NoError(t, err)
	err = resolveMany(ctx, &g, root, spec.Features.Features, cacheDir)
	assert.NoError(t, err)
	installationOrder := topologicalSort(root, g.nodes)
	emptyWkspDir, err := os.MkdirTemp(os.TempDir(), "empty_workspace")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(emptyWkspDir))
	}()
	dockerContext, err := prepareDockerBuildImageOnly(spec, installationOrder, spec.Image, cacheDir, emptyWkspDir)
	assert.NoError(t, err)
	// TODO(juf): Adjust so that this can run with podman or other OCI image builders
	cmd := exec.Command("docker", "build", "-")
	cmd.Stdin = dockerContext
	o, err := cmd.CombinedOutput()
	fmt.Println(string(o))
	assert.NoError(t, err)
}
