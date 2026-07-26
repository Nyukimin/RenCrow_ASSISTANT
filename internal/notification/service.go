package notification

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Nyukimin/RenCrow_ASSISTANT/internal/coreclient"
	"github.com/Nyukimin/RenCrow_ASSISTANT/internal/delivery"
)

// LineTransport delivers one notification through CORE's existing LINE
// adapter.
type LineTransport interface {
	Deliver(ctx context.Context, request coreclient.LineNotificationRequest) (coreclient.LineNotificationResponse, error)
}

// Input is a surface-independent ASSISTANT notification request.
type Input struct {
	DeliveryID string
	TraceID    string
	UserID     string
	Source     string
	Category   string
	Title      string
	Body       string
}

// IDGenerator creates trace and delivery identifiers with the supplied prefix.
type IDGenerator func(prefix string) (string, error)

// Service owns notification correlation and delivery state while delegating
// the low-level LINE push to CORE.
type Service struct {
	transport LineTransport
	store     delivery.Store
	newID     IDGenerator
	now       func() time.Time
}

// NewService creates the ASSISTANT notification application service.
func NewService(
	transport LineTransport,
	store delivery.Store,
	newID IDGenerator,
	now func() time.Time,
) *Service {
	if newID == nil {
		newID = generateID
	}
	if now == nil {
		now = time.Now
	}
	return &Service{transport: transport, store: store, newID: newID, now: now}
}

// SendLine records state and delegates exactly one LINE push attempt to CORE.
func (s *Service) SendLine(ctx context.Context, input Input) (delivery.Record, error) {
	if s == nil || s.transport == nil || s.store == nil {
		return delivery.Record{}, fmt.Errorf("notification service is unavailable")
	}
	input = normalizeInput(input)
	if err := validateInput(input); err != nil {
		return delivery.Record{}, err
	}
	var err error
	if input.TraceID == "" {
		input.TraceID, err = s.newID("trc_")
		if err != nil {
			return delivery.Record{}, fmt.Errorf("generate trace_id: %w", err)
		}
	}
	if input.DeliveryID == "" {
		input.DeliveryID, err = s.newID("dly_")
		if err != nil {
			return delivery.Record{}, fmt.Errorf("generate delivery_id: %w", err)
		}
	}
	if existing, found, err := s.store.Latest(ctx, input.DeliveryID); err != nil {
		return delivery.Record{}, err
	} else if found {
		if existing.TraceID != input.TraceID {
			return delivery.Record{}, fmt.Errorf("delivery_id is already associated with another trace_id")
		}
		switch existing.Status {
		case delivery.StatusSent:
			return existing, nil
		case delivery.StatusPending, delivery.StatusUncertain:
			return existing, coreclient.NewUncertainError(fmt.Errorf("delivery %s requires reconciliation", input.DeliveryID))
		}
	}

	now := s.now().UTC()
	record := delivery.Record{
		DeliveryID: input.DeliveryID,
		TraceID:    input.TraceID,
		UserID:     input.UserID,
		Source:     input.Source,
		Category:   input.Category,
		Channel:    "line",
		Status:     delivery.StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.store.Save(ctx, record); err != nil {
		return delivery.Record{}, err
	}

	result, sendErr := s.transport.Deliver(ctx, coreclient.LineNotificationRequest{
		DeliveryID: input.DeliveryID,
		TraceID:    input.TraceID,
		UserID:     input.UserID,
		Title:      input.Title,
		Body:       input.Body,
	})
	record.UpdatedAt = s.now().UTC()
	if sendErr != nil {
		if coreclient.IsUncertain(sendErr) {
			record.Status = delivery.StatusUncertain
		} else {
			record.Status = delivery.StatusFailed
		}
		if err := s.store.Save(ctx, record); err != nil {
			return record, fmt.Errorf("send error: %v; persist delivery state: %w", sendErr, err)
		}
		return record, sendErr
	}
	record.Status = delivery.StatusSent
	record.Duplicate = result.Duplicate
	if err := s.store.Save(ctx, record); err != nil {
		return record, fmt.Errorf("persist sent delivery state: %w", err)
	}
	return record, nil
}

func normalizeInput(input Input) Input {
	input.DeliveryID = strings.TrimSpace(input.DeliveryID)
	input.TraceID = strings.TrimSpace(input.TraceID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.Source = strings.TrimSpace(input.Source)
	input.Category = strings.TrimSpace(input.Category)
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	return input
}

func validateInput(input Input) error {
	for name, value := range map[string]string{
		"user_id":  input.UserID,
		"source":   input.Source,
		"category": input.Category,
		"title":    input.Title,
		"body":     input.Body,
	} {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if utf8.RuneCountInString(input.Title) > 20 {
		return fmt.Errorf("title must be 20 characters or fewer")
	}
	if utf8.RuneCountInString(input.Body) > 100 {
		return fmt.Errorf("body must be 100 characters or fewer")
	}
	for name, value := range map[string]string{
		"delivery_id": input.DeliveryID,
		"trace_id":    input.TraceID,
		"user_id":     input.UserID,
	} {
		if len(value) > 128 || strings.ContainsAny(value, "\r\n\t") {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	return nil
}

func generateID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%s%08x-%04x-%04x-%04x-%012x",
		prefix,
		raw[0:4],
		raw[4:6],
		raw[6:8],
		raw[8:10],
		raw[10:16],
	), nil
}
