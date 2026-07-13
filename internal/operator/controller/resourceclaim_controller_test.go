package controller

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	formav1alpha1 "github.com/primadi/forma/internal/operator/api/v1alpha1"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := formav1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return s
}

func signedClaim(t *testing.T, priv ed25519.PrivateKey) *formav1alpha1.ResourceClaim {
	t.Helper()
	claim := &formav1alpha1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "bank-pg-claim", Namespace: "default"},
		Spec: formav1alpha1.ResourceClaimSpec{
			Datastore:  "pg-bank",
			Workspace:  "bank-prod",
			Permission: "read-write",
			GrantedBy:  "cloud-owner",
			GrantedAt:  "2026-07-10T10:00:00Z",
		},
	}
	claim.Spec.Signature = hex.EncodeToString(ed25519.Sign(priv, claim.Spec.SignedMessage()))
	return claim
}

func TestResourceClaimReconcile(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	datastore := &formav1alpha1.Datastore{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-bank", Namespace: "default"},
		Spec: formav1alpha1.DatastoreSpec{
			Driver:            "postgres",
			EndpointSecretRef: formav1alpha1.SecretKeyRef{Name: "pg-creds", Key: "dsn"},
			AllowedTenants:    []string{"workspace:bank-prod"},
			OwnerPublicKey:    hex.EncodeToString(pub),
		},
	}

	cases := []struct {
		name      string
		mutate    func(*formav1alpha1.ResourceClaim, *formav1alpha1.Datastore)
		wantReady bool
		reason    string
	}{
		{
			name:      "valid signature and allowed workspace",
			mutate:    func(*formav1alpha1.ResourceClaim, *formav1alpha1.Datastore) {},
			wantReady: true,
			reason:    "ClaimVerified",
		},
		{
			name: "tenant not in allowedTenants",
			mutate: func(c *formav1alpha1.ResourceClaim, _ *formav1alpha1.Datastore) {
				c.Spec.Workspace = "intruder"
			},
			wantReady: false,
			reason:    "TenantNotAllowed",
		},
		{
			name: "tampered permission breaks signature",
			mutate: func(c *formav1alpha1.ResourceClaim, _ *formav1alpha1.Datastore) {
				c.Spec.Permission = "read-only" // signed as read-write
			},
			wantReady: false,
			reason:    "SignatureMismatch",
		},
		{
			name: "datastore without owner key",
			mutate: func(_ *formav1alpha1.ResourceClaim, ds *formav1alpha1.Datastore) {
				ds.Spec.OwnerPublicKey = ""
			},
			wantReady: false,
			reason:    "MissingOwnerKey",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claim := signedClaim(t, priv)
			ds := datastore.DeepCopy()
			tc.mutate(claim, ds)

			// TenantNotAllowed case must keep the tenant list mismatch, not
			// re-sign — the signature stays for the original workspace.
			cl := fake.NewClientBuilder().
				WithScheme(newTestScheme(t)).
				WithObjects(claim, ds).
				WithStatusSubresource(claim).
				Build()

			r := &ResourceClaimReconciler{Client: cl}
			_, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace},
			})
			if err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			var got formav1alpha1.ResourceClaim
			if err := cl.Get(context.Background(), types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace}, &got); err != nil {
				t.Fatalf("get claim: %v", err)
			}

			ready := meta.IsStatusConditionTrue(got.Status.Conditions, formav1alpha1.ConditionReady)
			denied := meta.IsStatusConditionTrue(got.Status.Conditions, formav1alpha1.ConditionDenied)
			if ready != tc.wantReady || denied == tc.wantReady {
				t.Fatalf("ready=%v denied=%v, want ready=%v", ready, denied, tc.wantReady)
			}
			cond := meta.FindStatusCondition(got.Status.Conditions, formav1alpha1.ConditionReady)
			if cond.Reason != tc.reason {
				t.Errorf("reason = %q, want %q", cond.Reason, tc.reason)
			}
		})
	}
}

func TestResourceClaimReconcile_DevSkipsSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	_ = pub
	claim := signedClaim(t, priv)
	claim.Spec.Signature = "" // no signature at all
	ds := &formav1alpha1.Datastore{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-bank", Namespace: "default"},
		Spec: formav1alpha1.DatastoreSpec{
			Driver:            "postgres",
			EndpointSecretRef: formav1alpha1.SecretKeyRef{Name: "pg-creds", Key: "dsn"},
			AllowedTenants:    []string{"workspace:bank-prod"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(claim, ds).
		WithStatusSubresource(claim).
		Build()

	r := &ResourceClaimReconciler{Client: cl, InsecureSkipSignatureVerify: true}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got formav1alpha1.ResourceClaim
	if err := cl.Get(context.Background(), types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !meta.IsStatusConditionTrue(got.Status.Conditions, formav1alpha1.ConditionReady) {
		t.Fatal("dev mode should mark unsigned claim Ready")
	}
}

func TestDesiredReplicas(t *testing.T) {
	ws := &formav1alpha1.Workspace{}
	class := &formav1alpha1.ClusterClass{
		Spec: formav1alpha1.ClusterClassSpec{Scaling: formav1alpha1.ClusterClassScaling{MinReplicas: 2}},
	}
	if got := desiredReplicas(ws, class); got != 2 {
		t.Errorf("premium replicas = %d, want 2", got)
	}

	economy := &formav1alpha1.ClusterClass{
		Spec: formav1alpha1.ClusterClassSpec{Scaling: formav1alpha1.ClusterClassScaling{MinReplicas: 0, ScaleToZero: true}},
	}
	if got := desiredReplicas(ws, economy); got != 1 {
		t.Errorf("active economy replicas = %d, want 1", got)
	}

	idle := &formav1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{idleAnnotation: "true"}},
	}
	if got := desiredReplicas(idle, economy); got != 0 {
		t.Errorf("idle economy replicas = %d, want 0", got)
	}
}

func TestDatastoreEnvName(t *testing.T) {
	if got := datastoreEnvName("pg-bank.mandiri"); got != "FORMA_DATASTORE_PG_BANK_MANDIRI_DSN" {
		t.Errorf("env name = %q", got)
	}
}
