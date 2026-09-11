package controller

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	formspecv1alpha1 "github.com/primadi/formspec/internal/operator/api/v1alpha1"
)

// ResourceClaimReconciler verifies permission grants
// (docs/runtimes/03-formspec-operator.md §2.4):
//
//  1. Verify the ed25519 signature against the datastore owner's public key.
//  2. Check the claiming workspace is in the datastore's allowedTenants.
//  3. Valid   → condition Ready=True; the WorkspaceReconciler injects the
//     datastore credentials into the workspace pod.
//  4. Invalid → condition Denied=True with a reason; nothing is injected.
type ResourceClaimReconciler struct {
	client.Client
	// InsecureSkipSignatureVerify accepts claims without a signature or
	// owner key. Development only — never set in production.
	InsecureSkipSignatureVerify bool
}

// Reconcile sets Ready / Denied conditions on a ResourceClaim.
func (r *ResourceClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var claim formspecv1alpha1.ResourceClaim
	if err := r.Get(ctx, req.NamespacedName, &claim); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ready, reason, msg := r.evaluate(ctx, &claim)

	readyStatus, deniedStatus := metav1.ConditionTrue, metav1.ConditionFalse
	if !ready {
		readyStatus, deniedStatus = metav1.ConditionFalse, metav1.ConditionTrue
	}
	meta.SetStatusCondition(&claim.Status.Conditions, metav1.Condition{
		Type: formspecv1alpha1.ConditionReady, Status: readyStatus,
		Reason: reason, Message: msg, ObservedGeneration: claim.Generation,
	})
	meta.SetStatusCondition(&claim.Status.Conditions, metav1.Condition{
		Type: formspecv1alpha1.ConditionDenied, Status: deniedStatus,
		Reason: reason, Message: msg, ObservedGeneration: claim.Generation,
	})

	if err := r.Status().Update(ctx, &claim); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

func (r *ResourceClaimReconciler) evaluate(ctx context.Context, claim *formspecv1alpha1.ResourceClaim) (ready bool, reason, msg string) {
	if claim.Spec.Datastore == "" || claim.Spec.Workspace == "" {
		return false, "IncompleteSpec", "spec.datastore and spec.workspace are required"
	}

	var ds formspecv1alpha1.Datastore
	if err := r.Get(ctx, types.NamespacedName{Name: claim.Spec.Datastore, Namespace: claim.Namespace}, &ds); err != nil {
		if apierrors.IsNotFound(err) {
			return false, "DatastoreNotFound", fmt.Sprintf("datastore %q not found", claim.Spec.Datastore)
		}
		return false, "DatastoreUnreadable", err.Error()
	}

	// allowedTenants gate (§2.4 step 2)
	tenant := "workspace:" + claim.Spec.Workspace
	allowed := false
	for _, t := range ds.Spec.AllowedTenants {
		if t == tenant {
			allowed = true
			break
		}
	}
	if !allowed {
		return false, "TenantNotAllowed",
			fmt.Sprintf("%s is not in datastore %q allowedTenants", tenant, ds.Name)
	}

	// signature gate (§2.4 step 1)
	if ok, reason, msg := r.verifySignature(claim, &ds); !ok {
		return false, reason, msg
	}

	return true, "ClaimVerified", "signature valid and tenant allowed"
}

func (r *ResourceClaimReconciler) verifySignature(claim *formspecv1alpha1.ResourceClaim, ds *formspecv1alpha1.Datastore) (ok bool, reason, msg string) {
	if r.InsecureSkipSignatureVerify {
		return true, "SignatureSkipped", "signature verification disabled (dev mode)"
	}
	if ds.Spec.OwnerPublicKey == "" {
		return false, "MissingOwnerKey",
			fmt.Sprintf("datastore %q has no spec.ownerPublicKey to verify claims against", ds.Name)
	}
	if claim.Spec.Signature == "" {
		return false, "MissingSignature", "spec.signature is required"
	}

	pubKey, err := hex.DecodeString(ds.Spec.OwnerPublicKey)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		return false, "InvalidOwnerKey", "spec.ownerPublicKey is not a hex-encoded ed25519 public key"
	}
	sig, err := hex.DecodeString(claim.Spec.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false, "InvalidSignature", "spec.signature is not a hex-encoded ed25519 signature"
	}
	if !ed25519.Verify(ed25519.PublicKey(pubKey), claim.Spec.SignedMessage(), sig) {
		return false, "SignatureMismatch", "ed25519 signature does not match the claim contents"
	}
	return true, "", ""
}

// SetupWithManager also watches Datastores: a change to allowedTenants or
// the owner key re-evaluates every claim on that datastore.
func (r *ResourceClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formspecv1alpha1.ResourceClaim{}).
		Watches(&formspecv1alpha1.Datastore{}, datastoreToClaims(mgr)).
		Complete(r)
}
