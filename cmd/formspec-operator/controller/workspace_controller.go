// Package controller implements the formspec-operator reconcilers
// (docs/runtimes/03-formspec-operator.md §2): Workspace → Deployment/Service/
// Secret/ConfigMap/HPA, Datastore → credential validation, ResourceClaim →
// signature-verified credential injection.
package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	formspecv1alpha1 "github.com/primadi/formspec/internal/operator/api/v1alpha1"
)

const (
	// managedByLabel marks resources created by this operator.
	managedByLabel = "app.kubernetes.io/managed-by"
	managedByValue = "formspec-operator"
	workspaceLabel = "formspec.dev/workspace"

	// binaryHashAnnotation on a Workspace is copied to the pod template to
	// trigger a rolling restart when the artifact's compiled handler binary
	// changes (docs/architecture/06-k8s-operator.md §5). Cluster Control
	// surfaces restart_required; whoever consumes it stamps this annotation.
	binaryHashAnnotation = "formspec.dev/artifact-binary-hash"

	// idleAnnotation marks a workspace as idle; with a scale-to-zero
	// ClusterClass the Deployment is scaled to 0 replicas.
	idleAnnotation = "formspec.dev/idle"

	featureAutoScaling = "auto-scaling"

	resourceContainerPort = 8080
)

// WorkspaceStatusReporter forwards workspace status changes to Cluster
// Control (docs/runtimes/03-formspec-operator.md §3.2). Implementations must
// be non-blocking or fast; a nil reporter disables reporting.
type WorkspaceStatusReporter interface {
	ReportWorkspaceStatus(ws *formspecv1alpha1.Workspace)
}

// WorkspaceReconciler reconciles a Workspace into a formspec-resource
// Deployment + Service + ConfigMap (+HPA), with datastore credentials
// injected from approved ResourceClaims.
type WorkspaceReconciler struct {
	client.Client
	// ResourceImage is the generic formspec-resource image, version-pinned
	// (e.g. "formahub/formspec-resource:1.4.2").
	ResourceImage string
	// ControlClusterURL is injected into workspace pods as CONTROL_CLUSTER_URL.
	ControlClusterURL string
	// Reporter, if set, receives workspace status on every reconcile.
	Reporter WorkspaceStatusReporter
}

// Reconcile drives the loop from docs/runtimes/03-formspec-operator.md §2.3.
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var ws formspecv1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		// Deleted: Deployment/Service/ConfigMap/HPA are garbage-collected via
		// owner references. Secrets are intentionally retained (§2.3).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 1. ClusterClass → scaling policy
	var class formspecv1alpha1.ClusterClass
	if err := r.Get(ctx, types.NamespacedName{Name: ws.Spec.ClusterClass}, &class); err != nil {
		if apierrors.IsNotFound(err) {
			return r.updateStatus(ctx, &ws, 0, metav1.Condition{
				Type: formspecv1alpha1.ConditionDegraded, Status: metav1.ConditionTrue,
				Reason:  "ClusterClassNotFound",
				Message: fmt.Sprintf("ClusterClass %q not found", ws.Spec.ClusterClass),
			})
		}
		return ctrl.Result{}, err
	}

	// 2. Approved claims → credential env vars
	claimEnv, err := r.claimEnvVars(ctx, &ws)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 3. Deployment
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName(&ws), Namespace: ws.Namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		return r.buildDeployment(&ws, &class, claimEnv, deploy)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile deployment: %w", err)
	}
	if op != controllerutil.OperationResultNone {
		log.Info("deployment reconciled", "workspace", ws.Name, "op", op)
	}

	// 4. Service
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: deploymentName(&ws), Namespace: ws.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		return r.buildService(&ws, svc)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile service: %w", err)
	}

	// 5. ConfigMap
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: deploymentName(&ws) + "-config", Namespace: ws.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		return r.buildConfigMap(&ws, &class, cm)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile configmap: %w", err)
	}

	// 6. HPA (only when the class enables auto-scaling)
	if err := r.reconcileHPA(ctx, &ws, &class); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile hpa: %w", err)
	}

	// 7. Status
	cond := metav1.Condition{
		Type: formspecv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: "Progressing", Message: "Deployment not fully available yet",
	}
	if deploy.Status.ReadyReplicas >= desiredReplicas(&ws, &class) {
		cond.Status = metav1.ConditionTrue
		cond.Reason = "DeploymentAvailable"
		cond.Message = fmt.Sprintf("%d/%d replicas ready", deploy.Status.ReadyReplicas, desiredReplicas(&ws, &class))
	}
	return r.updateStatus(ctx, &ws, deploy.Status.ReadyReplicas, cond)
}

