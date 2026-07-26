package notification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Nyukimin/RenCrow_ASSISTANT/internal/coreclient"
	"github.com/Nyukimin/RenCrow_ASSISTANT/internal/delivery"
)

type recordingTransport struct {
	requests []coreclient.LineNotificationRequest
	result   coreclient.LineNotificationResponse
	err      error
}

func (t *recordingTransport) Deliver(_ context.Context, request coreclient.LineNotificationRequest) (coreclient.LineNotificationResponse, error) {
	t.requests = append(t.requests, request)
	return t.result, t.err
}

type memoryDeliveryStore struct {
	records []delivery.Record
}

func (s *memoryDeliveryStore) Save(_ context.Context, record delivery.Record) error {
	s.records = append(s.records, record)
	return nil
}

func (s *memoryDeliveryStore) Latest(_ context.Context, deliveryID string) (delivery.Record, bool, error) {
	for index := len(s.records) - 1; index >= 0; index-- {
		if s.records[index].DeliveryID == deliveryID {
			return s.records[index], true, nil
		}
	}
	return delivery.Record{}, false, nil
}

func TestServiceCreatesRequiredIDsAndRecordsSentState(t *testing.T) {
	transport := &recordingTransport{result: coreclient.LineNotificationResponse{Status: "sent"}}
	store := &memoryDeliveryStore{}
	counter := 0
	service := NewService(transport, store,
		func(prefix string) (string, error) {
			counter++
			return prefix + "generated-" + string(rune('0'+counter)), nil
		},
		func() time.Time { return time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC) },
	)

	record, err := service.SendLine(context.Background(), Input{
		UserID:   "user-001",
		Title:    "RenCrow",
		Body:     "通知です",
		Source:   "manual",
		Category: "notice",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(record.TraceID, "trc_") || !strings.HasPrefix(record.DeliveryID, "dly_") {
		t.Fatalf("required IDs not generated: %#v", record)
	}
	if record.Status != delivery.StatusSent || len(transport.requests) != 1 {
		t.Fatalf("record=%#v requests=%d", record, len(transport.requests))
	}
	if len(store.records) != 2 || store.records[0].Status != delivery.StatusPending || store.records[1].Status != delivery.StatusSent {
		t.Fatalf("state transitions=%#v", store.records)
	}
}

func TestServiceDoesNotResendCompletedDelivery(t *testing.T) {
	transport := &recordingTransport{}
	store := &memoryDeliveryStore{records: []delivery.Record{{
		DeliveryID: "dly_existing",
		TraceID:    "trc_existing",
		Status:     delivery.StatusSent,
	}}}
	service := NewService(transport, store, nil, time.Now)
	record, err := service.SendLine(context.Background(), Input{
		DeliveryID: "dly_existing",
		TraceID:    "trc_existing",
		UserID:     "user-001",
		Title:      "RenCrow",
		Body:       "通知です",
		Source:     "manual",
		Category:   "notice",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != delivery.StatusSent || len(transport.requests) != 0 {
		t.Fatalf("record=%#v requests=%d", record, len(transport.requests))
	}
}

func TestServiceRecordsUncertainWithoutAutomaticRetry(t *testing.T) {
	transport := &recordingTransport{err: coreclient.NewUncertainError(errors.New("connection lost"))}
	store := &memoryDeliveryStore{}
	service := NewService(transport, store, nil, time.Now)
	_, err := service.SendLine(context.Background(), Input{
		DeliveryID: "dly_uncertain",
		TraceID:    "trc_uncertain",
		UserID:     "user-001",
		Title:      "RenCrow",
		Body:       "通知です",
		Source:     "manual",
		Category:   "notice",
	})
	if err == nil || !coreclient.IsUncertain(err) {
		t.Fatalf("err=%v want uncertain", err)
	}
	if got := store.records[len(store.records)-1].Status; got != delivery.StatusUncertain {
		t.Fatalf("status=%s want=%s", got, delivery.StatusUncertain)
	}
}
