/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	devcontainerv1alpha1 "everproc.com/devcontainer/api/v1alpha1"
	"everproc.com/devcontainer/internal/controller/conditions"
	"everproc.com/devcontainer/internal/maps"
	"everproc.com/devcontainer/internal/parsing"
)

var WorkspacePodLabelKey = devcontainerv1alpha1.SchemeBuilder.GroupVersion.Version + "." + devcontainerv1alpha1.SchemeBuilder.GroupVersion.Group + "/workspaceRef"
var WorkspaceDeploymentExecutedPostCreateAnnotationKey = devcontainerv1alpha1.SchemeBuilder.GroupVersion.Version + "." + devcontainerv1alpha1.SchemeBuilder.GroupVersion.Group + "/executedPostCreate"

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Config *rest.Config
	Scheme *runtime.Scheme
}

var DefinitionFinalizerForRelatedWorkspacesName = devcontainerv1alpha1.SchemeBuilder.GroupVersion.Group + "/relatedWorkspaceFinalizer"

func DefinitionFinalizerForRelatedWorkspaces() (string, func(ctx context.Context, cl client.Reader, definitionID string) error) {
	fn := func(ctx context.Context, cl client.Reader, definitionID string) error {
		relatedWorkspaces := &devcontainerv1alpha1.WorkspaceList{}
		// Let's hope this works.
		// Why are we doing this?
		// We want to remove any outdated deployments when the definitionID changes
		err := cl.List(ctx, relatedWorkspaces, client.MatchingLabels{
			LabelDefinitionMapKey: definitionID,
		})
		return err
	}
	return DefinitionFinalizerForRelatedWorkspacesName, fn
}