func deploymentName(ws *formspecv1alpha1.Workspace) string {
	return "formspec-resource-" + ws.Name
}

// desiredReplicas applies the ClusterClass scaling policy (D-ARCH-31):
// the floor is minReplicas; a scale-to-zero class drops an idle-annotated
// workspace to 0; otherwise at least one replica runs.
func desiredReplicas(ws *formspecv1alpha1.Workspace, class *formspecv1alpha1.ClusterClass) int32 {
	scaling := class.Spec.Scaling
	if scaling.ScaleToZero && ws.Annotations[idleAnnotation] == "true" {
		return 0
	}
	if scaling.MinReplicas < 1 {
		return 1
	}
	return scaling.MinReplicas
}

func (r *WorkspaceReconciler) buildDeployment(ws *formspecv1alpha1.Workspace, class *formspecv1alpha1.ClusterClass, claimEnv []corev1.EnvVar, deploy *appsv1.Deployment) error {
	replicas := desiredReplicas(ws, class)
	labels := map[string]string{
		managedByLabel: managedByValue,
		workspaceLabel: ws.Name,
	}

	podAnnotations := map[string]string{}
	if hash := ws.Annotations[binaryHashAnnotation]; hash != "" {
		podAnnotations[binaryHashAnnotation] = hash
	}

	env := []corev1.EnvVar{
		{Name: "CONTROL_CLUSTER_URL", Value: r.ControlClusterURL},
		{Name: "WORKSPACE_ID", Value: ws.Name},
	}
	env = append(env, claimEnv...)

	container := corev1.Container{
		Name:  "formspec-resource",
		Image: r.ResourceImage,
		Env:   env,
		Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: resourceContainerPort}},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: "/health", Port: intstrFromInt(resourceContainerPort)},
			},
			InitialDelaySeconds: 5, PeriodSeconds: 10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: "/health", Port: intstrFromInt(resourceContainerPort)},
			},
			InitialDelaySeconds: 2, PeriodSeconds: 5,
		},
	}

	if res := ws.Spec.Resources; res != nil {
		requests := corev1.ResourceList{}
		if res.CPU != "" {
			q, err := apiresource.ParseQuantity(res.CPU)
			if err != nil {
				return fmt.Errorf("spec.resources.cpu %q: %w", res.CPU, err)
			}
			requests[corev1.ResourceCPU] = q
		}
		if res.Memory != "" {
			q, err := apiresource.ParseQuantity(res.Memory)
			if err != nil {
				return fmt.Errorf("spec.resources.memory %q: %w", res.Memory, err)
			}
			requests[corev1.ResourceMemory] = q
		}
		container.Resources = corev1.ResourceRequirements{Requests: requests, Limits: requests}
	}

	podSpec := corev1.PodSpec{
		Containers:   []corev1.Container{container},
		NodeSelector: nodeSelector(ws),
	}

	// Anti-affinity spreads HA replicas across nodes (§2.3 step 2).
	if class.Spec.Scaling.MinReplicas >= 2 {
		podSpec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{workspaceLabel: ws.Name}},
						TopologyKey:   "kubernetes.io/hostname",
					},
				}},
			},
		}
	}

	deploy.Labels = labels
	deploy.Spec.Replicas = &replicas
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{workspaceLabel: ws.Name}}
	deploy.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: labels, Annotations: podAnnotations},
		Spec:       podSpec,
	}
	return controllerutil.SetControllerReference(ws, deploy, r.Scheme())
}

