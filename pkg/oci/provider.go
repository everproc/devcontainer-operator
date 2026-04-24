package oci

import (
	"context"

	"everproc.com/devcontainer/internal/features"
	"everproc.com/devcontainer/internal/parsing"
)

func Prepare(ctx context.Context, spec *parsing.DevContainerSpec, workspaceDir string) (string, error) {
	return features.Prepare(ctx, spec, workspaceDir)
}
