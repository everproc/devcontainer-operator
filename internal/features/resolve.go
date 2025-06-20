package features

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"slices"
	"sort"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tailscale/hujson"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"

	"everproc.com/devcontainer/internal/parsing"
)

type id interface {
	ID() string
	Name() string
}

type Graph[T id] struct {
	nodes []*Node[T]
}

func (g *Graph[T]) Get(n *Node[T]) (*Node[T], bool) {
	for _, gn := range g.nodes {
		if gn.ID() == n.ID() {
			return gn, true
		}
	}
	return nil, false
}

func (g *Graph[T]) Add(n *Node[T]) {
	if _, ok := g.Get(n); !ok {
		g.nodes = append(g.nodes, n)
	}
}

func EmptyGraph() Graph[*Feature] {
	return Graph[*Feature]{}
}

func Root() *Node[*Feature] {
	return &Node[*Feature]{
		Data:  nil,
		Leafs: nil,
	}

}

type Node[T id] struct {
	Data T
	// Notates outgoind nodes, i.e., dependencies
	Leafs  []*Node[T]
	Parent *Node[T]
}

func (t *Node[T]) ID() string {
	return t.Data.ID()
}

func (t *Node[T]) AddDependency(dep *Node[T]) {
	if t.Leafs == nil {
		t.Leafs = append(t.Leafs, dep)
		return
	}
	// Ok for small sets of nodes
	for _, l := range t.Leafs {
		if l.ID() == dep.ID() {
			return
		}
	}
	t.Leafs = append(t.Leafs, dep)
}

func (t *Node[T]) HasCycle() bool {
	// TODO(juf): Maybe use sets from k8s tools package
	visited := []string{}
	visit := func(id string) bool {
		if slices.Contains(visited, id) {
			return true
		}
		visited = append(visited, id)
		return false
	}
	remaining := []*Node[T]{}
	remaining = append(remaining, t)
	const max_iter = 99999
	i := 0
	for len(remaining) > 0 {
		i++
		if i >= max_iter {
			panic("Max iteration limit reached, this is usually because of inifite loops, i.e., bug")
		}
		n := remaining[0]
		remaining = remaining[1:] // pop_front
		if visit(n.ID()) {
			fmt.Printf("Cycle at %s, after visiting: %+v", n.ID(), visited)
			return true
		}
		remaining = append(remaining, n.Leafs...)
	}
	return false
}

type Feature struct {
	// TODO(juf): Cache Path or something
	Spec       parsing.FeatureSpec
	Config     parsing.FeatureRequest
	Digest     string
	Entrypoint string
	Installer  []byte // install.sh
	Descriptor []byte // devcontainer-feature.json
}

func (f *Feature) Name() string {
	return fmt.Sprintf("%s [%s]", f.Spec.Name, f.ID())
}

func (f *Feature) ID() string {
	s := []string{}
	for k := range f.Config.Options {
		s = append(s, k)
	}
	if len(s) > 0 {
		sort.Strings(s)
		h := sha256.New()
		for _, k := range s {
			h.Write([]byte(k))
			_, err := fmt.Fprintf(h, "%v", f.Config.Options[k])
			if err != nil {
				// Let's panic for now.
				panic(err)
			}
		}
		h.Write([]byte(f.Digest))
		return fmt.Sprintf("%x", h.Sum(nil))
	}
	return f.Digest
}

func FromParsed(p *parsing.FeatureRequest) *Feature {
	return &Feature{
		Config: *p,
	}
}

const INSTALLER_NAME = "install.sh"
const DESCRIPTOR_NAME = "devcontainer-feature.json"

func mergeOptions(user map[string]any, feature parsing.FeatureSpecOptionList) map[string]any {
	out := map[string]any{}
	// the feature is authoritative, i.e., if a key is not in the feature option map
	// we consider it as non-existing.
	// A neat thing would be to print a warning or something similar if this happens,
	// but I dont care about that right now.
	// I changed my stance on this: I will now throw a panic now
	// Felt right, might change later
	uKeys := slices.Collect(maps.Keys(user))
	fKeys := []string{}
	for fk, fv := range feature.Options {
		uv, ok := user[fk]
		if !ok && fv.Default != "" {
			out[fk] = fv.Default
		} else {
			out[fk] = uv
		}
		fKeys = append(fKeys, fk)
	}
	// TODO(juf): This whole function might be utterly inefficient, this is basically just a MAKE IT WORK implementation
	for _, uk := range uKeys {
		_, ok := slices.BinarySearch(fKeys, uk)
		if !ok {
			panic(fmt.Sprintf("error during mergeOptions: key %s is present in user configuration but is not available in the feature spec", uk))
		}
	}
	return out
}

