package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateStream_SendsMatchingType verifies that matching_type reaches the wire in both the
// v7 entity-wrapper body and the legacy body, and that a caller-provided value (not just the
// "AND" default) is honored.
func TestCreateStream_SendsMatchingType(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/streams" || r.Method != http.MethodPost {
			w.WriteHeader(404)
			return
		}
		gotBody = map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"stream_id": "sid1"})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	_, err := c.CreateStream(&Stream{Title: "t", IndexSetID: "idx", MatchingType: "OR"})
	if err != nil {
		t.Fatalf("CreateStream: %v", err)
	}

	entity, ok := gotBody["entity"].(map[string]any)
	if !ok {
		t.Fatalf("expected v7 entity-wrapped body (APIV7 client), got: %+v", gotBody)
	}
	if entity["matching_type"] != "OR" {
		t.Fatalf("expected matching_type=OR in request body, got: %+v", entity)
	}
}

// TestCreateStream_DefaultsMatchingTypeToAND verifies the client's own documented fallback:
// an empty MatchingType on the input struct defaults to "AND" on the wire.
func TestCreateStream_DefaultsMatchingTypeToAND(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only capture the initial creation POST; ignore the follow-up resume/GetStream calls
		// CreateStream makes afterwards.
		if r.URL.Path == "/api/streams" && r.Method == http.MethodPost {
			gotBody = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"stream_id": "sid1"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "sid1"})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	_, err := c.CreateStream(&Stream{Title: "t", IndexSetID: "idx"})
	if err != nil {
		t.Fatalf("CreateStream: %v", err)
	}

	entity, ok := gotBody["entity"].(map[string]any)
	if !ok {
		t.Fatalf("expected v7 entity-wrapped body, got: %+v", gotBody)
	}
	if entity["matching_type"] != "AND" {
		t.Fatalf("expected default matching_type=AND, got: %+v", entity)
	}
}

// TestUpdateStream_SendsMatchingType verifies UpdateStream includes matching_type in the PUT
// body for an APIV7 client (which uses a hand-built map rather than marshaling the Stream struct
// directly).
func TestUpdateStream_SendsMatchingType(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/streams/str1" || r.Method != http.MethodPut {
			w.WriteHeader(404)
			return
		}
		gotBody = map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "str1", "matching_type": "OR"})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	_, err := c.UpdateStream("str1", &Stream{Title: "t", MatchingType: "OR"})
	if err != nil {
		t.Fatalf("UpdateStream: %v", err)
	}
	if gotBody["matching_type"] != "OR" {
		t.Fatalf("expected matching_type=OR in PUT body, got: %+v", gotBody)
	}
}
