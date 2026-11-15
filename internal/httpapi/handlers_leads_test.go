package httpapi

import (
	"testing"

	"zoo-go/internal/domain"
)

func TestCreatePilotLead(t *testing.T) {
	api := newTestAPI(t)

	created := api.request(t, "POST", "/v1/leads", "", map[string]any{
		"role":      domain.LeadRoleOwner,
		"name":      "张女士",
		"phone":     "13800138000",
		"community": "朗润园",
		"note":      "周末需要喂猫",
		"source":    "website",
	}, 201)
	data, _ := created["data"].(map[string]any)
	if data["id"] == nil || data["id"] == "" {
		t.Fatalf("expected lead id, got %#v", created)
	}

	api.request(t, "POST", "/v1/leads", "", map[string]any{
		"role":  domain.LeadRoleOwner,
		"name":  "张女士",
		"phone": "13800138000",
	}, 429)

	api.request(t, "POST", "/v1/leads", "", map[string]any{
		"role":  "invalid",
		"name":  "李同学",
		"phone": "13900139000",
	}, 400)

	listed := api.adminRequest(t, "GET", "/v1/admin/leads?status=NEW", nil, 200)
	items, _ := listed["data"].([]any)
	if len(items) != 1 {
		t.Fatalf("admin leads len = %d, want 1, body=%#v", len(items), listed)
	}
}
