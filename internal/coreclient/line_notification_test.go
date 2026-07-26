package coreclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLineNotificationClientUsesAssistantProfileAndCorrelation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/assistant/notifications/line" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-RenCrow-Client"); got != "RenCrow_ASSISTANT" {
			t.Fatalf("client header=%q", got)
		}
		if got := r.Header.Get("X-RenCrow-Interaction-Profile"); got != "assistant-core" {
			t.Fatalf("profile header=%q", got)
		}
		var request LineNotificationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.DeliveryID != "dly_001" || request.TraceID != "trc_001" {
			t.Fatalf("correlation lost: %#v", request)
		}
		_ = json.NewEncoder(w).Encode(LineNotificationResponse{
			Status:     "sent",
			DeliveryID: request.DeliveryID,
			TraceID:    request.TraceID,
		})
	}))
	defer server.Close()

	client, err := NewLineNotificationClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Deliver(context.Background(), LineNotificationRequest{
		DeliveryID: "dly_001",
		TraceID:    "trc_001",
		UserID:     "user-001",
		Title:      "RenCrow",
		Body:       "通知です",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "sent" || result.DeliveryID != "dly_001" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestLineNotificationClientRejectsNonLoopbackCoreURL(t *testing.T) {
	if _, err := NewLineNotificationClient("http://core.example.test:18790", http.DefaultClient); err == nil {
		t.Fatal("non-loopback CORE URL must be rejected")
	}
}

func TestLineNotificationClientMapsConflictToUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "delivery state is uncertain", http.StatusConflict)
	}))
	defer server.Close()
	client, err := NewLineNotificationClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Deliver(context.Background(), LineNotificationRequest{
		DeliveryID: "dly_001",
		TraceID:    "trc_001",
		UserID:     "user-001",
		Title:      "RenCrow",
		Body:       "通知です",
	})
	if err == nil || !IsUncertain(err) {
		t.Fatalf("err=%v want uncertain", err)
	}
}

func TestLineNotificationClientMapsBadGatewayToUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "external send failed", http.StatusBadGateway)
	}))
	defer server.Close()
	client, err := NewLineNotificationClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Deliver(context.Background(), LineNotificationRequest{
		DeliveryID: "dly_001",
		TraceID:    "trc_001",
		UserID:     "user-001",
		Title:      "RenCrow",
		Body:       "通知です",
	})
	if err == nil || !IsUncertain(err) {
		t.Fatalf("err=%v want uncertain", err)
	}
}