// nodeSelector maps workspace placement fields onto the formspec.dev node
// labels (docs/architecture/06-k8s-operator.md §4).
func nodeSelector(ws *formspecv1alpha1.Workspace) map[string]string {
	sel := map[string]string{}
	if ws.Spec.Environment != "" {
		sel["formspec.dev/environment"] = ws.Spec.Environment
	}
	if ws.Spec.Region != "" {
		sel["formspec.dev/region"] = ws.Spec.Region
	}
	if len(sel) == 0 {
		return nil
	}
	return sel
}

func (r *WorkspaceReconciler) buildService(ws *formspecv1alpha1.Workspace, svc *corev1.Service) error {
	svc.Labels = map[string]string{managedByLabel: managedByValue, workspaceLabel: ws.Name}
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	svc.Spec.Selector = map[string]string{workspaceLabel: ws.Name}
	svc.Spec.Ports = []corev1.ServicePort{{
		Name: "http", Port: 80, TargetPort: intstrFromInt(resourceContainerPort),
	}}
	return controllerutil.SetControllerReference(ws, svc, r.Scheme())
}

func (r *WorkspaceReconciler) buildConfigMap(ws *formspecv1alpha1.Workspace, class *formspecv1alpha1.ClusterClass, cm *corev1.ConfigMap) error {
	cm.Labels = map[string]string{managedByLabel: managedByValue, workspaceLabel: ws.Name}
	cm.Data = map[string]string{
		"workspace":    ws.Name,
		"region":       ws.Spec.Region,
		"environment":  ws.Spec.Environment,
		"clusterClass": class.Name,
	}
	return controllerutil.SetControllerReference(ws, cm, r.Scheme())
}

func (r *WorkspaceReconciler) reconcileHPA(ctx context.Context, ws *formspecv1alpha1.Workspace, class *formspecv1alpha1.ClusterClass) error {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: deploymentName(ws), Namespace: ws.Namespace}}

	if !class.HasFeature(featureAutoScaling) {
		err := r.Delete(ctx, hpa)
		return client.IgnoreNotFound(err)
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, hpa, func() error {
		minReplicas := class.Spec.Scaling.MinReplicas
		if minReplicas < 1 {
			minReplicas = 1
		}
		maxReplicas := minReplicas * 5
		if maxReplicas < 4 {
			maxReplicas = 4
		}
		cpuTarget := int32(70)

		hpa.Labels = map[string]string{managedByLabel: managedByValue, workspaceLabel: ws.Name}
		hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1", Kind: "Deployment", Name: deploymentName(ws),
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics: []autoscalingv2.MetricSpec{{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &cpuTarget,
					},
				},
			}},
		}
		return controllerutil.SetControllerReference(ws, hpa, r.Scheme())
	})
	return err
}

// claimEnvVars collects credential env vars from ResourceClaims that name
// this workspace and carry a Ready condition (§2.3 step 4). Each approved
// claim contributes FORMA_DATASTORE_<NAME>_DSN sourced from the datastore's
// endpoint Secret — the operator never reads the credential value itself.
func (r *WorkspaceReconciler) claimEnvVars(ctx context.Context, ws *formspecv1alpha1.Workspace) ([]corev1.EnvVar, error) {
	var claims formspecv1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(ws.Namespace)); err != nil {
		return nil, fmt.Errorf("list resource claims: %w", err)
	}

	var env []corev1.EnvVar
	for i := range claims.Items {
		claim := &claims.Items[i]
		if claim.Spec.Workspace != ws.Name {
			continue
		}
		if !meta.IsStatusConditionTrue(claim.Status.Conditions, formspecv1alpha1.ConditionReady) {
			continue
		}

		var ds formspecv1alpha1.Datastore
		if err := r.Get(ctx, types.NamespacedName{Name: claim.Spec.Datastore, Namespace: ws.Namespace}, &ds); err != nil {
			if apierrors.IsNotFound(err) {
				continue // claim references a datastore that no longer exists
			}
			return nil, err
		}

		env = append(env, corev1.EnvVar{
			Name: datastoreEnvName(ds.Name),
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ds.Spec.EndpointSecretRef.Name},
					Key:                  ds.Spec.EndpointSecretRef.Key,
				},
			},
		})
	}

	sort.Slice(env, func(i, j int) bool { return env[i].Name < env[j].Name })
	return env, nil
}

