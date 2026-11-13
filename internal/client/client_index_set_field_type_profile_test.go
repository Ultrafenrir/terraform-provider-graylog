package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestIndexSetFieldTypeProfileCRUD(t *testing.T) {
	profile := &IndexSetFieldTypeProfile{ID: "caller-id-must-not-be-reused", Name: "Profile", Description: "", CustomFieldMappings: []CustomFieldMapping{{Field: "client.ip", Type: "ip"}, {Field: "count", Type: "long"}}, IndexSetIDs: []string{"must-not-write"}}
	methods := []string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			if r.URL.Path != fieldTypeProfilesPath {
				t.Errorf("wrong write path: %s", r.URL.Path)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["description"] != "" {
				t.Errorf("clear description omitted: %#v", body)
			}
			if _, ok := body["index_set_ids"]; ok {
				t.Errorf("read-only usages written: %#v", body)
			}
			if !reflect.DeepEqual(body["custom_field_mappings"], []any{map[string]any{"field": "client.ip", "type": "ip"}, map[string]any{"field": "count", "type": "long"}}) {
				t.Errorf("wrong mapping format: %#v", body)
			}
			if r.Method == http.MethodPut {
				if body["id"] != "profile-id" {
					t.Errorf("missing ID in collection PUT: %#v", body)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if _, ok := body["id"]; ok {
				t.Errorf("create should not specify ID: %#v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "profile-id", "name": "Profile", "description": "", "custom_field_mappings": profile.CustomFieldMappings})
		case http.MethodGet:
			if r.URL.Path != fieldTypeProfilesPath+"/profile-id" {
				t.Errorf("wrong GET path: %s", r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "profile-id", "name": "Profile", "description": "", "custom_field_mappings": profile.CustomFieldMappings, "index_set_ids": []string{"set-id"}})
		case http.MethodDelete:
			if r.URL.Path != fieldTypeProfilesPath+"/profile-id" {
				t.Errorf("wrong DELETE path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer ts.Close()
	c := newIdxTestClient(ts.URL)
	c.APIVersion = APIV7
	created, err := c.CreateIndexSetFieldTypeProfile(profile)
	if err != nil || created.ID != "profile-id" {
		t.Fatalf("create: %#v %v", created, err)
	}
	got, err := c.GetIndexSetFieldTypeProfile("profile-id")
	if err != nil || !reflect.DeepEqual(got.IndexSetIDs, []string{"set-id"}) {
		t.Fatalf("GET: %#v %v", got, err)
	}
	if err := c.UpdateIndexSetFieldTypeProfile("profile-id", profile); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteIndexSetFieldTypeProfile("profile-id"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(methods, []string{"POST", "GET", "PUT", "DELETE"}) {
		t.Fatal(methods)
	}
}

func TestIndexSetFieldTypeProfileEmptyMappingsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		response string
		want     string
	}{
		{"empty mappings", 200, `{"id":"profile-id","custom_field_mappings":[]}`, ""},
		{"invalid JSON", 200, `{`, "decode field type profile"},
		{"missing ID", 200, `{}`, "no id"},
		{"invalid mapping", 400, `{"message":"Invalid type provided"}`, "400"},
		{"forbidden", 403, `{"message":"Forbidden"}`, "403"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				mappings, ok := body["custom_field_mappings"].([]any)
				if !ok || len(mappings) != 0 {
					t.Errorf("must send empty array, not null: %#v", body)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer ts.Close()
			c := newIdxTestClient(ts.URL)
			c.APIVersion = APIV7
			_, err := c.CreateIndexSetFieldTypeProfile(&IndexSetFieldTypeProfile{Name: "Profile"})
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %s, got %v", tc.want, err)
			}
		})
	}
}

func TestIndexSetFieldTypeProfileNotFoundAndUnsupported(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(http.StatusNotFound) }))
	defer ts.Close()
	c := newIdxTestClient(ts.URL)
	c.APIVersion = APIV7
	if _, err := c.GetIndexSetFieldTypeProfile("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := c.UpdateIndexSetFieldTypeProfile("missing", &IndexSetFieldTypeProfile{}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := c.DeleteIndexSetFieldTypeProfile("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	before := requests
	c.APIVersion = APIV5
	checks := []func() error{
		func() error { _, err := c.GetIndexSetFieldTypeProfile("id"); return err },
		func() error { _, err := c.CreateIndexSetFieldTypeProfile(&IndexSetFieldTypeProfile{}); return err },
		func() error { return c.UpdateIndexSetFieldTypeProfile("id", &IndexSetFieldTypeProfile{}) },
		func() error { return c.DeleteIndexSetFieldTypeProfile("id") },
	}
	for _, check := range checks {
		if err := check(); err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("unsupported must give explicit error: %v", err)
		}
	}
	if requests != before {
		t.Fatal("unsupported operations sent API requests")
	}
}

func TestIndexSetProfileBindingUpdate(t *testing.T) {
	attached, switched, detached := "profile-one", "profile-two", ""
	rotation := map[string]any{"type": "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategyConfig", "max_docs_per_index": float64(123), "opaque": map[string]any{"keep": true}}
	for _, tc := range []struct {
		name  string
		input *string
		want  any
	}{
		{"unmanaged preserve", nil, attached}, {"switch", &switched, switched}, {"detach", &detached, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			cycles := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "cycle") {
					cycles++
				}
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(&IndexSet{ID: "set", FieldTypeProfile: &attached, RotationStrategyConfig: rotation})
					return
				}
				if r.Method != http.MethodPut || r.URL.Path != "/api/system/indices/index_sets/set" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer ts.Close()
			c := newIdxTestClient(ts.URL)
			c.APIVersion = APIV7
			_, err := c.UpdateIndexSet("set", &IndexSet{Title: "metadata change", FieldTypeProfile: tc.input})
			if err != nil {
				t.Fatal(err)
			}
			if body["field_type_profile"] != tc.want {
				t.Fatalf("wrong binding: %#v", body)
			}
			if _, ok := body["field_type_profile"]; !ok {
				t.Fatal("detach must serialize null")
			}
			if !reflect.DeepEqual(body["rotation_strategy"], rotation) {
				t.Fatalf("unmanaged rotation changed: %#v", body)
			}
			if cycles != 0 {
				t.Fatal("profile update must not rotate indices")
			}
		})
	}
}

