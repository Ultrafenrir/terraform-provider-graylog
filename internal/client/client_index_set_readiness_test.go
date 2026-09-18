package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEnsureIndexSetReady_WaitsForBackgroundInitializationWithoutCycling(t *testing.T) {
	statusRequests := 0
	cycleRequests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			cycleRequests++
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/system/deflector/idx-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		statusRequests++
		if statusRequests < 3 {
			_, _ = w.Write([]byte(`{"is_up":false,"current_target":""}`))
		} else {
			_, _ = w.Write([]byte(`{"is_up":true,"current_target":"logs_0"}`))
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL).WithContext(context.Background())
	if err := c.EnsureIndexSetReady("idx-1", time.Millisecond); err != nil {
		t.Fatalf("EnsureIndexSetReady returned an error: %v", err)
	}
	if statusRequests != 3 {
		t.Fatalf("expected 3 readiness checks, got %d", statusRequests)
	}
	if cycleRequests != 0 {
		t.Fatalf("expected no manual cycle requests, got %d", cycleRequests)
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

func TestEnsureIndexSetReady_ConcurrentWaitersOnlyObserveGraylog(t *testing.T) {
	var requestCounts sync.Map
	var mutationRequests atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutationRequests.Add(1)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/system/deflector/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/system/deflector/")
		counterValue, _ := requestCounts.LoadOrStore(id, &atomic.Int32{})
		if counterValue.(*atomic.Int32).Add(1) < 3 {
			_, _ = w.Write([]byte(`{"is_up":false,"current_target":""}`))
			return
		}
		_, _ = w.Write([]byte(`{"is_up":true,"current_target":"logs_0"}`))
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL).WithContext(context.Background())
	const count = 8
	errors := make(chan error, count)
	var workers sync.WaitGroup
	workers.Add(count)
	for i := 0; i < count; i++ {
		go func(id string) {
			defer workers.Done()
			errors <- c.EnsureIndexSetReady(id, time.Millisecond)
		}(fmt.Sprintf("idx-%d", i))
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("EnsureIndexSetReady returned an error: %v", err)
		}
	}
	if got := mutationRequests.Load(); got != 0 {
		t.Fatalf("expected readiness waiters to issue no mutating requests, got %d", got)
	}
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("idx-%d", i)
		counterValue, ok := requestCounts.Load(id)
		if !ok || counterValue.(*atomic.Int32).Load() < 3 {
			t.Fatalf("expected index set %s to be polled until ready", id)
		}
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
