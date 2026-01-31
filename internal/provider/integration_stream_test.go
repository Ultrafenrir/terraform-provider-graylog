//go:build integration

package provider

import (
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func TestIntegration_StreamCRUD(t *testing.T) {
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

	// Lookup default index set ID using client (handles version-specific paths)
	sets, err := c.ListIndexSets()
	if err != nil {
		t.Fatalf("list index sets error: %v", err)
	}
	var defaultIS string
	for _, is := range sets {
		if is.Default {
			defaultIS = is.ID
			break
		}
	}
	if defaultIS == "" {
		t.Fatal("default index set not found")
	}

	// Create stream bound to the default index set
	created, err := c.CreateStream(&client.Stream{
		Title:       "tf-itest-stream",
		Description: "integration test stream",
		IndexSetID:  defaultIS,
	})
	if err != nil {
		t.Fatalf("CreateStream error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created stream to have ID")
	}

	got, err := c.GetStream(created.ID)
	if err != nil {
		t.Fatalf("GetStream error: %v", err)
	}
	if got.Title == "" {
		t.Fatalf("unexpected GetStream result: %+v", got)
	}

	got.Title = "tf-itest-stream-upd"
	got.Disabled = true
	upd, err := c.UpdateStream(got.ID, got)
	if err != nil {
		t.Fatalf("UpdateStream error: %v", err)
	}
	if upd.Title != "tf-itest-stream-upd" || !upd.Disabled {
		t.Fatalf("stream not updated as expected: %+v", upd)
	}

	if err := c.DeleteStream(created.ID); err != nil {
		t.Fatalf("DeleteStream error: %v", err)
	}
}
