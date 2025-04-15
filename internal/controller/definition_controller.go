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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	devcontainerv1alpha1 "everproc.com/devcontainer/api/v1alpha1"
)

const UtilRoleName = "devcontainer-utility"
const UtilRoleBindingName = UtilRoleName
const UtilServiceAccountName = UtilRoleName

var LabelDefinitionMapKey = devcontainerv1alpha1.SchemeBuilder.GroupVersion.Version + "." + devcontainerv1alpha1.SchemeBuilder.GroupVersion.Group + "/definitionID"

func init() {
	res := validation.IsQualifiedName(LabelDefinitionMapKey)
	if res != nil {
		panic(fmt.Sprintf("Invalid LabelDefinitionMapKey %q: %v", LabelDefinitionMapKey, res))
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
		if err := r.updateStatusMany(ctx, req.NamespacedName, instance, devcontainerv1alpha1.InitialConditionsDefinition()); err != nil {
			return ctrl.Result{}, err
		}
	}

	{
		on := os.Getenv("ENABLE_FINALIZER")
		// Finalizer Section
		finalizerName, executor := DefinitionFinalizerForRelatedWorkspaces()
		// See documentation of the field, it's enlightning
		if instance.ObjectMeta.DeletionTimestamp.IsZero() {
			// TODO(juf): AddFinalizer is probably idempotent and tells us if it's a no-op,
			// so we probably could and should remove the Contains check. Only if it makes sense though.
			if on == "yes" {
				if !controllerutil.ContainsFinalizer(instance, finalizerName) {
					if controllerutil.AddFinalizer(instance, finalizerName) {
						if err := r.Update(ctx, instance); err != nil {
							return ctrl.Result{}, err
						}
					}
				}
			}
		} else {
			// Return with positive result if this detects object is being deleted
			if controllerutil.ContainsFinalizer(instance, finalizerName) {
				definitionID := GetDefinitionIDLabel(instance)
				if definitionID == "" {
					log.Info("Empty definitionID found, not executing any finalizers")
					return ctrl.Result{}, nil
				}
				if err := executor(ctx, r, definitionID); err != nil {
					return ctrl.Result{}, err
				}
				// Reconcile is done, the resource will not exist any longer after we return a positive result
				return ctrl.Result{}, nil
			} else {
				return ctrl.Result{}, nil
			}
		}
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
	previousDefinitionID := GetDefinitionIDLabel(instance)
	if previousDefinitionID == "" {
		log.Info("Definition does not have a git hash based ID yet", "definitionID", definitionID)
	} else {
		if previousDefinitionID != definitionID {
			log.Info("Definition git hash has changed based on definition hash ID", "old", previousDefinitionID, "new", definitionID)
		} else {
			log.Info("Definition ID did not change")
		}
	}

	pvcRes, err := r.ensurePvc(ctx, instance, definitionID)
	if err != nil {
		return pvcRes, err
	} else {
		if !pvcRes.IsZero() {
			log.Info("Ensure PVC returned non-zero object")
			return pvcRes, nil
		}
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

	podRes, err := r.ensureSetupPod(ctx, instance, src, definitionID)
	if err != nil {
		return podRes, err
	} else {
		if !podRes.IsZero() {
			log.Info("Ensure Pod returned non-zero object")
			return podRes, nil
		}
	}

	cmList := &corev1.ConfigMapList{}
	err = r.List(ctx, cmList, client.MatchingLabels{
		LabelDefinitionMapKey: definitionID,
	}, client.InNamespace(instance.Namespace))
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(cmList.Items) == 0 {
		log.Info("No ConfigMap for match definitionID found", "definitionID", definitionID)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	} else if len(cmList.Items) > 0 {
		for _, d := range cmList.Items {
			id := GetDefinitionIDLabel(&d)
			if id == "" || id != definitionID {
				err = r.Delete(ctx, &d)
				if err != nil {
					log.Error(err, "Failed to delete old ConfigMap", "configMap", d.Name)
					return ctrl.Result{}, err
				}
			} else {
				if err := ctrl.SetControllerReference(instance, &d, r.Scheme); err != nil {
					log.Error(err, "Failed to set owner reference on config map")
					return ctrl.Result{}, err
				} else {
					log.Info("Updated OwnerReferences on ConfigMap", "configMap", d.Name)
				}
				if err := r.Update(ctx, &d); err != nil {
					log.Error(err, "Failed to update config map")
					return ctrl.Result{}, err
				}
				data, ok := d.BinaryData["definition"]
				if !ok {
					err := errors.New("ConfigMap should have definition key but it's missing")
					log.Error(err, "Missing definition key entry from ConfigMap")
					return ctrl.Result{}, err
				}
				if len(data) == 0 {
					err := errors.New("ConfigMap should have non-empty data under key definition")
					log.Error(err, "Empty definition in ConfigMap")
					return ctrl.Result{}, err
				}
				parsedDevcontainer := &devcontainerv1alpha1.ParsedDefinition{}
				if err := json.Unmarshal(data, &parsedDevcontainer); err != nil {
					log.Error(err, "Could not parse definition from ConfigMap. Either the config map is too old or corrupted")
					return ctrl.Result{}, err
				}

				// check if Dockerfile reference exists
				if parsedDevcontainer.Build.Dockerfile != "" && parsedDevcontainer.Image == "" {
					podRes, err := r.ensureKanikoPod(ctx, instance, src, definitionID, parsedDevcontainer.Build.Dockerfile, parsedDevcontainer.GitHash)
					if err != nil {
						return podRes, err
					} else {
						if !podRes.IsZero() {
							log.Info("Ensure Kaniko Pod returned non-zero object")
							return podRes, nil
						}
					}
				}

				// TODO(juf): This might not be 100% correct, I am not sure if the equality is applicable here, I did not check every field
				// TODO make it comparable
				// I hate Go sometimes
				if !devcontainerv1alpha1.EqualParsedDefinitions(&instance.Parsed, parsedDevcontainer) {
					wanted := *instance
					wanted.Parsed = *parsedDevcontainer
					data, err := client.MergeFrom(instance).Data(&wanted)
					if err != nil {
						log.Error(err, "Could not create MergePatch for definition")
						return ctrl.Result{}, err
					}
					patch := client.RawPatch(types.MergePatchType, data)
					if err := r.Patch(ctx, instance, patch); err != nil {
						// if err := r.Update(ctx, instance); err != nil {
						time.Sleep(3 * time.Second)
						log.Error(err, "Failed to update Definition with parsed Devcontainer JSON info")
						return ctrl.Result{}, err
					}
					log.Info("Successfully updated Definition with Devcontainer JSON info")
				} else {
					log.Info("No change in Devcontainer JSON info")
				}
			}
		}
	}

	if err := r.updateStatusMany(ctx, req.NamespacedName, instance, []metav1.Condition{
		{Type: devcontainerv1alpha1.DefinitionCondTypeParsed, Status: metav1.ConditionTrue, Reason: "ParsePodSucceeded", Message: "Parsing JSON succeeded"},
		{Type: devcontainerv1alpha1.DefinitionCondTypeReady, Status: metav1.ConditionTrue, Reason: "ReconcileFinished", Message: fmt.Sprintf("Reconcile finished for id %q", definitionID)}}); err != nil {
		return ctrl.Result{}, err
	}
	patch := patchDefinitionIDLabel(definitionID)
	if err := r.Patch(ctx, instance, patchDefinitionIDLabel(definitionID)); err != nil {
		data, otherErr := patch.Data(instance)
		if otherErr != nil {
			log.Error(err, "Patch is broken")
			time.Sleep(10 * time.Second)
		}
		log.Info(fmt.Sprintf("Patch: %+v", string(data)))
		time.Sleep(3 * time.Second)
		log.Error(err, "Failed to patch instance with definition ID label")
		return ctrl.Result{}, err
	}
	log.Info("Resource was successfully reconciled")
	return ctrl.Result{}, nil
}

func patchDefinitionIDLabel(newDefinitionID string) client.Patch {
	escapedLabelKey := strings.ReplaceAll(LabelDefinitionMapKey, "/", "~1")
	op := []byte(fmt.Sprintf(`[{"op": "add", "path": "/metadata/labels/%s", "value": %q}]`, escapedLabelKey, newDefinitionID))
	return client.RawPatch(types.JSONPatchType, op)
}

func (r *DefinitionReconciler) ensureSetupPod(ctx context.Context, instance *devcontainerv1alpha1.Definition, src *devcontainerv1alpha1.Source, definitionID string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	createFn := func() (ctrl.Result, error) {
		if err := r.updateStatus(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeParsed, Status: metav1.ConditionUnknown, Reason: "ProvisioningPodParse", Message: "Provisioning Parse Pod"}); err != nil {
			log.Info("Failed to update status during pod setup")
			return ctrl.Result{}, err
		}
		pod, err := r.setupPod(instance, src, definitionID, WorkspacePVCName(instance))
		if err != nil {
			log.Error(err, "Failed to construct parse pod spec")
			return ctrl.Result{RequeueAfter: 15 * time.Second}, err
		}
		AttachDefinitionIDLabel(pod, definitionID)
		err = r.Create(ctx, pod)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.Info("Pod already exists, rescheduling...")
				return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
			} else {
				log.Error(err, "Failed to create parse pod")
				if err := r.updateStatus(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeParsed, Status: metav1.ConditionFalse, Reason: "ProvisioningPodParseErr", Message: err.Error()}); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
		}
		// TODO (juf): make configurable
		log.Info("Waiting for pod to be scheduled...")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	setupPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: r.setupPodName(instance), Namespace: instance.Namespace}, setupPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Not found, let's schedule the parse pod for creation")
			return createFn()
		} else {
			log.Error(err, "Failed to get pod info")
			return ctrl.Result{}, err
		}
	}
	podDefinitionID := GetDefinitionIDLabel(setupPod)
	if podDefinitionID != definitionID {
		log.Info("The current Pod does not match the given definitionID, deleting...")
		if err := r.Delete(ctx, setupPod); err != nil {
			if apierrors.IsNotFound(err) {
				// Do nothing, the pod is somehow already gone
			} else {
				return ctrl.Result{}, err
			}
		}
		return createFn()
	}
	if !PodIsReadyOrFinished(setupPod) {
		log.Info("Parse pod is not ready")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	// Nothing to do continue
	return ctrl.Result{}, nil
}

