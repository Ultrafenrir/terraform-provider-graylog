package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// helper to create a minimal client pointed to test server
func newIdxTestClient(base string) *Client {
	return &Client{
		BaseURL:    base,
		HTTP:       &http.Client{Timeout: 2 * time.Second},
		MaxRetries: 0,
		RetryWait:  time.Millisecond,
		logger:     NoopLogger{},
		APIVersion: APIV7, // актуальная ветка; код сам использует snake_case под v7
	}
}

func TestUpdateIndexSet_FallbackPUTToPOST(t *testing.T) {
	// Эмуляция API: первый PUT -> 405, затем POST на тот же путь -> 200 с JSON
	path := "/api/system/indices/index_sets/abc"
	var putCalled, postCalled bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(404)
			return
		}
		switch r.Method {
		case http.MethodPut:
			putCalled = true
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "method not allowed"})
		case http.MethodPost:
			postCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc", "title": "T"})
		default:
			w.WriteHeader(405)
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL)
	_, err := c.UpdateIndexSet("abc", &IndexSet{Title: "T", IndexPrefix: "p"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !putCalled || !postCalled {
		t.Fatalf("expected PUT then POST fallbacks, got put=%v post=%v", putCalled, postCalled)
	}
}

func TestUpdateIndexSet_FallbackToLegacyPath(t *testing.T) {
	// Эмуляция: /api/... возвращает 404 на любые методы, а legacy /system/... принимает PUT
	apiPath := "/api/system/indices/index_sets/idx"
	legacyPath := "/system/indices/index_sets/idx"
	var triedAPIpost, usedLegacyPut bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiPath:
			if r.Method == http.MethodPost {
				triedAPIpost = true
			}
			w.WriteHeader(404)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "not found"})
		case legacyPath:
			if r.Method == http.MethodPut {
				usedLegacyPut = true
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "idx", "title": "X"})
				return
			}
			w.WriteHeader(405)
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL)
	if _, err := c.UpdateIndexSet("idx", &IndexSet{Title: "X", IndexPrefix: "px"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !triedAPIpost || !usedLegacyPut {
		t.Fatalf("expected POST /api... then PUT legacy, got apiPost=%v legacyPut=%v", triedAPIpost, usedLegacyPut)
	}
}

func TestCreateUpdateIndexSet_ConfigTypeInference(t *testing.T) {
	// Сервер ничего не валидирует — просто принимает и возвращает то, что пришло
	var captured map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/system/indices/index_sets" {
			_ = json.NewDecoder(r.Body).Decode(&captured)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "new"})
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/api/system/indices/index_sets/new" {
			_ = json.NewDecoder(r.Body).Decode(&captured)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "new"})
			return
		}
		w.WriteHeader(404)
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL)
	// create без type в config — должен быть подставлен <Class>Config
	_, err := c.CreateIndexSet(&IndexSet{
		Title:                 "T",
		IndexPrefix:           "pref",
		RotationStrategyClass: "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy",
		RotationStrategyConfig: map[string]any{
			"max_docs_per_index": 100,
			// без type
		},
		RetentionStrategyClass: "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy",
		RetentionStrategyConfig: map[string]any{
			"max_number_of_indices": 3,
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// проверить, что type подставился
	rot, ok := captured["rotation_strategy"].(map[string]any)
	if !ok || rot["type"] == "" {
		t.Fatalf("rotation type not inferred: %+v", captured["rotation_strategy"])
	}
	ret, ok := captured["retention_strategy"].(map[string]any)
	if !ok || ret["type"] == "" {
		t.Fatalf("retention type not inferred: %+v", captured["retention_strategy"])
	}

	// update без config — должны быть безопасные дефолты
	_, err = c.UpdateIndexSet("new", &IndexSet{Title: "T2", IndexPrefix: "pref"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if rot, ok := captured["rotation_strategy"].(map[string]any); !ok || rot["type"] == nil {
		t.Fatalf("rotation defaults not provided: %+v", captured["rotation_strategy"])
	}
	if ret, ok := captured["retention_strategy"].(map[string]any); !ok || ret["type"] == nil {
		t.Fatalf("retention defaults not provided: %+v", captured["retention_strategy"])
	}
}
