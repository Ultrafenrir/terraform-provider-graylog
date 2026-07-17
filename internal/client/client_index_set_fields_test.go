package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUpdateIndexSet_ClearsDescription verifies that passing an empty Description actually
// clears it server-side, instead of being silently ignored because of a `!= ""` guard (the old
// bug: description could never be cleared once set, causing a permanent diff).
func TestUpdateIndexSet_ClearsDescription(t *testing.T) {
	var putBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "abc",
				"title":          "T",
				"description":    "old description",
				"index_prefix":   "p",
				"shards":         1,
				"replicas":       0,
				"index_analyzer": "standard",
			})
		case http.MethodPut:
			putBody = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc"})
		default:
			w.WriteHeader(405)
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL)
	// Caller explicitly wants the description cleared.
	_, err := c.UpdateIndexSet("abc", &IndexSet{Title: "T", IndexPrefix: "p", Description: ""})
	if err != nil {
		t.Fatalf("UpdateIndexSet: %v", err)
	}
	if desc, ok := putBody["description"]; !ok || desc != "" {
		t.Fatalf("expected description to be cleared to \"\" in PUT body, got: %+v", putBody["description"])
	}
}

// TestUpdateIndexSet_PropagatesIndexPrefix verifies UpdateIndexSet forwards the caller's
// index_prefix into the PUT body instead of always resending whatever GET returned (the old bug:
// index_prefix changes were silently dropped, producing a permanent diff on every plan).
func TestUpdateIndexSet_PropagatesIndexPrefix(t *testing.T) {
	var putBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "abc",
				"title":          "T",
				"index_prefix":   "old-prefix",
				"shards":         1,
				"replicas":       0,
				"index_analyzer": "standard",
			})
		case http.MethodPut:
			putBody = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc"})
		default:
			w.WriteHeader(405)
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL)
	_, err := c.UpdateIndexSet("abc", &IndexSet{Title: "T", IndexPrefix: "new-prefix"})
	if err != nil {
		t.Fatalf("UpdateIndexSet: %v", err)
	}
	if putBody["index_prefix"] != "new-prefix" {
		t.Fatalf("expected index_prefix='new-prefix' in PUT body, got: %+v", putBody["index_prefix"])
	}
}
