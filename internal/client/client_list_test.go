package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(base string) *Client {
	c := &Client{
		BaseURL:    base,
		HTTP:       &http.Client{Timeout: 2 * time.Second},
		MaxRetries: 0,
		RetryWait:  time.Millisecond,
		logger:     NoopLogger{},
	}
	// Treat test server as v5-like API by default
	c.APIVersion = APIV5
	return c
}

func TestListInputs_WrappedAndArray(t *testing.T) {
	inputs := []Input{{ID: "1", Title: "in1", Type: "raw"}, {ID: "2", Title: "in2", Type: "gelf"}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/inputs":
			if r.Header.Get("X-Requested-By") == "terraform-provider" {
				// first call: wrapped, second call: array
				if r.Header.Get("X-Case") == "array" {
					_ = json.NewEncoder(w).Encode(inputs)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"inputs": inputs})
				}
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"inputs": inputs})
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)

	// Wrapped
	got, err := c.ListInputs()
	if err != nil {
		t.Fatalf("wrapped: %v", err)
	}
	if len(got) != 2 || got[0].Title != "in1" {
		t.Fatalf("unexpected wrapped result: %+v", got)
	}

	// Array (simulate via header switch)
	// Reuse client and change header by temporary override in doRequest is not trivial;
	// instead, call directly with header hint by using another server handler variant.
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/system/inputs" {
			r.Header.Set("X-Case", "array")
			_ = json.NewEncoder(w).Encode(inputs)
			return
		}
		w.WriteHeader(404)
	})

	got2, err := c.ListInputs()
	if err != nil {
		t.Fatalf("array: %v", err)
	}
	if len(got2) != 2 || got2[1].ID != "2" {
		t.Fatalf("unexpected array result: %+v", got2)
	}
}

func TestListUsers_WrappedAndArray(t *testing.T) {
	users := []User{{ID: "u1", Username: "alice"}, {ID: "u2", Username: "bob", Disabled: true}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users":
			// first: wrapped, second: array
			if r.Header.Get("X-Case") == "array" {
				_ = json.NewEncoder(w).Encode(users)
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"users": users})
			}
			return
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)

	// Wrapped
	got, err := c.ListUsers()
	if err != nil {
		t.Fatalf("wrapped: %v", err)
	}
	if len(got) != 2 || got[0].Username != "alice" {
		t.Fatalf("unexpected wrapped result: %+v", got)
	}

	// Array
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/users" {
			r.Header.Set("X-Case", "array")
			_ = json.NewEncoder(w).Encode(users)
			return
		}
		w.WriteHeader(404)
	})

	got2, err := c.ListUsers()
	if err != nil {
		t.Fatalf("array: %v", err)
	}
	if len(got2) != 2 || !got2[1].Disabled {
		t.Fatalf("unexpected array result: %+v", got2)
	}
}
