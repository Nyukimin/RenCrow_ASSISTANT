package delivery

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Status is the ASSISTANT-owned delivery state.
type Status string

const (
	StatusPending   Status = "pending"
	StatusSent      Status = "sent"
	StatusFailed    Status = "failed"
	StatusUncertain Status = "uncertain"
)

// Record is ASSISTANT's notification delivery state. Message content is not
// persisted in this transport-oriented record.
type Record struct {
	DeliveryID string    `json:"delivery_id"`
	TraceID    string    `json:"trace_id"`
	UserID     string    `json:"user_id"`
	Source     string    `json:"source"`
	Category   string    `json:"category"`
	Channel    string    `json:"channel"`
	Status     Status    `json:"status"`
	Duplicate  bool      `json:"duplicate,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Store persists and resolves the latest state for a delivery ID.
type Store interface {
	Save(ctx context.Context, record Record) error
	Latest(ctx context.Context, deliveryID string) (Record, bool, error)
}

// JSONLStore is the local-first ASSISTANT delivery state store.
type JSONLStore struct {
	path string
	mu   sync.Mutex
}

// NewJSONLStore creates the single-process local-first delivery store.
func NewJSONLStore(path string) *JSONLStore {
	return &JSONLStore{path: strings.TrimSpace(path)}
}

func (s *JSONLStore) Save(_ context.Context, record Record) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return fmt.Errorf("delivery store path is required")
	}
	if err := validateRecord(record); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create delivery state directory: %w", err)
	}
	file, err := os.OpenFile(s.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open delivery state: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat delivery state: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("delivery state must be a regular file with 0600 permissions")
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode delivery state: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append delivery state: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync delivery state: %w", err)
	}
	return nil
}

func (s *JSONLStore) Latest(_ context.Context, deliveryID string) (Record, bool, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return Record{}, false, fmt.Errorf("delivery store path is required")
	}
	deliveryID = strings.TrimSpace(deliveryID)
	if deliveryID == "" {
		return Record{}, false, fmt.Errorf("delivery_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("open delivery state: %w", err)
	}
	defer file.Close()
	var latest Record
	found := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return Record{}, false, fmt.Errorf("decode delivery state: %w", err)
		}
		if record.DeliveryID == deliveryID {
			latest = record
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		return Record{}, false, fmt.Errorf("read delivery state: %w", err)
	}
	return latest, found, nil
}

func validateRecord(record Record) error {
	for name, value := range map[string]string{
		"delivery_id": record.DeliveryID,
		"trace_id":    record.TraceID,
		"user_id":     record.UserID,
		"source":      record.Source,
		"category":    record.Category,
		"channel":     record.Channel,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	switch record.Status {
	case StatusPending, StatusSent, StatusFailed, StatusUncertain:
	default:
		return fmt.Errorf("unsupported delivery status %q", record.Status)
	}
	if record.CreatedAt.IsZero() || record.UpdatedAt.IsZero() {
		return fmt.Errorf("delivery timestamps are required")
	}
	return nil
}
