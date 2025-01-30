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
	"crypto/sha1"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	devcontainerv1alpha1 "everproc.com/devcontainer/api/v1alpha1"
)

const UtilRoleName = "devcontainer-utility"
const UtilRoleBindingName = UtilRoleName
const UtilServiceAccountName = UtilRoleName

var LabelDefinitionMapKey = strings.ReplaceAll(devcontainerv1alpha1.SchemeBuilder.GroupVersion.String()+"/definitionID", "/", ".")

func init() {
	res := validation.IsQualifiedName(LabelDefinitionMapKey)
	if res != nil {
		panic(fmt.Sprintf("Invalid LabelDefinitionMapKey: %v", res))
	}
}

// DefinitionReconciler reconciles a Definition object
type DefinitionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=devcontainer.everproc.com,resources=definitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devcontainer.everproc.com,resources=definitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=devcontainer.everproc.com,resources=definitions/finalizers,verbs=update

func (r *DefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	instance := &devcontainerv1alpha1.Definition{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Not found, probably deleted")
			// return no error, stops the reconciliation for this object
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get instance")
		return ctrl.Result{}, err
	} // Let's just set the status as Unknown when no status is available
	if instance.Status.Conditions == nil || len(instance.Status.Conditions) == 0 {
		r.updateStatusMany(ctx, req.NamespacedName, instance, devcontainerv1alpha1.InitialConditionsDefinition())
	}

	src := &devcontainerv1alpha1.Source{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Spec.Source, Namespace: instance.Namespace}, src)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.updateStatus(ctx, req.NamespacedName, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeRemoteCloned, Status: metav1.ConditionFalse, Reason: "MissingSource", Message: fmt.Sprintf("Source %q is missing", instance.Spec.Source)}); err != nil {
				return ctrl.Result{}, err
			}
			// Try again
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	definitionID := definitionID(instance, src)

	pvc := &corev1.PersistentVolumeClaim{}
	err = r.Get(ctx, types.NamespacedName{Name: r.pvcName(instance), Namespace: instance.Namespace}, pvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Not found, let's schedule a PVC for creation")
			if err := r.updateStatus(ctx, req.NamespacedName, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeRemoteCloned, Status: metav1.ConditionUnknown, Reason: "ProvisioningPVC", Message: "Provisioning PVC"}); err != nil {
				return ctrl.Result{}, err
			}
			pvc, err := r.pvcForGitRepo(instance)
			if err != nil {
				log.Error(err, "Failed to construct PVC spec")
				return ctrl.Result{}, err
			}
			attachDefinitionIDLabel(pvc, definitionID)
			err = r.Create(ctx, pvc)
			if err != nil {
				log.Error(err, "Failed to create PVC")
				return ctrl.Result{}, err
			}
			// TODO (juf): make configurable
			return ctrl.Result{RequeueAfter: 20 * time.Second}, nil
		} else {
			log.Error(err, "Failed to get pvc info")
			return ctrl.Result{}, err
		}
	}

	// TODO (juf): The PVC will not be bound until we have a pod assigned to it and no PV will be there based on the storageclass,
	// e.g., on kind (local) the behaviour is something like WaitForConsumer. Therefore this code needs to also handle this
	// Ignore PVC/PV statuses for now
	//if pvc.Status.Phase != corev1.ClaimBound {
	//	pv := &corev1.PersistentVolume{}
	//	err = r.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName, Namespace: instance.Namespace}, pv)
	//	if apierrors.IsNotFound(err) {
	//		log.Info("PV Not found")
	//		if err := r.updateStatus(ctx, req.NamespacedName, instance, metav1.Condition{Type: typeAvailable, Status: metav1.ConditionUnknown, Reason: "ProvisioningPV", Message: "Provisioning PV"}); err != nil {
	//			return ctrl.Result{}, err
	//		}
	//		// TODO (juf): make configurable
	//		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	//	} else {
	//		log.Error(err, "Failed to get PV info")
	//		return ctrl.Result{}, err
	//	}
	//}

	// PVC is ready? I guess so
	clonePod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: r.clonePodName(instance), Namespace: instance.Namespace}, clonePod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Not found, let's schedule the clone pod for creation")
			if err := r.updateStatus(ctx, req.NamespacedName, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeRemoteCloned, Status: metav1.ConditionUnknown, Reason: "ProvisioningPodGit", Message: "Provisioning Clone Pod"}); err != nil {
				return ctrl.Result{}, err
			}
			pod, err := r.gitClonePod(instance, src)
			if err != nil {
				log.Error(err, "Failed to construct clone pod spec")
				return ctrl.Result{}, err
			}
			attachDefinitionIDLabel(pod, definitionID)
			err = r.Create(ctx, pod)
			if err != nil {
				log.Error(err, "Failed to create clone pod")
				return ctrl.Result{}, err
			}
			// TODO (juf): make configurable
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		} else {
			log.Error(err, "Failed to get pod info")
			return ctrl.Result{}, err
		}
	}

	if !podIsReadyOrFinished(clonePod) {
		log.Info("Clone pod is not ready")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if err := r.updateStatus(ctx, req.NamespacedName, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeRemoteCloned, Status: metav1.ConditionTrue, Reason: "RemoteClonedSuccessfully", Message: "Git remote has been cloned to PVC"}); err != nil {
		return ctrl.Result{}, err
	}

	// TODO (juf): This should probably not live here.
	// Why? Because these checks run on EVERY Reconcile loop, when this should more or less only run on Controller/Manager startup
	if err := r.ensureUtilRoles(ctx, instance.Namespace); err != nil {
		log.Error(err, "Failed to ensure roles")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	if err := r.ensureUtilRoleBindings(ctx, instance.Namespace); err != nil {
		log.Error(err, "Failed to ensure role bindings")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	if err := r.ensureServiceAccount(ctx, instance.Namespace); err != nil {
		log.Error(err, "Failed to ensure service account")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	parsePod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: r.parsePodName(instance), Namespace: instance.Namespace}, parsePod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Not found, let's schedule the parse pod for creation")
			if err := r.updateStatus(ctx, req.NamespacedName, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeParsed, Status: metav1.ConditionUnknown, Reason: "ProvisioningPodParse", Message: "Provisioning Parse Pod"}); err != nil {
				return ctrl.Result{}, err
			}
			pod, err := r.parsePod(instance)
			if err != nil {
				log.Error(err, "Failed to construct parse pod spec")
				return ctrl.Result{RequeueAfter: 15 * time.Second}, err
			}
			attachDefinitionIDLabel(pod, definitionID)
			err = r.Create(ctx, pod)
			if err != nil {
				log.Error(err, "Failed to create parse pod")
				if err := r.updateStatus(ctx, req.NamespacedName, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeParsed, Status: metav1.ConditionFalse, Reason: "ProvisioningPodParseErr", Message: err.Error()}); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: 15 * time.Second}, err
			}
			// TODO (juf): make configurable
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		} else {
			log.Error(err, "Failed to get pod info")
			return ctrl.Result{}, err
		}
	}

	if !podIsReadyOrFinished(parsePod) {
		log.Info("Parse pod is not ready")
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.updateStatus(ctx, req.NamespacedName, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeParsed, Status: metav1.ConditionTrue, Reason: "ParsePodSucceeded", Message: "Parsing JSON succeeded"}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func definitionID(instance *devcontainerv1alpha1.Definition, src *devcontainerv1alpha1.Source) string {
	h := sha1.New()
	_, err := io.WriteString(h, instance.Name+instance.Spec.GitHashOrTag+src.Spec.GitURL)
	if err != nil {
		panic(fmt.Errorf("could not generate definitionID: %w", err))
	}
	out := h.Sum(nil)
	return fmt.Sprintf("%x", out)
}

func podIsReadyOrFinished(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodSucceeded {
		return true
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type != corev1.PodReady {
			continue
		}
		if cond.Status != corev1.ConditionTrue {
			continue
		}
		return true
	}
	return false
}

func (r *DefinitionReconciler) updateStatusMany(ctx context.Context, namespacedName types.NamespacedName, instance *devcontainerv1alpha1.Definition, conditions []metav1.Condition) error {
	log := log.FromContext(ctx)
	if err := r.Get(ctx, namespacedName, instance); err != nil {
		log.Error(err, "Failed to re-fetch resource")
		return err
	}
	anyChange := false
	for _, cond := range conditions {
		// TODO (juf): Honor return value (maybe)
		anyChange = meta.SetStatusCondition(&instance.Status.Conditions, cond) || anyChange
	}
	if !anyChange {
		log.Info("No change in conditions detected, not performing update API call")
		return nil
	}
	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update Instance status")
		return err
	}
	return nil
}