func (r *DefinitionReconciler) ensureKanikoPod(ctx context.Context, instance *devcontainerv1alpha1.Definition, src *devcontainerv1alpha1.Source, definitionID string, dockerfile string, gitHash string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	createFn := func() (ctrl.Result, error) {
		if err := r.updateStatus(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeBuilt, Status: metav1.ConditionUnknown, Reason: "ProvisioningDockerBuild", Message: "Provisioning Docker Build"}); err != nil {
			log.Info("Failed to update status during Kaniko pod setup")
			return ctrl.Result{}, err
		}
		pod, err := r.kanikoPod(instance, src, definitionID, WorkspacePVCName(instance), dockerfile, gitHash)
		if err != nil {
			log.Error(err, "Failed to construct parse Kaniko pod spec")
			return ctrl.Result{RequeueAfter: 15 * time.Second}, err
		}
		AttachDefinitionIDLabel(pod, definitionID)
		err = r.Create(ctx, pod)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.Info("Kaniko pod already exists, rescheduling...")
				return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
			} else {
				log.Error(err, "Failed to create parse Kaniko pod")
				if err := r.updateStatus(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeBuilt, Status: metav1.ConditionFalse, Reason: "ProvisioningDockerBuildErr", Message: err.Error()}); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
		}
		// TODO (juf): make configurable
		log.Info("Waiting for Kaniko pod to be scheduled...")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	kanikoPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: r.kanikoPodName(instance), Namespace: instance.Namespace}, kanikoPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Not found, let's schedule the parse Kaniko pod for creation")
			return createFn()
		} else {
			log.Error(err, "Failed to get Kaniko pod info")
			return ctrl.Result{}, err
		}
	}
	podDefinitionID := GetDefinitionIDLabel(kanikoPod)
	if podDefinitionID != definitionID {
		log.Info("The current Kaniko Pod does not match the given definitionID, deleting...")
		if err := r.Delete(ctx, kanikoPod); err != nil {
			if apierrors.IsNotFound(err) {
				// Do nothing, the pod is somehow already gone
			} else {
				return ctrl.Result{}, err
			}
		}
		return createFn()
	}
	if !PodIsReadyOrFinished(kanikoPod) {
		log.Info("Kaniko pod is not ready")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	// Nothing to do continue
	return ctrl.Result{}, nil
}

