package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	formspecv1alpha1 "github.com/primadi/formspec/internal/operator/api/v1alpha1"
)

func intstrFromInt(port int) intstr.IntOrString {
	return intstr.FromInt32(int32(port))
}

// secretToDatastores requeues every Datastore whose endpointSecretRef names
// the changed Secret.
func secretToDatastores(mgr ctrl.Manager) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}
		var list formspecv1alpha1.DatastoreList
		if err := mgr.GetClient().List(ctx, &list, client.InNamespace(secret.Namespace)); err != nil {
			return nil
		}
		var reqs []ctrl.Request
		for i := range list.Items {
			if list.Items[i].Spec.EndpointSecretRef.Name == secret.Name {
				reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{
					Name: list.Items[i].Name, Namespace: list.Items[i].Namespace,
				}})
			}
		}
		return reqs
	})
}

// datastoreToClaims requeues every ResourceClaim that targets the changed
// Datastore.
func datastoreToClaims(mgr ctrl.Manager) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		var list formspecv1alpha1.ResourceClaimList
		if err := mgr.GetClient().List(ctx, &list, client.InNamespace(obj.GetNamespace())); err != nil {
			return nil
		}
		var reqs []ctrl.Request
		for i := range list.Items {
			if list.Items[i].Spec.Datastore == obj.GetName() {
				reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{
					Name: list.Items[i].Name, Namespace: list.Items[i].Namespace,
				}})
			}
		}
		return reqs
	})
}