func (r *DefinitionReconciler) updateStatus(ctx context.Context, namespacedName types.NamespacedName, instance *devcontainerv1alpha1.Definition, condition metav1.Condition) error {
	log := log.FromContext(ctx)
	for i := range 3 {
		err := r.updateStatusMany(ctx, namespacedName, instance, []metav1.Condition{condition})
		if err == nil {
			return err
		}
		if apierrors.IsConflict(err) {
			log.Error(err, "Failed to update status due to conflict", "retry", i)
			continue
		}
		return err
	}
	return nil
}

func (r *DefinitionReconciler) pvcName(inst *devcontainerv1alpha1.Definition) string {
	return fmt.Sprintf("%s-git", inst.Name)
}

func (r *DefinitionReconciler) clonePodName(inst *devcontainerv1alpha1.Definition) string {
	return fmt.Sprintf("%s-git-clone", inst.Name)
}

func (r *DefinitionReconciler) parsePodName(inst *devcontainerv1alpha1.Definition) string {
	return fmt.Sprintf("%s-parse", inst.Name)
}

func (r *DefinitionReconciler) ensureServiceAccount(ctx context.Context, namespace string) error {
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbac.SchemeGroupVersion.String(),
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      UtilServiceAccountName,
			Namespace: namespace,
		},
	}
	// TODO (juf): This should somehow be linked to the controller pod/deployment/statefulset
	return r.ensureResource(ctx, sa)
}

