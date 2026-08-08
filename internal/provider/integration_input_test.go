//go:build integration

package provider

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func integrationInputCredentials(t *testing.T) (string, string) {
	t.Helper()

	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	return strings.TrimRight(baseURL, "/"), token
}

func integrationInputClient(t *testing.T) *client.Client {
	t.Helper()
	baseURL, token := integrationInputCredentials(t)
	return client.New(baseURL, token)
}

func createIntegrationLookupTable(t *testing.T) string {
	t.Helper()
	baseURL, token := integrationInputCredentials(t)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	create := func(path string, payload map[string]any) string {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal lookup fixture %s: %v", path, err)
		}
		req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("build lookup fixture request %s: %v", path, err)
		}
		req.Header.Set("Authorization", "Basic "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-By", "terraform-provider-graylog-integration-test")
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("create lookup fixture %s: %v", path, err)
		}
		defer resp.Body.Close()
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read lookup fixture response %s: %v", path, err)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			t.Fatalf("create lookup fixture %s: HTTP %d: %s", path, resp.StatusCode, responseBody)
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(responseBody, &created); err != nil {
			t.Fatalf("decode lookup fixture response %s: %v", path, err)
		}
		if created.ID == "" {
			t.Fatalf("lookup fixture %s returned an empty ID: %s", path, responseBody)
		}
		return created.ID
	}

	cleanup := func(path, id string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodDelete, baseURL+path+"/"+id, nil)
		if err != nil {
			t.Errorf("build lookup fixture cleanup request %s: %v", path, err)
			return
		}
		req.Header.Set("Authorization", "Basic "+token)
		req.Header.Set("X-Requested-By", "terraform-provider-graylog-integration-test")
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Errorf("delete lookup fixture %s/%s: %v", path, id, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			responseBody, _ := io.ReadAll(resp.Body)
			t.Errorf("delete lookup fixture %s/%s: HTTP %d: %s", path, id, resp.StatusCode, responseBody)
		}
	}

	cacheID := create("/system/lookup/caches", map[string]any{
		"title":       "Terraform integration cache " + suffix,
		"name":        "tf-itest-cache-" + suffix,
		"description": "Extractor integration test fixture",
		"config":      map[string]any{"type": "none"},
	})
	t.Cleanup(func() { cleanup("/system/lookup/caches", cacheID) })

	adapterID := create("/system/lookup/adapters", map[string]any{
		"title":       "Terraform integration adapter " + suffix,
		"name":        "tf-itest-adapter-" + suffix,
		"description": "Extractor integration test fixture",
		"config": map[string]any{
			"type":                       "dnslookup",
			"lookup_type":                "A",
			"server_ips":                 "",
			"request_timeout":            1000,
			"cache_ttl_override_enabled": false,
		},
	})
	t.Cleanup(func() { cleanup("/system/lookup/adapters", adapterID) })

	tableName := "tf-itest-table-" + suffix
	tableID := create("/system/lookup/tables", map[string]any{
		"title":                     "Terraform integration table " + suffix,
		"name":                      tableName,
		"description":               "Extractor integration test fixture",
		"cache_id":                  cacheID,
		"data_adapter_id":           adapterID,
		"default_single_value":      "",
		"default_single_value_type": "NULL",
		"default_multi_value":       "",
		"default_multi_value_type":  "NULL",
	})
	t.Cleanup(func() { cleanup("/system/lookup/tables", tableID) })

	return tableName
}

// TestIntegration_InputCRUD validates Input CRUD against a real Graylog
func TestIntegration_InputCRUD(t *testing.T) {
	c := integrationInputClient(t)

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
	t.Cleanup(func() {
		if err := c.DeleteInput(created.ID); err != nil {
			t.Errorf("cleanup input %q: %v", created.ID, err)
		}
	})

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

}

