//go:build integration

package provider

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
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

	// Lookup default index set ID (required by Graylog 5.x API)
	listPath := "/system/indices/index_sets"
	if c.APIVersion == client.APIV6 || c.APIVersion == client.APIV7 {
		listPath = "/api/system/indices/index_sets"
	}
	req, _ := http.NewRequest("GET", c.BaseURL+listPath, nil)
	req.Header.Set("Authorization", "Basic "+token)
	req.Header.Set("X-Requested-By", "terraform-provider-test")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("list index sets request error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("unexpected list index sets status: %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode index sets response: %v", err)
	}
	var defaultIS string
	if arr, ok := payload["index_sets"].([]any); ok {
		for _, it := range arr {
			if m, ok := it.(map[string]any); ok {
				if def, _ := m["default"].(bool); def {
					if id, ok := m["id"].(string); ok {
						defaultIS = id
						break
					}
				}
			}
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
