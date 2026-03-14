package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClientForOS(base string) *Client {
	c := NewWithOptions("http://graylog.local", Options{OpenSearchURL: base})
	// Make retries deterministic/fast in tests
	c.MaxRetries = 0
	return c
}

func TestOSGetSnapshotRepository_Object(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/_snapshot/test" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"test": {"type":"fs", "settings": {"location":"/snapshots","compress":true}}}`))
	}))
	defer ts.Close()

	c := newTestClientForOS(ts.URL)
	typ, settings, err := c.OSGetSnapshotRepository("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != "fs" {
		t.Fatalf("expected type fs, got %s", typ)
	}
	if settings["location"].(string) != "/snapshots" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestOSGetSnapshotRepository_ArrayFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"type":"fs","settings":{"location":"/snapshots"}}]`))
	}))
	defer ts.Close()

	c := newTestClientForOS(ts.URL)
	typ, settings, err := c.OSGetSnapshotRepository("ignored")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != "fs" || settings["location"].(string) != "/snapshots" {
		t.Fatalf("unexpected result: typ=%s settings=%+v", typ, settings)
	}
}

func TestOSGetSnapshotRepository_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"missing"}`))
	}))
	defer ts.Close()

	c := newTestClientForOS(ts.URL)
	_, _, err := c.OSGetSnapshotRepository("missing")
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
