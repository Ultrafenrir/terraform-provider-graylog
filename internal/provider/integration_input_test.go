//go:build integration

package provider

import (
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

// TestIntegration_InputCRUD validates Input CRUD against a real Graylog
func TestIntegration_InputCRUD(t *testing.T) {
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	c := client.New(baseURL, token)

	time.Sleep(2 * time.Second)

	// minimal GELF UDP input
	cfg := map[string]any{
		"bind_address": "0.0.0.0",
		"port":         12201,
	}
	created, err := c.CreateInput(&client.Input{
		Title:         "tf-itest-input",
		Type:          "org.graylog2.inputs.gelf.udp.GELFUDPInput",
		Global:        true,
		Configuration: cfg,
	})
	if err != nil {
		t.Fatalf("CreateInput error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created input to have ID")
	}

	got, err := c.GetInput(created.ID)
	if err != nil {
		t.Fatalf("GetInput error: %v", err)
	}
	if got.Title == "" || got.Type == "" {
		t.Fatalf("unexpected GetInput result: %+v", got)
	}

	got.Title = "tf-itest-input-upd"
	// Ensure configuration is present on update (Graylog 5.x requires non-null configuration)
	if got.Configuration == nil || len(got.Configuration) == 0 {
		got.Configuration = cfg
	}
	// Build update payload without ID field (Graylog rejects 'id' in update body)
	updPayload := &client.Input{
		Title:         got.Title,
		Type:          got.Type,
		Global:        got.Global,
		Node:          got.Node,
		Configuration: got.Configuration,
	}
	if _, err := c.UpdateInput(got.ID, updPayload); err != nil {
		t.Fatalf("UpdateInput error: %v", err)
	}
	// Graylog 5.x may not echo full object on update; verify by fetching
	after, err := c.GetInput(got.ID)
	if err != nil {
		t.Fatalf("GetInput after update error: %v", err)
	}
	if after.Title != "tf-itest-input-upd" {
		t.Fatalf("title was not updated: %+v", after)
	}

	if err := c.DeleteInput(created.ID); err != nil {
		t.Fatalf("DeleteInput error: %v", err)
	}
}
