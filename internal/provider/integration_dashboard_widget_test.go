//go:build integration

package provider

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func TestIntegration_DashboardWidgetCRUD(t *testing.T) {
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	c := client.New(baseURL, token)

	// Classic dashboards CRUD is not available on some Graylog versions/images
	if c.APIVersion == client.APIV5 || c.APIVersion == client.APIV6 {
		t.Skip("Dashboard CRUD is not supported by this Graylog version/image; skipping integration test")
	}

	// Prepare dashboard fixture
	dash, err := c.CreateDashboard(&client.Dashboard{Title: "tf-itest-dash", Description: "itest"})
	if err != nil {
		t.Fatalf("CreateDashboard error: %v", err)
	}
	defer func() { _ = c.DeleteDashboard(dash.ID) }()

	// Create a simple search result count widget (classic dashboards expect a type and config)
	cfg := map[string]any{
		"timerange": map[string]any{"type": "relative", "range": 300},
		"query":     "*",
		"stream_id": nil,
	}
	w, err := c.CreateDashboardWidget(dash.ID, &client.DashboardWidget{
		Type:          "SEARCH_RESULT_COUNT",
		Description:   "tf-itest widget",
		CacheTime:     1,
		Configuration: cfg,
	})
	if err != nil {
		t.Fatalf("CreateDashboardWidget error: %v", err)
	}
	defer func() { _ = c.DeleteDashboardWidget(dash.ID, w.ID) }()

	got, err := c.GetDashboardWidget(dash.ID, w.ID)
	if err != nil {
		t.Fatalf("GetDashboardWidget error: %v", err)
	}
	if got.Type == "" || got.ID == "" {
		t.Fatalf("unexpected widget: %+v", got)
	}

	// Update description
	got.Description = "tf-itest widget upd"
	got.CacheTime = 2
	if _, err := c.UpdateDashboardWidget(dash.ID, got.ID, got); err != nil {
		t.Fatalf("UpdateDashboardWidget error: %v", err)
	}

	if err := c.DeleteDashboardWidget(dash.ID, w.ID); err != nil {
		t.Fatalf("DeleteDashboardWidget error: %v", err)
	}
}
