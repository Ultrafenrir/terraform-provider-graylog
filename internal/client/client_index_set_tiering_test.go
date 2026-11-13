package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCreateIndexSet_DataTiering(t *testing.T) {
	for _, version := range []APIVersion{APIV6, APIV7} {
		t.Run(string(rune('5'+version)), func(t *testing.T) {
			var body map[string]any
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/system/indices/index_sets" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "tiered"})
			}))
			defer ts.Close()
			c := newIdxTestClient(ts.URL)
			c.APIVersion = version
			legacy := false
			_, err := c.CreateIndexSet(&IndexSet{
				Title: "tiered", IndexPrefix: "tiered",
				DataTiering:       json.RawMessage(`{"type":"hot_only","index_lifetime_min":"P30D","index_lifetime_max":"P40D"}`),
				UseLegacyRotation: &legacy,
			})
			if err != nil {
				t.Fatal(err)
			}
			if body["use_legacy_rotation"] != false {
				t.Fatalf("false must be serialized explicitly: %#v", body)
			}
			config, ok := body["data_tiering"].(map[string]any)
			if !ok || config["type"] != "hot_only" || config["index_lifetime_max"] != "P40D" {
				t.Fatalf("expected a nested tiering object: %#v", body)
			}
		})
	}
}

func TestUpdateIndexSet_DataTiering(t *testing.T) {
	// Include an unknown nested field to prove a metadata-only update preserves
	// the complete server document, even for future/Enterprise tier types.
	stored := json.RawMessage(`{"type":"future_tier","nested":{"enabled":true,"values":[1,2]}}`)
	replacement := json.RawMessage(`{"type":"hot_only","index_lifetime_min":"P60D","index_lifetime_max":"P70D"}`)
	falseValue, trueValue := false, true
	for _, tc := range []struct {
		name   string
		input  IndexSet
		want   json.RawMessage
		legacy bool
	}{
		{"preserve unmanaged tiering", IndexSet{}, stored, false},
		{"replace managed tiering", IndexSet{DataTiering: replacement, UseLegacyRotation: &falseValue}, replacement, false},
		{"switch to legacy", IndexSet{UseLegacyRotation: &trueValue}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					// Real GET responses may omit use_legacy_rotation.
					_ = json.NewEncoder(w).Encode(map[string]any{"id": "tiered", "title": "before", "index_prefix": "tiered", "data_tiering": stored})
				case http.MethodPut:
					_ = json.NewDecoder(r.Body).Decode(&body)
					_ = json.NewEncoder(w).Encode(body)
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer ts.Close()
			c := newIdxTestClient(ts.URL)
			tc.input.Title = "after"
			tc.input.IndexPrefix = "tiered"
			if _, err := c.UpdateIndexSet("tiered", &tc.input); err != nil {
				t.Fatal(err)
			}
			var want any
			if tc.want != nil {
				if err := json.Unmarshal(tc.want, &want); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(body["data_tiering"], want) || body["use_legacy_rotation"] != tc.legacy || body["title"] != "after" {
				t.Fatalf("unexpected PUT: %#v", body)
			}
		})
	}
}

func TestIndexSet_LegacyCompatibility(t *testing.T) {
	var bodies []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			bodies = append(bodies, body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "legacy", "title": "legacy", "index_prefix": "legacy"})
	}))
	defer ts.Close()
	c := newIdxTestClient(ts.URL)
	c.APIVersion = APIV5
	input := &IndexSet{Title: "legacy", IndexPrefix: "legacy"}
	if _, err := c.CreateIndexSet(input); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateIndexSet("legacy", input); err != nil {
		t.Fatal(err)
	}
	for _, body := range bodies {
		for _, key := range []string{"data_tiering", "use_legacy_rotation"} {
			if _, present := body[key]; present {
				t.Fatalf("legacy request includes %s: %#v", key, body)
			}
		}
	}
	legacy := false
	input.UseLegacyRotation = &legacy
	if _, err := c.CreateIndexSet(input); err == nil {
		t.Fatal("expected unsupported version error")
	}
	if _, err := c.UpdateIndexSet("legacy", input); err == nil {
		t.Fatal("expected unsupported version error")
	}
	if len(bodies) != 2 {
		t.Fatal("unsupported requests must fail before writing")
	}
}
