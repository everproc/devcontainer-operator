package features

import (
	"bytes"
	"os"
	"path"
	"strings"
	"text/template"

	"everproc.com/devcontainer/internal/parsing"
)

var featureLayerTpl = template.Must(template.New("dockerfile.feature-layers.tpl").Parse(`
FROM {{ .BaseImage }}
# Devcontainer Setup
USER root
COPY . /tmp/feature-build/
RUN ls -lah /tmp/feature-build/
RUN chmod -R 0755 /tmp/feature-build/

{{.FeatureLayer}}

{{.UserDockerfile}}
`))
var singleFeatureTpl = template.Must(template.New("dockerfile.feature.tpl").Parse(`

# START {{.FeatureName}}
{{ range .Options }}
ARG _{{ .EnvName }}={{ .DefaultValue }}
ENV {{ .EnvName }}="${_{{ .EnvName }}}"
RUN echo ":{{ .EnvName }}:"
RUN echo ${{ .EnvName }}
{{ end }}
RUN /tmp/feature-build/{{.FeatureDir}}/install.sh
# END {{.FeatureName}}
`))

type optTplData struct {
	GeneratedName string
	DefaultValue  any // welp
	EnvName       string
}
type tplData struct {
	FeatureName string
	FeatureDir  string
	Options     []optTplData
}

// This method only works when there is NO custom dockerfile from the developer and only an image is specified
func PrepareDockerBuildImageOnly(spec *parsing.DevContainerSpec, installationOrder []*Node[*Feature], baseImage string, cacheDir string) (*bytes.Buffer, error) {
	dirName, err := os.MkdirTemp(os.TempDir(), "devcontainer_")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(dirName); err != nil {
			panic(err)
		}
	}()
	dockerfileFeatureLayer := &strings.Builder{}
	containerUser := spec.ContainerUser
	if containerUser == "" {
		containerUser = "root"
	}
	remoteUser := spec.RemoteUser
	if remoteUser == "" {
		remoteUser = spec.ContainerUser
	}
	for _, ft := range installationOrder {
		ftDirPath := featureCacheSubDir(cacheDir, ft.Data.Digest)
		ftDir := path.Base(ftDirPath)
		optData := []optTplData{}
		for k, v := range ft.Data.Config.Options {
			optData = append(optData, optTplData{
				DefaultValue: v,
				EnvName:      strings.ToUpper(k),
			})
		}
		// TODO(juf): This is not 100% spec-compliant, technically we need to extract the OCI artifact metadata
		// by either (a) downloading the image if it's a OCI ref or (b) by parsing the Dockerfile and taking the last USER statement
		// yay!
		optData = append(optData, optTplData{
			DefaultValue: remoteUser,
			EnvName:      "_REMOTE_USER",
		})
		optData = append(optData, optTplData{
			DefaultValue: containerUser,
			EnvName:      "_CONTAINER_USER",
		})
		err = singleFeatureTpl.Execute(dockerfileFeatureLayer, tplData{
			FeatureName: ft.Data.Name(),
			FeatureDir:  ftDir,
			Options:     optData,
		})
		if err != nil {
			panic(err)
		}
	}

	dockerfile := &strings.Builder{}

	err = featureLayerTpl.Execute(dockerfile, struct {
		BaseImage         string
		PathToFeatureData string
		FeatureLayer      string
		UserDockerfile    string
	}{
		BaseImage:         baseImage,
		PathToFeatureData: dirName,
		FeatureLayer:      dockerfileFeatureLayer.String(),
		UserDockerfile:    "",
	})
	if err != nil {
		panic(err)
	}
	dockerContext := bytes.NewBuffer(nil)
	if err := TarGzWithInjectedFile(cacheDir, []byte(dockerfile.String()), "Dockerfile", dockerContext); err != nil {
		return nil, err
	}

	return dockerContext, nil
}
