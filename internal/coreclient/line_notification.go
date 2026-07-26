package coreclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const lineNotificationPath = "/internal/assistant/notifications/line"

// LineNotificationRequest is ASSISTANT's low-level CORE transport request.
type LineNotificationRequest struct {
	DeliveryID string `json:"delivery_id"`
	TraceID    string `json:"trace_id"`
	UserID     string `json:"user_id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
}

// LineNotificationResponse is CORE's correlation-preserving transport result.
type LineNotificationResponse struct {
	Status     string `json:"status"`
	DeliveryID string `json:"delivery_id"`
	TraceID    string `json:"trace_id"`
	Duplicate  bool   `json:"duplicate"`
	TargetType string `json:"target_type"`
}

type uncertainError struct {
	cause error
}

func (e *uncertainError) Error() string {
	return fmt.Sprintf("delivery state is uncertain: %v", e.cause)
}

func (e *uncertainError) Unwrap() error {
	return e.cause
}

// NewUncertainError marks an external send whose result cannot be proven.
func NewUncertainError(cause error) error {
	if cause == nil {
		cause = errors.New("unknown transport result")
	}
	return &uncertainError{cause: cause}
}

// IsUncertain reports whether a delivery must be reconciled before retry.
func IsUncertain(err error) bool {
	var target *uncertainError
	return errors.As(err, &target)
}

// LineNotificationClient calls the localhost-only CORE LINE transport.
type LineNotificationClient struct {
	endpoint   string
	httpClient *http.Client
}

// NewLineNotificationClient creates a client and rejects non-loopback CORE
// URLs until a remote mutual-authentication contract exists.
func NewLineNotificationClient(baseURL string, httpClient *http.Client) (*LineNotificationClient, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return nil, fmt.Errorf("parse CORE URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("CORE URL must use http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("CORE URL must not include credentials, query, or fragment")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if (ip == nil || !ip.IsLoopback()) && !strings.EqualFold(host, "localhost") {
		return nil, fmt.Errorf("CORE LINE notification endpoint is localhost-only")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, fmt.Errorf("CORE URL must not include a path")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &LineNotificationClient{
		endpoint:   strings.TrimRight(parsed.String(), "/") + lineNotificationPath,
		httpClient: httpClient,
	}, nil
}

// Deliver sends one correlation-preserving notification to CORE.
func (c *LineNotificationClient) Deliver(
	ctx context.Context,
	request LineNotificationRequest,
) (LineNotificationResponse, error) {
	var response LineNotificationResponse
	if c == nil || c.httpClient == nil || strings.TrimSpace(c.endpoint) == "" {
		return response, fmt.Errorf("CORE LINE notification client is unavailable")
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return response, fmt.Errorf("encode LINE notification request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return response, fmt.Errorf("create LINE notification request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("X-RenCrow-Client", "RenCrow_ASSISTANT")
	httpRequest.Header.Set("X-RenCrow-Interaction-Profile", "assistant-core")

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return response, NewUncertainError(err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode == http.StatusConflict {
		return response, NewUncertainError(errors.New("CORE reported an uncertain delivery"))
	}
	if httpResponse.StatusCode == http.StatusBadGateway {
		return response, NewUncertainError(errors.New("CORE reported an ambiguous external send"))
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(httpResponse.Body, 4096))
		return response, fmt.Errorf("CORE LINE notification failed with status %d", httpResponse.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(httpResponse.Body, 16*1024)).Decode(&response); err != nil {
		return response, NewUncertainError(fmt.Errorf("decode CORE LINE notification response: %w", err))
	}
	if response.DeliveryID != request.DeliveryID || response.TraceID != request.TraceID || response.Status != "sent" {
		return response, NewUncertainError(errors.New("CORE returned mismatched delivery correlation"))
	}
	return response, nil
}
