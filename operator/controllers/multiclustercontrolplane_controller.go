/*
Copyright 2023.

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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "../api/v1alpha1"
	// operatorv1alpha1 "github.com/stolostron/multicluster-controlplane/operator/api/v1alpha1"
)

const multiclustercontrolplaneFinalizer = "multicluster-controlplane.operator.open-cluster-management.io/finalizer"

// Definitions to manage status conditions
const (
	// typeAvailableMulticlusterControlplane represents the status of the Deployment reconciliation
	typeAvailableMulticlusterControlplane = "Available"
	// typeDegradedMulticlusterControlplane represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedMulticlusterControlplane = "Degraded"
)

// MulticlusterControlplaneReconciler reconciles a MulticlusterControlplane object
type MulticlusterControlplaneReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=operator.open-cluster-management.io,resources=multiclustercontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.open-cluster-management.io,resources=multiclustercontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.open-cluster-management.io,resources=multiclustercontrolplanes/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MulticlusterControlplane object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *MulticlusterControlplaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// get MulticlusterControlplane
	mc := &operatorv1alpha1.MulticlusterControlplane{}
	err := r.Get(ctx, req.NamespacedName, mc)
	if errors.IsNotFound(err) {
		// MulticlusterControlplane not found, should create one.
		log.Info("MulticlusterControlplane resource not found")
		//TODO(ycyaoxdu): add code logic, name? namespace?
		mc := &operatorv1alpha1.MulticlusterControlplane{
			//
		}
		err = r.Create(ctx, mc)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		log.Error(err, "fail to get MulticlusterControlplane")
		return ctrl.Result{}, err
	}

	if mc.Status.Conditions == nil || len(mc.Status.Conditions) == 0 {
		meta.SetStatusCondition(&mc.Status.Conditions, metav1.Condition{Type: typeAvailableMulticlusterControlplane, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, mc); err != nil {
			log.Error(err, "Failed to update MulticlusterControlplane status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the MulticlusterControlplane Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, mc); err != nil {
			log.Error(err, "Failed to re-fetch mc")
			return ctrl.Result{}, err
		}
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(mc, multiclustercontrolplaneFinalizer) {
		log.Info("Adding Finalizer for MulticlusterControlplane")
		if ok := controllerutil.AddFinalizer(mc, multiclustercontrolplaneFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err = r.Update(ctx, mc); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the MulticlusterControlplane instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if mc.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(mc, multiclustercontrolplaneFinalizer) {
			log.Info("Performing Finalizer Operations for MulticlusterControlplane before delete CR")

			// Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&mc.Status.Conditions, metav1.Condition{Type: typeDegradedMulticlusterControlplane,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", mc.Name)})

			if err := r.Status().Update(ctx, mc); err != nil {
				log.Error(err, "Failed to update MulticlusterControlplane status")
				return ctrl.Result{}, err
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			//TODO(ycyaoxdu): implement this function
			r.doFinalizerOperationsForMulticlusterControlplane(mc)

			// TODO(user): If you add operations to the doFinalizerOperationsForMulticlusterControlplane method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the mc Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, mc); err != nil {
				log.Error(err, "Failed to re-fetch mc")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&mc.Status.Conditions, metav1.Condition{Type: typeDegradedMulticlusterControlplane,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", mc.Name)})

			if err := r.Status().Update(ctx, mc); err != nil {
				log.Error(err, "Failed to update MulticlusterControlplane status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for MulticlusterControlplane after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(mc, multiclustercontrolplaneFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for MulticlusterControlplane")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, mc); err != nil {
				log.Error(err, "Failed to remove finalizer for MulticlusterControlplane")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: mc.Name, Namespace: mc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		// TODO(ycyaoxdu): implement this
		dep, err := r.deploymentForMulticlusterControlplane(mc)
		if err != nil {
			log.Error(err, "Failed to define new Deployment resource for MulticlusterControlplane")

			// The following implementation will update the status
			meta.SetStatusCondition(&mc.Status.Conditions, metav1.Condition{Type: typeAvailableMulticlusterControlplane,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", mc.Name, err)})

			if err := r.Status().Update(ctx, mc); err != nil {
				log.Error(err, "Failed to update MulticlusterControlplane status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment",
			"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			log.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}

		// Deployment created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// The following implementation will update the status
	meta.SetStatusCondition(&mc.Status.Conditions, metav1.Condition{Type: typeAvailableMulticlusterControlplane,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Deployment for custom resource (%s) created successfully", mc.Name)})

	if err := r.Status().Update(ctx, mc); err != nil {
		log.Error(err, "Failed to update MulticlusterControlplane status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// finalizeMulticlusterControlplane will perform the required operations before delete the CR.
func (r *MulticlusterControlplaneReconciler) doFinalizerOperationsForMulticlusterControlplane(cr *operatorv1alpha1.MulticlusterControlplane) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended of the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the Deployment will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))
}

// deploymentForMulticlusterControlplane returns a MulticlusterControlplane Deployment object
func (r *MulticlusterControlplaneReconciler) deploymentForMulticlusterControlplane(
	mc *operatorv1alpha1.MulticlusterControlplane) (*appsv1.Deployment, error) {

	//TODO(ycyaoxdu): modify the deployment and struct
	//TODO(ycyaoxdu): generate the cert

	// Get the image from spec
	image := mc.Spec.ControlplaneImagePullSpec
	// set labels
	ls := labelsForMulticlusterControlplane(mc.Name, image)
	// handle deploy mode
	if mc.StorageOption.Mode == operatorv1alpha1.StorageModeEmbedded {
		//TODO
	} else {
		//TODO
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc.Name,
			Namespace: mc.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           image,
						Name:            "multicluster-controlplane",
						ImagePullPolicy: corev1.PullIfNotPresent,
						// Ensure restrictive context for the container
						// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
						SecurityContext: &corev1.SecurityContext{
							// WARNING: Ensure that the image used defines an UserID in the Dockerfile
							// otherwise the Pod will not run and will fail with "container has runAsNonRoot and image has non-numeric user"".
							// If you want your workloads admitted in namespaces enforced with the restricted mode in OpenShift/OKD vendors
							// then, you MUST ensure that the Dockerfile defines a User ID OR you MUST leave the "RunAsNonRoot" and
							// "RunAsUser" fields empty.
							RunAsNonRoot: &[]bool{true}[0],
							// The multicluster-controlplane image does not use a non-zero numeric user as the default user.
							// Due to RunAsNonRoot field being set to true, we need to force the user in the
							// container to a non-zero numeric user. We do this using the RunAsUser field.
							// However, if you are looking to provide solution for K8s vendors like OpenShift
							// be aware that you cannot run under its restricted-v2 SCC if you set this value.
							RunAsUser:                &[]int64{1001}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
						Env: []corev1.EnvVar{
							corev1.EnvVar{
								Name:  "CONFIG_POLICY_CONTROLLER_IMAGE",
								Value: "quay.io/open-cluster-management/config-policy-controller:latest",
							},
							corev1.EnvVar{
								Name:  "KUBE_RBAC_PROXY_IMAGE",
								Value: "registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.10",
							},
							corev1.EnvVar{
								Name:  "GOVERNANCE_POLICY_FRAMEWORK_ADDON_IMAGE",
								Value: "quay.io/open-cluster-management/governance-policy-framework-addon:latest",
							},
							corev1.EnvVar{
								Name:  "MANAGED_SERVICE_ACCOUNT_IMAGE",
								Value: "quay.io/open-cluster-management/managed-serviceaccount:latest",
							},
						},
						Args: []string{
							"/multicluster-controlplane",
							"--authorization-mode=RBAC",
							"--enable-bootstrap-token-auth",
							"--service-account-key-file=/controlplane/cert/kube-serviceaccount.key",
							"--client-ca-file=/controlplane/cert/client-ca.crt",
							"--client-key-file=/controlplane/cert/client-ca.key",
							"--enable-bootstrap-token-auth",
							"--enable-priority-and-fairness=false",
							"--api-audiences=",
							"--v=1",
							"--service-account-lookup=false",
							"--service-account-signing-key-file=/controlplane/cert/kube-serviceaccount.key",
							"--enable-admission-plugins=NamespaceLifecycle,ServiceAccount,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,ManagedClusterMutating,ManagedClusterValidating,ManagedClusterSetBindingValidating",
							"--bind-address=0.0.0.0",
							"--secure-port=9443",
							"--tls-cert-file=/controlplane/cert/serving-kube-apiserver.crt",
							"--tls-private-key-file=/controlplane/cert/serving-kube-apiserver.key",
							"--feature-gates=DefaultClusterSet=true,OpenAPIV3=false,AddonManagement=true",
							"--storage-backend=etcd3",
							"--enable-embedded-etcd=true",
							"--embedded-etcd-directory=/.embedded-etcd",
							"--etcd-servers=https://127.0.0.1:2379",
							"--service-cluster-ip-range=10.0.0.0/24",
							"--service-account-issuer=https://kubernetes.default.svc",
							"--external-hostname=API_HOST",
							"--profiling=false",
						},
						VolumeMounts: []corev1.VolumeMount{
							corev1.VolumeMount{
								Name:      "controlplane-cert",
								MountPath: "/controlplane/cert",
								ReadOnly:  true,
							},
							corev1.VolumeMount{
								Name:      "embedded-etcd",
								MountPath: "/.embedded-etcd",
							},
						},
					}},
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "controlplane-cert",
							Secret: corev1.SecretVolumeSource{
								SecretName: "controlplane-cert",
							},
						},
						corev1.Volume{
							Name: "embedded-etcd",
							EmptyDir: corev1.EmptyDirVolumeSource{
								SizeLimit: resource.NewQuantity(500*1024*1024, resource.BinarySI),
							}
						},
					},
				},
			},
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(mc, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}

// labelsForMulticlusterControlplane returns the labels for selecting the resources
// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
func labelsForMulticlusterControlplane(name string, image string) map[string]string {
	var imageTag string
	imageTag = strings.Split(image, ":")[1]

	return map[string]string{"app.kubernetes.io/name": "MulticlusterControlplane",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "multicluster-controlplane-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MulticlusterControlplaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.MulticlusterControlplane{}).
		Complete(r)
}
