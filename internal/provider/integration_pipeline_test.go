//go:build integration

package provider

import (
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func TestIntegration_PipelineCRUD(t *testing.T) {
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

	// Graylog 5.x requires non-empty pipeline source
	minimalSource := "pipeline \"tf_itest\"\nstage 0 match either\nend\n"
	created, err := c.CreatePipeline(&client.Pipeline{
		Title:       "tf-itest-pipeline",
		Description: "integration test pipeline",
		Source:      minimalSource,
	})
	if err != nil {
		t.Fatalf("CreatePipeline error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created pipeline to have ID")
	}

	got, err := c.GetPipeline(created.ID)
	if err != nil {
		t.Fatalf("GetPipeline error: %v", err)
	}
	if got.Title == "" {
		t.Fatalf("unexpected GetPipeline result: %+v", got)
	}

	got.Description = "integration test pipeline upd"
	// keep a valid source on update as well
	if got.Source == "" {
		got.Source = minimalSource
	}
	upd, err := c.UpdatePipeline(got.ID, got)
	if err != nil {
		t.Fatalf("UpdatePipeline error: %v", err)
	}
	if upd.Description != "integration test pipeline upd" {
		t.Fatalf("pipeline not updated as expected: %+v", upd)
	}

	if err := c.DeletePipeline(created.ID); err != nil {
		t.Fatalf("DeletePipeline error: %v", err)
	}
}
