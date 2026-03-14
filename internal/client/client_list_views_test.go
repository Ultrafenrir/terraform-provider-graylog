package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListViews_WrappedArrayAndMap(t *testing.T) {
	views := []View{{ID: "v1", Title: "Main", Description: "desc"}, {ID: "v2", Title: "Ops"}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/views":
			// default: wrapped
			if r.Header.Get("X-Case") == "array" {
				_ = json.NewEncoder(w).Encode(views)
				return
			}
			if r.Header.Get("X-Case") == "map" {
				m := map[string]any{"views": map[string]any{
					"v1": views[0],
					"v2": views[1],
				}}
				_ = json.NewEncoder(w).Encode(m)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"views": views})
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL, HTTP: &http.Client{Timeout: 2 * time.Second}, MaxRetries: 0, RetryWait: time.Millisecond, logger: NoopLogger{}}
	c.APIVersion = APIV6

	// Wrapped
	got, err := c.ListViews()
	if err != nil {
		t.Fatalf("wrapped: %v", err)
	}
	if len(got) != 2 || got[0].ID != "v1" || got[0].Title != "Main" {
		t.Fatalf("unexpected wrapped result: %+v", got)
	}

	// Array
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/views" {
			r.Header.Set("X-Case", "array")
			_ = json.NewEncoder(w).Encode(views)
			return
		}
		w.WriteHeader(404)
	})
	got2, err := c.ListViews()
	if err != nil {
		t.Fatalf("array: %v", err)
	}
	if len(got2) != 2 || got2[1].ID != "v2" {
		t.Fatalf("unexpected array result: %+v", got2)
	}

	// Map under views
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/views" {
			r.Header.Set("X-Case", "map")
			m := map[string]any{"views": map[string]any{
				"v1": views[0],
				"v2": views[1],
			}}
			_ = json.NewEncoder(w).Encode(m)
			return
		}
		w.WriteHeader(404)
	})
	got3, err := c.ListViews()
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(got3) != 2 || got3[0].ID == "" || got3[1].Title == "" {
		t.Fatalf("unexpected map result: %+v", got3)
	}
}
