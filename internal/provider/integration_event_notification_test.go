//go:build integration

package provider

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

// TestIntegration_EventNotificationCRUD validates CRUD for event notifications against a live Graylog
func TestIntegration_EventNotificationCRUD(t *testing.T) {
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	c := client.New(baseURL, token)

	// Graylog 5 has different payload schema for notifications; skip until v5 mapping is implemented
	if c.APIVersion == client.APIV5 {
		t.Skip("Event Notification payload differs on Graylog 5.x; skipping integration test for v5")
	}

	// minimal email notification
	emailCfg := map[string]any{
		"sender":           "noreply@example.com",
		"subject":          "Test subject",
		"body_template":    "Test body",
		"user_recipients":  []string{"admin"},
		"email_recipients": []string{"root@example.com"},
	}
	created := createNotification(t, c, "tf-itest-email", "email", emailCfg)
	defer func() { _ = c.DeleteEventNotification(created.ID) }()

	// read
	got, err := c.GetEventNotification(created.ID)
	if err != nil {
		t.Fatalf("read notification: %v", err)
	}
	if got.Title != "tf-itest-email" || got.Type != "email" {
		b, _ := json.Marshal(got)
		t.Fatalf("unexpected notification payload: %s", string(b))
	}

	// update: change description
	got.Description = "updated via test"
	if _, err := c.UpdateEventNotification(got.ID, got); err != nil {
		t.Fatalf("update notification: %v", err)
	}

	// delete
	if err := c.DeleteEventNotification(got.ID); err != nil {
		t.Fatalf("delete notification: %v", err)
	}
}

func createNotification(t *testing.T, c *client.Client, title, ntype string, cfg map[string]any) *client.EventNotification {
	t.Helper()
	n := &client.EventNotification{
		Title:       title,
		Type:        ntype,
		Description: "created by integration test",
		Config:      cfg,
	}
	created, err := c.CreateEventNotification(n)
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("empty id for created notification")
	}
	return created
}
