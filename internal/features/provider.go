package features

import (
	"context"
	"io"
	"os"
	"path"

	"everproc.com/devcontainer/internal/parsing"
)

func Prepare(ctx context.Context, spec *parsing.DevContainerSpec, workspaceDir string) (string, error) {
	g := EmptyGraph()
	root := Root()
	ctxDir := path.Join(workspaceDir, ".docker_context")
	if err := createDirIfNotExists(ctxDir); err != nil {
		return "", err
	}
	cacheDir, err := getOrCreateCacheDir(workspaceDir)
	if err != nil {
		return "", err
	}
	err = resolveMany(ctx, &g, root, spec.Features.Features, cacheDir)
	if err != nil {
		return "", err
	}
	installationOrder := topologicalSort(root, g.nodes)
	dockerContext, err := prepareDockerBuild(spec, installationOrder, cacheDir, workspaceDir)
	if err != nil {
		return "", err
	}
	ctxFilePath := path.Join(ctxDir, "context.tgz")
	f, err := os.OpenFile(ctxFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, dockerContext)
	if err != nil {
		return "", err
	}
	return ctxFilePath, nil
}
