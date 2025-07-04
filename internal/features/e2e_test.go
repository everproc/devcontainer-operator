package features

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/assert"

	"everproc.com/devcontainer/internal/parsing"
	"everproc.com/devcontainer/internal/testing_util"
)

func Test_parse_and_build_docker_image(t *testing.T) {
	emptyWkspDir, err := os.MkdirTemp(os.TempDir(), "empty_workspace")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(emptyWkspDir))
	}()
	_, err = git.PlainClone(emptyWkspDir, &git.CloneOptions{
		URL: "https://github.com/daemonfire300/sample-jupyter-devcontainer.git",
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	spec := &parsing.DevContainerSpec{}
	rawSpec, err := os.ReadFile(path.Join(emptyWkspDir, ".devcontainer/devcontainer.json"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	err = json.Unmarshal(rawSpec, &spec)
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
	f, err := os.OpenFile(path.Join(emptyWkspDir, "sample.txt"), os.O_CREATE|os.O_WRONLY, 0655)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer MustClose(f)
	_, err = f.WriteString("dummy")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	dockerContext, err := prepareDockerBuild(spec, installationOrder, cacheDir, emptyWkspDir)
	assert.NoError(t, err)
	// TODO(juf): Adjust so that this can run with podman or other OCI image builders
	cmd := exec.Command("docker", "build", "-")
	cmd.Stdin = dockerContext
	o, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.NotEmpty(t, o)
	fmt.Println(string(o))
}
