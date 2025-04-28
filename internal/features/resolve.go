package features

import (
	"context"
	"fmt"

	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"

	"everproc.com/devcontainer/internal/parsing"
)

func resolve(ctx context.Context, feature parsing.Feature) ([]string, error) {
	repo, err := remote.NewRepository(feature.Name)
	if err != nil {
		return nil, err
	}
	// TODO(juf): Find out how to pull based on hash
	store := memory.New()
	manifest, err := oras.Copy(ctx, repo, feature.Version, store, feature.Version, oras.DefaultCopyOptions)
	if err != nil {
		return nil, err
	}
	fmt.Println("Manifest Artefact Type:", manifest.ArtifactType)
	fmt.Println("Manifest Annotations:", manifest.Annotations)
	_, err = store.Resolve(ctx, feature.Version)
	if err != nil {
		return nil, err
	}

	return []string{}, nil
}
