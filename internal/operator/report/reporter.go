// Package report implements the Operator → Cluster Control reporting
// contract (docs/runtimes/03-forma-operator.md §3.2): node health every 15
// seconds, workspace status on change.
//
// The endpoints (/v1/node-health, /v1/workspace-status) are a proposed
// extension of the plane protocol — forma-ctl does not serve them yet, so
// failures are logged at low volume and never block reconciliation.
package report

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	formav1alpha1 "github.com/primadi/forma/internal/operator/api/v1alpha1"
)

// NodeHealthInterval is the node reporting cadence
// (docs/architecture/06-k8s-operator.md §6).
const NodeHealthInterval = 15 * time.Second

// Reporter posts cluster telemetry to Cluster Control. A Reporter with an
// empty ControlURL is a no-op, so callers never need nil checks.
type Reporter struct {
	controlURL string
	reader     client.Reader
	httpClient *http.Client

	mu         sync.Mutex
	lastFailed time.Time // rate-limits connection-error logging
}

// New creates a reporter. controlURL may be empty to disable reporting.
// reader is used to enumerate nodes and workspaces for node-health reports.
func New(controlURL string, reader client.Reader) *Reporter {
	return &Reporter{
		controlURL: controlURL,
		reader:     reader,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type nodeHealthReport struct {
	ReportedAt     time.Time    `json:"reported_at"`
	WorkspaceCount int          `json:"workspace_count"`
	Nodes          []nodeStatus `json:"nodes"`
}

type nodeStatus struct {
	Name   string            `json:"name"`
	Status string            `json:"status"` // healthy | degraded
	Labels map[string]string `json:"labels,omitempty"`
}

type workspaceStatusReport struct {
	Workspace     string    `json:"workspace"`
	Namespace     string    `json:"namespace"`
	Phase         string    `json:"phase"`
	ReadyReplicas int32     `json:"ready_replicas"`
	ReportedAt    time.Time `json:"reported_at"`
}

// RunNodeHealthLoop posts node health every NodeHealthInterval until ctx is
// cancelled. Start it with the manager (mgr.Add or a goroutine after cache
// sync).
func (r *Reporter) RunNodeHealthLoop(ctx context.Context) error {
	if r.controlURL == "" {
		return nil
	}
	ticker := time.NewTicker(NodeHealthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.reportNodeHealth(ctx)
		}
	}
}

func (r *Reporter) reportNodeHealth(ctx context.Context) {
	var nodes corev1.NodeList
	if err := r.reader.List(ctx, &nodes); err != nil {
		return
	}
	var workspaces formav1alpha1.WorkspaceList
	if err := r.reader.List(ctx, &workspaces); err != nil {
		return
	}

	report := nodeHealthReport{ReportedAt: time.Now().UTC(), WorkspaceCount: len(workspaces.Items)}
	for i := range nodes.Items {
		node := &nodes.Items[i]
		status := "degraded"
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				status = "healthy"
				break
			}
		}
		report.Nodes = append(report.Nodes, nodeStatus{
			Name: node.Name, Status: status, Labels: formaLabels(node.Labels),
		})
	}
	r.post(ctx, "/v1/node-health", report)
}

// ReportWorkspaceStatus posts one workspace's status; called by the
// WorkspaceReconciler after each status update (on-change semantics).
func (r *Reporter) ReportWorkspaceStatus(ws *formav1alpha1.Workspace) {
	if r.controlURL == "" {
		return
	}
	go r.post(context.Background(), "/v1/workspace-status", workspaceStatusReport{
		Workspace:     ws.Name,
		Namespace:     ws.Namespace,
		Phase:         ws.Status.Phase,
		ReadyReplicas: ws.Status.ReadyReplicas,
		ReportedAt:    time.Now().UTC(),
	})
}

func (r *Reporter) post(ctx context.Context, path string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.controlURL+path, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.logFailure(path, err)
		return
	}
	resp.Body.Close()
}

func (r *Reporter) logFailure(path string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Cluster Control may not serve these endpoints yet (§3.2) — log at
	// most once a minute instead of every 15s tick.
	if time.Since(r.lastFailed) < time.Minute {
		return
	}
	r.lastFailed = time.Now()
	logf.Log.WithName("reporter").V(1).Info("control plane report failed", "path", path, "error", err.Error())
}

func formaLabels(labels map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range labels {
		if len(k) > 10 && k[:10] == "forma.dev/" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