// +kubebuilder:rbac:groups=devcontainer.everproc.com,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devcontainer.everproc.com,resources=workspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=devcontainer.everproc.com,resources=workspaces/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workspace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	instance := &devcontainerv1alpha1.Workspace{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Not found, probably deleted")
			// return no error, stops the reconciliation for this object
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Error during retrieval")
			return ctrl.Result{}, err
		}
	}
	def, err := r.getDefinition(ctx, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Definition not found, requeing", "definitionID", instance.Spec.DefinitionRef)
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	if !conditions.IsTrue(devcontainerv1alpha1.DefinitionCondTypeReady, def.Status.Conditions) {
		// Definition is not ready yet
		log.Info("Definition is not in a ready state yet", "definitionID", instance.Spec.DefinitionRef)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	src := &devcontainerv1alpha1.Source{}
	err = r.Get(ctx, types.NamespacedName{Namespace: def.Namespace, Name: def.Spec.Source}, src)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Source of Definition not found, requeing", "definitionID", instance.Spec.DefinitionRef)
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	// TODO (juf): Assert this is non-empty
	definitionID := GetDefinitionIDLabel(def)

	ownedDeployments := &appsv1.DeploymentList{}
	// Let's hope this works.
	// Why are we doing this?
	// We want to remove any outdated deployments when the definitionID changes
	err = r.List(ctx, ownedDeployments, client.MatchingFields{
		"metadata.ownerReferences.kind": instance.Kind,
		"metadata.ownerReferences.name": instance.Name,
	})
	// If size equals 0, we have a problem,
	// if size equals 1 we are already in a healthy state,
	// if size is greater than 1 we need to clean up
	var currentDeployment *appsv1.Deployment
	if len(ownedDeployments.Items) == 0 {
		pvc, err := r.ensurePVC(ctx, instance)
		if err != nil {
			log.Error(err, "Failed to create PVC")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		depl, err := r.createDeployment(instance, def.Parsed.PodTpl, &src.Spec, def, definitionID, pvc.Name)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.ensureResource(ctx, depl)
		if err != nil {
			return ctrl.Result{}, err
		}
		// we requeue after the deployment is there
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	} else if len(ownedDeployments.Items) >= 1 {
		for _, d := range ownedDeployments.Items {
			id := GetDefinitionIDLabel(&d)
			if id == "" || id != definitionID {
				err = r.Delete(ctx, &d)
				if err != nil {
					log.Error(err, "Failed to delete old deployment", "deploymentName", d.Name)
					return ctrl.Result{}, err
				}
			} else {
				currentDeployment = &d
			}
		}
	}
	if currentDeployment != nil {
		if !isDeploymentReady(currentDeployment) {
			log.Info("Deployment is not ready yet")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		log.Info("Deployment ready, let's execute any PostCreationCommands")
		v, ok := currentDeployment.Annotations[WorkspaceDeploymentExecutedPostCreateAnnotationKey]
		if !ok || v == "false" {
			// Currently "false" is an impossible value
			log.Info("Executing PostCreationCommands")
			if err := r.parseAndExecPostCreationCommands(ctx, instance, def); err != nil {
				return ctrl.Result{}, err
			}
			currentDeployment.Annotations[WorkspaceDeploymentExecutedPostCreateAnnotationKey] = "true"
			if err := r.Update(ctx, currentDeployment); err != nil {
				log.Error(err, "Failed to update deployment")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Deployment already has PostCreateCommand annotation, assuming command has been executed")
		}
	} else {
		log.Info("Deployment not there yet, waiting")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	log.Info("Workspace seems fully reconciled")
	return ctrl.Result{}, nil
}

func (r *WorkspaceReconciler) ensurePVC(ctx context.Context, inst *devcontainerv1alpha1.Workspace) (*corev1.PersistentVolumeClaim, error) {
	log := log.FromContext(ctx)
	labels := map[string]string{
		LabelDefinitionMapKey: GetDefinitionIDLabel(inst),
		WorkspacePodLabelKey:  inst.Name,
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.List(ctx, pvcList, client.MatchingLabels(labels)); err != nil {
		log.Error(err, "Failed to query for existing PVCs")
		return nil, err
	}
	if len(pvcList.Items) > 0 {
		log.Info("PVC seems to already exist, no new PVC will be created", "workspace", inst.Name)
		return nil, nil
	} else {
		log.Info("Did not found a matching PVC, let's schedule it", "workspace", inst.Name)
	}

	pvcName := names.SimpleNameGenerator.GenerateName("wkspce-")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					// TODO(juf): this should somehow be dynamic based on the git repo size
					corev1.ResourceStorage: resource.MustParse("100Mi"),
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(inst, pvc, r.Scheme); err != nil {
		return nil, err
	}
	if err := r.Create(ctx, pvc); err != nil {
		// return PVC in any case if we want to introspect it
		return pvc, err
	}
	return pvc, nil
}

func (r *WorkspaceReconciler) injectSecret(spec *devcontainerv1alpha1.SourceSpec, tpl *corev1.PodTemplateSpec) {
	if spec.GitSecret != "" {
		tpl.Spec.Volumes = append(tpl.Spec.Volumes, corev1.Volume{
			Name: "git-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  spec.GitSecret,
					DefaultMode: pointer.Int32(0600),
				},
			},
		})
		tpl.Spec.InitContainers[0].VolumeMounts = append(tpl.Spec.InitContainers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "git-secret",
			MountPath: "/root/.ssh",
			ReadOnly:  true,
		})
	}
}

func (r *WorkspaceReconciler) injectImage(def *devcontainerv1alpha1.Definition, spec *devcontainerv1alpha1.SourceSpec, tpl *corev1.PodTemplateSpec, gitHash string) {
	tpl.Spec.Containers[0].Image = fmt.Sprintf("%s/%s:%s", spec.DockerRegistry, def.Spec.Source, gitHash)
}

func (r *WorkspaceReconciler) injectPVC(pvcName, gitUrl, gitDomain, gitHash string, tpl *corev1.PodTemplateSpec) {
	// Assume there is only one
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      pvcName,
			MountPath: "/workspace",
		},
	}
	tpl.Spec.Containers[0].VolumeMounts = volumeMounts
	tpl.Spec.InitContainers = append(tpl.Spec.InitContainers, corev1.Container{
		Name:         "git-clone",
		Image:        GIT_IMAGE_NAME,
		VolumeMounts: volumeMounts,
		Env: []corev1.EnvVar{
			{
				Name:  "REPO_URL",
				Value: gitUrl,
			},
			{
				Name:  "REPO_DOMAIN",
				Value: gitDomain,
			},
			{
				Name:  "GIT_HASH_OR_BRANCH",
				Value: gitHash,
			},
		},
	})
	tpl.Spec.Volumes = []corev1.Volume{
		{
			Name: pvcName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}
}

// This is done to prevent the container from exiting and make sure the default dir is the workspace
func injectContainerOverwrites(tpl *corev1.PodTemplateSpec) {
	tpl.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "sleep infinity"}
	tpl.Spec.Containers[0].WorkingDir = "/workspace"
}

func isDeploymentReady(depl *appsv1.Deployment) bool {
	desired := *depl.Spec.Replicas
	status := depl.Status
	// Not sure if this is a good approach
	return status.Replicas == desired && status.AvailableReplicas == desired && status.ReadyReplicas == desired && status.UpdatedReplicas == desired
}

func (r *WorkspaceReconciler) parseAndExecPostCreationCommands(ctx context.Context, inst *devcontainerv1alpha1.Workspace, def *devcontainerv1alpha1.Definition) error {
	log := log.FromContext(ctx)
	selectorLabels := map[string]string{
		"app":          "devcontainer",
		"definitionID": GetDefinitionIDLabel(def),
	}
	pods := &corev1.PodList{}
	err := r.List(ctx, pods,
		client.MatchingLabels(
			selectorLabels,
		),
	)
	if err != nil {
		return err
	}

	data := parsing.DevContainerSpec{}
	err = json.Unmarshal([]byte(def.Parsed.RawDefinition), &data)
	if err != nil {
		return err
	}

	if cmd := data.PostCreateCommand; cmd != nil {
		if cmd.String != "" {
			for _, p := range pods.Items {
				podName := p.Name
				if !PodIsReadyOrFinished(&p) {
					log.Error(errors.New("should be impossible"), "Cannot execute PostCreateCommand for pod", "pod", podName)
				}
				log.Info("Executing PostCreateCommand for pod", "pod", podName)
				var command []string
				if len(cmd.Array) > 0 {
					command = cmd.Array
				} else {
					command = strings.Split(cmd.String, " ")
				}
				err := r.execPostCreationCommand(ctx, podName, "main", command, inst.Namespace)
				if err != nil {
					return err
				}
			}
		} else {
			log.Info("PostCreateCommand found, but empty")
		}
	} else {
		log.Info("No PostCreateCommand found")
	}
	return nil
}

func (r *WorkspaceReconciler) execPostCreationCommand(ctx context.Context, podName, containerName string, command []string, ns string) error {
	log := log.FromContext(ctx)
	execOpts := corev1.PodExecOptions{
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
		Container: containerName,
		Command:   command,
	}
	cs, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return err
	}
	req := cs.CoreV1().RESTClient().Post().Namespace(ns).Resource("pods").Name(podName).SubResource("exec").VersionedParams(&execOpts, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(r.Config, http.MethodPost, req.URL())
	if err != nil {
		log.Error(err, "Failed to create SDPY executor")
		return err
	}
	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
		Stdin:  nil,
	})
	if err != nil {
		log.Error(err, "Failed to exec StreamWithContext", "stderr", stderr.String())
		return err
	}
	log.Info("Completed, output:", "stdout", stdout.String())
	return nil
}

