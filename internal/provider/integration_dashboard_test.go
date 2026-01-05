//go:build integration

package provider

import (
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func TestIntegration_DashboardCRUD(t *testing.T) {
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	c := client.New(baseURL, token)

	// Graylog 5.x no longer supports legacy /dashboards CRUD (migrated to Views API)
	if c.APIVersion == client.APIV5 {
		t.Skip("dashboard CRUD is not supported in Graylog 5.x; skipping test")
	}

	time.Sleep(2 * time.Second)

	created, err := c.CreateDashboard(&client.Dashboard{
		Title:       "tf-itest-dashboard",
		Description: "integration test dashboard",
	})
	if err != nil {
		t.Fatalf("CreateDashboard error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created dashboard to have ID")
	}

	got, err := c.GetDashboard(created.ID)
	if err != nil {
		t.Fatalf("GetDashboard error: %v", err)
	}
	if got.Title == "" {
		t.Fatalf("unexpected GetDashboard result: %+v", got)
	}

	got.Title = "tf-itest-dashboard-upd"
	upd, err := c.UpdateDashboard(got.ID, got)
	if err != nil {
		t.Fatalf("UpdateDashboard error: %v", err)
	}
	if upd.Title != "tf-itest-dashboard-upd" {
		t.Fatalf("dashboard not updated as expected: %+v", upd)
	}

	if err := c.DeleteDashboard(created.ID); err != nil {
		t.Fatalf("DeleteDashboard error: %v", err)
	}
}
