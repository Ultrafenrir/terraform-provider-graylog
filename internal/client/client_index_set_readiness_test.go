package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEnsureIndexSetReady_InitializesAndWaitsForDeflectorTarget(t *testing.T) {
	statusRequests := 0
	cycleRequests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/cluster/deflector/idx-1/cycle" {
			cycleRequests++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/api/system/deflector/idx-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		statusRequests++
		switch statusRequests {
		case 1:
			_, _ = w.Write([]byte(`{"is_up":false,"current_target":""}`))
		default:
			_, _ = w.Write([]byte(`{"is_up":true,"current_target":"logs_0"}`))
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL).WithContext(context.Background())
	if err := c.EnsureIndexSetReady("idx-1", time.Millisecond); err != nil {
		t.Fatalf("EnsureIndexSetReady returned an error: %v", err)
	}
	if statusRequests != 2 {
		t.Fatalf("expected 2 readiness checks, got %d", statusRequests)
	}
	if cycleRequests != 1 {
		t.Fatalf("expected one initialization request, got %d", cycleRequests)
	}
}

func TestEnsureIndexSetReady_DoesNotCycleReadyIndexSet(t *testing.T) {
	cycleRequests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			cycleRequests++
		}
		_, _ = w.Write([]byte(`{"is_up":true,"current_target":"logs_0"}`))
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL).WithContext(context.Background())
	if err := c.EnsureIndexSetReady("idx-1", time.Millisecond); err != nil {
		t.Fatalf("EnsureIndexSetReady returned an error: %v", err)
	}
	if cycleRequests != 0 {
		t.Fatalf("expected no initialization request for a ready index set, got %d", cycleRequests)
	}
}

func TestEnsureIndexSetReady_PollsAfterCycleResponseTimeoutWithoutRetrying(t *testing.T) {
	var ready atomic.Bool
	var cycleRequests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			if ready.Load() {
				_, _ = w.Write([]byte(`{"is_up":true,"current_target":"logs_0"}`))
			} else {
				_, _ = w.Write([]byte(`{"is_up":false,"current_target":""}`))
			}
		case r.Method == http.MethodPost:
			cycleRequests.Add(1)
			<-r.Context().Done()
			ready.Store(true)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL).WithContext(context.Background())
	c.MaxRetries = 3
	c.HTTP.Timeout = 10 * time.Millisecond
	if err := c.EnsureIndexSetReady("idx-1", time.Millisecond); err != nil {
		t.Fatalf("EnsureIndexSetReady returned an error after cycle response timeout: %v", err)
	}
	if got := cycleRequests.Load(); got != 1 {
		t.Fatalf("expected exactly one non-idempotent initialization request, got %d", got)
	}
}

func TestEnsureIndexSetReady_PollsAfterClusterProxyTimeoutWithoutRetrying(t *testing.T) {
	statusRequests := 0
	cycleRequests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			statusRequests++
			if statusRequests > 1 {
				_, _ = w.Write([]byte(`{"is_up":true,"current_target":"logs_0"}`))
			} else {
				_, _ = w.Write([]byte(`{"is_up":false,"current_target":""}`))
			}
		case r.Method == http.MethodPost:
			cycleRequests++
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"type":"ApiError","message":"timeout"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL).WithContext(context.Background())
	c.MaxRetries = 3
	if err := c.EnsureIndexSetReady("idx-1", time.Millisecond); err != nil {
		t.Fatalf("EnsureIndexSetReady returned an error after proxy timeout: %v", err)
	}
	if cycleRequests != 1 {
		t.Fatalf("expected exactly one non-idempotent initialization request, got %d", cycleRequests)
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
