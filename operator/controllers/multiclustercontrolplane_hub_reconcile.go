package controller

import (
	"context"

	routev1 "github.com/openshift/api/route/v1"
	operatorv1alpha1 "github.com/stolostron/multicluster-controlplane/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ServiceResourceFile = "hack/deploy/controlplane/service.yaml"
	RouteResourceFile   = "hack/deploy/controlplane/route.yaml"
	PVCResourceFile     = "hack/deploy/controlplane/pvc.yaml"
	RbacResourceFiles   = []string{
		"hack/deploy/controlplane/serviceaccount.yaml",
		"hack/deploy/controlplane/clusterrole.yaml",
		"hack/deploy/controlplane/clusterrolebinding.yaml",
		"hack/deploy/controlplane/role.yaml",
		"hack/deploy/controlplane/rolebinding.yaml",
	}
)

type HubReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *HubReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HubReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.MulticlusterControlplane{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Service{}).
		Owns(&routev1.Route{}).
		Owns(&rbacv1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}
