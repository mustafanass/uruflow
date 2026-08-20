package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/mustafanass/uruflow/internal/services"
)

func TestWebhookHeadersRequirePushAndDeliveryID(t *testing.T) {
	request := httptest.NewRequest("POST", "/webhook", nil)
	request.Header.Set("X-GitHub-Event", "push")
	request.Header.Set("X-GitHub-Delivery", "delivery-1")
	if id, ok := webhookHeaders(request, services.ProviderGitHub); !ok || id != "delivery-1" {
		t.Fatalf("valid headers = %q, %v", id, ok)
	}
	request.Header.Set("X-GitHub-Event", "issues")
	if _, ok := webhookHeaders(request, services.ProviderGitHub); ok {
		t.Fatal("non-push event was accepted")
	}
	request.Header.Set("X-GitHub-Event", "push")
	request.Header.Del("X-GitHub-Delivery")
	if _, ok := webhookHeaders(request, services.ProviderGitHub); ok {
		t.Fatal("event without a delivery id was accepted")
	}
}
