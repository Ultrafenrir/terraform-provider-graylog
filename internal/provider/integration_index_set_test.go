//go:build integration

package provider

import (
	"context"
	"encoding/base64"
	"errors"
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
// getIndexSetEventually retries a not-found read for a short while.
//
// A GET issued immediately after a successful create can transiently 404 —
// the resource itself already compensates for this (see the retry loop in
// resource_index_set.go). It shows up most often on a cold Graylog 7, which
// is exactly what CI starts, so without the same tolerance here the test
// fails on the timing rather than on behaviour.
func getIndexSetEventually(t *testing.T, c *client.Client, id string) *client.IndexSet {
	t.Helper()
	is, err := c.GetIndexSet(id)
	for attempt := 0; attempt < 5 && errors.Is(err, client.ErrNotFound); attempt++ {
		time.Sleep(300 * time.Millisecond)
		is, err = c.GetIndexSet(id)
	}
	if err != nil {
		t.Fatalf("GetIndexSet error: %v", err)
	}
	return is
}

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
		Title:                    "tf-prov-itest",
		Description:              "integration test index set",
		IndexPrefix:              "tf_itest_" + time.Now().Format("150405"),
		Shards:                   1,
		Replicas:                 0,
		FieldTypeRefreshInterval: 7000,
		Default:                  false,
	}
	if c.APIVersion == client.APIV7 {
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
	readyCtx, cancelReady := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelReady()
	if err := c.WithContext(readyCtx).EnsureIndexSetReady(created.ID, time.Second); err != nil {
		t.Fatalf("index set did not become write-ready: %v", err)
	}

	// Get
	got := getIndexSetEventually(t, c, created.ID)
	if got.IndexPrefix == "" || got.Title == "" {
		t.Fatalf("unexpected GetIndexSet result: %+v", got)
	}
	if got.FieldTypeRefreshInterval != 7000 {
		t.Fatalf("field_type_refresh_interval was not persisted on create: want 7000, got %d", got.FieldTypeRefreshInterval)
	}

	// Update
	got.Title = "tf-prov-itest-upd"
	got.FieldTypeRefreshInterval = 9000
	upd, err := c.UpdateIndexSet(got.ID, got)
	if err != nil {
		t.Fatalf("UpdateIndexSet error: %v", err)
	}
	if upd.Title != "tf-prov-itest-upd" {
		t.Fatalf("title was not updated: %+v", upd)
	}
	after := getIndexSetEventually(t, c, got.ID)
	if after.FieldTypeRefreshInterval != 9000 {
		t.Fatalf("field_type_refresh_interval was not persisted on update: want 9000, got %d", after.FieldTypeRefreshInterval)
	}

	// Delete. The same transient not-found applies here, and an index set
	// that is already gone is the desired end state anyway.
	err = c.DeleteIndexSet(created.ID)
	for attempt := 0; attempt < 5 && errors.Is(err, client.ErrNotFound); attempt++ {
		time.Sleep(300 * time.Millisecond)
		err = c.DeleteIndexSet(created.ID)
	}
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		t.Fatalf("DeleteIndexSet error: %v", err)
	}
}
