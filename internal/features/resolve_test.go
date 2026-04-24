package features

import (
	"context"
	"os"
	"testing"
	"time"

	"everproc.com/devcontainer/internal/parsing"
	"everproc.com/devcontainer/internal/testing_util"
	"github.com/stretchr/testify/assert"
)

func Test_resolve(t *testing.T) {
	raw := "ghcr.io/duduribeiro/devcontainer-features/neovim:1"
	feat, err := parsing.ParseDependency(raw)
	assert.NoError(t, err)
	ctx, cancel := testing_util.GetTestCtxWithEnvTimeoutOrDefault(context.Background(), time.Minute)
	defer cancel()
	g := EmptyGraph()
	root := Root()
	dir, err := os.MkdirTemp(os.TempDir(), "test_resolve_")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(dir))
	}()
	cacheDir, err := getOrCreateCacheDir(dir)
	assert.NoError(t, err)
	err = resolve(ctx, &g, root, *feat, cacheDir)
	assert.NoError(t, err)

	RenderAsciiGraph(g.nodes[0])
	installationOrder := topologicalSort(root, g.nodes)
	assert.Len(t, installationOrder, 3)
	RenderInstallationOrder(installationOrder)
}
