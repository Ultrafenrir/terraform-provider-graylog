package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListStreamRules_WrappedResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/streams/str1/rules" || r.Method != http.MethodGet {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"stream_rules": []map[string]any{
				{"id": "r1", "field": "source", "type": 1, "value": "acc"},
			},
		})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	rules, err := c.ListStreamRules("str1")
	if err != nil {
		t.Fatalf("ListStreamRules: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != "r1" || rules[0].Field != "source" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}

func TestListStreamRules_DirectArrayResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "r2", "field": "message", "type": 3, "value": ".*err.*"},
		})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	rules, err := c.ListStreamRules("str1")
	if err != nil {
		t.Fatalf("ListStreamRules: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != "r2" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}

func TestCreateStreamRule_PathAndBody(t *testing.T) {
	var gotBody StreamRule
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/streams/str1/rules" || r.Method != http.MethodPost {
			w.WriteHeader(404)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "new-rule-id"})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	rule := &StreamRule{Field: "source", Type: 1, Value: "acc", Inverted: true, Description: "d"}
	out, err := c.CreateStreamRule("str1", rule)
	if err != nil {
		t.Fatalf("CreateStreamRule: %v", err)
	}
	if out.ID != "new-rule-id" {
		t.Fatalf("expected id 'new-rule-id', got %+v", out)
	}
	if gotBody.Field != "source" || gotBody.Type != 1 || gotBody.Value != "acc" || !gotBody.Inverted {
		t.Fatalf("unexpected request body sent: %+v", gotBody)
	}
}

func TestCreateStreamRule_FallsBackToListWhenNoIDInResponse(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			callCount++
			// No id anywhere in the response.
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		// GET (list) — used as fallback lookup.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"stream_rules": []map[string]any{
				{"id": "found-id", "field": "source", "type": 1, "value": "acc", "inverted": false},
			},
		})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	rule := &StreamRule{Field: "source", Type: 1, Value: "acc"}
	out, err := c.CreateStreamRule("str1", rule)
	if err != nil {
		t.Fatalf("CreateStreamRule: %v", err)
	}
	if out.ID != "found-id" {
		t.Fatalf("expected fallback lookup to find id 'found-id', got %+v", out)
	}
}

func TestDeleteStreamRule_Path(t *testing.T) {
	var gotPath, gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	if err := c.DeleteStreamRule("str1", "rule1"); err != nil {
		t.Fatalf("DeleteStreamRule: %v", err)
	}
	if gotPath != "/api/streams/str1/rules/rule1" {
		t.Fatalf("expected path '/api/streams/str1/rules/rule1', got %q", gotPath)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %q", gotMethod)
	}
}
