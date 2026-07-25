package delivery

import "testing"

func TestRenderInteractionOutputUsesDeviceCapabilities(t *testing.T) {
	output := InteractionOutput{
		DeliveryID: "delivery-001",
		TraceID:    "trace-001",
		UserID:     "user-001",
		Source:     "assistant",
		Category:   "morning_brief",
		Text:       "午後は雨です",
		Speech:     "午後は雨です。",
	}

	rendered, err := Render(output, DeviceCapabilities{
		Display: true,
		Speaker: true,
		Haptics: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered.DeliveryID != output.DeliveryID || rendered.TraceID != output.TraceID {
		t.Fatalf("correlation was not preserved: %#v", rendered)
	}
	if rendered.Text != output.Text || rendered.Speech != output.Speech || !rendered.Haptic {
		t.Fatalf("unexpected rendered delivery: %#v", rendered)
	}
}

func TestRenderInteractionOutputRejectsMissingIdentity(t *testing.T) {
	_, err := Render(InteractionOutput{
		DeliveryID: "delivery-001",
		UserID:     "user-001",
		Source:     "assistant",
		Category:   "alarm",
		Text:       "起きる時間です",
	}, DeviceCapabilities{Display: true})
	if err == nil {
		t.Fatal("missing trace_id must be rejected")
	}
}

func TestRenderInteractionOutputRejectsDeviceWithoutOutputCapability(t *testing.T) {
	_, err := Render(InteractionOutput{
		DeliveryID: "delivery-001",
		TraceID:    "trace-001",
		UserID:     "user-001",
		Source:     "assistant",
		Category:   "alarm",
		Text:       "起きる時間です",
	}, DeviceCapabilities{})
	if err == nil {
		t.Fatal("device without output capability must be rejected")
	}
}
