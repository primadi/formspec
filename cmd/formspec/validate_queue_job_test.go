package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// queueEntity builds an Entity whose `paid` event delivers a `queue` entry with
// the given job, so a test states only the part it varies.
func queueEntity(source, job string) manifest.RawManifest {
	deliver := map[string]any{"channel": "queue"}
	if job != "" {
		deliver["job"] = job
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: "order", Module: "billing"},
		Spec: map[string]any{
			"version":        "v1",
			"characteristic": "transaction",
			"fields":         []any{map[string]any{"name": "status", "type": "string"}},
			"events": []any{map[string]any{
				"name":    "paid",
				"type":    "async",
				"publish": map[string]any{"durable": true},
				"deliver": []any{deliver},
			}},
		},
	}
}

// queueService declares a Service with the given action, the home a `job:` names.
func queueService(source, module, name string, actions ...string) manifest.RawManifest {
	raw := make([]any, 0, len(actions))
	for _, a := range actions {
		raw = append(raw, map[string]any{"name": a})
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Service",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: name, Module: module},
		Spec:       map[string]any{"version": "v1", "actions": raw},
	}
}

// TestValidateEventTargets_QueueJobResolves is the acceptance for the `queue`
// job contract: the job names a Service action, and naming one that exists
// validates clean.
func TestValidateEventTargets_QueueJobResolves(t *testing.T) {
	manifests := []manifest.RawManifest{
		queueEntity("order.yaml", "receipt-jobs.generate-receipt"),
		queueService("receipt-jobs.yaml", "billing", "receipt-jobs", "generate-receipt"),
	}
	if rejects := validateEventTargets(manifests); len(rejects) != 0 {
		t.Fatalf("a resolving job must be accepted, got %v", rejects)
	}
}

// TestValidateEventTargets_QueueJobMissing: without a job the worker has nothing
// to call, so it is refused rather than left to dead-letter silently.
func TestValidateEventTargets_QueueJobMissing(t *testing.T) {
	msg, ok := validateEventTargets([]manifest.RawManifest{queueEntity("order.yaml", "")})["order.yaml"]
	if !ok {
		t.Fatal("a queue entry without `job:` must be rejected")
	}
	if !strings.Contains(msg, "without a `job:`") {
		t.Errorf("error %q should say the job is missing", msg)
	}
}

// TestValidateEventTargets_QueueJobDoesNotExist: the measured failure mode — a
// job naming an action that is not there retried to dead-letter while validate
// stayed green.
func TestValidateEventTargets_QueueJobDoesNotExist(t *testing.T) {
	manifests := []manifest.RawManifest{
		queueEntity("order.yaml", "receipt-jobs.generate-receipt"),
		queueService("receipt-jobs.yaml", "billing", "receipt-jobs", "something-else"),
	}
	msg, ok := validateEventTargets(manifests)["order.yaml"]
	if !ok {
		t.Fatal("a job naming a non-existent Service action must be rejected")
	}
	for _, want := range []string{"generate-receipt", "does not exist"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
}

// TestValidateEventTargets_QueueJobBareNameRejected: a bare name has no Service
// to belong to, so it is refused instead of resolved against a convention
// nobody declared.
func TestValidateEventTargets_QueueJobBareNameRejected(t *testing.T) {
	msg, ok := validateEventTargets([]manifest.RawManifest{queueEntity("order.yaml", "generate-receipt")})["order.yaml"]
	if !ok {
		t.Fatal("a bare job name must be rejected")
	}
	if !strings.Contains(msg, "service.action") {
		t.Errorf("error %q should show the expected forms", msg)
	}
}

// TestValidateEventTargets_QueueModuleQualifiedJob: the explicit form resolves
// against another module, which is how a job owned by a different module is
// declared.
func TestValidateEventTargets_QueueModuleQualifiedJob(t *testing.T) {
	manifests := []manifest.RawManifest{
		queueEntity("order.yaml", "gl.journal-jobs.post"), // publisher module is billing
		queueService("journal-jobs.yaml", "gl", "journal-jobs", "post"),
	}
	if rejects := validateEventTargets(manifests); len(rejects) != 0 {
		t.Fatalf("a module-qualified job must resolve, got %v", rejects)
	}
}
