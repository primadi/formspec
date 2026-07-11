package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	formav1alpha1 "github.com/primadi/forma/internal/operator/api/v1alpha1"
)

// DatastoreReconciler validates registered datastores: the referenced
// endpoint Secret must exist and hold a non-empty connection string
// (docs/runtimes/03-forma-operator.md §1). Credentials stay in the Secret —
// the operator checks presence, it does not read them into its own state.
//
// Actually dialing the endpoint is driver-specific and deliberately not
// done here; the workspace pod reports connectivity through its own health.
type DatastoreReconciler struct {
	client.Client
}

// Reconcile sets the Validated / Denied conditions on a Datastore.
func (r *DatastoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ds formav1alpha1.Datastore
	if err := r.Get(ctx, req.NamespacedName, &ds); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	validated, reason, msg := r.validate(ctx, &ds)

	status := metav1.ConditionTrue
	if !validated {
		status = metav1.ConditionFalse
	}
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type: formav1alpha1.ConditionValidated, Status: status,
		Reason: reason, Message: msg, ObservedGeneration: ds.Generation,
	})
	denied := metav1.ConditionFalse
	if !validated {
		denied = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type: formav1alpha1.ConditionDenied, Status: denied,
		Reason: reason, ObservedGeneration: ds.Generation,
	})

	if err := r.Status().Update(ctx, &ds); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

func (r *DatastoreReconciler) validate(ctx context.Context, ds *formav1alpha1.Datastore) (ok bool, reason, msg string) {
	if ds.Spec.Driver == "" {
		return false, "MissingDriver", "spec.driver is required"
	}
	ref := ds.Spec.EndpointSecretRef
	if ref.Name == "" || ref.Key == "" {
		return false, "MissingSecretRef", "spec.endpointSecretRef.name and .key are required"
	}

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ds.Namespace}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return false, "SecretNotFound", fmt.Sprintf("endpoint secret %q not found", ref.Name)
		}
		return false, "SecretUnreadable", err.Error()
	}
	if len(secret.Data[ref.Key]) == 0 {
		return false, "SecretKeyEmpty", fmt.Sprintf("secret %q has no data at key %q", ref.Name, ref.Key)
	}
	return true, "EndpointSecretPresent", "endpoint credentials present"
}

// SetupWithManager also watches Secrets so a late-created credential secret
// flips the datastore to Validated without manual intervention.
func (r *DatastoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formav1alpha1.Datastore{}).
		Watches(&corev1.Secret{}, secretToDatastores(mgr)).
		Complete(r)
}
