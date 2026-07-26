package delivery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONLStorePersistsLatestStateWithPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "deliveries.jsonl")
	store := NewJSONLStore(path)
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	record := Record{
		DeliveryID: "dly_001",
		TraceID:    "trc_001",
		UserID:     "user-001",
		Source:     "manual",
		Category:   "notice",
		Channel:    "line",
		Status:     StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := store.Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	record.Status = StatusSent
	record.UpdatedAt = now.Add(time.Second)
	if err := store.Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	latest, found, err := store.Latest(context.Background(), "dly_001")
	if err != nil {
		t.Fatal(err)
	}
	if !found || latest.Status != StatusSent {
		t.Fatalf("latest=%#v found=%t", latest, found)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions=%o want=600", info.Mode().Perm())
	}
}
