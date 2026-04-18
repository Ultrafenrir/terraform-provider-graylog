package client

import (
	"encoding/json"
	"errors"
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

func TestUpdateIndexSet_SuccessfulPUT(t *testing.T) {
	// Проверяем что UpdateIndexSet работает корректно с PUT запросом
	path := "/api/system/indices/index_sets/abc"
	var getCalled, putCalled bool
	var receivedBody map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(404)
			return
		}

		if r.Method == http.MethodGet {
			getCalled = true
			// Возвращаем текущее состояние объекта
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                                  "abc",
				"title":                               "Old Title",
				"index_prefix":                        "p",
				"shards":                              1,
				"replicas":                            0,
				"index_analyzer":                      "standard",
				"field_type_refresh_interval":         5000,
				"index_optimization_max_num_segments": 1,
				"index_optimization_disabled":         false,
				"rotation_strategy_class":             "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy",
				"rotation_strategy": map[string]any{
					"type":               "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategyConfig",
					"max_docs_per_index": 20000000,
				},
				"retention_strategy_class": "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy",
				"retention_strategy": map[string]any{
					"type":                  "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategyConfig",
					"max_number_of_indices": 20,
				},
			})
			return
		}

		if r.Method == http.MethodPut {
			putCalled = true
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)

			// Проверяем что все необходимые поля присутствуют
			if _, hasPrefix := receivedBody["index_prefix"]; !hasPrefix {
				t.Error("body should contain 'index_prefix' field")
			}
			if _, hasShards := receivedBody["shards"]; !hasShards {
				t.Error("body should contain 'shards' field")
			}

			_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc", "title": receivedBody["title"]})
			return
		}

		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "method not allowed"})
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL)
	_, err := c.UpdateIndexSet("abc", &IndexSet{Title: "T", IndexPrefix: "p"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !getCalled {
		t.Fatal("expected GET to be called first")
	}
	if !putCalled {
		t.Fatal("expected PUT to be called")
	}
}

func TestUpdateIndexSet_ErrorOn405(t *testing.T) {
	// Проверяем что 405 ошибка возвращается правильно
	path := "/api/system/indices/index_sets/idx"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(404)
			return
		}

		if r.Method == http.MethodGet {
			// Возвращаем текущее состояние объекта
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                                  "idx",
				"title":                               "Old",
				"index_prefix":                        "px",
				"shards":                              1,
				"replicas":                            0,
				"index_analyzer":                      "standard",
				"field_type_refresh_interval":         5000,
				"index_optimization_max_num_segments": 1,
				"index_optimization_disabled":         false,
			})
			return
		}

		// Для PUT всегда возвращаем 405
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "method not allowed"})
	}))
	defer ts.Close()

	c := newIdxTestClient(ts.URL)
	_, err := c.UpdateIndexSet("idx", &IndexSet{Title: "X", IndexPrefix: "px"})
	if err == nil {
		t.Fatal("expected error on 405 response")
	}

	// Проверяем что это действительно GraylogError с кодом 405
	var ge *GraylogError
	if !errors.As(err, &ge) {
		t.Fatalf("expected GraylogError, got %T", err)
	}
	if ge.Status != 405 {
		t.Errorf("expected status 405, got %d", ge.Status)
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
		if r.Method == http.MethodGet && r.URL.Path == "/api/system/indices/index_sets/new" {
			// Возвращаем текущее состояние объекта для Update
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                                  "new",
				"title":                               "T",
				"index_prefix":                        "pref",
				"shards":                              1,
				"replicas":                            0,
				"index_analyzer":                      "standard",
				"field_type_refresh_interval":         5000,
				"index_optimization_max_num_segments": 1,
				"index_optimization_disabled":         false,
			})
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
