package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/primadi/formspec/internal/artifact"
)

// EvidenceSender buffers evidence records locally and sends them to the
// Control Plane in batches. Buffering is disk-backed for reliability.
type EvidenceSender struct {
	mu         sync.Mutex
	controlURL string
	instanceID string
	sequence   int64
	httpClient *http.Client
	bufferPath string
	buffer     []artifact.EvidenceRecord
}

// NewEvidenceSender creates a new evidence sender with disk-backed buffer.
func NewEvidenceSender(controlURL, instanceID, dataDir string) (*EvidenceSender, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	s := &EvidenceSender{
		controlURL: controlURL,
		instanceID: instanceID,
		sequence:   0,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		bufferPath: filepath.Join(dataDir, "evidence_buffer.json"),
		buffer:     make([]artifact.EvidenceRecord, 0),
	}

	// Load buffered evidence from disk
	s.loadBuffer()

	return s, nil
}

// SubmitDeployStatus sends a deploy status evidence record.
func (s *EvidenceSender) SubmitDeployStatus(payload *artifact.DeployStatusPayload) {
	payloadData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[resource] Evidence marshal error: %v", err)
		return
	}

	s.enqueue(artifact.EvidenceRecord{
		Type:       artifact.EvidenceDeployStatus,
		InstanceID: s.instanceID,
		Sequence:   s.nextSequence(),
		Payload:    payloadData,
	})
}

// SubmitHealth sends a heartbeat evidence record.
func (s *EvidenceSender) SubmitHealth(up time.Duration, specCount int) {
	payload := map[string]any{
		"uptime_seconds": int(up.Seconds()),
		"spec_count":     specCount,
	}
	payloadData, _ := json.Marshal(payload)

	s.enqueue(artifact.EvidenceRecord{
		Type:       artifact.EvidenceHealth,
		InstanceID: s.instanceID,
		Sequence:   s.nextSequence(),
		Payload:    payloadData,
	})
}

// Flush sends all buffered evidence records to the Control Plane.
// Returns the number of successfully sent records.
func (s *EvidenceSender) Flush() int {
	s.mu.Lock()
	if len(s.buffer) == 0 {
		s.mu.Unlock()
		return 0
	}

	// Take a snapshot of the buffer
	batch := make([]artifact.EvidenceRecord, len(s.buffer))
	copy(batch, s.buffer)
	s.buffer = s.buffer[:0]
	s.mu.Unlock()

	// Send batch
	sent, err := s.sendBatch(batch)
	if err != nil {
		log.Printf("[resource] Evidence flush failed: %v (re-buffering %d records)", err, len(batch))

		// Re-buffer failed records
		s.mu.Lock()
		s.buffer = append(batch, s.buffer...)
		s.mu.Unlock()
		s.saveBuffer()
		return 0
	}

	if sent > 0 {
		log.Printf("[resource] Evidence flushed: %d records sent", sent)
	}

	// Clean up saved buffer (all sent)
	s.saveBuffer()

	return sent
}

func (s *EvidenceSender) enqueue(record artifact.EvidenceRecord) {
	s.mu.Lock()
	s.buffer = append(s.buffer, record)
	s.mu.Unlock()

	// Auto-flush if buffer is large enough
	if len(s.buffer) >= 10 {
		s.Flush()
	}
}

func (s *EvidenceSender) nextSequence() int64 {
	s.sequence++
	return s.sequence
}

func (s *EvidenceSender) sendBatch(records []artifact.EvidenceRecord) (int, error) {
	batch := make([]map[string]any, len(records))
	for i, r := range records {
		batch[i] = map[string]any{
			"type":        string(r.Type),
			"instance_id": r.InstanceID,
			"sequence":    r.Sequence,
			"payload":     r.Payload,
			"signature":   r.Signature,
		}
	}

	body := map[string]any{"records": batch}
	bodyData, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal batch: %w", err)
	}

	url := fmt.Sprintf("%s/v1/evidence", s.controlURL)
	resp, err := s.httpClient.Post(url, "application/json", bytes.NewReader(bodyData))
	if err != nil {
		return 0, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("evidence POST returned HTTP %d", resp.StatusCode)
	}

	return len(records), nil
}

func (s *EvidenceSender) loadBuffer() {
	data, err := os.ReadFile(s.bufferPath)
	if err != nil {
		return // no buffer file, start fresh
	}

	var records []artifact.EvidenceRecord
	if err := json.Unmarshal(data, &records); err != nil {
		log.Printf("[resource] Warning: corrupted evidence buffer, starting fresh: %v", err)
		return
	}

	s.buffer = records
	if len(records) > 0 {
		log.Printf("[resource] Loaded %d buffered evidence records from disk", len(records))
	}
}

func (s *EvidenceSender) saveBuffer() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.buffer) == 0 {
		os.Remove(s.bufferPath)
		return
	}

	data, err := json.Marshal(s.buffer)
	if err != nil {
		log.Printf("[resource] Warning: failed to marshal evidence buffer: %v", err)
		return
	}

	if err := os.WriteFile(s.bufferPath, data, 0644); err != nil {
		log.Printf("[resource] Warning: failed to save evidence buffer: %v", err)
	}
}

// HealthTicker sends periodic health evidence.
func (s *EvidenceSender) HealthTicker(interval time.Duration, stop chan struct{}) {
	startTime := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.SubmitHealth(time.Since(startTime), 0)
			s.Flush()
		case <-stop:
			return
		}
	}
}