func resolveMany(ctx context.Context, graph *Graph[*Feature], parent *Node[*Feature], features []parsing.FeatureRequest, useCacheDir string) error {
	for _, ft := range features {
		if err := resolve(ctx, graph, parent, ft, useCacheDir); err != nil {
			return err
		}
	}
	return nil
}

const DEFAULT_CACHE_DIR = "_cache"

func getOrCreateDefaultCacheDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return getOrCreateCacheDir(cwd)
}

func getOrCreateCacheDir(relPath string) (string, error) {
	cacheDir := path.Join(relPath, DEFAULT_CACHE_DIR)
	return cacheDir, createDirIfNotExists(cacheDir)
}

func featureCacheSubDir(cacheDir, digest string) string {
	return path.Join(cacheDir, fmt.Sprintf("feat_%s", digest))
}

func resolve(ctx context.Context, graph *Graph[*Feature], parent *Node[*Feature], feature parsing.FeatureRequest, cacheDir string) error {
	repo, err := remote.NewRepository(feature.Ref.Context().Name())
	if err != nil {
		return err
	}
	// TODO(juf): Find out how to pull based on hash
	store := memory.New()
	manifestDesc, err := oras.Copy(ctx, repo, feature.Ref.Identifier(), store, feature.Ref.Identifier(), oras.DefaultCopyOptions)
	if err != nil {
		return err
	}

	_, data, err := oras.FetchBytes(ctx, repo, feature.Ref.Identifier(), oras.DefaultFetchBytesOptions)
	if err != nil {
		panic(err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		panic(err)
	}

	ft := FromParsed(&feature)
	// TODO(juf): Requires validation, whether this is actually the correct digest.
	// This digest should help with this: https://containers.dev/implementors/features/#definition-feature-equality
	ft.Digest = manifestDesc.Digest.String()
	n := &Node[*Feature]{
		Data:   ft,
		Parent: parent,
	}

	ftCachePath := featureCacheSubDir(cacheDir, ft.Digest)
	if err := createDirIfNotExists(ftCachePath); err != nil {
		return err
	}

	// Iterate over layers and print info and contents
	for _, layer := range manifest.Layers {
		// Fetch the layer content
		r, err := repo.Fetch(ctx, layer)
		if err != nil {
			panic(err)
		}
		func() {
			defer MustClose(r)
			tr := tar.NewReader(r)
			for {
				header, err := tr.Next()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					panic(err)
				}
				if header == nil {
					break
				}
				p := path.Join(ftCachePath, header.Name)
				if header.FileInfo().IsDir() {
					if err := createDirIfNotExists(p); err != nil {
						panic(err)
					}
					continue
				}
				contents, err := io.ReadAll(tr)
				if err != nil {
					panic(err)
				}
				f, err := os.OpenFile(p, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0700)
				if err != nil {
					panic(err)
				}

				_, err = f.Write(contents)
				if err != nil {
					panic(err)
				}
				MustClose(f)

				name := header.FileInfo().Name()
				// TODO(juf): This is medium obsolete due to caching _all_ the files, because
				// we need them for the OCI build context anyway.
				// Except for the Spec
				switch name {
				case DESCRIPTOR_NAME:
					ft.Descriptor = contents
					val, err := hujson.Parse(ft.Descriptor)
					if err != nil {
						panic(err)
					}
					val.Standardize()
					ft.Descriptor = val.Pack()
					var spec parsing.FeatureSpec
					err = json.Unmarshal(ft.Descriptor, &spec)
					if err != nil {
						panic(err)
					}
					ft.Spec = spec
				case INSTALLER_NAME:
					ft.Installer = contents
				}
			}
		}()
	}
	// I think this is an important step
	ft.Config.Options = mergeOptions(ft.Config.Options, ft.Spec.Options)
	if c, ok := graph.Get(n); ok {
		parent.AddDependency(c)
		// end recursion, we already have this node
		return nil
	}
	parent.AddDependency(n)
	graph.Add(n)
	if len(ft.Spec.DependsOn.Features) > 0 {
		for _, dep := range ft.Spec.DependsOn.Features {
			depCtx, cancel := context.WithTimeout(ctx, time.Second*15)
			defer cancel()
			err := resolve(depCtx, graph, n, dep, cacheDir)
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}