func datastoreEnvName(dsName string) string {
	sanitized := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(dsName))
	return "FORMA_DATASTORE_" + sanitized + "_DSN"
}

func (r *WorkspaceReconciler) updateStatus(ctx context.Context, ws *formspecv1alpha1.Workspace, readyReplicas int32, cond metav1.Condition) (ctrl.Result, error) {
	cond.ObservedGeneration = ws.Generation
	meta.SetStatusCondition(&ws.Status.Conditions, cond)

	// Keep the companion conditions coherent with the primary one.
	progressing := metav1.ConditionFalse
	if cond.Type == formspecv1alpha1.ConditionReady && cond.Status == metav1.ConditionFalse {
		progressing = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&ws.Status.Conditions, metav1.Condition{
		Type: formspecv1alpha1.ConditionProgressing, Status: progressing,
		Reason: "Reconcile", ObservedGeneration: ws.Generation,
	})

	switch {
	case meta.IsStatusConditionTrue(ws.Status.Conditions, formspecv1alpha1.ConditionDegraded):
		ws.Status.Phase = "Degraded"
	case meta.IsStatusConditionTrue(ws.Status.Conditions, formspecv1alpha1.ConditionReady):
		ws.Status.Phase = "Ready"
	default:
		ws.Status.Phase = "Pending"
	}
	ws.Status.ReadyReplicas = readyReplicas
	ws.Status.ObservedGeneration = ws.Generation

	if err := r.Status().Update(ctx, ws); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if r.Reporter != nil {
		r.Reporter.ReportWorkspaceStatus(ws)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager wires watches with default controller options.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerWithOptions(mgr, controller.Options{})
}

// SetupWithManagerWithOptions wires watches: owned resources requeue their
// workspace; ClusterClass changes fan out to every workspace using that
// class; ResourceClaim changes requeue the claiming workspace (there is no
// separate ClusterClassReconciler — the class is cache-only config, §2.1).
func (r *WorkspaceReconciler) SetupWithManagerWithOptions(mgr ctrl.Manager, opts controller.Options) error {
	mapClassToWorkspaces := func(ctx context.Context, obj client.Object) []ctrl.Request {
		var list formspecv1alpha1.WorkspaceList
		if err := mgr.GetClient().List(ctx, &list); err != nil {
			return nil
		}
		var reqs []ctrl.Request
		for i := range list.Items {
			if list.Items[i].Spec.ClusterClass == obj.GetName() {
				reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{
					Name: list.Items[i].Name, Namespace: list.Items[i].Namespace,
				}})
			}
		}
		return reqs
	}

	mapClaimToWorkspace := func(ctx context.Context, obj client.Object) []ctrl.Request {
		claim, ok := obj.(*formspecv1alpha1.ResourceClaim)
		if !ok || claim.Spec.Workspace == "" {
			return nil
		}
		return []ctrl.Request{{NamespacedName: types.NamespacedName{
			Name: claim.Spec.Workspace, Namespace: claim.Namespace,
		}}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&formspecv1alpha1.Workspace{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Watches(&formspecv1alpha1.ClusterClass{}, handler.EnqueueRequestsFromMapFunc(mapClassToWorkspaces)).
		Watches(&formspecv1alpha1.ResourceClaim{}, handler.EnqueueRequestsFromMapFunc(mapClaimToWorkspace)).
		Complete(r)
}