func (r *WorkspaceReconciler) createDeployment(inst *devcontainerv1alpha1.Workspace, tpl *corev1.PodTemplateSpec, spec *devcontainerv1alpha1.SourceSpec, def *devcontainerv1alpha1.Definition, definitionID, pvcName string) (*appsv1.Deployment, error) {
	selectorLabels := map[string]string{
		"app":          "devcontainer",
		"definitionID": definitionID,
	}
	selector := metav1.LabelSelector{
		MatchLabels: selectorLabels,
	}
	// TODO(juf): Debattable
	tpl.Labels = maps.UnionInPlace(tpl.Labels, selectorLabels)

	gitDomain, err := ParseGitUrl(spec.GitURL)
	if err != nil {
		return nil, err
	}

	data := parsing.DevContainerSpec{}
	err = json.Unmarshal([]byte(def.Parsed.RawDefinition), &data)
	if err != nil {
		return nil, err
	}

	r.injectPVC(pvcName, spec.GitURL, gitDomain, def.Spec.GitHashOrTag, tpl)
	r.injectSecret(spec, tpl)
	if data.Build.Dockerfile != "" {
		r.injectImage(def, spec, tpl, def.Parsed.GitHash)
	}
	injectContainerOverwrites(tpl)
	depl := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("workspace-%s-%s", inst.Spec.Owner, definitionID),
			Namespace: inst.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: *tpl,
			Selector: &selector,
		},
	}
	if err := ctrl.SetControllerReference(inst, depl, r.Scheme); err != nil {
		return nil, err
	}
	AttachDefinitionIDLabel(depl, definitionID)
	return depl, nil
}

func (r *WorkspaceReconciler) ensureResource(ctx context.Context, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)

	// Try to get the resource
	if err := r.Client.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// Resource does not exist, create it
			if err := r.Client.Create(ctx, obj); err != nil {
				return fmt.Errorf("failed to create resource: %w", err)
			}
			return nil
		}
		// Some other error occurred
		return fmt.Errorf("failed to get resource: %w", err)
	}

	// Resource already exists, no action needed
	return nil
}

func (r *WorkspaceReconciler) getDefinition(ctx context.Context, inst *devcontainerv1alpha1.Workspace) (*devcontainerv1alpha1.Definition, error) {
	log := log.FromContext(ctx)
	def := &devcontainerv1alpha1.Definition{}
	err := r.Get(ctx, types.NamespacedName{Namespace: inst.Namespace, Name: inst.Spec.DefinitionRef}, def)
	if err != nil {
		log.Error(err, "Error during retrieval")
		return nil, err
	}
	return def, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &appsv1.Deployment{}, "metadata.ownerReferences.kind", func(obj client.Object) []string {
		cm := obj.(*appsv1.Deployment)
		var kinds []string
		for _, owner := range cm.OwnerReferences {
			kinds = append(kinds, owner.Kind)
		}
		return kinds
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &appsv1.Deployment{}, "metadata.ownerReferences.name", func(obj client.Object) []string {
		cm := obj.(*appsv1.Deployment)
		var names []string
		for _, owner := range cm.OwnerReferences {
			names = append(names, owner.Name)
		}
		return names
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&devcontainerv1alpha1.Workspace{}).
		Owns(&appsv1.Deployment{}).
		Named("workspace").
		Complete(r)
}