func (r *DefinitionReconciler) ensurePvc(ctx context.Context, instance *devcontainerv1alpha1.Definition, definitionID string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	// Let's hope this works.
	// Why are we doing this?
	// We want to remove any outdated deployments when the definitionID changes
	pvc := &corev1.PersistentVolumeClaim{}
	id := types.NamespacedName{Name: WorkspacePVCName(instance), Namespace: instance.Namespace}
	err := r.Get(ctx, id, pvc)
	// If size equals 0, we have to create the PVC
	// if size equals 1, we need to check if the definitionID label matches, if it does, everything is fine, else schedule the PVC and delete the old PVC
	// if size is greater than 1 we need to clean up
	createFn := func() (ctrl.Result, error) {
		pvc, err := r.pvcForGitRepo(instance)
		if err != nil {
			log.Error(err, "Failed to construct PVC spec")
			return ctrl.Result{}, err
		}
		AttachDefinitionIDLabel(pvc, definitionID)
		err = r.Create(ctx, pvc)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.Error(err, "PVC already exists")
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			} else {
				log.Error(err, "Failed to create PVC")
				return ctrl.Result{}, err
			}
		}
		if err := r.updateStatus(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}, instance, metav1.Condition{Type: devcontainerv1alpha1.DefinitionCondTypeRemoteCloned, Status: metav1.ConditionUnknown, Reason: "ProvisioningPVC", Message: "Provisioning PVC"}); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Not found, let's schedule a PVC for creation")
			return createFn()
		} else {
			log.Error(err, "Error while fetching PVCs")
			return ctrl.Result{}, err
		}
	}
	pvcDefinitionID := GetDefinitionIDLabel(pvc)
	if pvcDefinitionID != definitionID {
		log.Info(fmt.Sprintf("The current PVC does not match the given definitionID, deleting...pvcID: %s, definitonID: %s", pvcDefinitionID, definitionID))
		if err := r.Delete(ctx, pvc); err != nil {
			if apierrors.IsNotFound(err) {
				// Do nothing, the pvc is somehow already gone
			} else {
				return ctrl.Result{}, err
			}
		}
		return createFn()
	}
	// Nothing to do continue
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

