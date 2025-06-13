package features

import (
	"context"
	"fmt"
	"testing"
	"time"

	"everproc.com/devcontainer/internal/parsing"
)

func Test_resolve(t *testing.T) {
	//raw := "ghcr.io/devcontainers/features/docker-in-docker:2"
	raw := "ghcr.io/duduribeiro/devcontainer-features/neovim:1"
	// TODO(juf): Start using: https://pkg.go.dev/github.com/distribution/reference
	feat, err := parsing.ParseDependency(raw)
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	//fmt.Printf("%+v\n", *feat)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	g := EmptyGraph()
	root := Root()
	cacheDir, err := getOrCreateDefaultCacheDir()
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	err = resolve(ctx, &g, root, *feat, cacheDir)
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	for _, n := range g.nodes {
		ft := n.Data
		fmt.Printf("%s - %s - %s\n", ft.Spec.ID, ft.Spec.Name, ft.Spec.Version)
	}

	fmt.Printf("\n\n\n---------\n\n\n")
	fmt.Println(RenderAsciiGraph(g.nodes[0]))
	fmt.Printf("---------\n\n\n")

	installationOrder := topologicalSort(root, g.nodes)
	fmt.Printf("\n\n\n---------\n\n\n")
	fmt.Println(RenderInstallationOrder(installationOrder))
	fmt.Printf("---------\n\n\n")
}