// TestIntegration_InputWithAllExtractorTypes verifies the complete extractor create payload
// against a real Graylog. In particular, it covers empty target_field/extractor_config values,
// which must be serialized as "" and {} rather than omitted/null.
func TestIntegration_InputWithAllExtractorTypes(t *testing.T) {
	c := integrationInputClient(t)
	lookupTableName := createIntegrationLookupTable(t)

	created, err := c.CreateInput(&client.Input{
		Title:  "tf-itest-input-extractors",
		Type:   "org.graylog2.inputs.gelf.udp.GELFUDPInput",
		Global: true,
		Configuration: map[string]any{
			"bind_address": "0.0.0.0",
			"port":         12202,
		},
	})
	if err != nil {
		t.Fatalf("CreateInput error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created input to have ID")
	}
	t.Cleanup(func() {
		if err := c.DeleteInput(created.ID); err != nil {
			t.Errorf("cleanup input %q: %v", created.ID, err)
		}
	})

	extractors := []client.Extractor{
		{
			Title:           "integration regex",
			ExtractorType:   "regex",
			SourceField:     "message",
			TargetField:     "itest_regex",
			CursorStrategy:  "copy",
			ExtractorConfig: map[string]any{"regex_value": `user=(\w+)`},
			ConditionType:   "none",
			ConditionValue:  "",
			Order:           0,
		},
		{
			Title:          "integration copy input with date converter",
			ExtractorType:  "copy_input",
			SourceField:    "time",
			TargetField:    "timestamp",
			CursorStrategy: "cut",
			// Intentionally nil: CreateInputExtractor must send {}.
			ExtractorConfig: nil,
			Converters: []client.ExtractorConverter{
				{
					Type: "date",
					Config: map[string]any{
						"date_format": "yyyy-MM-dd'T'HH:mm:ss.SSSSSSSSS'Z'",
					},
				},
			},
			ConditionType:  "none",
			ConditionValue: "",
			Order:          1,
		},
		{
			Title:          "integration json",
			ExtractorType:  "json",
			SourceField:    "log",
			TargetField:    "",
			CursorStrategy: "copy",
			ConditionType:  "regex",
			ConditionValue: `^\{\"`,
			Order:          2,
			ExtractorConfig: map[string]any{
				"list_separator":             ", ",
				"kv_separator":               "=",
				"key_prefix":                 "json_",
				"key_separator":              "_",
				"replace_key_whitespace":     false,
				"key_whitespace_replacement": "_",
			},
		},
		{
			Title:           "integration grok",
			ExtractorType:   "grok",
			SourceField:     "message",
			TargetField:     "itest_grok",
			CursorStrategy:  "copy",
			ExtractorConfig: map[string]any{"grok_pattern": `%{WORD:itest_word}`},
			ConditionType:   "none",
			ConditionValue:  "",
			Order:           3,
		},
		{
			Title:           "integration substring",
			ExtractorType:   "substring",
			SourceField:     "message",
			TargetField:     "itest_substring",
			CursorStrategy:  "copy",
			ExtractorConfig: map[string]any{"begin_index": 0, "end_index": 4},
			ConditionType:   "none",
			ConditionValue:  "",
			Order:           4,
		},
		{
			Title:           "integration split and index",
			ExtractorType:   "split_and_index",
			SourceField:     "message",
			TargetField:     "itest_split",
			CursorStrategy:  "copy",
			ExtractorConfig: map[string]any{"split_by": " ", "index": 0},
			ConditionType:   "string",
			ConditionValue:  " ",
			Order:           5,
		},
		{
			Title:           "integration regex replace",
			ExtractorType:   "regex_replace",
			SourceField:     "message",
			TargetField:     "itest_replaced",
			CursorStrategy:  "copy",
			ExtractorConfig: map[string]any{"regex": `\s+`, "replacement": "_", "replace_all": true},
			ConditionType:   "regex",
			ConditionValue:  `\s+`,
			Order:           6,
		},
		{
			Title:           "integration lookup table",
			ExtractorType:   "lookup_table",
			SourceField:     "source",
			TargetField:     "itest_lookup",
			CursorStrategy:  "copy",
			ExtractorConfig: map[string]any{"lookup_table_name": lookupTableName},
			ConditionType:   "none",
			ConditionValue:  "",
			Order:           7,
		},
	}

	wantIDs := make(map[string]string, len(extractors))
	for i := range extractors {
		extractor := &extractors[i]
		t.Run(extractor.ExtractorType, func(t *testing.T) {
			createdExtractor, err := c.CreateInputExtractor(created.ID, extractor)
			if err != nil {
				t.Fatalf("CreateInputExtractor(%s) error: %v", extractor.ExtractorType, err)
			}
			if createdExtractor.ID == "" {
				t.Fatalf("expected %s extractor to have ID", extractor.ExtractorType)
			}
			wantIDs[extractor.ExtractorType] = createdExtractor.ID
		})
	}

	got, err := c.ListInputExtractors(created.ID)
	if err != nil {
		t.Fatalf("ListInputExtractors error: %v", err)
	}
	gotByType := make(map[string]client.Extractor, len(got))
	for _, extractor := range got {
		gotByType[extractor.ExtractorType] = extractor
	}
	for extractorType, wantID := range wantIDs {
		extractor, ok := gotByType[extractorType]
		if !ok {
			t.Errorf("created extractor type %q is missing from list response", extractorType)
			continue
		}
		if extractor.ID != wantID {
			t.Errorf("extractor %q ID mismatch: create=%q list=%q", extractorType, wantID, extractor.ID)
		}
		if extractor.ExtractorConfig == nil {
			t.Errorf("extractor %q returned null extractor_config", extractorType)
		}
	}
	if len(gotByType) != len(extractors) {
		t.Errorf("expected %d extractor types, got %d: %s", len(extractors), len(gotByType), fmt.Sprint(gotByType))
	}
}
