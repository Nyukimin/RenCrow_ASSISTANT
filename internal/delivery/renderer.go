package delivery

import (
	"fmt"
	"strings"
)

// InteractionOutput is the surface-independent content handed to a device delivery.
type InteractionOutput struct {
	DeliveryID string `json:"delivery_id"`
	TraceID    string `json:"trace_id"`
	UserID     string `json:"user_id"`
	Source     string `json:"source"`
	Category   string `json:"category"`
	Text       string `json:"text,omitempty"`
	Speech     string `json:"speech,omitempty"`
}

// DeviceCapabilities contains only output capabilities used by the renderer.
type DeviceCapabilities struct {
	Display bool `json:"display"`
	Speaker bool `json:"speaker"`
	Haptics bool `json:"haptics"`
}

// DeviceDelivery is a capability-filtered representation of one InteractionOutput.
type DeviceDelivery struct {
	DeliveryID string `json:"delivery_id"`
	TraceID    string `json:"trace_id"`
	UserID     string `json:"user_id"`
	Source     string `json:"source"`
	Category   string `json:"category"`
	Text       string `json:"text,omitempty"`
	Speech     string `json:"speech,omitempty"`
	Haptic     bool   `json:"haptic,omitempty"`
}

// Render preserves identity and the required trace while adapting content to a device.
func Render(output InteractionOutput, capabilities DeviceCapabilities) (DeviceDelivery, error) {
	if err := validateInteractionOutput(output); err != nil {
		return DeviceDelivery{}, err
	}
	rendered := DeviceDelivery{
		DeliveryID: strings.TrimSpace(output.DeliveryID),
		TraceID:    strings.TrimSpace(output.TraceID),
		UserID:     strings.TrimSpace(output.UserID),
		Source:     strings.TrimSpace(output.Source),
		Category:   strings.TrimSpace(output.Category),
	}
	if capabilities.Display {
		rendered.Text = strings.TrimSpace(output.Text)
	}
	if capabilities.Speaker {
		rendered.Speech = strings.TrimSpace(output.Speech)
		if rendered.Speech == "" {
			rendered.Speech = strings.TrimSpace(output.Text)
		}
	}
	rendered.Haptic = capabilities.Haptics
	if rendered.Text == "" && rendered.Speech == "" && !rendered.Haptic {
		return DeviceDelivery{}, fmt.Errorf("device has no compatible output capability")
	}
	return rendered, nil
}

func validateInteractionOutput(output InteractionOutput) error {
	required := map[string]string{
		"delivery_id": output.DeliveryID,
		"trace_id":    output.TraceID,
		"user_id":     output.UserID,
		"source":      output.Source,
		"category":    output.Category,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if strings.TrimSpace(output.Text) == "" && strings.TrimSpace(output.Speech) == "" {
		return fmt.Errorf("text or speech is required")
	}
	return nil
}
