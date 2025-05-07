package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/tailscale/hujson"

	devcontainerv1alpha1 "everproc.com/devcontainer/api/v1alpha1"
	"everproc.com/devcontainer/internal/parsing"
)

func logDir(ctx context.Context, dir string) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		panic(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Error(err, "Could not read dir", "dir", dir)
		return
	}
	logger.Info(fmt.Sprintf("Listing entries for %s", dir))
	for _, e := range entries {
		logger.Info(fmt.Sprintf("Entry: %s (dir:%t)", e.Name(), e.IsDir()))
	}
}

var LabelDefinitionMapKey = devcontainerv1alpha1.SchemeBuilder.GroupVersion.Version +
	"." + devcontainerv1alpha1.SchemeBuilder.GroupVersion.Group + "/definitionID"

const DEFINITION_ENV_NAME = "DEFINITION_ENV_NAME"
const DEFINITION_ENV_ID = "DEFINITION_ENV_ID"

func main() {
	log := zap.New()
	ctx := logr.NewContext(context.Background(), log)
	k8sResourceName := os.Getenv(DEFINITION_ENV_NAME)
	if k8sResourceName == "" {
		log.Error(errors.New("no definition name given"), "No definition name given via env vars")
		return
	}
	k8sDefinitonID := os.Getenv(DEFINITION_ENV_ID)
	if k8sDefinitonID == "" {
		log.Error(errors.New("no definition id given"), "No definition id given via env vars")
		return
	}
	file := os.Args[1]
	logDir(ctx, ".")
	logDir(ctx, path.Dir("/workspace"))
	for {
		_, err := os.Stat("/workspace/.tmp/git_status/clone_done")
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				log.Info("File not there yet...")
				time.Sleep(1 * time.Second)
			} else {
				fmt.Println("Could not open tap file", err)
				os.Exit(-1)
			}
		} else {
			break
		}
	}
	gitHashReader, err := os.Open("/workspace/.tmp/git_status/clone_done")
	if err != nil {
		log.Error(err, fmt.Sprintf("could not open /workspace/.tmp/git_status/clone_done file: %v", err))
		os.Exit(-1)
		return
	}
	gitHashRaw, err := io.ReadAll(gitHashReader)
	if err != nil {
		log.Error(err, "could not read /workspace/.tmp/git_status/clone_done file: %v", err)
		os.Exit(-1)
		return
	}

	logDir(ctx, path.Dir(file))
	reader, err := os.Open(file)
	if err != nil {
		log.Error(err, fmt.Sprintf("could not open file: %v", err))
		os.Exit(-1)
		return
	}
	rawData, err := io.ReadAll(reader)
	if err != nil {
		log.Error(err, "could not open file (io.ReadAll)")
		os.Exit(-1)
		return
	}
	devContainerSpec := &parsing.DevContainerSpec{}
	// converts non-conformant JSON to conformant JSON,
	// e.g., strips comments, fixes trailing commas. For more info see hujson package.
	data, err := hujson.Standardize(rawData)
	if err != nil {
		log.Error(err, "could not standardize json")
		os.Exit(-1)
		return
	}

	if err := json.Unmarshal(data, &devContainerSpec); err != nil {
		log.Error(err, "could not parse json", "file", file, "data", string(data))
		os.Exit(-1)
		return
	}
	log.Info(fmt.Sprintf("%+v", devContainerSpec))

	// Step 2: Create or update the ParsedData custom resource
	dirs := strings.Split(path.Dir(file), "/")
	dir := dirs[len(dirs)-1]

	namespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	namespaceBytes, err := os.ReadFile(namespaceFile)
	if err != nil {
		log.Error(err, "Failed to get namespace name from file", "namespaceFile", namespaceFile)
		os.Exit(1)
	}
	namespace := string(namespaceBytes)

	orEmptySlice := func(s []string) []string {
		if s == nil {
			return make([]string, 0)
		}
		return s
	}
	if devContainerSpec.Build.Args == nil {
		devContainerSpec.Build.Args = make(map[string]string)
	}

	log.Info("Creating RD with dir and namespace", "name", dir, "namespace", namespace)

	// Step 3: Initialize a Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Failed to load kubeconfig with InClusterConfig")
		os.Exit(1)
	}

	// Create a Kubernetes client
	kubeClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Error(err, "Failed to create Kubernetes client")
		os.Exit(1)
	}

	// Step 4: Create or update the ParsedData resource
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	targetDefinition := &corev1.ConfigMap{
		BinaryData: make(map[string][]byte),
	}

	var ports []corev1.ContainerPort
	// len > 0 probably covers the nil check, remove in future
	if len(devContainerSpec.Ports) > 0 {
		ports = make([]corev1.ContainerPort, 0)
		for port, meta := range devContainerSpec.Ports {
			p, err := strconv.ParseInt(port, 10, 32)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to convert port %v to int32", port))
				os.Exit(1)
			}
			ports = append(ports, corev1.ContainerPort{Name: meta.Label, ContainerPort: int32(p)})
		}
	}
	log.Info(fmt.Sprintf("Spec has %d ports", len(ports)))
	rawData, err = json.Marshal(devContainerSpec)
	if err != nil {
		log.Error(err, "Failed to marshal spec")
		os.Exit(1)
	}

	targetDefinition.GenerateName = k8sResourceName + "-" // Set the name of the resource
	targetDefinition.Namespace = namespace                // Set the namespace
	parsedDefinition := devcontainerv1alpha1.ParsedDefinition{
		Image: devContainerSpec.Image,
		Build: devcontainerv1alpha1.BuildSpec(devContainerSpec.Build),
		Run: devcontainerv1alpha1.RunSpec{
			Args: orEmptySlice(devContainerSpec.RunArgs),
		},
		GitHash:       strings.TrimSpace(string(gitHashRaw)),
		RawDefinition: string(rawData),
		PodTpl: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "main",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports:           ports,
					},
				},
			},
		},
	}
	if devContainerSpec.Image != "" {
		parsedDefinition.PodTpl.Spec.Containers[0].Image = devContainerSpec.Image
	} // if empty, image gets injected after docker build
	targetDefinition.BinaryData["definition"], err = json.Marshal(parsedDefinition)
	attachDefinitionIDLabel(targetDefinition, k8sDefinitonID)
	if err != nil {
		log.Error(err, "Failed to json.Marshal ParsedData")
		os.Exit(1)
	}

	if err := kubeClient.Create(ctx, targetDefinition); err != nil {
		log.Error(err, "Failed to create ParsedData")
		log.Info(fmt.Sprintf("Resource: %+v", *targetDefinition))
		os.Exit(1)
	}
	log.Info("Created ParsedData resource via ConfigMap")
}

func attachDefinitionIDLabel(resource client.Object, definitionID string) {
	m := resource.GetLabels()
	if m == nil {
		m = make(map[string]string)
	}
	m[LabelDefinitionMapKey] = definitionID
	resource.SetLabels(m)
}
