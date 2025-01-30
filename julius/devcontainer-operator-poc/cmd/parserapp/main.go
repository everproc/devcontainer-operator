package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	devcontainerv1alpha1 "everproc.com/devcontainer/api/v1alpha1"
)

type DevContainerSpec struct {
	Name    string   `json:"name"`
	Image   string   `json:"image,omitempty"`
	RunArgs []string `json:"runArgs,omitempty"`
	Build   struct {
		Dockerfile string            `json:"dockerfile"`
		Args       map[string]string `json:"args"`
	} `json:"build,omitempty"`
}

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

const DEFINITION_ENV_NAME = "DEFINITION_ENV_NAME"

func main() {
	log := zap.New()
	ctx := logr.NewContext(context.Background(), log)
	k8sResourceName := os.Getenv(DEFINITION_ENV_NAME)
	if k8sResourceName == "" {
		log.Error(errors.New("no definition name given"), "No definition name given via env vars")
		return
	}
	logDir(ctx, ".")
	file := os.Args[1]
	logDir(ctx, path.Dir(file))
	reader, err := os.Open(file)
	if err != nil {
		fmt.Println(fmt.Errorf("could not open file: %v", err))
		// TODO(juf): Replace printf with logger everywhere
		log.Error(err, fmt.Sprintf("could not open file: %v", err))
		return
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		fmt.Println(fmt.Errorf("could not open file (io.ReadAll): %v", err))
		return
	}
	devContainerSpec := &DevContainerSpec{}
	if err := json.Unmarshal(data, &devContainerSpec); err != nil {
		fmt.Println(fmt.Errorf("could not parse json (%s): %v", file, err))
		return
	}
	fmt.Println(fmt.Sprintf("%+v", devContainerSpec))

	// Step 2: Create or update the ParsedData custom resource
	dirs := strings.Split(path.Dir(file), "/")
	dir := dirs[len(dirs)-1]

	namespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	namespaceBytes, err := os.ReadFile(namespaceFile)
	if err != nil {
		fmt.Printf("Failed to get namespace name from file %s: %v\n", namespaceFile, err)
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
		fmt.Printf("Failed to load kubeconfig with InClusterConfig: %v\n", err)
		os.Exit(1)
	}

	// Add the CRD to the scheme
	if err := devcontainerv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		fmt.Printf("Failed to add CRD to scheme: %v\n", err)
		os.Exit(1)
	}

	// Create a Kubernetes client
	kubeClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Printf("Failed to create Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// Step 4: Create or update the ParsedData resource
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	targetDefinition := &devcontainerv1alpha1.Definition{}
	err = kubeClient.Get(ctx, client.ObjectKey{Name: k8sResourceName, Namespace: namespace}, targetDefinition)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Resource should exist, return error
			// Something went wrong
			log.Error(err, "Resource should exist")
			os.Exit(1)
		} else {
		}
		log.Error(err, "Error while trying to retrieve container definition")
		os.Exit(1)
	}

	targetDefinition.Name = k8sResourceName // Set the name of the resource
	targetDefinition.Namespace = namespace  // Set the namespace
	targetDefinition.Spec.Build = devcontainerv1alpha1.BuildSpec(devContainerSpec.Build)
	targetDefinition.Spec.Run.Args = orEmptySlice(devContainerSpec.RunArgs)
	targetDefinition.Spec.Image = devContainerSpec.Image
	targetDefinition.Spec.RawDefinition = string(data)
	if err := kubeClient.Update(ctx, targetDefinition); err != nil {
		fmt.Printf("Failed to create ParsedData: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created ParsedData resource")
}