func WorkspacePVCNameFromDefinitionName(name string) string {
	return fmt.Sprintf("%s-git", name)
}

func WorkspacePVCName(inst *devcontainerv1alpha1.Definition) string {
	return WorkspacePVCNameFromDefinitionName(inst.Name)
}

func (r *DefinitionReconciler) setupPodName(inst *devcontainerv1alpha1.Definition) string {
	return fmt.Sprintf("%s-setup", inst.Name)
}

func (r *DefinitionReconciler) kanikoPodName(inst *devcontainerv1alpha1.Definition) string {
	return fmt.Sprintf("%s-docker-build", inst.Name)
}

func (r *DefinitionReconciler) clonePodName(inst *devcontainerv1alpha1.Definition) string {
	return fmt.Sprintf("%s-git-clone", inst.Name)
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
	// TODO(juf): refine and split this.
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
			{
				APIGroups: []string{""},
				Resources: []string{
					"configmaps",
				},
				Verbs: []string{"create"},
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

func AttachDefinitionIDLabel(resource client.Object, definitionID string) {
	m := resource.GetLabels()
	if m == nil {
		m = make(map[string]string)
	}
	m[LabelDefinitionMapKey] = definitionID
	resource.SetLabels(m)
}

func GetDefinitionIDLabel(resource client.Object) string {
	m := resource.GetLabels()
	if m == nil {
		m = make(map[string]string)
	}
	return m[LabelDefinitionMapKey]
}

func (r *DefinitionReconciler) pvcForGitRepo(inst *devcontainerv1alpha1.Definition) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkspacePVCName(inst),
			Namespace: inst.Namespace,
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
	return pvc, nil
}

func (r *DefinitionReconciler) setupPod(inst *devcontainerv1alpha1.Definition, src *devcontainerv1alpha1.Source, definitionID string, pvcName string) (*corev1.Pod, error) {
	cloneContainer, err := r.gitCloneContainer(inst, src)
	if err != nil {
		return nil, err
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.setupPodName(inst),
			Namespace: inst.Namespace,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: UtilServiceAccountName,
			RestartPolicy:      corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				*cloneContainer,
				r.parseContainer(inst, definitionID),
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
	if src.Spec.GitSecret != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "git-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  src.Spec.GitSecret,
					DefaultMode: pointer.Int32(0600),
				},
			},
		})
	}

	AttachDefinitionIDLabel(pod, definitionID)

	if err := ctrl.SetControllerReference(inst, pod, r.Scheme); err != nil {
		return nil, err
	}

	return pod, nil
}

