package interaction

import (
	"net/http"
	"testing"
)

func TestApplyCoreProfileUsesCanonicalAssistantIdentity(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://core.test/viewer/send", nil)
	if err != nil {
		t.Fatal(err)
	}

	ApplyCoreProfile(req)

	if got := req.Header.Get("X-RenCrow-Client"); got != "RenCrow_ASSISTANT" {
		t.Fatalf("X-RenCrow-Client = %q", got)
	}
	if got := req.Header.Get("X-RenCrow-Interaction-Profile"); got != "assistant-core" {
		t.Fatalf("X-RenCrow-Interaction-Profile = %q", got)
	}
}
