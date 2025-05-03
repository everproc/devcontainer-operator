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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	devcontainerv1alpha1 "everproc.com/devcontainer/api/v1alpha1"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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

	var workspace devcontainerv1alpha1.Workspace
	err := r.Get(ctx, req.NamespacedName, &workspace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Workspace not found, probably deleted")
			// return no error, stops the reconciliation for this object
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get workspace")
		return ctrl.Result{}, err
	}
	// Let's just set the status as Unknown when no status is available
	if workspace.Status.Conditions == nil || len(workspace.Status.Conditions) == 0 {
		changed := meta.SetStatusCondition(&workspace.Status.Conditions, metav1.Condition{Type: devcontainerv1alpha1.WorkspaceCondTypePending, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if changed {
			if err = r.Status().Update(ctx, &workspace); err != nil {
				log.Error(err, "Failed to update workspace status")
				return ctrl.Result{}, err
			}

			// Let's re-fetch the workspace Custom Resource after updating the status
			// so that we have the latest state of the resource on the cluster
			if err := r.Get(ctx, req.NamespacedName, &workspace); err != nil {
				log.Error(err, "Failed to re-fetch workspace")
				return ctrl.Result{}, err
			}
		}
	}

	sourceList := &devcontainerv1alpha1.SourceList{}
	err = r.List(ctx, sourceList, client.MatchingLabels{
		"app.kubernetes.io/name": "devcontainer",
		"ownerReferenceName":     workspace.Name,
		"ownerReferenceUID":      string(workspace.UID),
	})
	if err != nil {
		log.Error(err, "Failed to list owned sources")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	var source devcontainerv1alpha1.Source
	if len(sourceList.Items) == 0 {
		// create source cr
		source.Name = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", workspace.Name))
		source.Namespace = workspace.Namespace
		source.Labels = map[string]string{
			"app.kubernetes.io/name": "devcontainer",
			// for fast list query
			"ownerReferenceName": workspace.Name,
			"ownerReferenceUID":  string(workspace.UID),
		}
		source.Spec.GitURL = workspace.Spec.GitURL
		source.Spec.GitSecret = workspace.Spec.GitSecret
		source.Spec.ContainerRegistry = workspace.Spec.ContainerRegistry
		source.Spec.RegistryCredentials = workspace.Spec.RegistryCredentials

		if err := ctrl.SetControllerReference(&workspace, &source, r.Scheme); err != nil {
			log.Error(err, "Failed to set reference, source owned by workspace ")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the workspace Custom Resource after updating the status
		// so that we have the latest state of the resource on the cluster
		// if err := r.Get(ctx, req.NamespacedName, &workspace); err != nil {
		// 	log.Error(err, "Failed to re-fetch workspace")
		// 	return ctrl.Result{}, err
		// }

		if err := r.Create(ctx, &source); err != nil {
			return ctrl.Result{}, fmt.Errorf("Failed to create source resource: %w", err)
		}
	}
	if len(sourceList.Items) == 1 {
		source = sourceList.Items[0]
	}
	if len(sourceList.Items) > 1 {
		log.Error(err, "Cleanup needed, two or more sources are owned by the workspace: %s", workspace.Name)
		return ctrl.Result{}, err
	}

	defList := &devcontainerv1alpha1.DefinitionList{}
	err = r.List(ctx, defList, client.MatchingLabels{
		"app.kubernetes.io/name": "devcontainer",
		"ownerReferenceName":     workspace.Name,
		"ownerReferenceUID":      string(workspace.UID),
	})
	if err != nil {
		log.Error(err, "Failed to list owned definitions")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	var def devcontainerv1alpha1.Definition
	if len(defList.Items) == 0 {
		// create definition cr
		def.Name = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", workspace.Name))
		def.Namespace = workspace.Namespace
		def.Labels = map[string]string{
			"app.kubernetes.io/name": "devcontainer",
			// for fast list query
			"ownerReferenceName": workspace.Name,
			"ownerReferenceUID":  string(workspace.UID),
		}
		def.Spec.Source = source.Name
		def.Spec.GitHashOrTag = workspace.Spec.GitHashOrTag
		if workspace.Spec.StorageClassName != "" {
			def.Spec.StorageClassName = workspace.Spec.StorageClassName
		}

		if err := ctrl.SetControllerReference(&workspace, &def, r.Scheme); err != nil {
			log.Error(err, "Failed to set reference, definition owned by workspace ")
			return ctrl.Result{}, err
		}

		// if err := r.Get(ctx, req.NamespacedName, &workspace); err != nil {
		// 	log.Error(err, "Failed to re-fetch workspace")
		// 	return ctrl.Result{}, err
		// }

		if err := r.Create(ctx, &def); err != nil {
			return ctrl.Result{}, fmt.Errorf("Failed to create definition resource: %w", err)
		}
	}
	if len(defList.Items) == 1 {
		def = defList.Items[0]
	}
	if len(defList.Items) > 1 {
		log.Error(err, "Cleanup needed, two or more definitions are owned by the workspace: %s", workspace.Name)
		return ctrl.Result{}, err
	}

	builderList := &devcontainerv1alpha1.BuilderList{}
	err = r.List(ctx, builderList, client.MatchingLabels{
		"app.kubernetes.io/name": "devcontainer",
		"ownerReferenceName":     workspace.Name,
		"ownerReferenceUID":      string(workspace.UID),
	})
	if err != nil {
		log.Error(err, "Failed to list owned builders")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if len(builderList.Items) == 0 {
		// create builder cr
		var builder devcontainerv1alpha1.Builder
		builder.Name = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", workspace.Name))
		builder.Namespace = workspace.Namespace
		builder.Labels = map[string]string{
			"app.kubernetes.io/name": "devcontainer",
			// for fast list query
			"ownerReferenceName": workspace.Name,
			"ownerReferenceUID":  string(workspace.UID),
		}
		if workspace.Spec.Owner == "" {
			builder.Spec.Owner = workspace.Name
		} else {
			builder.Spec.Owner = workspace.Spec.Owner
		}
		builder.Spec.DefinitionRef = def.Name
		if workspace.Spec.StorageClassName != "" {
			builder.Spec.StorageClassName = workspace.Spec.StorageClassName
		}

		if err := ctrl.SetControllerReference(&workspace, &builder, r.Scheme); err != nil {
			log.Error(err, "Failed to set reference, builder owned by workspace ")
			return ctrl.Result{}, err
		}

		// if err := r.Get(ctx, req.NamespacedName, &workspace); err != nil {
		// 	log.Error(err, "Failed to re-fetch workspace")
		// 	return ctrl.Result{}, err
		// }

		if err := r.Create(ctx, &builder); err != nil {
			return ctrl.Result{}, fmt.Errorf("Failed to create builder resource: %w", err)
		}
	}
	if len(builderList.Items) > 1 {
		log.Error(err, "Cleanup needed, two or more builders are owned by the workspace: %s", workspace.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devcontainerv1alpha1.Workspace{}).
		Named("workspace").
		Complete(r)
}
