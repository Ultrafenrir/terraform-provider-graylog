package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAttachOutputToStreamUsesGraylogRequestContract(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/api/streams/stream-1/outputs" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Outputs []string `json:"outputs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Outputs) != 1 || body.Outputs[0] != "output-1" {
			t.Fatalf("unexpected outputs payload: %#v", body.Outputs)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL)
	if err := c.AttachOutputToStream("stream-1", "output-1"); err != nil {
		t.Fatalf("AttachOutputToStream returned an error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected exactly one attach request, got %d", requests)
	}
}