func (r *DefinitionReconciler) kanikoPod(inst *devcontainerv1alpha1.Definition, src *devcontainerv1alpha1.Source, definitionID string, pvcName string, dockerfile string, gitHash string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.kanikoPodName(inst),
			Namespace: inst.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "kaniko",
					Image: "gcr.io/kaniko-project/executor:latest",
					Args: []string{
						fmt.Sprintf("--dockerfile=%s", dockerfile),
						"--context=dir://workspace",
						fmt.Sprintf("--destination=%s/%s:%s", src.Spec.DockerRegistry, src.Name, gitHash),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "docker-secret",
							MountPath: "/kaniko/.docker",
						},
						{
							Name:      pvcName,
							MountPath: "/workspace",
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "docker-secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: src.Spec.DockerSecret,
							Items: []corev1.KeyToPath{
								{
									Key:  ".dockerconfigjson",
									Path: "config.json",
								},
							},
						},
					},
				},
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

	AttachDefinitionIDLabel(pod, definitionID)

	if err := ctrl.SetControllerReference(inst, pod, r.Scheme); err != nil {
		return nil, err
	}

	return pod, nil
}

func (r *DefinitionReconciler) gitCloneContainer(inst *devcontainerv1alpha1.Definition, src *devcontainerv1alpha1.Source) (*corev1.Container, error) {
	gitDomain, err := ParseGitUrl(src.Spec.GitURL)
	if err != nil {
		return nil, err
	}
	pvcName := WorkspacePVCName(inst)
	cloneContainer := corev1.Container{
		Name: "git-clone",
		// TODO (juf): dont use latest
		// TODO (juf): make configurable
		Image:           GIT_IMAGE_NAME,
		ImagePullPolicy: corev1.PullAlways,
		//Command:         []string{"/bin/sh", "-c", "sleep infinity"},
		Env: []corev1.EnvVar{
			{
				Name:  "REPO_URL",
				Value: src.Spec.GitURL,
			},
			{
				Name:  "REPO_DOMAIN",
				Value: gitDomain,
			},
			{
				Name:  "GIT_HASH_OR_BRANCH",
				Value: inst.Spec.GitHashOrTag,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      pvcName,
				MountPath: "/workspace",
			},
		},
	}
	if src.Spec.GitSecret != "" {
		cloneContainer.VolumeMounts = append(cloneContainer.VolumeMounts, corev1.VolumeMount{
			Name:      "git-secret",
			MountPath: "/root/.ssh",
			ReadOnly:  true,
		})
	}
	return &cloneContainer, nil
}

func (r *DefinitionReconciler) parseContainer(inst *devcontainerv1alpha1.Definition, definitionID string) corev1.Container {
	pvcName := WorkspacePVCName(inst)
	return corev1.Container{
		Name: "parser",
		// TODO (juf): make configurable
		Image: PARSERAPP_IMAGE_NAME,
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
			{
				// Name of the resource it should update
				Name:  "DEFINITION_ENV_ID",
				Value: definitionID,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      pvcName,
				MountPath: "/workspace",
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.ConfigMap{}, "metadata.ownerReferences.kind", func(obj client.Object) []string {
		cm := obj.(*corev1.ConfigMap)
		var kinds []string
		for _, owner := range cm.OwnerReferences {
			kinds = append(kinds, owner.Kind)
		}
		return kinds
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.ConfigMap{}, "metadata.ownerReferences.name", func(obj client.Object) []string {
		cm := obj.(*corev1.ConfigMap)
		var names []string
		for _, owner := range cm.OwnerReferences {
			names = append(names, owner.Name)
		}
		return names
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&devcontainerv1alpha1.Definition{}).
		Named("definition").
		Complete(r)
}
