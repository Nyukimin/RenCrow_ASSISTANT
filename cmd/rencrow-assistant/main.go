package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nyukimin/RenCrow_ASSISTANT/internal/coreclient"
	"github.com/Nyukimin/RenCrow_ASSISTANT/internal/delivery"
	"github.com/Nyukimin/RenCrow_ASSISTANT/internal/notification"
)

func main() {
	if len(os.Args) < 3 || os.Args[1] != "notify" || os.Args[2] != "line" {
		fmt.Fprintln(os.Stderr, "usage: rencrow-assistant notify line --user USER --title TITLE --body BODY [--trace-id ID] [--delivery-id ID] [--core-url URL] [--state-dir DIR] [--json]")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("notify line", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	userID := flags.String("user", "", "ASSISTANT user ID")
	title := flags.String("title", "", "notification title (20 characters or fewer)")
	body := flags.String("body", "", "notification body (100 characters or fewer)")
	traceID := flags.String("trace-id", "", "existing trace ID")
	deliveryID := flags.String("delivery-id", "", "stable delivery ID used for idempotency")
	source := flags.String("source", "manual", "notification source")
	category := flags.String("category", "notice", "notification category")
	coreURL := flags.String("core-url", envOrDefault("RENCROW_CORE_URL", "http://127.0.0.1:18790"), "localhost CORE base URL")
	stateDir := flags.String("state-dir", defaultStateDir(), "ASSISTANT state directory")
	jsonOutput := flags.Bool("json", false, "write JSON result")
	if err := flags.Parse(os.Args[3:]); err != nil {
		os.Exit(2)
	}

	client, err := coreclient.NewLineNotificationClient(*coreURL, &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	store := delivery.NewJSONLStore(filepath.Join(strings.TrimSpace(*stateDir), "deliveries.jsonl"))
	service := notification.NewService(client, store, nil, time.Now)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	record, err := service.SendLine(ctx, notification.Input{
		DeliveryID: *deliveryID,
		TraceID:    *traceID,
		UserID:     *userID,
		Source:     *source,
		Category:   *category,
		Title:      *title,
		Body:       *body,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "LINE notification failed: %v\n", err)
		os.Exit(1)
	}
	if *jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(record)
		return
	}
	fmt.Printf(
		"LINE notification %s: delivery_id=%s trace_id=%s duplicate=%t\n",
		record.Status,
		record.DeliveryID,
		record.TraceID,
		record.Duplicate,
	)
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func defaultStateDir() string {
	if value := strings.TrimSpace(os.Getenv("RENCROW_ASSISTANT_STATE_DIR")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".rencrow/assistant/state"
	}
	return filepath.Join(home, ".rencrow", "assistant", "state")
}
