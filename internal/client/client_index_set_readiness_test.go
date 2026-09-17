package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWaitForIndexSetReady_WaitsForDeflectorTarget(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/system/deflector/idx-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		requests++
		switch requests {
		case 1:
			w.WriteHeader(http.StatusNotFound)
		case 2:
			_, _ = w.Write([]byte(`{"is_up":false,"current_target":""}`))
		default:
			_, _ = w.Write([]byte(`{"is_up":true,"current_target":"logs_0"}`))
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL).WithContext(context.Background())
	if err := c.WaitForIndexSetReady("idx-1", time.Millisecond); err != nil {
		t.Fatalf("WaitForIndexSetReady returned an error: %v", err)
	}
	if requests != 3 {
		t.Fatalf("expected 3 readiness checks, got %d", requests)
	}
}

func TestWaitForIndexSetReady_RequiresCurrentTarget(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"is_up":true,"current_target":""}`))
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	c := newIdxTestClient(ts.URL).WithContext(ctx)
	err := c.WaitForIndexSetReady("idx-1", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout while deflector has no current target")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if !strings.Contains(err.Error(), "timed out waiting for index set idx-1") {
		t.Fatalf("unexpected error: %v", err)
	}
}
