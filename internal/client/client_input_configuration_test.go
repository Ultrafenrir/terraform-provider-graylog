package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newInputTestClient(base string) *Client {
	return &Client{
		BaseURL:    base,
		HTTP:       &http.Client{Timeout: 2 * time.Second},
		MaxRetries: 0,
		RetryWait:  time.Millisecond,
		logger:     NoopLogger{},
		APIVersion: APIV7,
	}
}

// TestGetInput_ReadsConfigurationFromAttributesKey guards a bug confirmed against a live
// Graylog 6.0.14 server: GET /system/inputs/{id} nests the configuration map under
// "attributes", not "configuration" (unlike the create/update request body, which uses
// "configuration"). Without the fallback, GetInput silently returned Configuration == nil for
// every input, meaning Read() never actually detected/refreshed configuration drift.
func TestGetInput_ReadsConfigurationFromAttributesKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "input-1",
			"title":  "kafka",
			"type":   "org.graylog2.inputs.raw.kafka.RawKafkaInput",
			"global": true,
			"node":   nil,
			"attributes": map[string]any{
				"legacy_mode":       false,
				"bootstrap_server":  "kafka:9092",
				"topic_filter":      "^logs-.*$",
				"custom_properties": "security.protocol=SASL_SSL",
			},
		})
	}))
	defer ts.Close()

	c := newInputTestClient(ts.URL)
	in, err := c.GetInput("input-1")
	if err != nil {
		t.Fatalf("GetInput: %v", err)
	}
	if len(in.Configuration) != 4 {
		t.Fatalf("expected 4 configuration keys from 'attributes', got %d: %+v", len(in.Configuration), in.Configuration)
	}
	if in.Configuration["bootstrap_server"] != "kafka:9092" {
		t.Errorf("expected bootstrap_server='kafka:9092', got %+v", in.Configuration["bootstrap_server"])
	}
	if in.Configuration["custom_properties"] != "security.protocol=SASL_SSL" {
		t.Errorf("expected custom_properties to round-trip, got %+v", in.Configuration["custom_properties"])
	}
}

// TestListInputs_ReadsConfigurationFromAttributesKey is the same fix applied per-item to the
// list endpoint response.
func TestListInputs_ReadsConfigurationFromAttributesKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"inputs": []map[string]any{
				{
					"id":    "input-1",
					"title": "kafka",
					"type":  "org.graylog2.inputs.raw.kafka.RawKafkaInput",
					"attributes": map[string]any{
						"bootstrap_server": "kafka:9092",
					},
				},
			},
			"total": 1,
		})
	}))
	defer ts.Close()

	c := newInputTestClient(ts.URL)
	list, err := c.ListInputs()
	if err != nil {
		t.Fatalf("ListInputs: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 input, got %d", len(list))
	}
	if list[0].Configuration["bootstrap_server"] != "kafka:9092" {
		t.Errorf("expected bootstrap_server to be populated from 'attributes', got %+v", list[0].Configuration)
	}
}

// TestGetInput_PrefersConfigurationKeyWhenPresent ensures the "attributes" fallback doesn't
// override a "configuration" key when a response actually includes one (covering any Graylog
// version/response-shape variance).
func TestGetInput_PrefersConfigurationKeyWhenPresent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":            "input-1",
			"configuration": map[string]any{"from": "configuration-key"},
			"attributes":    map[string]any{"from": "attributes-key"},
		})
	}))
	defer ts.Close()

	c := newInputTestClient(ts.URL)
	in, err := c.GetInput("input-1")
	if err != nil {
		t.Fatalf("GetInput: %v", err)
	}
	if in.Configuration["from"] != "configuration-key" {
		t.Errorf("expected 'configuration' key to take precedence, got %+v", in.Configuration)
	}
}
