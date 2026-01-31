//go:build integration

package provider

import (
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// TestIntegration_IndexSetCRUD runs against a real Graylog instance started via docker-compose
func TestIntegration_IndexSetCRUD(t *testing.T) {
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")

	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}

	// Accept both raw "user:pass" and base64 strings for TOKEN
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}

	c := client.New(baseURL, token)

	// Give Graylog a little time just in case after healthcheck
	time.Sleep(2 * time.Second)

	// Prepare payload; Graylog 7 requires explicit replicas/indexOptimizationDisabled/isWritable
	idx := &client.IndexSet{
		Title:       "tf-prov-itest",
		Description: "integration test index set",
		IndexPrefix: "tf_itest_" + time.Now().Format("150405"),
		Shards:      1,
		Replicas:    0,
		Default:     false,
	}
	if c.APIVersion == client.APIV7 {
		idx.Replicas = 1
		idx.IsWritable = true
		idx.IndexOptimizationDisabled = true
	}
	// Create
	created, err := c.CreateIndexSet(idx)
	if err != nil {
		t.Fatalf("CreateIndexSet error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created index set to have ID")
	}

	// Get
	got, err := c.GetIndexSet(created.ID)
	if err != nil {
		t.Fatalf("GetIndexSet error: %v", err)
	}
	if got.IndexPrefix == "" || got.Title == "" {
		t.Fatalf("unexpected GetIndexSet result: %+v", got)
	}

	// Update
	got.Title = "tf-prov-itest-upd"
	upd, err := c.UpdateIndexSet(got.ID, got)
	if err != nil {
		t.Fatalf("UpdateIndexSet error: %v", err)
	}
	if upd.Title != "tf-prov-itest-upd" {
		t.Fatalf("title was not updated: %+v", upd)
	}

	// Delete
	if err := c.DeleteIndexSet(created.ID); err != nil {
		t.Fatalf("DeleteIndexSet error: %v", err)
	}
}