func TestIndexSetProfileBindingCreate(t *testing.T) {
	for _, value := range []string{"profile-id", ""} {
		t.Run(value, func(t *testing.T) {
			var body map[string]any
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&body)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "set"})
			}))
			defer ts.Close()
			c := newIdxTestClient(ts.URL)
			c.APIVersion = APIV7
			if _, err := c.CreateIndexSet(&IndexSet{Title: "Set", FieldTypeProfile: &value}); err != nil {
				t.Fatal(err)
			}
			if value != "" && body["field_type_profile"] != value {
				t.Fatalf("missing create binding: %#v", body)
			}
			if value == "" {
				if _, ok := body["field_type_profile"]; ok {
					t.Fatalf("empty create binding must be absent: %#v", body)
				}
			}
		})
	}
}

func TestIndexSetProfileDetachPreservesLargeNumbers(t *testing.T) {
	const largeNumber = "9007199254740993"
	var put []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(&IndexSet{ID: "set"})
			return
		}
		var raw json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&raw)
		put = raw
		_, _ = w.Write(raw)
	}))
	defer ts.Close()
	c := newIdxTestClient(ts.URL)
	c.APIVersion = APIV7
	empty := ""
	if _, err := c.UpdateIndexSet("set", &IndexSet{Shards: 9007199254740993, FieldTypeProfile: &empty}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(put), largeNumber) {
		t.Fatalf("detach changed a large JSON integer: %s", put)
	}
}

func TestIndexSetProfileBindingLegacyCompatibility(t *testing.T) {
	writes := 0
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			writes++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, present := body["field_type_profile"]; present {
				t.Errorf("unmanaged legacy request includes profile binding: %#v", body)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "legacy", "title": "Legacy", "index_prefix": "legacy"})
	}))
	defer ts.Close()
	c := newIdxTestClient(ts.URL)
	c.APIVersion = APIV5
	input := &IndexSet{Title: "Legacy", IndexPrefix: "legacy"}
	if _, err := c.CreateIndexSet(input); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateIndexSet("legacy", input); err != nil {
		t.Fatal(err)
	}
	if writes != 2 {
		t.Fatalf("expected create and update writes, got %d", writes)
	}
	before := requests
	empty := ""
	input.FieldTypeProfile = &empty
	if _, err := c.CreateIndexSet(input); err == nil {
		t.Fatal("configured profiles must fail on Graylog 5")
	}
	if _, err := c.UpdateIndexSet("legacy", input); err == nil {
		t.Fatal("configured profiles must fail on Graylog 5")
	}
	if requests != before {
		t.Fatal("unsupported binding requests reached the API")
	}
}
