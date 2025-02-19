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
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	devcontainerv1alpha1 "everproc.com/devcontainer/api/v1alpha1"
)

var WorkspacePodLabelKey = devcontainerv1alpha1.SchemeBuilder.GroupVersion.Version + "." + devcontainerv1alpha1.SchemeBuilder.GroupVersion.Group + "/workspaceRef"

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
		return ctrl.Result{}, err
	}
	// TODO (juf): Assert this is non-empty
	definitionID := GetDefinitionIDLabel(def)

	depl := createDeployment(instance, def.Parsed.PodTpl, definitionID)
	err = r.ensureResource(ctx, depl)
	if err != nil {
		return ctrl.Result{}, err
	}
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
	if len(ownedDeployments.Items) == 0 {
		// we requeue after the deployment is there
		return ctrl.Result{Requeue: true}, nil
	} else if len(ownedDeployments.Items) > 1 {
		for _, d := range ownedDeployments.Items {
			id := GetDefinitionIDLabel(&d)
			if id == "" || id != definitionID {
				err = r.Delete(ctx, &d)
				if err != nil {
					log.Error(err, "Failed to delete old deployment", "deploymentName", d.Name)
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func createDeployment(inst *devcontainerv1alpha1.Workspace, tpl *corev1.PodTemplateSpec, definitionID string) *appsv1.Deployment {
	depl := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("workspace-%s-%s", inst.Spec.Owner, definitionID),
		},
		Spec: appsv1.DeploymentSpec{
			Template: *tpl,
		},
	}
	AttachDefinitionIDLabel(depl, definitionID)
	return depl
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
	err := r.Get(ctx, types.NamespacedName{Namespace: inst.Namespace, Name: inst.Spec.DefinitionRef}, inst)
	if err != nil {
		log.Error(err, "Error during retrieval")
		return nil, err
	}
	return def, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devcontainerv1alpha1.Workspace{}).
		Named("workspace").
		Complete(r)
}
