package interaction

import "net/http"

const (
	ClientName    = "RenCrow_ASSISTANT"
	CoreProfile   = "assistant-core"
	ClientHeader  = "X-RenCrow-Client"
	ProfileHeader = "X-RenCrow-Interaction-Profile"
)

// ApplyCoreProfile marks a request as an ASSISTANT-to-CORE interaction.
// These headers select a capability policy; they are not authentication credentials.
func ApplyCoreProfile(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Set(ClientHeader, ClientName)
	req.Header.Set(ProfileHeader, CoreProfile)
}