func (r *DefinitionReconciler) ensureUtilRoles(ctx context.Context, namespace string) error {
	role := &rbac.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbac.SchemeGroupVersion.String(),
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      UtilRoleName,
			Namespace: namespace,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{
					devcontainerv1alpha1.GroupVersion.Group,
				},
				Resources: []string{
					"definitions",
					"workspaces",
					"sources",
				},
				Verbs: []string{"*"},
			},
		},
	}
	// TODO (juf): This should somehow be linked to the controller pod/deployment/statefulset
	return r.ensureResource(ctx, role)
}

func (r *DefinitionReconciler) ensureUtilRoleBindings(ctx context.Context, namespace string) error {
	binding := &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbac.SchemeGroupVersion.String(),
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      UtilRoleBindingName,
			Namespace: namespace,
		},
		Subjects: []rbac.Subject{{
			Kind:      "ServiceAccount",
			Name:      UtilServiceAccountName,
			Namespace: namespace,
		}},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.SchemeGroupVersion.Group,
			Kind:     "Role",
			Name:     UtilRoleName,
		},
	}
	// TODO (juf): This should somehow be linked to the controller pod/deployment/statefulset
	return r.ensureResource(ctx, binding)
}

func (r *DefinitionReconciler) ensureResource(ctx context.Context, obj client.Object) error {
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

func attachDefinitionIDLabel(resource client.Object, definitionID string) {
	m := resource.GetLabels()
	if m == nil {
		m = make(map[string]string)
	}
	m[LabelDefinitionMapKey] = definitionID
	resource.SetLabels(m)
}

func (r *DefinitionReconciler) pvcForGitRepo(inst *devcontainerv1alpha1.Definition) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.pvcName(inst),
			Namespace: inst.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					// TODO(juf): this should somehow be dynamic based on the git repo size
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(inst, pvc, r.Scheme); err != nil {
		return nil, err
	}
	return pvc, nil
}

func (r *DefinitionReconciler) gitClonePod(inst *devcontainerv1alpha1.Definition, src *devcontainerv1alpha1.Source) (*corev1.Pod, error) {
	pvcName := r.pvcName(inst)
	pvc := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.clonePodName(inst),
			Namespace: inst.Namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					// TODO (juf): dont use latest
					Name: "git-clone",
					//kind load
					//Image: "registry.hub.docker.com/library/alpine/git:latest",
					Image: "alpine/git:latest",
					Command: []string{
						// TODO(juf): check if this makes sense. I am pretty sure this does clone into workspace, so /workspace will become the repo
						// but maybe this is ok for us.
						"git", "clone", src.Spec.GitURL, "/workspace",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      pvcName,
							MountPath: "/workspace",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: pvcName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(inst, pvc, r.Scheme); err != nil {
		return nil, err
	}
	return pvc, nil
}

func (r *DefinitionReconciler) parsePod(inst *devcontainerv1alpha1.Definition) (*corev1.Pod, error) {
	pvcName := r.pvcName(inst)
	pvc := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.parsePodName(inst),
			Namespace: inst.Namespace,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: UtilServiceAccountName,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name: "parser",
					// TODO (juf): dont use latest
					//Image: "registry.hub.docker.com/library/_/busybox:latest",
					Image: "parserapp:0.0.1",
					Args: []string{
						// TODO(juf): check if this makes sense. I am pretty sure this does clone into workspace, so /workspace will become the repo
						// but maybe this is ok for us.
						// Replace by code that creates the next CRD
						"/workspace/.devcontainer/devcontainer.json",
					},
					Env: []corev1.EnvVar{
						{
							// Name of the resource it should update
							Name:  "DEFINITION_ENV_NAME",
							Value: inst.Name,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      pvcName,
							MountPath: "/workspace",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: pvcName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(inst, pvc, r.Scheme); err != nil {
		return nil, err
	}
	return pvc, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devcontainerv1alpha1.Definition{}).
		Named("definition").
		Complete(r)
}
